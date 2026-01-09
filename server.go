package main

import (
	"fmt"
	"log"
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
	mux.HandleFunc("/verify-email", s.handleVerifyEmail)
	// TODO: Add file upload/download routes
}

// handleWebSocket upgrades HTTP to WebSocket and creates a session.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("server: WebSocket upgrade failed: %v", err)
		return
	}

	remoteAddr := r.RemoteAddr
	if s.config.Server.UseXForwardedFor {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			remoteAddr = xff
		}
	}

	sess := NewSession(s.hub, conn, remoteAddr, s.handlers, s.config.Limits.RateLimitMessages)
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

// handleVerifyEmail handles email verification via token.
func (s *Server) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	// Only GET allowed
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Missing verification token", http.StatusBadRequest)
		return
	}

	// Verify the token
	userID, err := s.handlers.db.VerifyEmailByToken(r.Context(), token)
	if err != nil {
		log.Printf("server: email verification error: %v", err)
		http.Error(w, "Verification failed", http.StatusInternalServerError)
		return
	}

	if userID == nil {
		// Token not found or expired
		http.Error(w, "Invalid or expired verification link", http.StatusBadRequest)
		return
	}

	// Success - redirect to app or show success page
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Email Verified</title>
    <style>
        body { font-family: -apple-system, sans-serif; display: flex; justify-content: center; align-items: center; height: 100vh; margin: 0; background: #f5f5f5; }
        .card { background: white; padding: 40px; border-radius: 10px; text-align: center; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        h1 { color: #667eea; }
    </style>
</head>
<body>
    <div class="card">
        <h1>Email Verified!</h1>
        <p>Your email has been successfully verified.</p>
        <p>You can now close this window and return to the app.</p>
    </div>
</body>
</html>`)
}
