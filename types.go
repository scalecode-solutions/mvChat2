package main

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ClientMessage is a message from client to server.
type ClientMessage struct {
	ID string `json:"id,omitempty"`

	// Only one of these should be set
	Hi      *MsgClientHi      `json:"hi,omitempty"`
	Login   *MsgClientLogin   `json:"login,omitempty"`
	Acc     *MsgClientAcc     `json:"acc,omitempty"`
	Search  *MsgClientSearch  `json:"search,omitempty"`
	DM      *MsgClientDM      `json:"dm,omitempty"`
	Room    *MsgClientRoom    `json:"room,omitempty"`
	Send    *MsgClientSend    `json:"send,omitempty"`
	Get     *MsgClientGet     `json:"get,omitempty"`
	Edit    *MsgClientEdit    `json:"edit,omitempty"`
	Unsend  *MsgClientUnsend  `json:"unsend,omitempty"`
	Delete  *MsgClientDelete  `json:"delete,omitempty"`
	React   *MsgClientReact   `json:"react,omitempty"`
	Typing  *MsgClientTyping  `json:"typing,omitempty"`
	Read    *MsgClientRead    `json:"read,omitempty"`
	Recv    *MsgClientRecv    `json:"recv,omitempty"`
	Clear   *MsgClientClear   `json:"clear,omitempty"`
	Invite  *MsgClientInvite  `json:"invite,omitempty"`
	Contact *MsgClientContact `json:"contact,omitempty"`
	Pin     *MsgClientPin     `json:"pin,omitempty"`
}

// ServerMessage is a message from server to client.
type ServerMessage struct {
	// Control message (response to client request)
	Ctrl *MsgServerCtrl `json:"ctrl,omitempty"`
	// Data message (incoming message)
	Data *MsgServerData `json:"data,omitempty"`
	// Info message (typing, read, edit, unsend, react)
	Info *MsgServerInfo `json:"info,omitempty"`
	// Presence message (online/offline)
	Pres *MsgServerPres `json:"pres,omitempty"`
}

// ============================================================================
// Client Messages
// ============================================================================

// MsgClientHi is the handshake message.
type MsgClientHi struct {
	Version   string `json:"ver"`
	UserAgent string `json:"ua,omitempty"`
	DeviceID  string `json:"dev,omitempty"`
	Lang      string `json:"lang,omitempty"`
}

// MsgClientLogin is the authentication message.
type MsgClientLogin struct {
	Scheme string `json:"scheme"` // "basic" or "token"
	Secret string `json:"secret"` // base64 encoded
}

// MsgClientAcc is for account creation/update.
type MsgClientAcc struct {
	// For create: "new", for update: "me"
	User   string      `json:"user"`
	Scheme string      `json:"scheme,omitempty"`
	Secret string      `json:"secret,omitempty"`
	Login  bool        `json:"login,omitempty"`
	Desc   *MsgSetDesc `json:"desc,omitempty"`
	// For invite-based signup: the 10-digit invite code
	InviteCode string `json:"inviteCode,omitempty"`
	// For account update: new email address
	Email *string `json:"email,omitempty"`
	// For account update: preferred language (e.g., "en", "es", "fr")
	Lang *string `json:"lang,omitempty"`
}

// MsgSetDesc is public/private data for account or conversation.
type MsgSetDesc struct {
	Public  json.RawMessage `json:"public,omitempty"`
	Private json.RawMessage `json:"private,omitempty"`
}

// MsgClientSearch is for user search.
type MsgClientSearch struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

// MsgClientDM is for starting/managing DMs.
type MsgClientDM struct {
	// Start DM with user
	User string `json:"user,omitempty"`
	// Manage existing DM
	ConversationID string `json:"conv,omitempty"`
	// Settings
	Favorite *bool           `json:"favorite,omitempty"`
	Muted    *bool           `json:"muted,omitempty"`
	Blocked  *bool           `json:"blocked,omitempty"`
	Private  json.RawMessage `json:"private,omitempty"`
	// Disappearing messages TTL in seconds (nil = no change, 0 = disable)
	DisappearingTTL *int `json:"disappearingTTL,omitempty"`
}

