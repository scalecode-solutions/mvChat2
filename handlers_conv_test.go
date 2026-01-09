package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scalecode-solutions/mvchat2/crypto"
	"github.com/scalecode-solutions/mvchat2/store"
)

func TestHandleDM_NotAuthenticated(t *testing.T) {
	h := testHandlers(&store.MockStore{})
	sess := newTestSession(uuid.Nil) // Not authenticated

	msg := &ClientMessage{
		ID: "test-1",
		DM: &MsgClientDM{
			User: uuid.New().String(),
		},
	}

	h.handleDM(sess, msg)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeUnauthorized {
		t.Errorf("expected code %d, got %d", CodeUnauthorized, resp.Ctrl.Code)
	}
}

func TestHandleDM_MissingData(t *testing.T) {
	h := testHandlers(&store.MockStore{})
	userID := uuid.New()
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		DM: nil,
	}

	h.handleDM(sess, msg)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeBadRequest {
		t.Errorf("expected code %d, got %d", CodeBadRequest, resp.Ctrl.Code)
	}
}

func TestHandleStartDM_Success(t *testing.T) {
	userID := uuid.New()
	otherUserID := uuid.New()
	convID := uuid.New()

	mockStore := &store.MockStore{
		GetUserByIDFn: func(ctx context.Context, id uuid.UUID) (*store.User, error) {
			if id == otherUserID {
				return &store.User{
					ID:     otherUserID,
					State:  "ok",
					Public: json.RawMessage(`{"fn":"Other User"}`),
				}, nil
			}
			return nil, nil
		},
		CreateDMFn: func(ctx context.Context, user1ID, user2ID uuid.UUID) (*store.Conversation, bool, error) {
			return &store.Conversation{ID: convID, Type: "dm"}, true, nil
		},
	}

	h := testHandlers(mockStore)
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		DM: &MsgClientDM{
			User: otherUserID.String(),
		},
	}

	h.handleDM(sess, msg)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeCreated {
		t.Errorf("expected code %d, got %d: %s", CodeCreated, resp.Ctrl.Code, resp.Ctrl.Text)
	}
	if resp.Ctrl.Params["conv"] != convID.String() {
		t.Errorf("expected conv %s, got %v", convID.String(), resp.Ctrl.Params["conv"])
	}
}

func TestHandleStartDM_CannotDMSelf(t *testing.T) {
	userID := uuid.New()
	h := testHandlers(&store.MockStore{})
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		DM: &MsgClientDM{
			User: userID.String(), // Trying to DM self
		},
	}

	h.handleDM(sess, msg)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeBadRequest {
		t.Errorf("expected code %d, got %d", CodeBadRequest, resp.Ctrl.Code)
	}
	if resp.Ctrl.Text != "cannot DM yourself" {
		t.Errorf("unexpected error: %s", resp.Ctrl.Text)
	}
}

func TestHandleStartDM_UserNotFound(t *testing.T) {
	userID := uuid.New()
	otherUserID := uuid.New()

	mockStore := &store.MockStore{
		GetUserByIDFn: func(ctx context.Context, id uuid.UUID) (*store.User, error) {
			return nil, nil // User not found
		},
	}

	h := testHandlers(mockStore)
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		DM: &MsgClientDM{
			User: otherUserID.String(),
		},
	}

	h.handleDM(sess, msg)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeNotFound {
		t.Errorf("expected code %d, got %d", CodeNotFound, resp.Ctrl.Code)
	}
}

func TestHandleStartDM_InvalidUUID(t *testing.T) {
	userID := uuid.New()
	h := testHandlers(&store.MockStore{})
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		DM: &MsgClientDM{
			User: "not-a-uuid",
		},
	}

	h.handleDM(sess, msg)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeBadRequest {
		t.Errorf("expected code %d, got %d", CodeBadRequest, resp.Ctrl.Code)
	}
}

func TestHandleManageDM_Success(t *testing.T) {
	userID := uuid.New()
	convID := uuid.New()
	favorite := true

	mockStore := &store.MockStore{
		IsMemberFn: func(ctx context.Context, cID, uID uuid.UUID) (bool, error) {
			return cID == convID && uID == userID, nil
		},
		UpdateMemberSettingsFn: func(ctx context.Context, cID, uID uuid.UUID, settings store.MemberSettings) error {
			if cID != convID || uID != userID {
				t.Errorf("wrong IDs")
			}
			if settings.Favorite == nil || !*settings.Favorite {
				t.Errorf("expected favorite=true")
			}
			return nil
		},
	}

	h := testHandlers(mockStore)
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		DM: &MsgClientDM{
			ConversationID: convID.String(),
			Favorite:       &favorite,
		},
	}

	h.handleDM(sess, msg)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeOK {
		t.Errorf("expected code %d, got %d: %s", CodeOK, resp.Ctrl.Code, resp.Ctrl.Text)
	}
}

