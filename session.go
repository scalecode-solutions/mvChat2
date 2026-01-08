package main

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second
	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
	// Maximum message size allowed from peer.
	maxMessageSize = 64 * 1024 // 64KB
	// Send buffer size
	sendBufferSize = 128
)

// Session represents a WebSocket connection.
type Session struct {
	id         string
	hub        *Hub
	conn       *websocket.Conn
	send       chan *ServerMessage
	handlers   *Handlers
	remoteAddr string

	// Protected by mu - accessed from multiple goroutines
	mu        sync.RWMutex
	userID    uuid.UUID
	userAgent string
	deviceID  string
	lang      string
	ver       string

	// Last activity timestamp (atomic access)
	lastAction int64

	// Closing state
	closing int32
	once    sync.Once
}

// NewSession creates a new session.
func NewSession(hub *Hub, conn *websocket.Conn, remoteAddr string, handlers *Handlers) *Session {
	return &Session{
		id:         uuid.New().String(),
		hub:        hub,
		conn:       conn,
		send:       make(chan *ServerMessage, sendBufferSize),
		handlers:   handlers,
		remoteAddr: remoteAddr,
		lastAction: time.Now().UnixNano(),
	}
}

// ID returns the session ID.
func (s *Session) ID() string {
	return s.id
}

// UserID returns the authenticated user ID.
func (s *Session) UserID() uuid.UUID {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.userID
}

// SetUserID sets the authenticated user ID.
// This should be called from Hub.AuthenticateSession.
func (s *Session) SetUserID(id uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.userID = id
}

// IsAuthenticated returns true if the session is authenticated.
func (s *Session) IsAuthenticated() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.userID != uuid.Nil
}

// UserAgent returns the session's user agent.
func (s *Session) UserAgent() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.userAgent
}

// DeviceID returns the session's device ID.
func (s *Session) DeviceID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.deviceID
}

// Lang returns the session's language.
func (s *Session) Lang() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lang
}

// Ver returns the session's client version.
func (s *Session) Ver() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ver
}

// Send queues a message to be sent to the client.
// Safe to call from multiple goroutines.
func (s *Session) Send(msg *ServerMessage) {
	// Use a simple recover to handle the race condition where Close()
	// may close the channel between our check and the send operation.
	// This is more efficient than using a mutex for every send.
	defer func() {
		if r := recover(); r != nil {
			// Channel was closed, session is closing - ignore
		}
	}()

	if atomic.LoadInt32(&s.closing) == 1 {
		return
	}
	select {
	case s.send <- msg:
	default:
		// Buffer full, close the session
		go s.Close() // Close in goroutine to avoid deadlock
	}
}

// Close closes the session.
// Safe to call multiple times - only first call takes effect.
func (s *Session) Close() {
	s.once.Do(func() {
		atomic.StoreInt32(&s.closing, 1)
		close(s.send)
		s.conn.Close()
	})
}

// Run starts the session's read and write pumps.
func (s *Session) Run() {
	go s.writePump()
	s.readPump()
}

// readPump pumps messages from the WebSocket connection to the hub.
func (s *Session) readPump() {
	defer func() {
		s.hub.Unregister(s)
		s.Close()
	}()

	s.conn.SetReadLimit(maxMessageSize)
	s.conn.SetReadDeadline(time.Now().Add(pongWait))
	s.conn.SetPongHandler(func(string) error {
		s.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := s.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// Log unexpected close
			}
			break
		}

		atomic.StoreInt64(&s.lastAction, time.Now().UnixNano())

		var msg ClientMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			s.Send(CtrlError("", CodeBadRequest, "malformed message"))
			continue
		}

		s.dispatch(&msg)
	}
}

// writePump pumps messages from the hub to the WebSocket connection.
func (s *Session) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		s.Close()
	}()

	for {
		select {
		case msg, ok := <-s.send:
			s.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Channel closed
				s.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := s.conn.WriteJSON(msg); err != nil {
				return
			}

		case <-ticker.C:
			s.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := s.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// dispatch routes a client message to the appropriate handler.
func (s *Session) dispatch(msg *ClientMessage) {
	switch {
	case msg.Hi != nil:
		s.handleHi(msg)
	case msg.Login != nil:
		s.handleLogin(msg)
	case msg.Acc != nil:
		s.handleAcc(msg)
	case msg.Search != nil:
		s.handleSearch(msg)
	case msg.DM != nil:
		s.handleDM(msg)
	case msg.Room != nil:
		s.handleRoom(msg)
	case msg.Send != nil:
		s.handleSend(msg)
	case msg.Get != nil:
		s.handleGet(msg)
	case msg.Edit != nil:
		s.handleEdit(msg)
	case msg.Unsend != nil:
		s.handleUnsend(msg)
	case msg.Delete != nil:
		s.handleDelete(msg)
	case msg.React != nil:
		s.handleReact(msg)
	case msg.Typing != nil:
		s.handleTyping(msg)
	case msg.Read != nil:
		s.handleRead(msg)
	case msg.Invite != nil:
		s.handleInvite(msg)
	case msg.Contact != nil:
		s.handleContact(msg)
	default:
		s.Send(CtrlError(msg.ID, CodeBadRequest, "unknown message type"))
	}
}

// Handler stubs - to be implemented
func (s *Session) handleHi(msg *ClientMessage) {
	hi := msg.Hi

	s.mu.Lock()
	s.ver = hi.Version
	s.userAgent = hi.UserAgent
	s.deviceID = hi.DeviceID
	s.lang = hi.Lang
	s.mu.Unlock()

	s.Send(CtrlSuccess(msg.ID, CodeOK, map[string]any{
		"ver":   "0.1.0",
		"build": buildstamp,
		"sid":   s.id,
	}))
}

func (s *Session) handleLogin(msg *ClientMessage) {
	s.handlers.HandleLogin(s, msg)
}

func (s *Session) handleAcc(msg *ClientMessage) {
	s.handlers.HandleAcc(s, msg)
}

func (s *Session) handleSearch(msg *ClientMessage) {
	s.handlers.HandleSearch(s, msg)
}

func (s *Session) handleDM(msg *ClientMessage) {
	s.handlers.HandleDM(s, msg)
}

func (s *Session) handleRoom(msg *ClientMessage) {
	s.handlers.HandleRoom(s, msg)
}

func (s *Session) handleSend(msg *ClientMessage) {
	s.handlers.HandleSend(s, msg)
}

func (s *Session) handleGet(msg *ClientMessage) {
	s.handlers.HandleGet(s, msg)
}

func (s *Session) handleEdit(msg *ClientMessage) {
	s.handlers.HandleEdit(s, msg)
}

func (s *Session) handleUnsend(msg *ClientMessage) {
	s.handlers.HandleUnsend(s, msg)
}

func (s *Session) handleDelete(msg *ClientMessage) {
	s.handlers.HandleDelete(s, msg)
}

func (s *Session) handleReact(msg *ClientMessage) {
	s.handlers.HandleReact(s, msg)
}

func (s *Session) handleTyping(msg *ClientMessage) {
	s.handlers.HandleTyping(s, msg)
}

func (s *Session) handleRead(msg *ClientMessage) {
	s.handlers.HandleRead(s, msg)
}

func (s *Session) handleInvite(msg *ClientMessage) {
	s.handlers.HandleInvite(s, msg)
}

func (s *Session) handleContact(msg *ClientMessage) {
	s.handlers.HandleContact(s, msg)
}
