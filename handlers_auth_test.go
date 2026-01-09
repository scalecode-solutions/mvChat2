package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scalecode-solutions/mvchat2/auth"
	"github.com/scalecode-solutions/mvchat2/config"
	"github.com/scalecode-solutions/mvchat2/store"
)

// testAuthConfig returns a test auth configuration.
func testAuthConfig() auth.Config {
	return auth.Config{
		TokenKey:          []byte("test-secret-key-for-testing-only-32b"),
		TokenExpiry:       24 * time.Hour,
		MinUsernameLength: 3,
		MinPasswordLength: 6,
	}
}

// testHandlersWithAuth creates handlers with mock store and real auth for testing.
func testHandlersWithAuth(mockStore *store.MockStore) *Handlers {
	authCfg := testAuthConfig()
	a := auth.New(authCfg)
	return &Handlers{
		db:   mockStore,
		auth: a,
		cfg:  &config.Config{},
	}
}

func TestHandleBasicLogin_Success(t *testing.T) {
	userID := uuid.New()
	username := "testuser"
	password := "testpassword123"

	// Create a single auth instance for both hashing and verifying
	authCfg := testAuthConfig()
	a := auth.New(authCfg)
	hashedPassword, _ := a.HashPassword(password)

	mockStore := &store.MockStore{
		GetAuthByUsernameFn: func(ctx context.Context, uname string) (*store.AuthRecord, error) {
			if uname == username {
				return &store.AuthRecord{
					ID:     uuid.New(),
					UserID: userID,
					Scheme: "basic",
					Secret: hashedPassword,
					Uname:  &username,
				}, nil
			}
			return nil, nil
		},
		GetUserByIDFn: func(ctx context.Context, id uuid.UUID) (*store.User, error) {
			if id == userID {
				return &store.User{
					ID:            userID,
					State:         "ok",
					Public:        json.RawMessage(`{"fn":"Test User"}`),
					EmailVerified: true,
				}, nil
			}
			return nil, nil
		},
		UpdateUserLastSeenFn: func(ctx context.Context, uid uuid.UUID, userAgent string) error {
			return nil
		},
	}

	// Use the same auth instance that hashed the password
	h := &Handlers{
		db:   mockStore,
		auth: a,
		cfg:  &config.Config{},
	}
	sess := newTestSession(uuid.Nil) // Not authenticated yet

	// Encode credentials
	secret := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))

	msg := &ClientMessage{
		ID: "test-1",
		Login: &MsgClientLogin{
			Scheme: "basic",
			Secret: secret,
		},
	}

	h.handleBasicLogin(context.Background(), sess, msg, secret)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response message")
	}
	if resp.Ctrl == nil {
		t.Fatal("expected ctrl message")
	}
	if resp.Ctrl.Code != CodeOK {
		t.Errorf("expected code %d, got %d: %s", CodeOK, resp.Ctrl.Code, resp.Ctrl.Text)
	}
	if resp.Ctrl.Params["user"] != userID.String() {
		t.Errorf("expected user %s, got %v", userID.String(), resp.Ctrl.Params["user"])
	}
	if resp.Ctrl.Params["token"] == nil {
		t.Error("expected token in response")
	}
}

func TestHandleBasicLogin_InvalidCredentials(t *testing.T) {
	mockStore := &store.MockStore{
		GetAuthByUsernameFn: func(ctx context.Context, uname string) (*store.AuthRecord, error) {
			return nil, nil // User not found
		},
	}

	h := testHandlersWithAuth(mockStore)
	sess := newTestSession(uuid.Nil)

	secret := base64.StdEncoding.EncodeToString([]byte("baduser:badpassword"))

	msg := &ClientMessage{
		ID: "test-1",
		Login: &MsgClientLogin{
			Scheme: "basic",
			Secret: secret,
		},
	}

	h.handleBasicLogin(context.Background(), sess, msg, secret)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response message")
	}
	if resp.Ctrl == nil {
		t.Fatal("expected ctrl message")
	}
	if resp.Ctrl.Code != CodeUnauthorized {
		t.Errorf("expected code %d, got %d", CodeUnauthorized, resp.Ctrl.Code)
	}
}

