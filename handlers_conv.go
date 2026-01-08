package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// HandleDM processes DM requests (start DM, manage settings).
func (h *Handlers) HandleDM(s *Session, msg *ClientMessage) {
	if !s.IsAuthenticated() {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
		return
	}

	dm := msg.DM
	if dm == nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "missing dm data"))
		return
	}

	ctx := context.Background()

	// Start DM with user
	if dm.User != "" {
		h.handleStartDM(ctx, s, msg, dm)
		return
	}

	// Manage existing DM
	if dm.ConversationID != "" {
		h.handleManageDM(ctx, s, msg, dm)
		return
	}

	s.Send(CtrlError(msg.ID, CodeBadRequest, "missing user or conv"))
}

func (h *Handlers) handleStartDM(ctx context.Context, s *Session, msg *ClientMessage, dm *MsgClientDM) {
	otherUserID, err := uuid.Parse(dm.User)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid user id"))
		return
	}

	if otherUserID == s.userID {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "cannot DM yourself"))
		return
	}

	// Check if other user exists
	otherUser, err := h.db.GetUserByID(ctx, otherUserID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "database error"))
		return
	}
	if otherUser == nil {
		s.Send(CtrlError(msg.ID, CodeNotFound, "user not found"))
		return
	}

	// Create or get existing DM
	conv, created, err := h.db.CreateDM(ctx, s.userID, otherUserID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to create dm"))
		return
	}

	code := CodeOK
	if created {
		code = CodeCreated
	}

	s.Send(CtrlSuccess(msg.ID, code, map[string]any{
		"conv":    conv.ID.String(),
		"created": created,
		"user": map[string]any{
			"id":     otherUser.ID.String(),
			"public": otherUser.Public,
			"online": h.hub.IsOnline(otherUser.ID),
		},
	}))
}

func (h *Handlers) handleManageDM(ctx context.Context, s *Session, msg *ClientMessage, dm *MsgClientDM) {
	convID, err := uuid.Parse(dm.ConversationID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid conv id"))
		return
	}

	// Check membership
	isMember, err := h.db.IsMember(ctx, convID, s.userID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "database error"))
		return
	}
	if !isMember {
		s.Send(CtrlError(msg.ID, CodeForbidden, "not a member"))
		return
	}

	// Build updates
	updates := make(map[string]any)
	if dm.Favorite != nil {
		updates["favorite"] = *dm.Favorite
	}
	if dm.Muted != nil {
		updates["muted"] = *dm.Muted
	}
	if dm.Blocked != nil {
		updates["blocked"] = *dm.Blocked
	}
	if dm.Private != nil {
		updates["private"] = dm.Private
	}

	if len(updates) == 0 {
		s.Send(CtrlSuccess(msg.ID, CodeOK, nil))
		return
	}

	if err := h.db.UpdateMemberSettings(ctx, convID, s.userID, updates); err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to update"))
		return
	}

	s.Send(CtrlSuccess(msg.ID, CodeOK, nil))
}

// HandleGroup processes group requests.
func (h *Handlers) HandleGroup(s *Session, msg *ClientMessage) {
	if !s.IsAuthenticated() {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
		return
	}

	group := msg.Group
	if group == nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "missing group data"))
		return
	}

	ctx := context.Background()

	switch group.Action {
	case "create":
		h.handleCreateGroup(ctx, s, msg, group)
	default:
		// TODO: join, leave, invite, kick, update
		s.Send(CtrlError(msg.ID, CodeInternalError, "not implemented"))
	}
}

func (h *Handlers) handleCreateGroup(ctx context.Context, s *Session, msg *ClientMessage, group *MsgClientGroup) {
	var public json.RawMessage
	if group.Desc != nil {
		public = group.Desc.Public
	}

	conv, err := h.db.CreateGroup(ctx, s.userID, public)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to create group"))
		return
	}

	s.Send(CtrlSuccess(msg.ID, CodeCreated, map[string]any{
		"conv":   conv.ID.String(),
		"public": conv.Public,
	}))
}

// HandleGet processes get requests (conversations, messages, members).
func (h *Handlers) HandleGet(s *Session, msg *ClientMessage) {
	if !s.IsAuthenticated() {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
		return
	}

	get := msg.Get
	if get == nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "missing get data"))
		return
	}

	ctx := context.Background()

	switch get.What {
	case "conversations":
		h.handleGetConversations(ctx, s, msg)
	case "messages":
		h.handleGetMessages(ctx, s, msg, get)
	case "members":
		h.handleGetMembers(ctx, s, msg, get)
	case "receipts":
		h.handleGetReceipts(ctx, s, msg, get)
	default:
		s.Send(CtrlError(msg.ID, CodeBadRequest, "unknown what"))
	}
}