func TestHandleManageDM_NotMember(t *testing.T) {
	userID := uuid.New()
	convID := uuid.New()
	favorite := true

	mockStore := &store.MockStore{
		IsMemberFn: func(ctx context.Context, cID, uID uuid.UUID) (bool, error) {
			return false, nil // Not a member
		},
	}

	h := testHandlers(mockStore)
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		DM: &MsgClientDM{
			ConversationID: convID.String(),
			Favorite:       &favorite,
		},
	}

	h.handleDM(sess, msg)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeForbidden {
		t.Errorf("expected code %d, got %d", CodeForbidden, resp.Ctrl.Code)
	}
}

func TestHandleGetConversations_Success(t *testing.T) {
	userID := uuid.New()
	convID := uuid.New()
	otherUserID := uuid.New()
	now := time.Now()

	mockStore := &store.MockStore{
		GetUserConversationsFn: func(ctx context.Context, uID uuid.UUID) ([]store.ConversationWithMember, error) {
			return []store.ConversationWithMember{
				{
					Conversation: store.Conversation{
						ID:        convID,
						Type:      "dm",
						LastSeq:   10,
						LastMsgAt: &now,
					},
					ReadSeq:  5,
					Favorite: true,
					OtherUser: &store.User{
						ID:     otherUserID,
						Public: json.RawMessage(`{"fn":"Other"}`),
					},
				},
			}, nil
		},
	}

	h := testHandlers(mockStore)
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		Get: &MsgClientGet{
			What: "conversations",
		},
	}

	h.handleGetConversations(context.Background(), sess, msg)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeOK {
		t.Errorf("expected code %d, got %d", CodeOK, resp.Ctrl.Code)
	}
	convs, ok := resp.Ctrl.Params["conversations"].([]map[string]any)
	if !ok || len(convs) != 1 {
		t.Errorf("expected 1 conversation, got %v", resp.Ctrl.Params["conversations"])
	}
}

func TestHandleGetMessages_Success(t *testing.T) {
	userID := uuid.New()
	convID := uuid.New()
	fromUserID := uuid.New()
	now := time.Now()

	encryptor, _ := crypto.NewEncryptor([]byte("test-key-32-bytes-long-for-test!"))

	mockStore := &store.MockStore{
		GetMemberFn: func(ctx context.Context, cID, uID uuid.UUID) (*store.Member, error) {
			if cID == convID && uID == userID {
				return &store.Member{ClearSeq: 0}, nil
			}
			return nil, nil
		},
		GetMessagesFn: func(ctx context.Context, cID, uID uuid.UUID, before, limit int, clearSeq int) ([]store.Message, error) {
			content, _ := encryptor.Encrypt([]byte("test message"))
			return []store.Message{
				{
					ID:             uuid.New(),
					ConversationID: convID,
					FromUserID:     fromUserID,
					Seq:            1,
					Content:        content,
					CreatedAt:      now,
				},
			}, nil
		},
	}

	h := &Handlers{
		db:        mockStore,
		encryptor: encryptor,
	}
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		Get: &MsgClientGet{
			What:           "messages",
			ConversationID: convID.String(),
			Limit:          20,
		},
	}

	h.handleGetMessages(context.Background(), sess, msg, msg.Get)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeOK {
		t.Errorf("expected code %d, got %d: %s", CodeOK, resp.Ctrl.Code, resp.Ctrl.Text)
	}
}

func TestHandleGetMessages_NotMember(t *testing.T) {
	userID := uuid.New()
	convID := uuid.New()

	mockStore := &store.MockStore{
		GetMemberFn: func(ctx context.Context, cID, uID uuid.UUID) (*store.Member, error) {
			return nil, nil // Not a member
		},
	}

	h := testHandlers(mockStore)
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		Get: &MsgClientGet{
			What:           "messages",
			ConversationID: convID.String(),
		},
	}

	h.handleGetMessages(context.Background(), sess, msg, msg.Get)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeForbidden {
		t.Errorf("expected code %d, got %d", CodeForbidden, resp.Ctrl.Code)
	}
}