// MsgClientRoom is for room management.
type MsgClientRoom struct {
	// Create: "new", Join/Leave/Manage: room ID
	ID string `json:"id"`
	// Action: "create", "join", "leave", "invite", "kick", "update"
	Action string `json:"action"`
	// For invite/kick
	User string `json:"user,omitempty"`
	// For create/update
	Desc *MsgSetDesc `json:"desc,omitempty"`
	// Disappearing messages TTL in seconds (nil = no change, 0 = disable)
	DisappearingTTL *int `json:"disappearingTTL,omitempty"`
	// No-screenshots flag (nil = no change)
	NoScreenshots *bool `json:"noScreenshots,omitempty"`
}

// MsgClientSend is for sending a message.
type MsgClientSend struct {
	ConversationID string          `json:"conv"`
	Content        json.RawMessage `json:"content"` // Irido format
	// Optional: reply to message seq
	ReplyTo int `json:"replyTo,omitempty"`
	// View-once message: disappears after recipient views it
	ViewOnce bool `json:"viewOnce,omitempty"`
	// TTL in seconds after viewing: 10, 30, 60, 300, 3600, 86400, 604800
	ViewOnceTTL int `json:"viewOnceTTL,omitempty"`
}

// MsgClientGet is for fetching data.
type MsgClientGet struct {
	// What to get: "conversations", "conversation", "messages", "members", "receipts", "contacts", "user"
	What string `json:"what"`
	// For messages/members/receipts/conversation: conversation ID
	ConversationID string `json:"conv,omitempty"`
	// For user: user ID
	User string `json:"user,omitempty"`
	// Pagination
	Before int `json:"before,omitempty"`
	Limit  int `json:"limit,omitempty"`
}

// MsgClientEdit is for editing a message.
type MsgClientEdit struct {
	ConversationID string          `json:"conv"`
	Seq            int             `json:"seq"`
	Content        json.RawMessage `json:"content"`
}

// MsgClientUnsend is for unsending a message.
type MsgClientUnsend struct {
	ConversationID string `json:"conv"`
	Seq            int    `json:"seq"`
}

// MsgClientDelete is for deleting messages.
type MsgClientDelete struct {
	ConversationID string `json:"conv"`
	Seq            int    `json:"seq"`
	// If true, delete for everyone (sender only). If false, delete for me.
	ForEveryone bool `json:"forEveryone,omitempty"`
}

// MsgClientReact is for adding/removing reactions.
type MsgClientReact struct {
	ConversationID string `json:"conv"`
	Seq            int    `json:"seq"`
	Emoji          string `json:"emoji"`
}

// MsgClientTyping is the typing indicator.
type MsgClientTyping struct {
	ConversationID string `json:"conv"`
}

// MsgClientRead is the read receipt.
type MsgClientRead struct {
	ConversationID string `json:"conv"`
	Seq            int    `json:"seq"`
}

// MsgClientRecv is the delivery receipt (message received by client).
type MsgClientRecv struct {
	ConversationID string `json:"conv"`
	Seq            int    `json:"seq"`
}

// MsgClientClear clears conversation history up to a sequence number.
type MsgClientClear struct {
	ConversationID string `json:"conv"`
	Seq            int    `json:"seq"` // Clear messages with seq <= this value
}

// ============================================================================
// Server Messages
// ============================================================================

// MsgServerCtrl is a control/response message.
type MsgServerCtrl struct {
	ID     string         `json:"id,omitempty"`
	Code   int            `json:"code"`
	Text   string         `json:"text,omitempty"`
	Topic  string         `json:"topic,omitempty"`
	Params map[string]any `json:"params,omitempty"`
	Ts     time.Time      `json:"ts"`
}

// MsgServerData is an incoming message.
type MsgServerData struct {
	ConversationID string          `json:"conv"`
	Seq            int             `json:"seq"`
	From           string          `json:"from"`
	Content        json.RawMessage `json:"content"`
	Head           map[string]any  `json:"head,omitempty"`
	Ts             time.Time       `json:"ts"`
}

// MsgServerInfo is a notification (typing, read, edit, unsend, react).
type MsgServerInfo struct {
	ConversationID string          `json:"conv"`
	From           string          `json:"from"`
	What           string          `json:"what"` // "typing", "read", "edit", "unsend", "react"
	Seq            int             `json:"seq,omitempty"`
	Content        json.RawMessage `json:"content,omitempty"` // For edit
	Emoji          string          `json:"emoji,omitempty"`   // For react
	Ts             time.Time       `json:"ts"`
}