func (h *Handlers) handleGetConversations(ctx context.Context, s *Session, msg *ClientMessage) {
	convs, err := h.db.GetUserConversations(ctx, s.userID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to get conversations"))
		return
	}

	results := make([]map[string]any, 0, len(convs))
	for _, c := range convs {
		item := map[string]any{
			"id":       c.Conversation.ID.String(),
			"type":     c.Type,
			"lastSeq":  c.LastSeq,
			"readSeq":  c.ReadSeq,
			"unread":   c.LastSeq - c.ReadSeq,
			"favorite": c.Favorite,
			"muted":    c.Muted,
		}
		if c.LastMsgAt != nil {
			item["lastMsgAt"] = c.LastMsgAt
		}
		if c.Type == "dm" && c.OtherUser != nil {
			item["user"] = map[string]any{
				"id":       c.OtherUser.ID.String(),
				"public":   c.OtherUser.Public,
				"online":   h.hub.IsOnline(c.OtherUser.ID),
				"lastSeen": c.OtherUser.LastSeen,
			}
		} else if c.Type == "group" {
			item["public"] = c.Public
		}
		results = append(results, item)
	}

	s.Send(CtrlSuccess(msg.ID, CodeOK, map[string]any{
		"conversations": results,
	}))
}

func (h *Handlers) handleGetMessages(ctx context.Context, s *Session, msg *ClientMessage, get *MsgClientGet) {
	if get.ConversationID == "" {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "missing conv"))
		return
	}

	convID, err := uuid.Parse(get.ConversationID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid conv id"))
		return
	}

	// Check membership and get clear_seq
	member, err := h.db.GetMember(ctx, convID, s.userID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "database error"))
		return
	}
	if member == nil {
		s.Send(CtrlError(msg.ID, CodeForbidden, "not a member"))
		return
	}

	messages, err := h.db.GetMessages(ctx, convID, s.userID, get.Before, get.Limit, member.ClearSeq)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to get messages"))
		return
	}

	results := make([]map[string]any, 0, len(messages))
	for _, m := range messages {
		item := map[string]any{
			"seq":  m.Seq,
			"from": m.FromUserID.String(),
			"ts":   m.CreatedAt,
		}
		if m.DeletedAt != nil {
			item["deleted"] = true
		} else {
			// Decrypt content
			plaintext, err := h.encryptor.Decrypt(m.Content)
			if err == nil {
				item["content"] = plaintext
			} else {
				item["content"] = m.Content // Fallback for unencrypted messages
			}
		}
		if m.Head != nil {
			item["head"] = m.Head
		}
		results = append(results, item)
	}

	s.Send(CtrlSuccess(msg.ID, CodeOK, map[string]any{
		"messages": results,
	}))
}

func (h *Handlers) handleGetMembers(ctx context.Context, s *Session, msg *ClientMessage, get *MsgClientGet) {
	if get.ConversationID == "" {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "missing conv"))
		return
	}

	convID, err := uuid.Parse(get.ConversationID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid conv id"))
		return
	}

	// Check membership
	isMember, err := h.db.IsMember(ctx, convID, s.userID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "database error"))
		return
	}
	if !isMember {
		s.Send(CtrlError(msg.ID, CodeForbidden, "not a member"))
		return
	}

	memberIDs, err := h.db.GetConversationMembers(ctx, convID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to get members"))
		return
	}

	results := make([]map[string]any, 0, len(memberIDs))
	for _, uid := range memberIDs {
		user, _ := h.db.GetUserByID(ctx, uid)
		if user != nil {
			results = append(results, map[string]any{
				"id":       user.ID.String(),
				"public":   user.Public,
				"online":   h.hub.IsOnline(user.ID),
				"lastSeen": user.LastSeen,
			})
		}
	}

	s.Send(CtrlSuccess(msg.ID, CodeOK, map[string]any{
		"members": results,
	}))
}