func TestHandleGetContacts_Success(t *testing.T) {
	userID := uuid.New()
	contactID := uuid.New()
	nickname := "My Friend"

	mockStore := &store.MockStore{
		GetContactsFn: func(ctx context.Context, uID uuid.UUID) ([]store.Contact, error) {
			return []store.Contact{
				{
					UserID:    userID,
					ContactID: contactID,
					Nickname:  &nickname,
					Source:    "manual",
					CreatedAt: time.Now(),
				},
			}, nil
		},
		GetUserByIDFn: func(ctx context.Context, id uuid.UUID) (*store.User, error) {
			if id == contactID {
				return &store.User{
					ID:     contactID,
					Public: json.RawMessage(`{"fn":"Contact Name"}`),
				}, nil
			}
			return nil, nil
		},
	}

	h := testHandlers(mockStore)
	sess := newTestSession(userID)

	msg := &ClientMessage{ID: "test-1"}

	h.handleGetContacts(context.Background(), sess, msg)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeOK {
		t.Errorf("expected code %d, got %d", CodeOK, resp.Ctrl.Code)
	}
	contacts, ok := resp.Ctrl.Params["contacts"].([]map[string]any)
	if !ok || len(contacts) != 1 {
		t.Errorf("expected 1 contact")
	}
}

func TestHandleSend_Success(t *testing.T) {
	userID := uuid.New()
	convID := uuid.New()
	now := time.Now()

	encryptor, _ := crypto.NewEncryptor([]byte("test-key-32-bytes-long-for-test!"))

	mockStore := &store.MockStore{
		GetMemberFn: func(ctx context.Context, cID, uID uuid.UUID) (*store.Member, error) {
			return &store.Member{}, nil
		},
		GetConversationByIDFn: func(ctx context.Context, id uuid.UUID) (*store.Conversation, error) {
			return &store.Conversation{ID: convID, Type: "dm"}, nil
		},
		CreateMessageFn: func(ctx context.Context, cID, fromID uuid.UUID, content []byte, head json.RawMessage) (*store.Message, error) {
			return &store.Message{
				ID:             uuid.New(),
				ConversationID: cID,
				FromUserID:     fromID,
				Seq:            1,
				CreatedAt:      now,
			}, nil
		},
		GetConversationMembersFn: func(ctx context.Context, cID uuid.UUID) ([]uuid.UUID, error) {
			return []uuid.UUID{userID}, nil
		},
	}

	h := &Handlers{
		db:        mockStore,
		encryptor: encryptor,
	}
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		Send: &MsgClientSend{
			ConversationID: convID.String(),
			Content:        []byte("Hello world"),
		},
	}

	h.handleSend(sess, msg)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeAccepted {
		t.Errorf("expected code %d, got %d: %s", CodeAccepted, resp.Ctrl.Code, resp.Ctrl.Text)
	}
}

func TestHandleSend_NotMember(t *testing.T) {
	userID := uuid.New()
	convID := uuid.New()

	encryptor, _ := crypto.NewEncryptor([]byte("test-key-32-bytes-long-for-test!"))

	mockStore := &store.MockStore{
		GetMemberFn: func(ctx context.Context, cID, uID uuid.UUID) (*store.Member, error) {
			return nil, nil // Not a member
		},
	}

	h := &Handlers{
		db:        mockStore,
		encryptor: encryptor,
	}
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		Send: &MsgClientSend{
			ConversationID: convID.String(),
			Content:        []byte("Hello"),
		},
	}

	h.handleSend(sess, msg)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeForbidden {
		t.Errorf("expected code %d, got %d", CodeForbidden, resp.Ctrl.Code)
	}
}

func TestHandleRead_Success(t *testing.T) {
	userID := uuid.New()
	convID := uuid.New()
	updateCalled := false

	mockStore := &store.MockStore{
		UpdateReadSeqFn: func(ctx context.Context, cID, uID uuid.UUID, seq int) error {
			updateCalled = true
			if cID != convID || uID != userID || seq != 10 {
				t.Errorf("wrong params")
			}
			return nil
		},
		GetConversationMembersFn: func(ctx context.Context, cID uuid.UUID) ([]uuid.UUID, error) {
			return []uuid.UUID{userID}, nil
		},
	}

	h := testHandlers(mockStore)
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		Read: &MsgClientRead{
			ConversationID: convID.String(),
			Seq:            10,
		},
	}

	h.handleRead(sess, msg)

	if !updateCalled {
		t.Error("UpdateReadSeq not called")
	}

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeOK {
		t.Errorf("expected code %d, got %d", CodeOK, resp.Ctrl.Code)
	}
}

