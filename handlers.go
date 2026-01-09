package main

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/scalecode-solutions/mvchat2/auth"
	"github.com/scalecode-solutions/mvchat2/config"
	"github.com/scalecode-solutions/mvchat2/crypto"
	"github.com/scalecode-solutions/mvchat2/email"
	"github.com/scalecode-solutions/mvchat2/store"
)

// Default timeout for handler database operations.
const handlerTimeout = 30 * time.Second

// handlerCtx creates a context with the standard handler timeout.
func handlerCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), handlerTimeout)
}

// parseUUID parses a UUID string and sends an error response if invalid.
// Returns the parsed UUID and true on success, or uuid.Nil and false on failure.
func parseUUID(s *Session, msgID, uuidStr, field string) (uuid.UUID, bool) {
	id, err := uuid.Parse(uuidStr)
	if err != nil {
		s.Send(CtrlError(msgID, CodeBadRequest, "invalid "+field))
		return uuid.Nil, false
	}
	return id, true
}

// decodeCredentials decodes a base64-encoded "username:password" string.
// Returns the username, password, and true on success.
func decodeCredentials(s *Session, msgID, secret string) (username, password string, ok bool) {
	decoded, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		s.Send(CtrlError(msgID, CodeBadRequest, "invalid secret encoding"))
		return "", "", false
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		s.Send(CtrlError(msgID, CodeBadRequest, "invalid secret format"))
		return "", "", false
	}
	return parts[0], parts[1], true
}

// requireMember checks if the user is a member of the conversation.
// Returns true if member, false otherwise (after sending error response).
func (h *Handlers) requireMember(ctx context.Context, s *Session, msgID string, convID uuid.UUID) bool {
	isMember, err := h.db.IsMember(ctx, convID, s.UserID())
	if err != nil {
		s.Send(CtrlError(msgID, CodeInternalError, "database error"))
		return false
	}
	if !isMember {
		s.Send(CtrlError(msgID, CodeForbidden, "not a member"))
		return false
	}
	return true
}

// broadcastToConv sends an Info message to all members of a conversation.
func (h *Handlers) broadcastToConv(ctx context.Context, convID uuid.UUID, info *MsgServerInfo, skipSession string) {
	memberIDs, _ := h.db.GetConversationMembers(ctx, convID)
	h.hub.SendToUsers(memberIDs, &ServerMessage{Info: info}, skipSession)
}

// Handlers holds dependencies for request handlers.
type Handlers struct {
	db           store.Store
	auth         *auth.Auth
	hub          *Hub
	encryptor    *crypto.Encryptor
	email        *email.Service
	inviteTokens *crypto.InviteTokenGenerator
	cfg          *config.Config
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(db store.Store, a *auth.Auth, hub *Hub, enc *crypto.Encryptor, emailSvc *email.Service, inviteTokens *crypto.InviteTokenGenerator, cfg *config.Config) *Handlers {
	return &Handlers{
		db:           db,
		auth:         a,
		hub:          hub,
		encryptor:    enc,
		email:        emailSvc,
		inviteTokens: inviteTokens,
		cfg:          cfg,
	}
}

// HandleLogin processes login requests.
func (h *Handlers) HandleLogin(s *Session, msg *ClientMessage) {
	login := msg.Login
	if login == nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "missing login data"))
		return
	}

	ctx, cancel := handlerCtx()
	defer cancel()

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
	username, password, ok := decodeCredentials(s, msg.ID, secret)
	if !ok {
		return
	}

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
	h.db.UpdateUserLastSeen(ctx, user.ID, s.UserAgent())

	params := map[string]any{
		"user":          user.ID.String(),
		"token":         token,
		"expires":       expiresAt,
		"emailVerified": user.EmailVerified,
		"desc": map[string]any{
			"public": user.Public,
		},
	}
	if user.MustChangePassword {
		params["mustChangePassword"] = true
	}
	s.Send(CtrlSuccess(msg.ID, CodeOK, params))
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
	h.db.UpdateUserLastSeen(ctx, user.ID, s.UserAgent())

	// Generate new token (refresh)
	token, expiresAt, err := h.auth.GenerateToken(user.ID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to generate token"))
		return
	}

	params := map[string]any{
		"user":          user.ID.String(),
		"token":         token,
		"expires":       expiresAt,
		"emailVerified": user.EmailVerified,
		"desc": map[string]any{
			"public": user.Public,
		},
	}
	if user.MustChangePassword {
		params["mustChangePassword"] = true
	}
	s.Send(CtrlSuccess(msg.ID, CodeOK, params))
}