func TestHandleBasicLogin_WrongPassword(t *testing.T) {
	userID := uuid.New()
	username := "testuser"

	// Hash a different password
	authCfg := testAuthConfig()
	a := auth.New(authCfg)
	hashedPassword, _ := a.HashPassword("correctpassword")

	mockStore := &store.MockStore{
		GetAuthByUsernameFn: func(ctx context.Context, uname string) (*store.AuthRecord, error) {
			if uname == username {
				return &store.AuthRecord{
					ID:     uuid.New(),
					UserID: userID,
					Scheme: "basic",
					Secret: hashedPassword,
					Uname:  &username,
				}, nil
			}
			return nil, nil
		},
	}

	h := testHandlersWithAuth(mockStore)
	sess := newTestSession(uuid.Nil)

	// Try with wrong password
	secret := base64.StdEncoding.EncodeToString([]byte(username + ":wrongpassword"))

	msg := &ClientMessage{
		ID: "test-1",
		Login: &MsgClientLogin{
			Scheme: "basic",
			Secret: secret,
		},
	}

	h.handleBasicLogin(context.Background(), sess, msg, secret)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response message")
	}
	if resp.Ctrl == nil {
		t.Fatal("expected ctrl message")
	}
	if resp.Ctrl.Code != CodeUnauthorized {
		t.Errorf("expected code %d, got %d", CodeUnauthorized, resp.Ctrl.Code)
	}
}

func TestHandleBasicLogin_InvalidSecretFormat(t *testing.T) {
	mockStore := &store.MockStore{}

	h := testHandlersWithAuth(mockStore)
	sess := newTestSession(uuid.Nil)

	// Invalid base64
	msg := &ClientMessage{
		ID: "test-1",
		Login: &MsgClientLogin{
			Scheme: "basic",
			Secret: "not-valid-base64!!!",
		},
	}

	h.handleBasicLogin(context.Background(), sess, msg, "not-valid-base64!!!")

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

func TestHandleCreateAccount_Success(t *testing.T) {
	username := "newuser"
	password := "newpassword123"
	createUserCalled := false
	createAuthCalled := false

	mockStore := &store.MockStore{
		UsernameExistsFn: func(ctx context.Context, uname string) (bool, error) {
			return false, nil // Username available
		},
		CreateUserWithOptionsFn: func(ctx context.Context, public json.RawMessage, mustChangePassword bool, email *string, emailVerified bool) (uuid.UUID, error) {
			createUserCalled = true
			return uuid.New(), nil
		},
		CreateAuthRecordFn: func(ctx context.Context, userID uuid.UUID, scheme, secret string, uname *string) error {
			createAuthCalled = true
			if scheme != "basic" {
				t.Errorf("expected scheme 'basic', got %s", scheme)
			}
			if uname == nil || *uname != username {
				t.Errorf("expected username %s", username)
			}
			return nil
		},
		GetInviteByCodeFn: func(ctx context.Context, code string) (*store.InviteCode, error) {
			return nil, nil
		},
	}

	h := testHandlersWithAuth(mockStore)
	sess := newTestSession(uuid.Nil)

	secret := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))

	msg := &ClientMessage{
		ID: "test-1",
		Acc: &MsgClientAcc{
			User:   "new",
			Scheme: "basic",
			Secret: secret,
		},
	}

	h.handleCreateAccount(context.Background(), sess, msg, msg.Acc)

	if !createUserCalled {
		t.Error("expected CreateUserWithOptions to be called")
	}
	if !createAuthCalled {
		t.Error("expected CreateAuthRecord to be called")
	}

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response message")
	}
	if resp.Ctrl == nil {
		t.Fatal("expected ctrl message")
	}
	if resp.Ctrl.Code != CodeCreated {
		t.Errorf("expected code %d, got %d: %s", CodeCreated, resp.Ctrl.Code, resp.Ctrl.Text)
	}
}

func TestHandleCreateAccount_UsernameTaken(t *testing.T) {
	mockStore := &store.MockStore{
		UsernameExistsFn: func(ctx context.Context, uname string) (bool, error) {
			return true, nil // Username taken
		},
	}

	h := testHandlersWithAuth(mockStore)
	sess := newTestSession(uuid.Nil)

	secret := base64.StdEncoding.EncodeToString([]byte("existinguser:password123"))

	msg := &ClientMessage{
		ID: "test-1",
		Acc: &MsgClientAcc{
			User:   "new",
			Scheme: "basic",
			Secret: secret,
		},
	}

	h.handleCreateAccount(context.Background(), sess, msg, msg.Acc)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response message")
	}
	if resp.Ctrl == nil {
		t.Fatal("expected ctrl message")
	}
	if resp.Ctrl.Code != CodeConflict {
		t.Errorf("expected code %d, got %d", CodeConflict, resp.Ctrl.Code)
	}
}