// MsgServerPres is a presence notification.
type MsgServerPres struct {
	UserID   string     `json:"user"`
	What     string     `json:"what"` // "on", "off"
	LastSeen *time.Time `json:"lastSeen,omitempty"`
}

// MsgClientInvite is for invite code management.
type MsgClientInvite struct {
	// Create a new invite
	Create *MsgClientInviteCreate `json:"create,omitempty"`
	// List user's invites
	List bool `json:"list,omitempty"`
	// Revoke an invite by ID
	Revoke string `json:"revoke,omitempty"`
	// Redeem an invite code (for existing users)
	Redeem string `json:"redeem,omitempty"`
}

// MsgClientInviteCreate is for creating an invite.
type MsgClientInviteCreate struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

// MsgClientContact is for contact management.
type MsgClientContact struct {
	// Add a user as contact
	Add string `json:"add,omitempty"`
	// Remove a contact
	Remove string `json:"remove,omitempty"`
	// Update nickname for a contact
	User     string  `json:"user,omitempty"`
	Nickname *string `json:"nickname,omitempty"`
}

// MsgClientPin is for pinning/unpinning messages.
type MsgClientPin struct {
	ConversationID string `json:"conv"`
	// Seq of message to pin (0 or omit to unpin)
	Seq int `json:"seq,omitempty"`
}

// ============================================================================
// Response Helpers
// ============================================================================

// CtrlSuccess creates a success response.
func CtrlSuccess(id string, code int, params map[string]any) *ServerMessage {
	return &ServerMessage{
		Ctrl: &MsgServerCtrl{
			ID:     id,
			Code:   code,
			Text:   "ok",
			Params: params,
			Ts:     time.Now().UTC(),
		},
	}
}

// CtrlError creates an error response.
func CtrlError(id string, code int, text string) *ServerMessage {
	return &ServerMessage{
		Ctrl: &MsgServerCtrl{
			ID:   id,
			Code: code,
			Text: text,
			Ts:   time.Now().UTC(),
		},
	}
}

// Common error codes
const (
	CodeOK              = 200
	CodeCreated         = 201
	CodeAccepted        = 202
	CodeNoContent       = 204
	CodeBadRequest      = 400
	CodeUnauthorized    = 401
	CodeForbidden       = 403
	CodeNotFound        = 404
	CodeConflict        = 409
	CodeGone            = 410
	CodeTooManyRequests = 429
	CodeInternalError   = 500
)

// ============================================================================
// User Info (for responses)
// ============================================================================

// UserInfo is user data returned in responses.
type UserInfo struct {
	ID       uuid.UUID       `json:"id"`
	Public   json.RawMessage `json:"public,omitempty"`
	Online   bool            `json:"online,omitempty"`
	LastSeen *time.Time      `json:"lastSeen,omitempty"`
}

// ConversationInfo is conversation data returned in responses.
type ConversationInfo struct {
	ID        uuid.UUID       `json:"id"`
	Type      string          `json:"type"` // "dm" or "room"
	Public    json.RawMessage `json:"public,omitempty"`
	Private   json.RawMessage `json:"private,omitempty"`
	LastSeq   int             `json:"lastSeq"`
	ReadSeq   int             `json:"readSeq"`
	Unread    int             `json:"unread"`
	LastMsgAt *time.Time      `json:"lastMsgAt,omitempty"`
	Favorite  bool            `json:"favorite,omitempty"`
	Muted     bool            `json:"muted,omitempty"`
	// For DMs: the other user
	User *UserInfo `json:"user,omitempty"`
	// Disappearing messages TTL in seconds (nil = disabled)
	DisappearingTTL *int `json:"disappearingTTL,omitempty"`
	// Pinned message info
	PinnedSeq *int       `json:"pinnedSeq,omitempty"`
	PinnedAt  *time.Time `json:"pinnedAt,omitempty"`
	PinnedBy  *string    `json:"pinnedBy,omitempty"`
	// No-screenshots flag
	NoScreenshots bool `json:"noScreenshots,omitempty"`
}
