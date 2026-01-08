package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/scalecode-solutions/mvchat2/config"
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

	// Initialize hub
	hub := NewHub()
	go hub.Run()

	// Initialize server
	srv := NewServer(hub, cfg)
	mux := http.NewServeMux()
	srv.SetupRoutes(mux)

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
	hub.Shutdown()
	httpServer.Close()
}
