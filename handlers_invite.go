package main

import (
	"context"
	"encoding/json"
	"log"
	"net/mail"
	"strings"

	"github.com/google/uuid"
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

	ctx := context.Background()

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
	// Validate email
	email := strings.TrimSpace(strings.ToLower(create.Email))
	if _, err := mail.ParseAddress(email); err != nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid email address"))
		return
	}

	// Optional name
	var name *string
	if create.Name != "" {
		n := strings.TrimSpace(create.Name)
		name = &n
	}

	// Create invite code
	invite, err := h.db.CreateInviteCode(ctx, s.userID, email, name)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to create invite"))
		return
	}

	// Get inviter's display name for the email
	inviterName := "Someone"
	inviter, _ := h.db.GetUserByID(ctx, s.userID)
	if inviter != nil && inviter.Public != nil {
		// Try to extract display name from public data
		var pub map[string]any
		if json.Unmarshal(inviter.Public, &pub) == nil {
			if fn, ok := pub["fn"].(string); ok && fn != "" {
				inviterName = fn
			}
		}
	}

	// Send invite email
	toName := ""
	if name != nil {
		toName = *name
	}
	if h.email != nil && h.email.IsEnabled() {
		go func() {
			if err := h.email.SendInvite(invite.Email, toName, invite.Code, inviterName); err != nil {
				log.Printf("invite: failed to send email to %s: %v", invite.Email, err)
			} else {
				log.Printf("invite: email sent to %s", invite.Email)
			}
		}()
	}

	s.Send(CtrlSuccess(msg.ID, CodeCreated, map[string]any{
		"id":        invite.ID.String(),
		"code":      invite.Code,
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
		if inv.Status == "pending" {
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
func (h *Handlers) handleRedeemInviteExisting(ctx context.Context, s *Session, msg *ClientMessage, code string) {
	// Use the invite code
	invite, err := h.db.UseInviteCode(ctx, code, s.userID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to redeem invite"))
		return
	}
	if invite == nil {
		s.Send(CtrlError(msg.ID, CodeNotFound, "invite code not found or expired"))
		return
	}

	// Create DM and contact with the inviter
	conv, _, err := h.db.CreateDM(ctx, invite.InviterID, s.userID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to create conversation"))
		return
	}

	// Add as contacts
	if err := h.db.AddContact(ctx, invite.InviterID, s.userID, "invite", &invite.ID); err != nil {
		log.Printf("invite: failed to add contact between %s and %s: %v", invite.InviterID, s.userID, err)
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
// It also auto-redeems any other pending invites to the same email.
func (h *Handlers) RedeemInviteCode(ctx context.Context, code string, newUserID uuid.UUID) ([]uuid.UUID, error) {
	// Use the invite code
	invite, err := h.db.UseInviteCode(ctx, code, newUserID)
	if err != nil {
		return nil, err
	}
	if invite == nil {
		return nil, nil // Code not found or expired
	}

	var connectedUsers []uuid.UUID

	// Create DM and contact with the inviter
	_, _, err = h.db.CreateDM(ctx, invite.InviterID, newUserID)
	if err == nil {
		if err := h.db.AddContact(ctx, invite.InviterID, newUserID, "invite", &invite.ID); err != nil {
			log.Printf("invite: failed to add contact between %s and %s: %v", invite.InviterID, newUserID, err)
		}
		connectedUsers = append(connectedUsers, invite.InviterID)
	}

	// Check for other pending invites to the same email
	otherInvites, err := h.db.GetPendingInvitesByEmail(ctx, invite.Email)
	if err != nil {
		return connectedUsers, nil // Don't fail, just return what we have
	}

	for _, other := range otherInvites {
		// Skip the one we just used
		if other.ID == invite.ID {
			continue
		}

		// Mark as used
		_, err := h.db.UseInviteCode(ctx, other.Code, newUserID)
		if err != nil {
			continue
		}

		// Create DM and contact with this inviter too
		_, _, err = h.db.CreateDM(ctx, other.InviterID, newUserID)
		if err == nil {
			if err := h.db.AddContact(ctx, other.InviterID, newUserID, "invite", &other.ID); err != nil {
				log.Printf("invite: failed to add contact between %s and %s: %v", other.InviterID, newUserID, err)
			}
			connectedUsers = append(connectedUsers, other.InviterID)
		}
	}

	return connectedUsers, nil
}