// HandleAcc processes account creation/update requests.
func (h *Handlers) HandleAcc(s *Session, msg *ClientMessage) {
	acc := msg.Acc
	if acc == nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "missing account data"))
		return
	}

	ctx, cancel := handlerCtx()
	defer cancel()

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

	username, password, ok := decodeCredentials(s, msg.ID, acc.Secret)
	if !ok {
		return
	}

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

	// Detect if user is signing up with invite code as password (temporary password)
	// In this case, they must change their password after login
	mustChangePassword := acc.InviteCode != "" && password == acc.InviteCode

	// If invite code provided, look up the invite to get the email
	var userEmail *string
	if acc.InviteCode != "" {
		invite, _ := h.db.GetInviteByCode(ctx, acc.InviteCode)
		if invite != nil {
			userEmail = &invite.Email
		}
	}

	// Determine email verification status based on config
	// For DV safety: if verification is DISABLED (default), mark as verified
	// If verification is ENABLED, mark as unverified (needs email confirmation)
	emailVerified := !h.cfg.Email.Verification.Enabled

	// Create user with email from invite (if available)
	userID, err := h.db.CreateUserWithOptions(ctx, public, mustChangePassword, userEmail, emailVerified)
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

	// If verification is enabled and user has an email, send verification email
	if h.cfg.Email.Verification.Enabled && userEmail != nil {
		token, err := h.generateVerificationToken()
		if err == nil {
			expiryHours := h.cfg.Email.Verification.TokenExpiryHours
			if expiryHours <= 0 {
				expiryHours = 24
			}
			expiresAt := time.Now().UTC().Add(time.Duration(expiryHours) * time.Hour)

			if err := h.db.SetEmailVerificationToken(ctx, userID, token, expiresAt); err == nil {
				// Send verification email (don't fail account creation if email fails)
				_ = h.email.SendVerification(*userEmail, token)
			}
		}
	}

	// If invite code provided, redeem it (creates DMs and contacts with inviters)
	var connectedInviters []string
	if acc.InviteCode != "" && userEmail != nil {
		inviters, err := h.RedeemInviteCode(ctx, acc.InviteCode, userID, *userEmail)
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
			"user":          userID.String(),
			"token":         token,
			"expires":       expiresAt,
			"emailVerified": emailVerified,
			"desc": map[string]any{
				"public": public,
			},
		}
		if len(connectedInviters) > 0 {
			params["inviters"] = connectedInviters
		}
		if mustChangePassword {
			params["mustChangePassword"] = true
		}
		s.Send(CtrlSuccess(msg.ID, CodeCreated, params))
	} else {
		params := map[string]any{
			"user":          userID.String(),
			"emailVerified": emailVerified,
			"desc": map[string]any{
				"public": public,
			},
		}
		if len(connectedInviters) > 0 {
			params["inviters"] = connectedInviters
		}
		if mustChangePassword {
			params["mustChangePassword"] = true
		}
		s.Send(CtrlSuccess(msg.ID, CodeCreated, params))
	}
}

func (h *Handlers) handleUpdateAccount(ctx context.Context, s *Session, msg *ClientMessage, acc *MsgClientAcc) {
	if !s.RequireAuth(msg.ID) {
		return
	}

	// Update public data if provided
	if acc.Desc != nil && acc.Desc.Public != nil {
		if err := h.db.UpdateUserPublic(ctx, s.UserID(), acc.Desc.Public); err != nil {
			s.Send(CtrlError(msg.ID, CodeInternalError, "failed to update profile"))
			return
		}
	}

	// Handle password change if secret is provided
	if acc.Secret != "" {
		oldPassword, newPassword, ok := decodeCredentials(s, msg.ID, acc.Secret)
		if !ok {
			return
		}

		// Validate new password
		if err := h.auth.ValidatePassword(newPassword); err != nil {
			s.Send(CtrlError(msg.ID, CodeBadRequest, "new password too short"))
			return
		}

		// Get current auth record
		authRecord, err := h.db.GetAuthByUserID(ctx, s.UserID())
		if err != nil {
			s.Send(CtrlError(msg.ID, CodeInternalError, "database error"))
			return
		}
		if authRecord == nil {
			s.Send(CtrlError(msg.ID, CodeNotFound, "auth record not found"))
			return
		}

		// Verify old password
		if !h.auth.VerifyPassword(oldPassword, authRecord.Secret) {
			s.Send(CtrlError(msg.ID, CodeForbidden, "incorrect current password"))
			return
		}

		// Hash new password
		hashedPassword, err := h.auth.HashPassword(newPassword)
		if err != nil {
			s.Send(CtrlError(msg.ID, CodeInternalError, "failed to hash password"))
			return
		}

		// Update password
		if err := h.db.UpdatePassword(ctx, s.UserID(), hashedPassword); err != nil {
			s.Send(CtrlError(msg.ID, CodeInternalError, "failed to update password"))
			return
		}

		// Clear the must_change_password flag since user has now changed their password
		if err := h.db.ClearMustChangePassword(ctx, s.UserID()); err != nil {
			// Log but don't fail - password was successfully changed
		}
	}

	// Update email if provided
	if acc.Email != nil {
		if err := h.db.UpdateUserEmail(ctx, s.UserID(), acc.Email); err != nil {
			s.Send(CtrlError(msg.ID, CodeInternalError, "failed to update email"))
			return
		}
	}

	s.Send(CtrlSuccess(msg.ID, CodeOK, nil))
}

// HandleSearch processes user search requests.
func (h *Handlers) HandleSearch(s *Session, msg *ClientMessage) {
	if !s.RequireAuth(msg.ID) {
		return
	}

	search := msg.Search
	if search == nil || search.Query == "" {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "missing search query"))
		return
	}

	ctx, cancel := handlerCtx()
	defer cancel()
	users, err := h.db.SearchUsers(ctx, search.Query, search.Limit)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "search failed"))
		return
	}

	// Convert to response format
	results := make([]map[string]any, 0, len(users))
	for _, user := range users {
		// Don't return self
		if user.ID == s.UserID() {
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

// generateVerificationToken generates a cryptographically secure random token
// for email verification. Returns a URL-safe base64 encoded string.
func (h *Handlers) generateVerificationToken() (string, error) {
	b := make([]byte, 32)
	if _, err := cryptorand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