func TestHandleReact_Success(t *testing.T) {
	userID := uuid.New()
	convID := uuid.New()
	reactCalled := false

	mockStore := &store.MockStore{
		IsMemberFn: func(ctx context.Context, cID, uID uuid.UUID) (bool, error) {
			return true, nil
		},
		AddReactionFn: func(ctx context.Context, cID uuid.UUID, seq int, uID uuid.UUID, emoji string) error {
			reactCalled = true
			if emoji != "👍" {
				t.Errorf("expected emoji 👍, got %s", emoji)
			}
			return nil
		},
		GetConversationMembersFn: func(ctx context.Context, cID uuid.UUID) ([]uuid.UUID, error) {
			return []uuid.UUID{userID}, nil
		},
	}

	h := testHandlers(mockStore)
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		React: &MsgClientReact{
			ConversationID: convID.String(),
			Seq:            5,
			Emoji:          "👍",
		},
	}

	h.handleReact(sess, msg)

	if !reactCalled {
		t.Error("AddReaction not called")
	}

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeOK {
		t.Errorf("expected code %d, got %d", CodeOK, resp.Ctrl.Code)
	}
}

func TestHandleEdit_Success(t *testing.T) {
	userID := uuid.New()
	convID := uuid.New()
	now := time.Now().Add(-5 * time.Minute) // Within edit window

	encryptor, _ := crypto.NewEncryptor([]byte("test-key-32-bytes-long-for-test!"))

	mockStore := &store.MockStore{
		GetMessageBySeqFn: func(ctx context.Context, cID uuid.UUID, seq int) (*store.Message, error) {
			return &store.Message{
				ID:             uuid.New(),
				ConversationID: convID,
				FromUserID:     userID,
				Seq:            seq,
				CreatedAt:      now,
			}, nil
		},
		GetEditCountFn: func(ctx context.Context, cID uuid.UUID, seq int) (int, error) {
			return 0, nil
		},
		EditMessageFn: func(ctx context.Context, cID uuid.UUID, seq int, content []byte) error {
			return nil
		},
		GetConversationMembersFn: func(ctx context.Context, cID uuid.UUID) ([]uuid.UUID, error) {
			return []uuid.UUID{userID}, nil
		},
	}

	h := &Handlers{
		db:        mockStore,
		encryptor: encryptor,
	}
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		Edit: &MsgClientEdit{
			ConversationID: convID.String(),
			Seq:            1,
			Content:        []byte("edited content"),
		},
	}

	h.handleEdit(sess, msg)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeOK {
		t.Errorf("expected code %d, got %d: %s", CodeOK, resp.Ctrl.Code, resp.Ctrl.Text)
	}
}

func TestHandleEdit_NotYourMessage(t *testing.T) {
	userID := uuid.New()
	otherUserID := uuid.New()
	convID := uuid.New()
	now := time.Now().Add(-5 * time.Minute)

	encryptor, _ := crypto.NewEncryptor([]byte("test-key-32-bytes-long-for-test!"))

	mockStore := &store.MockStore{
		GetMessageBySeqFn: func(ctx context.Context, cID uuid.UUID, seq int) (*store.Message, error) {
			return &store.Message{
				FromUserID: otherUserID, // Different user
				CreatedAt:  now,
			}, nil
		},
	}

	h := &Handlers{
		db:        mockStore,
		encryptor: encryptor,
	}
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		Edit: &MsgClientEdit{
			ConversationID: convID.String(),
			Seq:            1,
			Content:        []byte("edited"),
		},
	}

	h.handleEdit(sess, msg)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeForbidden {
		t.Errorf("expected code %d, got %d", CodeForbidden, resp.Ctrl.Code)
	}
}

func TestHandleEdit_WindowExpired(t *testing.T) {
	userID := uuid.New()
	convID := uuid.New()
	now := time.Now().Add(-20 * time.Minute) // Outside 15 min window

	encryptor, _ := crypto.NewEncryptor([]byte("test-key-32-bytes-long-for-test!"))

	mockStore := &store.MockStore{
		GetMessageBySeqFn: func(ctx context.Context, cID uuid.UUID, seq int) (*store.Message, error) {
			return &store.Message{
				FromUserID: userID,
				CreatedAt:  now,
			}, nil
		},
	}

	h := &Handlers{
		db:        mockStore,
		encryptor: encryptor,
	}
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		Edit: &MsgClientEdit{
			ConversationID: convID.String(),
			Seq:            1,
			Content:        []byte("edited"),
		},
	}

	h.handleEdit(sess, msg)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeForbidden {
		t.Errorf("expected code %d, got %d", CodeForbidden, resp.Ctrl.Code)
	}
	if resp.Ctrl.Text != "edit window expired" {
		t.Errorf("unexpected error: %s", resp.Ctrl.Text)
	}
}

