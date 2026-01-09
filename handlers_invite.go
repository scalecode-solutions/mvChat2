package main

import (
	"context"
	"encoding/json"
	"log"
	"net/mail"
	"strings"

	"github.com/google/uuid"
	"github.com/scalecode-solutions/mvchat2/crypto"
)

// HandleInvite processes invite code requests.
func (h *Handlers) HandleInvite(s *Session, msg *ClientMessage) {
	if !s.RequireAuth(msg.ID) {
		return
	}

	invite := msg.Invite
	if invite == nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "missing invite data"))
		return
	}

	ctx, cancel := handlerCtx()
	defer cancel()

	switch {
	case invite.Create != nil:
		h.handleCreateInvite(ctx, s, msg, invite.Create)
	case invite.List:
		h.handleListInvites(ctx, s, msg)
	case invite.Revoke != "":
		h.handleRevokeInvite(ctx, s, msg, invite.Revoke)
	case invite.Redeem != "":
		h.handleRedeemInviteExisting(ctx, s, msg, invite.Redeem)
	default:
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid invite request"))
	}
}

func (h *Handlers) handleCreateInvite(ctx context.Context, s *Session, msg *ClientMessage, create *MsgClientInviteCreate) {
	// Validate invitee email
	inviteeEmail := strings.TrimSpace(strings.ToLower(create.Email))
	if _, err := mail.ParseAddress(inviteeEmail); err != nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid email address"))
		return
	}

	// Get inviter's username (needed for cryptographic token)
	inviterUsername, err := h.db.GetUserUsername(ctx, s.userID)
	if err != nil || inviterUsername == "" {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to get user info"))
		return
	}

	// Optional name
	var name *string
	if create.Name != "" {
		n := strings.TrimSpace(create.Name)
		name = &n
	}

	// Generate cryptographic token (inviter username + invitee email)
	token, err := h.inviteTokens.Generate(inviterUsername, inviteeEmail)
	if err != nil {
		log.Printf("invite: failed to generate token: %v", err)
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to generate invite code"))
		return
	}

	// Generate short code from the token (before encryption)
	shortCode := h.inviteTokens.ShortCode(token)

	// Encrypt token for secure database storage
	encryptedToken, err := h.inviteTokens.EncryptForStorage(token)
	if err != nil {
		log.Printf("invite: failed to encrypt token: %v", err)
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to create invite"))
		return
	}

	// Create invite record in database with both code and encrypted token
	invite, err := h.db.CreateInviteCode(ctx, s.userID, shortCode, encryptedToken, inviteeEmail, name)
	if err != nil {
		log.Printf("invite: failed to create invite: %v", err)
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to create invite"))
		return
	}

	// Get inviter's display name for the email
	inviterName := "Someone"
	inviter, _ := h.db.GetUserByID(ctx, s.userID)
	if inviter != nil && inviter.Public != nil {
		var pub map[string]any
		if json.Unmarshal(inviter.Public, &pub) == nil {
			if fn, ok := pub["fn"].(string); ok && fn != "" {
				inviterName = fn
			}
		}
	}

	// Send invite email with the short code (user-friendly)
	toName := ""
	if name != nil {
		toName = *name
	}
	if h.email != nil && h.email.IsEnabled() {
		go func() {
			if err := h.email.SendInvite(inviteeEmail, toName, shortCode, inviterName); err != nil {
				log.Printf("invite: failed to send email to %s: %v", maskEmail(inviteeEmail), err)
			} else {
				log.Printf("invite: email sent to %s", maskEmail(inviteeEmail))
			}
		}()
	}

	s.Send(CtrlSuccess(msg.ID, CodeCreated, map[string]any{
		"id":        invite.ID.String(),
		"code":      shortCode,
		"email":     invite.Email,
		"name":      invite.InviteeName,
		"expiresAt": invite.ExpiresAt,
	}))
}

func (h *Handlers) handleListInvites(ctx context.Context, s *Session, msg *ClientMessage) {
	invites, err := h.db.GetUserInvites(ctx, s.userID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to get invites"))
		return
	}

	results := make([]map[string]any, 0, len(invites))
	for _, inv := range invites {
		item := map[string]any{
			"id":        inv.ID.String(),
			"email":     inv.Email,
			"status":    inv.Status,
			"createdAt": inv.CreatedAt,
			"expiresAt": inv.ExpiresAt,
		}
		if inv.InviteeName != nil {
			item["name"] = *inv.InviteeName
		}
		// Return the short code for pending invites (stored in DB)
		if inv.Status == "pending" && inv.Code != "" {
			item["code"] = inv.Code
		}
		if inv.UsedAt != nil {
			item["usedAt"] = inv.UsedAt
		}
		if inv.UsedBy != nil {
			item["usedBy"] = inv.UsedBy.String()
		}
		results = append(results, item)
	}

	s.Send(CtrlSuccess(msg.ID, CodeOK, map[string]any{
		"invites": results,
	}))
}

func (h *Handlers) handleRevokeInvite(ctx context.Context, s *Session, msg *ClientMessage, inviteIDStr string) {
	inviteID, err := uuid.Parse(inviteIDStr)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid invite id"))
		return
	}

	err = h.db.RevokeInvite(ctx, inviteID, s.userID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeNotFound, "invite not found or already used"))
		return
	}

	s.Send(CtrlSuccess(msg.ID, CodeOK, nil))
}

