package main

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/scalecode-solutions/mvchat2/store"
)

// testSession is a minimal session for testing that captures sent messages.
// It implements SessionInterface.
type testSession struct {
	id       string
	userID   uuid.UUID
	messages []*ServerMessage
	mu       sync.Mutex
}

// Compile-time check that testSession implements SessionInterface.
var _ SessionInterface = (*testSession)(nil)

func newTestSession(userID uuid.UUID) *testSession {
	return &testSession{
		id:       uuid.New().String(),
		userID:   userID,
		messages: make([]*ServerMessage, 0),
	}
}

func (s *testSession) ID() string { return s.id }

func (s *testSession) UserID() uuid.UUID { return s.userID }

func (s *testSession) IsAuthenticated() bool { return s.userID != uuid.Nil }

func (s *testSession) RequireAuth(msgID string) bool {
	if !s.IsAuthenticated() {
		s.Send(CtrlError(msgID, CodeUnauthorized, "not authenticated"))
		return false
	}
	return true
}

func (s *testSession) Send(msg *ServerMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
}

func (s *testSession) LastMessage() *ServerMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.messages) == 0 {
		return nil
	}
	return s.messages[len(s.messages)-1]
}

func (s *testSession) MessageCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.messages)
}

// testHandlers creates handlers with a mock store for testing.
func testHandlers(mockStore *store.MockStore) *Handlers {
	return &Handlers{
		db: mockStore,
	}
}

func TestHandleAddContact_Success(t *testing.T) {
	userID := uuid.New()
	contactID := uuid.New()

	mockStore := &store.MockStore{
		GetUserByIDFn: func(ctx context.Context, id uuid.UUID) (*store.User, error) {
			if id == contactID {
				return &store.User{
					ID:     contactID,
					State:  "ok",
					Public: json.RawMessage(`{"fn":"Test User"}`),
				}, nil
			}
			return nil, nil
		},
		AddContactFn: func(ctx context.Context, uid, cid uuid.UUID, source string, inviteID *uuid.UUID) error {
			if uid != userID || cid != contactID {
				t.Errorf("unexpected contact add: %v -> %v", uid, cid)
			}
			if source != "manual" {
				t.Errorf("expected source 'manual', got %s", source)
			}
			return nil
		},
	}

	h := testHandlers(mockStore)
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		Contact: &MsgClientContact{
			Add: contactID.String(),
		},
	}

	h.handleAddContact(context.Background(), sess, msg, contactID.String())

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response message")
	}
	if resp.Ctrl == nil {
		t.Fatal("expected ctrl message")
	}
	if resp.Ctrl.Code != CodeOK {
		t.Errorf("expected code %d, got %d", CodeOK, resp.Ctrl.Code)
	}
}

func TestHandleAddContact_InvalidUUID(t *testing.T) {
	userID := uuid.New()

	h := testHandlers(&store.MockStore{})
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		Contact: &MsgClientContact{
			Add: "not-a-uuid",
		},
	}

	h.handleAddContact(context.Background(), sess, msg, "not-a-uuid")

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response message")
	}
	if resp.Ctrl == nil {
		t.Fatal("expected ctrl message")
	}
	if resp.Ctrl.Code != CodeBadRequest {
		t.Errorf("expected code %d, got %d", CodeBadRequest, resp.Ctrl.Code)
	}
}

func TestHandleAddContact_CannotAddSelf(t *testing.T) {
	userID := uuid.New()

	h := testHandlers(&store.MockStore{})
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		Contact: &MsgClientContact{
			Add: userID.String(),
		},
	}

	h.handleAddContact(context.Background(), sess, msg, userID.String())

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response message")
	}
	if resp.Ctrl == nil {
		t.Fatal("expected ctrl message")
	}
	if resp.Ctrl.Code != CodeBadRequest {
		t.Errorf("expected code %d, got %d", CodeBadRequest, resp.Ctrl.Code)
	}
	if resp.Ctrl.Text != "cannot add yourself as contact" {
		t.Errorf("unexpected error message: %s", resp.Ctrl.Text)
	}
}

