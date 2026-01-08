package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/scalecode-solutions/mvchat2/auth"
	"github.com/scalecode-solutions/mvchat2/crypto"
	"github.com/scalecode-solutions/mvchat2/email"
	"github.com/scalecode-solutions/mvchat2/store"
)

// Handlers holds dependencies for request handlers.
type Handlers struct {
	db        *store.DB
	auth      *auth.Auth
	hub       *Hub
	encryptor *crypto.Encryptor
	email     *email.Service
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(db *store.DB, a *auth.Auth, hub *Hub, enc *crypto.Encryptor, emailSvc *email.Service) *Handlers {
	return &Handlers{
		db:        db,
		auth:      a,
		hub:       hub,
		encryptor: enc,
		email:     emailSvc,
	}
}

// HandleLogin processes login requests.
func (h *Handlers) HandleLogin(s *Session, msg *ClientMessage) {
	login := msg.Login
	if login == nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "missing login data"))
		return
	}

	ctx := context.Background()

	switch login.Scheme {
	case "basic":
		h.handleBasicLogin(ctx, s, msg, login.Secret)
	case "token":
		h.handleTokenLogin(ctx, s, msg, login.Secret)
	default:
		s.Send(CtrlError(msg.ID, CodeBadRequest, "unknown auth scheme"))
	}
}

func (h *Handlers) handleBasicLogin(ctx context.Context, s *Session, msg *ClientMessage, secret string) {
	// Decode base64 secret (username:password)
	decoded, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid secret encoding"))
		return
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid secret format"))
		return
	}
	username, password := parts[0], parts[1]

	// Look up auth record
	authRec, err := h.db.GetAuthByUsername(ctx, username)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "database error"))
		return
	}
	if authRec == nil {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "invalid credentials"))
		return
	}

	// Verify password
	if !h.auth.VerifyPassword(password, authRec.Secret) {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "invalid credentials"))
		return
	}

	// Get user
	user, err := h.db.GetUserByID(ctx, authRec.UserID)
	if err != nil || user == nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "user not found"))
		return
	}

	// Generate token
	token, expiresAt, err := h.auth.GenerateToken(user.ID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to generate token"))
		return
	}

	// Authenticate session
	h.hub.AuthenticateSession(s, user.ID)

	// Update last seen
	h.db.UpdateUserLastSeen(ctx, user.ID, s.userAgent)

	s.Send(CtrlSuccess(msg.ID, CodeOK, map[string]any{
		"user":    user.ID.String(),
		"token":   token,
		"expires": expiresAt,
		"desc": map[string]any{
			"public": user.Public,
		},
	}))
}

func (h *Handlers) handleTokenLogin(ctx context.Context, s *Session, msg *ClientMessage, secret string) {
	// Validate token
	claims, err := h.auth.ValidateToken(secret)
	if err != nil {
		if err == auth.ErrTokenExpired {
			s.Send(CtrlError(msg.ID, CodeUnauthorized, "token expired"))
		} else {
			s.Send(CtrlError(msg.ID, CodeUnauthorized, "invalid token"))
		}
		return
	}

	// Get user
	user, err := h.db.GetUserByID(ctx, claims.UserID)
	if err != nil || user == nil {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "user not found"))
		return
	}

	// Authenticate session
	h.hub.AuthenticateSession(s, user.ID)

	// Update last seen
	h.db.UpdateUserLastSeen(ctx, user.ID, s.userAgent)

	// Generate new token (refresh)
	token, expiresAt, err := h.auth.GenerateToken(user.ID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to generate token"))
		return
	}

	s.Send(CtrlSuccess(msg.ID, CodeOK, map[string]any{
		"user":    user.ID.String(),
		"token":   token,
		"expires": expiresAt,
		"desc": map[string]any{
			"public": user.Public,
		},
	}))
}

// HandleAcc processes account creation/update requests.
func (h *Handlers) HandleAcc(s *Session, msg *ClientMessage) {
	acc := msg.Acc
	if acc == nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "missing account data"))
		return
	}

	ctx := context.Background()

	switch acc.User {
	case "new":
		h.handleCreateAccount(ctx, s, msg, acc)
	case "me":
		h.handleUpdateAccount(ctx, s, msg, acc)
	default:
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid user field"))
	}
}

