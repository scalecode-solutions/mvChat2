package main

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/scalecode-solutions/mvchat2/auth"
	"github.com/scalecode-solutions/mvchat2/config"
	"github.com/scalecode-solutions/mvchat2/crypto"
	"github.com/scalecode-solutions/mvchat2/email"
	"github.com/scalecode-solutions/mvchat2/media"
	"github.com/scalecode-solutions/mvchat2/middleware"
	"github.com/scalecode-solutions/mvchat2/redis"
	"github.com/scalecode-solutions/mvchat2/store"
)

const (
	currentVersion = "0.1.0"
)

var buildstamp = "dev"

func main() {
	configFile := flag.String("config", "mvchat2.yaml", "Path to config file")
	initDB := flag.Bool("init-db", false, "Initialize database schema")
	generateKeys := flag.Bool("generate-keys", false, "Generate secure cryptographic keys and exit")
	flag.Parse()

	// Handle key generation
	if *generateKeys {
		printGeneratedKeys()
		return
	}

	fmt.Printf("mvChat2 v%s (build: %s)\n", currentVersion, buildstamp)

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize database
	db, err := store.New(&cfg.Database)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	fmt.Println("Connected to database")

	// Initialize schema if requested
	if *initDB {
		fmt.Println("Initializing database schema...")
		if err := db.InitSchema(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize schema: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Schema initialized")
	}

	// Check schema version
	version, err := db.GetSchemaVersion()
	if err != nil {
		fmt.Println("Warning: Could not get schema version (run with -init-db to initialize)")
	} else {
		fmt.Printf("Schema version: %d\n", version)
	}

	// Initialize auth
	tokenKey, err := base64.StdEncoding.DecodeString(cfg.Auth.Token.Key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to decode token key: %v\n", err)
		os.Exit(1)
	}
	authService := auth.New(auth.Config{
		TokenKey:          tokenKey,
		TokenExpiry:       time.Duration(cfg.Auth.Token.ExpireIn) * time.Second,
		MinUsernameLength: cfg.Auth.Basic.MinLoginLength,
		MinPasswordLength: cfg.Auth.Basic.MinPasswordLength,
	})

	// Initialize Redis (optional)
	var redisClient *redis.Client
	if cfg.Redis.Enabled {
		var err error
		redisClient, err = redis.New(redis.Config{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
			NodeID:   cfg.Redis.NodeID,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to connect to Redis: %v\n", err)
			os.Exit(1)
		}
		defer redisClient.Close()
		fmt.Printf("Connected to Redis (node: %s)\n", cfg.Redis.NodeID)
	}

	// Initialize hub
	hub := NewHub()
	hub.SetRedis(redisClient)
	go hub.Run()

	// Start Redis pub/sub listener if enabled
	var pubsubCancel context.CancelFunc
	if redisClient != nil {
		var pubsubCtx context.Context
		pubsubCtx, pubsubCancel = context.WithCancel(context.Background())

		// Subscribe to node-specific channel
		nodePubsub := redisClient.NewPubSub(hub.HandlePubSubMessage)
		nodePubsub.SubscribeToNode(pubsubCtx)
		go nodePubsub.Listen(pubsubCtx)

		// Subscribe to user channels (pattern)
		userPubsub := redisClient.NewPubSub(hub.HandlePubSubMessage)
		userPubsub.SubscribeToUsers(pubsubCtx)
		go userPubsub.Listen(pubsubCtx)
	}

	// Initialize presence manager
	presence := NewPresenceManager(hub, db)
	hub.SetPresence(presence)

	// Start presence heartbeat if Redis is enabled
	if redisClient != nil {
		presence.StartHeartbeat(context.Background())
	}

	// Initialize encryptor for message content
	encryptor, err := crypto.NewEncryptorFromBase64(cfg.Database.EncryptionKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize encryptor: %v\n", err)
		os.Exit(1)
	}

	// Initialize email service
	emailService := email.New(email.Config{
		Enabled:  cfg.Email.Enabled,
		Host:     cfg.Email.Host,
		Port:     cfg.Email.Port,
		Username: cfg.Email.Username,
		Password: cfg.Email.Password,
		From:     cfg.Email.From,
		FromName: cfg.Email.FromName,
		BaseURL:  cfg.Email.BaseURL,
	})

	// Initialize invite token generator (uses token key, 7-day TTL)
	inviteTokenGen, err := crypto.NewInviteTokenGenerator(tokenKey, 7*24*time.Hour)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize invite token generator: %v\n", err)
		os.Exit(1)
	}

	// Initialize handlers
	handlers := NewHandlers(db, authService, hub, encryptor, emailService, inviteTokenGen, cfg)

	// Initialize media processor
	mediaProcessor := media.NewProcessor(media.Config{
		UploadPath:    cfg.Media.UploadDir,
		MaxUploadSize: cfg.Media.MaxSize,
		ThumbWidth:    256,
		ThumbHeight:   256,
		ThumbQuality:  80,
	})

	// Initialize file handlers
	authValidator := auth.NewValidator(authService)
	fileHandlers := NewFileHandlers(db, mediaProcessor, authValidator)

	// Initialize server
	srv := NewServer(hub, cfg, handlers)
	mux := http.NewServeMux()
	srv.SetupRoutes(mux)
	fileHandlers.SetupRoutes(mux)

	// Configure CORS middleware
	corsMiddleware := middleware.CORS(middleware.CORSConfig{
		AllowedOrigins: cfg.Server.AllowedOrigins,
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization", "X-Requested-With"},
		MaxAge:         86400, // 24 hours
	})

	// Wrap handler with CORS middleware
	handler := corsMiddleware(mux)

	// Start HTTP server with timeouts
	httpServer := &http.Server{
		Addr:         cfg.Server.Listen,
		Handler:      handler,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
	}

	go func() {
		fmt.Printf("Listening on %s (timeouts: read=%ds, write=%ds, idle=%ds)\n",
			cfg.Server.Listen, cfg.Server.ReadTimeout, cfg.Server.WriteTimeout, cfg.Server.IdleTimeout)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("Shutting down...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(),
		time.Duration(cfg.Server.ShutdownTimeout)*time.Second)
	defer shutdownCancel()

	// Cancel pub/sub first
	if pubsubCancel != nil {
		pubsubCancel()
	}

	// Shutdown hub (closes WebSocket connections)
	hub.Shutdown()

	// Gracefully shutdown HTTP server
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "HTTP server shutdown error: %v\n", err)
		httpServer.Close() // Force close if graceful shutdown fails
	}

	fmt.Println("Server stopped")
}

