package main

import (
	"context"

	"github.com/google/uuid"
)

// HandleContact processes contact management requests.
func (h *Handlers) HandleContact(s *Session, msg *ClientMessage) {
	h.handleContact(s, msg)
}

func (h *Handlers) handleContact(s SessionInterface, msg *ClientMessage) {
	if !s.RequireAuth(msg.ID) {
		return
	}

	contact := msg.Contact
	if contact == nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "missing contact data"))
		return
	}

	ctx, cancel := handlerCtx()
	defer cancel()

	switch {
	case contact.Add != "":
		h.handleAddContact(ctx, s, msg, contact.Add)
	case contact.Remove != "":
		h.handleRemoveContact(ctx, s, msg, contact.Remove)
	case contact.User != "" && contact.Nickname != nil:
		h.handleUpdateContactNickname(ctx, s, msg, contact.User, contact.Nickname)
	default:
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid contact request"))
	}
}

func (h *Handlers) handleAddContact(ctx context.Context, s SessionInterface, msg *ClientMessage, userIDStr string) {
	contactID, err := uuid.Parse(userIDStr)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid user id"))
		return
	}

	// Can't add yourself
	if contactID == s.UserID() {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "cannot add yourself as contact"))
		return
	}

	// Check if user exists
	user, err := h.db.GetUserByID(ctx, contactID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "database error"))
		return
	}
	if user == nil {
		s.Send(CtrlError(msg.ID, CodeNotFound, "user not found"))
		return
	}

	// Add contact (manual source, no invite)
	err = h.db.AddContact(ctx, s.UserID(), contactID, "manual", nil)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to add contact"))
		return
	}

	s.Send(CtrlSuccess(msg.ID, CodeOK, map[string]any{
		"user":   contactID.String(),
		"public": user.Public,
	}))
}

func (h *Handlers) handleRemoveContact(ctx context.Context, s SessionInterface, msg *ClientMessage, userIDStr string) {
	contactID, err := uuid.Parse(userIDStr)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid user id"))
		return
	}

	err = h.db.RemoveContact(ctx, s.UserID(), contactID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to remove contact"))
		return
	}

	s.Send(CtrlSuccess(msg.ID, CodeOK, nil))
}

func (h *Handlers) handleUpdateContactNickname(ctx context.Context, s SessionInterface, msg *ClientMessage, userIDStr string, nickname *string) {
	contactID, err := uuid.Parse(userIDStr)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid user id"))
		return
	}

	err = h.db.UpdateContactNickname(ctx, s.UserID(), contactID, nickname)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to update nickname"))
		return
	}

	s.Send(CtrlSuccess(msg.ID, CodeOK, nil))
}
