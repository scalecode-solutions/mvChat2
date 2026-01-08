package main

import (
	"context"
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
	"github.com/scalecode-solutions/mvchat2/media"
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
	flag.Parse()

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
		pubsub := redisClient.NewPubSub(hub.HandlePubSubMessage)
		pubsub.SubscribeToNode(pubsubCtx)
		go pubsub.Listen(pubsubCtx)
	}

	// Initialize presence manager
	presence := NewPresenceManager(hub, db)
	hub.SetPresence(presence)

	// Initialize encryptor for message content
	encryptor, err := crypto.NewEncryptorFromBase64(cfg.Database.EncryptionKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize encryptor: %v\n", err)
		os.Exit(1)
	}

	// Initialize handlers
	handlers := NewHandlers(db, authService, hub, encryptor)

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

	// Start HTTP server
	httpServer := &http.Server{
		Addr:    cfg.Server.Listen,
		Handler: mux,
	}

	go func() {
		fmt.Printf("Listening on %s\n", cfg.Server.Listen)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("Shutting down...")
	if pubsubCancel != nil {
		pubsubCancel()
	}
	hub.Shutdown()
	httpServer.Close()
}