func TestHandleUnsend_Success(t *testing.T) {
	userID := uuid.New()
	convID := uuid.New()
	now := time.Now().Add(-2 * time.Minute) // Within 5 min window

	mockStore := &store.MockStore{
		GetMessageBySeqFn: func(ctx context.Context, cID uuid.UUID, seq int) (*store.Message, error) {
			return &store.Message{
				FromUserID: userID,
				CreatedAt:  now,
			}, nil
		},
		UnsendMessageFn: func(ctx context.Context, cID uuid.UUID, seq int) error {
			return nil
		},
		GetConversationMembersFn: func(ctx context.Context, cID uuid.UUID) ([]uuid.UUID, error) {
			return []uuid.UUID{userID}, nil
		},
	}

	h := testHandlers(mockStore)
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		Unsend: &MsgClientUnsend{
			ConversationID: convID.String(),
			Seq:            1,
		},
	}

	h.handleUnsend(sess, msg)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeOK {
		t.Errorf("expected code %d, got %d", CodeOK, resp.Ctrl.Code)
	}
}

func TestHandleDelete_ForMe(t *testing.T) {
	userID := uuid.New()
	convID := uuid.New()
	msgID := uuid.New()
	deleteCalled := false

	mockStore := &store.MockStore{
		GetMessageBySeqFn: func(ctx context.Context, cID uuid.UUID, seq int) (*store.Message, error) {
			return &store.Message{
				ID:         msgID,
				FromUserID: userID,
			}, nil
		},
		DeleteMessageForUserFn: func(ctx context.Context, mID, uID uuid.UUID) error {
			deleteCalled = true
			return nil
		},
	}

	h := testHandlers(mockStore)
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		Delete: &MsgClientDelete{
			ConversationID: convID.String(),
			Seq:            1,
			ForEveryone:    false,
		},
	}

	h.handleDelete(sess, msg)

	if !deleteCalled {
		t.Error("DeleteMessageForUser not called")
	}

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeOK {
		t.Errorf("expected code %d, got %d", CodeOK, resp.Ctrl.Code)
	}
}

func TestHandleTyping_Success(t *testing.T) {
	userID := uuid.New()
	convID := uuid.New()

	mockStore := &store.MockStore{
		IsMemberFn: func(ctx context.Context, cID, uID uuid.UUID) (bool, error) {
			return true, nil
		},
		GetConversationMembersFn: func(ctx context.Context, cID uuid.UUID) ([]uuid.UUID, error) {
			return []uuid.UUID{userID}, nil
		},
	}

	h := testHandlers(mockStore)
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		Typing: &MsgClientTyping{
			ConversationID: convID.String(),
		},
	}

	h.handleTyping(sess, msg)

	// Typing doesn't send response to sender
	if sess.MessageCount() != 0 {
		t.Errorf("expected no response for typing, got %d messages", sess.MessageCount())
	}
}

func TestHandleGet_UnknownWhat(t *testing.T) {
	userID := uuid.New()
	h := testHandlers(&store.MockStore{})
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		Get: &MsgClientGet{
			What: "unknown",
		},
	}

	h.handleGet(sess, msg)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeBadRequest {
		t.Errorf("expected code %d, got %d", CodeBadRequest, resp.Ctrl.Code)
	}
	if resp.Ctrl.Text != "unknown what" {
		t.Errorf("unexpected error: %s", resp.Ctrl.Text)
	}
}

func TestHandleRoom_Create(t *testing.T) {
	userID := uuid.New()
	convID := uuid.New()

	mockStore := &store.MockStore{
		CreateRoomFn: func(ctx context.Context, ownerID uuid.UUID, public json.RawMessage) (*store.Conversation, error) {
			return &store.Conversation{
				ID:      convID,
				Type:    "room",
				OwnerID: &ownerID,
				Public:  public,
			}, nil
		},
	}

	h := testHandlers(mockStore)
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		Room: &MsgClientRoom{
			Action: "create",
			Desc: &MsgSetDesc{
				Public: json.RawMessage(`{"name":"Test Room"}`),
			},
		},
	}

	h.handleRoom(sess, msg)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeCreated {
		t.Errorf("expected code %d, got %d: %s", CodeCreated, resp.Ctrl.Code, resp.Ctrl.Text)
	}
}