// handleRedeemInviteExisting allows an existing logged-in user to redeem an invite code.
// This connects them with the inviter without creating a new account.
// The code parameter is the short 10-char code that users share.
func (h *Handlers) handleRedeemInviteExisting(ctx context.Context, s *Session, msg *ClientMessage, code string) {
	// Look up the invite by short code
	invite, err := h.db.GetInviteByCode(ctx, code)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to verify invite"))
		return
	}
	if invite == nil {
		s.Send(CtrlError(msg.ID, CodeNotFound, "invite not found or already used"))
		return
	}

	// Decrypt the token from database storage
	token, err := h.inviteTokens.DecryptFromStorage(invite.Token)
	if err != nil {
		log.Printf("invite: failed to decrypt token: %v", err)
		s.Send(CtrlError(msg.ID, CodeNotFound, "invalid invite code"))
		return
	}

	// Verify the decrypted cryptographic token
	_, err = h.inviteTokens.Verify(token)
	if err != nil {
		if err == crypto.ErrInviteTokenExpired {
			s.Send(CtrlError(msg.ID, CodeNotFound, "invite code expired"))
		} else {
			s.Send(CtrlError(msg.ID, CodeNotFound, "invalid invite code"))
		}
		return
	}

	// Mark the invite as used
	usedInvite, err := h.db.UseInvite(ctx, invite.ID, s.userID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to redeem invite"))
		return
	}
	if usedInvite == nil {
		s.Send(CtrlError(msg.ID, CodeNotFound, "invite already used or expired"))
		return
	}

	// Create DM and contact with the inviter
	conv, _, err := h.db.CreateDM(ctx, invite.InviterID, s.userID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to create conversation"))
		return
	}

	// Add as contacts
	if err := h.db.AddContact(ctx, invite.InviterID, s.UserID(), "invite", &invite.ID); err != nil {
		log.Printf("invite: failed to add contact between %s and %s: %v", shortID(invite.InviterID), shortID(s.UserID()), err)
	}

	// Get inviter info
	inviter, _ := h.db.GetUserByID(ctx, invite.InviterID)
	var inviterPublic any
	if inviter != nil {
		inviterPublic = inviter.Public
	}

	s.Send(CtrlSuccess(msg.ID, CodeOK, map[string]any{
		"inviter":       invite.InviterID.String(),
		"inviterPublic": inviterPublic,
		"conv":          conv.ID.String(),
	}))
}

// RedeemInviteCode processes invite code redemption (signup via invite).
// This is called during account creation when an invite code is provided.
// The code parameter is the short 10-char code that users share.
// It also auto-redeems any other pending invites to the same email.
func (h *Handlers) RedeemInviteCode(ctx context.Context, code string, newUserID uuid.UUID, newUserEmail string) ([]uuid.UUID, error) {
	// Look up the invite by short code
	invite, err := h.db.GetInviteByCode(ctx, code)
	if err != nil || invite == nil {
		return nil, nil // Not found, but not a hard error
	}

	// Decrypt the token from database storage
	token, err := h.inviteTokens.DecryptFromStorage(invite.Token)
	if err != nil {
		return nil, nil // Decryption failed
	}

	// Verify the decrypted cryptographic token
	_, err = h.inviteTokens.Verify(token)
	if err != nil {
		return nil, nil // Invalid or expired token
	}

	// Mark as used
	_, err = h.db.UseInvite(ctx, invite.ID, newUserID)
	if err != nil {
		return nil, err
	}

	var connectedUsers []uuid.UUID

	// Create DM and contact with the inviter
	_, _, err = h.db.CreateDM(ctx, invite.InviterID, newUserID)
	if err == nil {
		if err := h.db.AddContact(ctx, invite.InviterID, newUserID, "invite", &invite.ID); err != nil {
			log.Printf("invite: failed to add contact between %s and %s: %v", shortID(invite.InviterID), shortID(newUserID), err)
		}
		connectedUsers = append(connectedUsers, invite.InviterID)
	}

	// Check for other pending invites to the same email
	otherInvites, err := h.db.GetPendingInvitesByEmail(ctx, newUserEmail)
	if err != nil {
		return connectedUsers, nil // Don't fail, just return what we have
	}

	for _, other := range otherInvites {
		// Skip the one we just used
		if other.ID == invite.ID {
			continue
		}

		// Mark as used
		_, err := h.db.UseInvite(ctx, other.ID, newUserID)
		if err != nil {
			continue
		}

		// Create DM and contact with this inviter too
		_, _, err = h.db.CreateDM(ctx, other.InviterID, newUserID)
		if err == nil {
			if err := h.db.AddContact(ctx, other.InviterID, newUserID, "invite", &other.ID); err != nil {
				log.Printf("invite: failed to add contact between %s and %s: %v", shortID(other.InviterID), shortID(newUserID), err)
			}
			connectedUsers = append(connectedUsers, other.InviterID)
		}
	}

	return connectedUsers, nil
}