func (h *Handlers) handleCreateAccount(ctx context.Context, s *Session, msg *ClientMessage, acc *MsgClientAcc) {
	if acc.Scheme != "basic" {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "only basic auth supported for account creation"))
		return
	}

	// Decode secret (username:password)
	decoded, err := base64.StdEncoding.DecodeString(acc.Secret)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid secret encoding"))
		return
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid secret format"))
		return
	}
	username, password := parts[0], parts[1]

	// Validate username and password
	if err := h.auth.ValidateUsername(username); err != nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "username too short"))
		return
	}
	if err := h.auth.ValidatePassword(password); err != nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "password too short"))
		return
	}

	// Check if username exists
	exists, err := h.db.UsernameExists(ctx, username)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "database error"))
		return
	}
	if exists {
		s.Send(CtrlError(msg.ID, CodeConflict, "username already taken"))
		return
	}

	// Get public data
	var public json.RawMessage
	if acc.Desc != nil && acc.Desc.Public != nil {
		public = acc.Desc.Public
	}

	// Create user
	userID, err := h.db.CreateUser(ctx, public)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to create user"))
		return
	}

	// Hash password
	hashedPassword, err := h.auth.HashPassword(password)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to hash password"))
		return
	}

	// Create auth record
	if err := h.db.CreateAuthRecord(ctx, userID, "basic", hashedPassword, &username); err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to create auth record"))
		return
	}

	// If invite code provided, redeem it (creates DMs and contacts with inviters)
	var connectedInviters []string
	if acc.InviteCode != "" {
		inviters, err := h.RedeemInviteCode(ctx, acc.InviteCode, userID)
		if err != nil {
			// Log but don't fail account creation
		}
		for _, inv := range inviters {
			connectedInviters = append(connectedInviters, inv.String())
		}
	}

	// If login requested, authenticate
	if acc.Login {
		h.hub.AuthenticateSession(s, userID)

		token, expiresAt, err := h.auth.GenerateToken(userID)
		if err != nil {
			s.Send(CtrlError(msg.ID, CodeInternalError, "failed to generate token"))
			return
		}

		params := map[string]any{
			"user":    userID.String(),
			"token":   token,
			"expires": expiresAt,
			"desc": map[string]any{
				"public": public,
			},
		}
		if len(connectedInviters) > 0 {
			params["inviters"] = connectedInviters
		}
		s.Send(CtrlSuccess(msg.ID, CodeCreated, params))
	} else {
		params := map[string]any{
			"user": userID.String(),
			"desc": map[string]any{
				"public": public,
			},
		}
		if len(connectedInviters) > 0 {
			params["inviters"] = connectedInviters
		}
		s.Send(CtrlSuccess(msg.ID, CodeCreated, params))
	}
}

func (h *Handlers) handleUpdateAccount(ctx context.Context, s *Session, msg *ClientMessage, acc *MsgClientAcc) {
	if !s.IsAuthenticated() {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
		return
	}

	// Update public data if provided
	if acc.Desc != nil && acc.Desc.Public != nil {
		if err := h.db.UpdateUserPublic(ctx, s.userID, acc.Desc.Public); err != nil {
			s.Send(CtrlError(msg.ID, CodeInternalError, "failed to update profile"))
			return
		}
	}

	// TODO: Handle password change if acc.Secret is provided

	s.Send(CtrlSuccess(msg.ID, CodeOK, nil))
}

// HandleSearch processes user search requests.
func (h *Handlers) HandleSearch(s *Session, msg *ClientMessage) {
	if !s.IsAuthenticated() {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
		return
	}

	search := msg.Search
	if search == nil || search.Query == "" {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "missing search query"))
		return
	}

	ctx := context.Background()
	users, err := h.db.SearchUsers(ctx, search.Query, search.Limit)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "search failed"))
		return
	}

	// Convert to response format
	results := make([]map[string]any, 0, len(users))
	for _, user := range users {
		// Don't return self
		if user.ID == s.userID {
			continue
		}
		results = append(results, map[string]any{
			"id":       user.ID.String(),
			"public":   user.Public,
			"online":   h.hub.IsOnline(user.ID),
			"lastSeen": user.LastSeen,
		})
	}

	s.Send(CtrlSuccess(msg.ID, CodeOK, map[string]any{
		"users": results,
	}))
}
