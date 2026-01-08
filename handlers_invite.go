package main

import (
	"context"
	"net/mail"
	"strings"

	"github.com/google/uuid"
)

// HandleInvite processes invite code requests.
func (h *Handlers) HandleInvite(s *Session, msg *ClientMessage) {
	if !s.IsAuthenticated() {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
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

	// TODO: Send email with invite code
	// For now, just return the code to the user

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

// HandleRedeemInvite processes invite code redemption (signup via invite).
// This is called during account creation when an invite code is provided.
func (h *Handlers) RedeemInviteCode(ctx context.Context, code string, newUserID uuid.UUID) (*uuid.UUID, error) {
	// Use the invite code
	invite, err := h.db.UseInviteCode(ctx, code, newUserID)
	if err != nil {
		return nil, err
	}
	if invite == nil {
		return nil, nil // Code not found or expired
	}

	// Create DM between inviter and new user
	_, _, err = h.db.CreateDM(ctx, invite.InviterID, newUserID)
	if err != nil {
		// Log but don't fail - user is created, DM can be created later
		return &invite.InviterID, nil
	}

	return &invite.InviterID, nil
}
