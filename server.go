package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/scalecode-solutions/mvchat2/config"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// Server handles HTTP and WebSocket connections.
type Server struct {
	hub    *Hub
	config *config.Config
}

// NewServer creates a new server.
func NewServer(hub *Hub, cfg *config.Config) *Server {
	return &Server{
		hub:    hub,
		config: cfg,
	}
}

// SetupRoutes configures HTTP routes.
func (s *Server) SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v0/ws", s.handleWebSocket)
	mux.HandleFunc("/health", s.handleHealth)
	// TODO: Add file upload/download routes
}

// handleWebSocket upgrades HTTP to WebSocket and creates a session.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
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

	sess := NewSession(s.hub, conn, remoteAddr)
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
