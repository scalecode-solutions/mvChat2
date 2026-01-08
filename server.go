package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/scalecode-solutions/mvchat2/config"
	"github.com/scalecode-solutions/mvchat2/middleware"
)

// Server handles HTTP and WebSocket connections.
type Server struct {
	hub      *Hub
	config   *config.Config
	handlers *Handlers
	upgrader websocket.Upgrader
}

// NewServer creates a new server.
func NewServer(hub *Hub, cfg *config.Config, handlers *Handlers) *Server {
	s := &Server{
		hub:      hub,
		config:   cfg,
		handlers: handlers,
	}

	// Configure WebSocket upgrader with origin checking
	s.upgrader = websocket.Upgrader{
		ReadBufferSize:    1024,
		WriteBufferSize:   1024,
		EnableCompression: true,
		CheckOrigin:       middleware.CheckOrigin(cfg.Server.AllowedOrigins),
	}

	return s
}

// SetupRoutes configures HTTP routes.
func (s *Server) SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v0/ws", s.handleWebSocket)
	mux.HandleFunc("/health", s.handleHealth)
	// TODO: Add file upload/download routes
}

// handleWebSocket upgrades HTTP to WebSocket and creates a session.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("WebSocket upgrade failed: %v\n", err)
		return
	}

	remoteAddr := r.RemoteAddr
	if s.config.Server.UseXForwardedFor {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			remoteAddr = xff
		}
	}

	sess := NewSession(s.hub, conn, remoteAddr, s.handlers)
	s.hub.Register(sess)

	// Run the session (blocks until session closes)
	sess.Run()
}

// handleHealth is a simple health check endpoint.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","sessions":%d,"online":%d}`, s.hub.SessionCount(), s.hub.OnlineCount())
}