func (h *Handlers) handleGetReceipts(ctx context.Context, s *Session, msg *ClientMessage, get *MsgClientGet) {
	if get.ConversationID == "" {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "missing conv"))
		return
	}

	convID, err := uuid.Parse(get.ConversationID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid conv id"))
		return
	}

	// Check membership
	isMember, err := h.db.IsMember(ctx, convID, s.userID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "database error"))
		return
	}
	if !isMember {
		s.Send(CtrlError(msg.ID, CodeForbidden, "not a member"))
		return
	}

	receipts, err := h.db.GetReadReceipts(ctx, convID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to get receipts"))
		return
	}

	results := make([]map[string]any, 0, len(receipts))
	for _, r := range receipts {
		results = append(results, map[string]any{
			"user":    r.UserID.String(),
			"readSeq": r.ReadSeq,
			"recvSeq": r.RecvSeq,
		})
	}

	s.Send(CtrlSuccess(msg.ID, CodeOK, map[string]any{
		"receipts": results,
	}))
}

// HandleSend processes send message requests.
func (h *Handlers) HandleSend(s *Session, msg *ClientMessage) {
	if !s.IsAuthenticated() {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
		return
	}

	send := msg.Send
	if send == nil || send.ConversationID == "" || send.Content == nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "missing send data"))
		return
	}

	ctx := context.Background()

	convID, err := uuid.Parse(send.ConversationID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid conv id"))
		return
	}

	// Check membership
	member, err := h.db.GetMember(ctx, convID, s.userID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "database error"))
		return
	}
	if member == nil {
		s.Send(CtrlError(msg.ID, CodeForbidden, "not a member"))
		return
	}

	// Check if blocked (for DMs)
	conv, err := h.db.GetConversationByID(ctx, convID)
	if err != nil || conv == nil {
		s.Send(CtrlError(msg.ID, CodeNotFound, "conversation not found"))
		return
	}

	if conv.Type == "dm" {
		// Check if the other user blocked us
		otherUser, _ := h.db.GetDMOtherUser(ctx, convID, s.userID)
		if otherUser != nil {
			blocked, _ := h.db.IsBlocked(ctx, convID, otherUser.ID, s.userID)
			if blocked {
				s.Send(CtrlError(msg.ID, CodeForbidden, "blocked"))
				return
			}
		}
	}

	// Build head
	var head json.RawMessage
	if send.ReplyTo > 0 {
		head, _ = json.Marshal(map[string]any{"reply_to": send.ReplyTo})
	}

	// Encrypt content
	content, err := h.encryptor.Encrypt(send.Content)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "encryption failed"))
		return
	}

	// Create message
	message, err := h.db.CreateMessage(ctx, convID, s.userID, content, head)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to send"))
		return
	}

	// Send confirmation to sender
	s.Send(CtrlSuccess(msg.ID, CodeAccepted, map[string]any{
		"conv": convID.String(),
		"seq":  message.Seq,
		"ts":   message.CreatedAt,
	}))

	// Broadcast to other members
	memberIDs, _ := h.db.GetConversationMembers(ctx, convID)
	var headMap map[string]any
	if head != nil {
		json.Unmarshal(head, &headMap)
	}
	dataMsg := &ServerMessage{
		Data: &MsgServerData{
			ConversationID: convID.String(),
			Seq:            message.Seq,
			From:           s.userID.String(),
			Content:        send.Content,
			Head:           headMap,
			Ts:             message.CreatedAt,
		},
	}
	h.hub.SendToUsers(memberIDs, dataMsg, s.id)
}