// generateSecureKey generates a cryptographically secure random key.
func generateSecureKey(bytes int) string {
	key := make([]byte, bytes)
	if _, err := cryptorand.Read(key); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate secure key: %v\n", err)
		os.Exit(1)
	}
	return base64.StdEncoding.EncodeToString(key)
}

// printGeneratedKeys outputs secure keys for configuration.
func printGeneratedKeys() {
	fmt.Println("# Generated secure keys for mvChat2 configuration")
	fmt.Println("# Copy these values to your mvchat2.yaml or set as environment variables")
	fmt.Println("#")
	fmt.Println("# WARNING: These keys are generated fresh each time.")
	fmt.Println("# Changing keys after deployment will invalidate existing data!")
	fmt.Println("")
	fmt.Println("# Environment variables (recommended for production):")
	fmt.Printf("export UID_KEY='%s'\n", generateSecureKey(16))
	fmt.Printf("export ENCRYPTION_KEY='%s'\n", generateSecureKey(32))
	fmt.Printf("export API_KEY_SALT='%s'\n", generateSecureKey(32))
	fmt.Printf("export TOKEN_KEY='%s'\n", generateSecureKey(32))
	fmt.Println("")
	fmt.Println("# Or YAML configuration:")
	fmt.Println("database:")
	fmt.Printf("  uid_key: %s\n", generateSecureKey(16))
	fmt.Printf("  encryption_key: %s\n", generateSecureKey(32))
	fmt.Println("auth:")
	fmt.Printf("  api_key_salt: %s\n", generateSecureKey(32))
	fmt.Println("  token:")
	fmt.Printf("    key: %s\n", generateSecureKey(32))
}