func TestHandleGetReceipts_Success(t *testing.T) {
	userID := uuid.New()
	convID := uuid.New()
	otherUserID := uuid.New()

	mockStore := &store.MockStore{
		IsMemberFn: func(ctx context.Context, cID, uID uuid.UUID) (bool, error) {
			return true, nil
		},
		GetReadReceiptsFn: func(ctx context.Context, cID uuid.UUID) ([]store.ReadReceipt, error) {
			return []store.ReadReceipt{
				{UserID: userID, ReadSeq: 10, RecvSeq: 15},
				{UserID: otherUserID, ReadSeq: 8, RecvSeq: 15},
			}, nil
		},
	}

	h := testHandlers(mockStore)
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		Get: &MsgClientGet{
			What:           "receipts",
			ConversationID: convID.String(),
		},
	}

	h.handleGetReceipts(context.Background(), sess, msg, msg.Get)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeOK {
		t.Errorf("expected code %d, got %d", CodeOK, resp.Ctrl.Code)
	}
}

func TestHandleGetMembers_Success(t *testing.T) {
	userID := uuid.New()
	convID := uuid.New()
	otherUserID := uuid.New()

	mockStore := &store.MockStore{
		IsMemberFn: func(ctx context.Context, cID, uID uuid.UUID) (bool, error) {
			return true, nil
		},
		GetConversationMembersFn: func(ctx context.Context, cID uuid.UUID) ([]uuid.UUID, error) {
			return []uuid.UUID{userID, otherUserID}, nil
		},
		GetUserByIDFn: func(ctx context.Context, id uuid.UUID) (*store.User, error) {
			return &store.User{
				ID:     id,
				Public: json.RawMessage(`{"fn":"User"}`),
			}, nil
		},
	}

	h := testHandlers(mockStore)
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		Get: &MsgClientGet{
			What:           "members",
			ConversationID: convID.String(),
		},
	}

	h.handleGetMembers(context.Background(), sess, msg, msg.Get)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeOK {
		t.Errorf("expected code %d, got %d", CodeOK, resp.Ctrl.Code)
	}
}

func TestHandleSend_Blocked(t *testing.T) {
	userID := uuid.New()
	convID := uuid.New()
	otherUserID := uuid.New()

	encryptor, _ := crypto.NewEncryptor([]byte("test-key-32-bytes-long-for-test!"))

	mockStore := &store.MockStore{
		GetMemberFn: func(ctx context.Context, cID, uID uuid.UUID) (*store.Member, error) {
			return &store.Member{}, nil
		},
		GetConversationByIDFn: func(ctx context.Context, id uuid.UUID) (*store.Conversation, error) {
			return &store.Conversation{ID: convID, Type: "dm"}, nil
		},
		GetDMOtherUserFn: func(ctx context.Context, cID, uID uuid.UUID) (*store.User, error) {
			return &store.User{ID: otherUserID}, nil
		},
		IsBlockedFn: func(ctx context.Context, cID, blockerID, blockedID uuid.UUID) (bool, error) {
			return true, nil // Blocked
		},
	}

	h := &Handlers{
		db:        mockStore,
		encryptor: encryptor,
	}
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		Send: &MsgClientSend{
			ConversationID: convID.String(),
			Content:        []byte("Hello"),
		},
	}

	h.handleSend(sess, msg)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeForbidden {
		t.Errorf("expected code %d, got %d", CodeForbidden, resp.Ctrl.Code)
	}
	if resp.Ctrl.Text != "blocked" {
		t.Errorf("unexpected error: %s", resp.Ctrl.Text)
	}
}

func TestHandleStartDM_DatabaseError(t *testing.T) {
	userID := uuid.New()
	otherUserID := uuid.New()

	mockStore := &store.MockStore{
		GetUserByIDFn: func(ctx context.Context, id uuid.UUID) (*store.User, error) {
			return nil, errors.New("database connection error")
		},
	}

	h := testHandlers(mockStore)
	sess := newTestSession(userID)

	msg := &ClientMessage{
		ID: "test-1",
		DM: &MsgClientDM{
			User: otherUserID.String(),
		},
	}

	h.handleDM(sess, msg)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Ctrl.Code != CodeInternalError {
		t.Errorf("expected code %d, got %d", CodeInternalError, resp.Ctrl.Code)
	}
}