func TestHandleCreateAccount_PasswordTooShort(t *testing.T) {
	mockStore := &store.MockStore{}

	h := testHandlersWithAuth(mockStore)
	sess := newTestSession(uuid.Nil)

	// Password "123" is too short (min 6)
	secret := base64.StdEncoding.EncodeToString([]byte("newuser:123"))

	msg := &ClientMessage{
		ID: "test-1",
		Acc: &MsgClientAcc{
			User:   "new",
			Scheme: "basic",
			Secret: secret,
		},
	}

	h.handleCreateAccount(context.Background(), sess, msg, msg.Acc)

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

func TestHandleUpdateAccount_PasswordChange(t *testing.T) {
	userID := uuid.New()
	oldPassword := "oldpassword123"
	newPassword := "newpassword456"

	authCfg := testAuthConfig()
	a := auth.New(authCfg)
	hashedOldPassword, _ := a.HashPassword(oldPassword)

	updatePasswordCalled := false
	clearFlagCalled := false

	mockStore := &store.MockStore{
		GetAuthByUserIDFn: func(ctx context.Context, uid uuid.UUID) (*store.AuthRecord, error) {
			if uid == userID {
				return &store.AuthRecord{
					ID:     uuid.New(),
					UserID: userID,
					Scheme: "basic",
					Secret: hashedOldPassword,
				}, nil
			}
			return nil, nil
		},
		UpdatePasswordFn: func(ctx context.Context, uid uuid.UUID, hashedPassword string) error {
			updatePasswordCalled = true
			return nil
		},
		ClearMustChangePasswordFn: func(ctx context.Context, uid uuid.UUID) error {
			clearFlagCalled = true
			return nil
		},
	}

	h := &Handlers{
		db:   mockStore,
		auth: a,
		cfg:  &config.Config{},
	}
	sess := newTestSession(userID) // Already authenticated

	secret := base64.StdEncoding.EncodeToString([]byte(oldPassword + ":" + newPassword))

	msg := &ClientMessage{
		ID: "test-1",
		Acc: &MsgClientAcc{
			User:   "me",
			Secret: secret,
		},
	}

	h.handleUpdateAccount(context.Background(), sess, msg, msg.Acc)

	if !updatePasswordCalled {
		t.Error("expected UpdatePassword to be called")
	}
	if !clearFlagCalled {
		t.Error("expected ClearMustChangePassword to be called")
	}

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response message")
	}
	if resp.Ctrl == nil {
		t.Fatal("expected ctrl message")
	}
	if resp.Ctrl.Code != CodeOK {
		t.Errorf("expected code %d, got %d: %s", CodeOK, resp.Ctrl.Code, resp.Ctrl.Text)
	}
}

func TestHandleUpdateAccount_WrongCurrentPassword(t *testing.T) {
	userID := uuid.New()

	authCfg := testAuthConfig()
	a := auth.New(authCfg)
	hashedPassword, _ := a.HashPassword("actualpassword")

	mockStore := &store.MockStore{
		GetAuthByUserIDFn: func(ctx context.Context, uid uuid.UUID) (*store.AuthRecord, error) {
			return &store.AuthRecord{
				ID:     uuid.New(),
				UserID: userID,
				Scheme: "basic",
				Secret: hashedPassword,
			}, nil
		},
	}

	h := &Handlers{
		db:   mockStore,
		auth: a,
		cfg:  &config.Config{},
	}
	sess := newTestSession(userID)

	// Wrong current password
	secret := base64.StdEncoding.EncodeToString([]byte("wrongpassword:newpassword123"))

	msg := &ClientMessage{
		ID: "test-1",
		Acc: &MsgClientAcc{
			User:   "me",
			Secret: secret,
		},
	}

	h.handleUpdateAccount(context.Background(), sess, msg, msg.Acc)

	resp := sess.LastMessage()
	if resp == nil {
		t.Fatal("expected response message")
	}
	if resp.Ctrl == nil {
		t.Fatal("expected ctrl message")
	}
	if resp.Ctrl.Code != CodeForbidden {
		t.Errorf("expected code %d, got %d", CodeForbidden, resp.Ctrl.Code)
	}
}