// HandleEdit processes edit message requests.
func (h *Handlers) HandleEdit(s *Session, msg *ClientMessage) {
	if !s.IsAuthenticated() {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
		return
	}

	edit := msg.Edit
	if edit == nil || edit.ConversationID == "" || edit.Seq <= 0 || edit.Content == nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "missing edit data"))
		return
	}

	ctx := context.Background()

	convID, err := uuid.Parse(edit.ConversationID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid conv id"))
		return
	}

	// Get original message
	origMsg, err := h.db.GetMessageBySeq(ctx, convID, edit.Seq)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "database error"))
		return
	}
	if origMsg == nil {
		s.Send(CtrlError(msg.ID, CodeNotFound, "message not found"))
		return
	}

	// Only sender can edit
	if origMsg.FromUserID != s.userID {
		s.Send(CtrlError(msg.ID, CodeForbidden, "not your message"))
		return
	}

	// Check time window (15 minutes)
	if time.Since(origMsg.CreatedAt) > 15*time.Minute {
		s.Send(CtrlError(msg.ID, CodeForbidden, "edit window expired"))
		return
	}

	// Check edit count (max 10)
	editCount, _ := h.db.GetEditCount(ctx, convID, edit.Seq)
	if editCount >= 10 {
		s.Send(CtrlError(msg.ID, CodeForbidden, "max edits reached"))
		return
	}

	// Encrypt content
	content, err := h.encryptor.Encrypt(edit.Content)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "encryption failed"))
		return
	}

	if err := h.db.EditMessage(ctx, convID, edit.Seq, content); err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to edit"))
		return
	}

	now := time.Now().UTC()
	s.Send(CtrlSuccess(msg.ID, CodeOK, map[string]any{
		"conv":     convID.String(),
		"seq":      edit.Seq,
		"editedAt": now,
	}))

	// Broadcast edit to members
	memberIDs, _ := h.db.GetConversationMembers(ctx, convID)
	infoMsg := &ServerMessage{
		Info: &MsgServerInfo{
			ConversationID: convID.String(),
			From:           s.userID.String(),
			What:           "edit",
			Seq:            edit.Seq,
			Content:        edit.Content,
			Ts:             now,
		},
	}
	h.hub.SendToUsers(memberIDs, infoMsg, s.id)
}

// HandleUnsend processes unsend message requests.
func (h *Handlers) HandleUnsend(s *Session, msg *ClientMessage) {
	if !s.IsAuthenticated() {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
		return
	}

	unsend := msg.Unsend
	if unsend == nil || unsend.ConversationID == "" || unsend.Seq <= 0 {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "missing unsend data"))
		return
	}

	ctx := context.Background()

	convID, err := uuid.Parse(unsend.ConversationID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid conv id"))
		return
	}

	// Get original message
	origMsg, err := h.db.GetMessageBySeq(ctx, convID, unsend.Seq)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "database error"))
		return
	}
	if origMsg == nil {
		s.Send(CtrlError(msg.ID, CodeNotFound, "message not found"))
		return
	}

	// Only sender can unsend
	if origMsg.FromUserID != s.userID {
		s.Send(CtrlError(msg.ID, CodeForbidden, "not your message"))
		return
	}

	// Check time window (10 minutes)
	if time.Since(origMsg.CreatedAt) > 10*time.Minute {
		s.Send(CtrlError(msg.ID, CodeForbidden, "unsend window expired"))
		return
	}

	if err := h.db.UnsendMessage(ctx, convID, unsend.Seq); err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to unsend"))
		return
	}

	now := time.Now().UTC()
	s.Send(CtrlSuccess(msg.ID, CodeOK, nil))

	// Broadcast unsend to members
	memberIDs, _ := h.db.GetConversationMembers(ctx, convID)
	infoMsg := &ServerMessage{
		Info: &MsgServerInfo{
			ConversationID: convID.String(),
			From:           s.userID.String(),
			What:           "unsend",
			Seq:            unsend.Seq,
			Ts:             now,
		},
	}
	h.hub.SendToUsers(memberIDs, infoMsg, s.id)
}

// HandleDelete processes delete message requests (for me or for everyone).
func (h *Handlers) HandleDelete(s *Session, msg *ClientMessage) {
	if !s.IsAuthenticated() {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
		return
	}

	del := msg.Delete
	if del == nil || del.ConversationID == "" || del.Seq <= 0 {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "missing delete data"))
		return
	}

	ctx := context.Background()

	convID, err := uuid.Parse(del.ConversationID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid conv id"))
		return
	}

	// Get message
	origMsg, err := h.db.GetMessageBySeq(ctx, convID, del.Seq)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "database error"))
		return
	}
	if origMsg == nil {
		s.Send(CtrlError(msg.ID, CodeNotFound, "message not found"))
		return
	}

	if del.ForEveryone {
		// Only sender can delete for everyone
		if origMsg.FromUserID != s.userID {
			s.Send(CtrlError(msg.ID, CodeForbidden, "not your message"))
			return
		}

		if err := h.db.UnsendMessage(ctx, convID, del.Seq); err != nil {
			s.Send(CtrlError(msg.ID, CodeInternalError, "failed to delete"))
			return
		}

		s.Send(CtrlSuccess(msg.ID, CodeOK, nil))

		// Broadcast to members
		memberIDs, _ := h.db.GetConversationMembers(ctx, convID)
		infoMsg := &ServerMessage{
			Info: &MsgServerInfo{
				ConversationID: convID.String(),
				From:           s.userID.String(),
				What:           "delete",
				Seq:            del.Seq,
				Ts:             time.Now().UTC(),
			},
		}
		h.hub.SendToUsers(memberIDs, infoMsg, s.id)
	} else {
		// Delete for me only
		if err := h.db.DeleteMessageForUser(ctx, origMsg.ID, s.userID); err != nil {
			s.Send(CtrlError(msg.ID, CodeInternalError, "failed to delete"))
			return
		}
		s.Send(CtrlSuccess(msg.ID, CodeOK, nil))
	}
}

