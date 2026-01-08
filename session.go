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
	userID     uuid.UUID
	userAgent  string
	deviceID   string
	lang       string
	ver        string
	remoteAddr string

	// Last activity timestamp
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
	return s.userID
}

// IsAuthenticated returns true if the session is authenticated.
func (s *Session) IsAuthenticated() bool {
	return s.userID != uuid.Nil
}

// Send queues a message to be sent to the client.
func (s *Session) Send(msg *ServerMessage) {
	if atomic.LoadInt32(&s.closing) == 1 {
		return
	}
	select {
	case s.send <- msg:
	default:
		// Buffer full, close the session
		s.Close()
	}
}

// Close closes the session.
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
	case msg.Group != nil:
		s.handleGroup(msg)
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
	default:
		s.Send(CtrlError(msg.ID, CodeBadRequest, "unknown message type"))
	}
}

// Handler stubs - to be implemented
func (s *Session) handleHi(msg *ClientMessage) {
	hi := msg.Hi
	s.ver = hi.Version
	s.userAgent = hi.UserAgent
	s.deviceID = hi.DeviceID
	s.lang = hi.Lang

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
	if !s.IsAuthenticated() {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
		return
	}
	// TODO: Implement DM start/manage
	s.Send(CtrlError(msg.ID, CodeInternalError, "not implemented"))
}

func (s *Session) handleGroup(msg *ClientMessage) {
	if !s.IsAuthenticated() {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
		return
	}
	// TODO: Implement group management
	s.Send(CtrlError(msg.ID, CodeInternalError, "not implemented"))
}

func (s *Session) handleSend(msg *ClientMessage) {
	if !s.IsAuthenticated() {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
		return
	}
	// TODO: Implement message sending
	s.Send(CtrlError(msg.ID, CodeInternalError, "not implemented"))
}

func (s *Session) handleGet(msg *ClientMessage) {
	if !s.IsAuthenticated() {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
		return
	}
	// TODO: Implement data fetching
	s.Send(CtrlError(msg.ID, CodeInternalError, "not implemented"))
}

func (s *Session) handleEdit(msg *ClientMessage) {
	if !s.IsAuthenticated() {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
		return
	}
	// TODO: Implement message editing
	s.Send(CtrlError(msg.ID, CodeInternalError, "not implemented"))
}

func (s *Session) handleUnsend(msg *ClientMessage) {
	if !s.IsAuthenticated() {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
		return
	}
	// TODO: Implement message unsending
	s.Send(CtrlError(msg.ID, CodeInternalError, "not implemented"))
}

func (s *Session) handleDelete(msg *ClientMessage) {
	if !s.IsAuthenticated() {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
		return
	}
	// TODO: Implement message deletion
	s.Send(CtrlError(msg.ID, CodeInternalError, "not implemented"))
}

func (s *Session) handleReact(msg *ClientMessage) {
	if !s.IsAuthenticated() {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
		return
	}
	// TODO: Implement reactions
	s.Send(CtrlError(msg.ID, CodeInternalError, "not implemented"))
}

func (s *Session) handleTyping(msg *ClientMessage) {
	if !s.IsAuthenticated() {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
		return
	}
	// TODO: Implement typing indicator
	s.Send(CtrlError(msg.ID, CodeInternalError, "not implemented"))
}

func (s *Session) handleRead(msg *ClientMessage) {
	if !s.IsAuthenticated() {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
		return
	}
	// TODO: Implement read receipts
	s.Send(CtrlError(msg.ID, CodeInternalError, "not implemented"))
}