func TestHandleAddContact_UserNotFound(t *testing.T) {
	userID := uuid.New()
	contactID := uuid.New()

	mockStore := &store.MockStore{
		GetUserByIDFn: func(ctx context.Context, id uuid.UUID) (*store.User, error) {
			return nil, nil // User not found
		},
	}

	h := testHandlers(mockStore)
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		Contact: &MsgClientContact{
			Add: contactID.String(),
		},
	}

	h.handleAddContact(context.Background(), sess, msg, contactID.String())

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response message")
	}
	if resp.Ctrl == nil {
		t.Fatal("expected ctrl message")
	}
	if resp.Ctrl.Code != CodeNotFound {
		t.Errorf("expected code %d, got %d", CodeNotFound, resp.Ctrl.Code)
	}
}

func TestHandleAddContact_DatabaseError(t *testing.T) {
	userID := uuid.New()
	contactID := uuid.New()

	mockStore := &store.MockStore{
		GetUserByIDFn: func(ctx context.Context, id uuid.UUID) (*store.User, error) {
			return nil, errors.New("database connection failed")
		},
	}

	h := testHandlers(mockStore)
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		Contact: &MsgClientContact{
			Add: contactID.String(),
		},
	}

	h.handleAddContact(context.Background(), sess, msg, contactID.String())

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response message")
	}
	if resp.Ctrl == nil {
		t.Fatal("expected ctrl message")
	}
	if resp.Ctrl.Code != CodeInternalError {
		t.Errorf("expected code %d, got %d", CodeInternalError, resp.Ctrl.Code)
	}
}

func TestHandleRemoveContact_Success(t *testing.T) {
	userID := uuid.New()
	contactID := uuid.New()
	removeCalled := false

	mockStore := &store.MockStore{
		RemoveContactFn: func(ctx context.Context, uid, cid uuid.UUID) error {
			removeCalled = true
			if uid != userID || cid != contactID {
				t.Errorf("unexpected contact remove: %v -> %v", uid, cid)
			}
			return nil
		},
	}

	h := testHandlers(mockStore)
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		Contact: &MsgClientContact{
			Remove: contactID.String(),
		},
	}

	h.handleRemoveContact(context.Background(), sess, msg, contactID.String())

	if !removeCalled {
		t.Error("expected RemoveContact to be called")
	}

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response message")
	}
	if resp.Ctrl == nil {
		t.Fatal("expected ctrl message")
	}
	if resp.Ctrl.Code != CodeOK {
		t.Errorf("expected code %d, got %d", CodeOK, resp.Ctrl.Code)
	}
}

func TestHandleUpdateContactNickname_Success(t *testing.T) {
	userID := uuid.New()
	contactID := uuid.New()
	nickname := "My Friend"
	updateCalled := false

	mockStore := &store.MockStore{
		UpdateContactNicknameFn: func(ctx context.Context, uid, cid uuid.UUID, nick *string) error {
			updateCalled = true
			if uid != userID || cid != contactID {
				t.Errorf("unexpected update: %v -> %v", uid, cid)
			}
			if nick == nil || *nick != nickname {
				t.Errorf("expected nickname %q, got %v", nickname, nick)
			}
			return nil
		},
	}

	h := testHandlers(mockStore)
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		Contact: &MsgClientContact{
			User:     contactID.String(),
			Nickname: &nickname,
		},
	}

	h.handleUpdateContactNickname(context.Background(), sess, msg, contactID.String(), &nickname)

	if !updateCalled {
		t.Error("expected UpdateContactNickname to be called")
	}

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response message")
	}
	if resp.Ctrl == nil {
		t.Fatal("expected ctrl message")
	}
	if resp.Ctrl.Code != CodeOK {
		t.Errorf("expected code %d, got %d", CodeOK, resp.Ctrl.Code)
	}
}