// HandleReact processes reaction requests.
func (h *Handlers) HandleReact(s *Session, msg *ClientMessage) {
	if !s.IsAuthenticated() {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
		return
	}

	react := msg.React
	if react == nil || react.ConversationID == "" || react.Seq <= 0 || react.Emoji == "" {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "missing react data"))
		return
	}

	ctx := context.Background()

	convID, err := uuid.Parse(react.ConversationID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid conv id"))
		return
	}

	// Check membership
	isMember, err := h.db.IsMember(ctx, convID, s.userID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "database error"))
		return
	}
	if !isMember {
		s.Send(CtrlError(msg.ID, CodeForbidden, "not a member"))
		return
	}

	if err := h.db.AddReaction(ctx, convID, react.Seq, s.userID, react.Emoji); err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to react"))
		return
	}

	now := time.Now().UTC()
	s.Send(CtrlSuccess(msg.ID, CodeOK, nil))

	// Broadcast to members
	memberIDs, _ := h.db.GetConversationMembers(ctx, convID)
	infoMsg := &ServerMessage{
		Info: &MsgServerInfo{
			ConversationID: convID.String(),
			From:           s.userID.String(),
			What:           "react",
			Seq:            react.Seq,
			Emoji:          react.Emoji,
			Ts:             now,
		},
	}
	h.hub.SendToUsers(memberIDs, infoMsg, s.id)
}

// HandleTyping processes typing indicator requests.
func (h *Handlers) HandleTyping(s *Session, msg *ClientMessage) {
	if !s.IsAuthenticated() {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
		return
	}

	typing := msg.Typing
	if typing == nil || typing.ConversationID == "" {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "missing typing data"))
		return
	}

	ctx := context.Background()

	convID, err := uuid.Parse(typing.ConversationID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid conv id"))
		return
	}

	// Check membership
	isMember, err := h.db.IsMember(ctx, convID, s.userID)
	if err != nil || !isMember {
		return // Silently ignore
	}

	// Broadcast to members (no response to sender)
	memberIDs, _ := h.db.GetConversationMembers(ctx, convID)
	infoMsg := &ServerMessage{
		Info: &MsgServerInfo{
			ConversationID: convID.String(),
			From:           s.userID.String(),
			What:           "typing",
			Ts:             time.Now().UTC(),
		},
	}
	h.hub.SendToUsers(memberIDs, infoMsg, s.id)
}

// HandleRead processes read receipt requests.
func (h *Handlers) HandleRead(s *Session, msg *ClientMessage) {
	if !s.IsAuthenticated() {
		s.Send(CtrlError(msg.ID, CodeUnauthorized, "authentication required"))
		return
	}

	read := msg.Read
	if read == nil || read.ConversationID == "" || read.Seq <= 0 {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "missing read data"))
		return
	}

	ctx := context.Background()

	convID, err := uuid.Parse(read.ConversationID)
	if err != nil {
		s.Send(CtrlError(msg.ID, CodeBadRequest, "invalid conv id"))
		return
	}

	// Update read seq
	if err := h.db.UpdateReadSeq(ctx, convID, s.userID, read.Seq); err != nil {
		s.Send(CtrlError(msg.ID, CodeInternalError, "failed to update"))
		return
	}

	s.Send(CtrlSuccess(msg.ID, CodeOK, nil))

	// Broadcast to members
	memberIDs, _ := h.db.GetConversationMembers(ctx, convID)
	infoMsg := &ServerMessage{
		Info: &MsgServerInfo{
			ConversationID: convID.String(),
			From:           s.userID.String(),
			What:           "read",
			Seq:            read.Seq,
			Ts:             time.Now().UTC(),
		},
	}
	h.hub.SendToUsers(memberIDs, infoMsg, s.id)
}
