// Package store provides database access for mvChat2.
package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Store defines the interface for all database operations.
// This interface enables mocking for unit tests.
type Store interface {
	// Close closes the database connection.
	Close()

	// Users
	CreateUser(ctx context.Context, public json.RawMessage) (uuid.UUID, error)
	CreateUserWithOptions(ctx context.Context, public json.RawMessage, mustChangePassword bool, email *string, emailVerified bool) (uuid.UUID, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	UpdateUserLastSeen(ctx context.Context, userID uuid.UUID, userAgent string) error
	UpdateUserPublic(ctx context.Context, userID uuid.UUID, public json.RawMessage) error
	UpdateUserEmail(ctx context.Context, userID uuid.UUID, email *string) error
	SearchUsers(ctx context.Context, query string, limit int) ([]User, error)

	// Auth
	CreateAuthRecord(ctx context.Context, userID uuid.UUID, scheme, secret string, uname *string) error
	GetAuthByUsername(ctx context.Context, username string) (*AuthRecord, error)
	GetAuthByUserID(ctx context.Context, userID uuid.UUID) (*AuthRecord, error)
	GetUserUsername(ctx context.Context, userID uuid.UUID) (string, error)
	UsernameExists(ctx context.Context, username string) (bool, error)
	UpdatePassword(ctx context.Context, userID uuid.UUID, hashedPassword string) error
	ClearMustChangePassword(ctx context.Context, userID uuid.UUID) error

	// Email verification
	SetEmailVerificationToken(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) error
	VerifyEmailByToken(ctx context.Context, token string) (*uuid.UUID, error)
	MarkEmailVerified(ctx context.Context, userID uuid.UUID) error

	// Conversations
	CreateDM(ctx context.Context, user1ID, user2ID uuid.UUID) (*Conversation, bool, error)
	CreateRoom(ctx context.Context, ownerID uuid.UUID, public json.RawMessage) (*Conversation, error)
	GetConversationByID(ctx context.Context, id uuid.UUID) (*Conversation, error)
	GetUserConversations(ctx context.Context, userID uuid.UUID) ([]ConversationWithMember, error)
	GetDMOtherUser(ctx context.Context, convID, userID uuid.UUID) (*User, error)
	GetConversationMembers(ctx context.Context, convID uuid.UUID) ([]uuid.UUID, error)

	// Members
	GetMember(ctx context.Context, convID, userID uuid.UUID) (*Member, error)
	IsMember(ctx context.Context, convID, userID uuid.UUID) (bool, error)
	IsBlocked(ctx context.Context, convID, blockerID, blockedID uuid.UUID) (bool, error)
	UpdateMemberSettings(ctx context.Context, convID, userID uuid.UUID, settings MemberSettings) error
	UpdateReadSeq(ctx context.Context, convID, userID uuid.UUID, seq int) error
	UpdateRecvSeq(ctx context.Context, convID, userID uuid.UUID, seq int) error
	UpdateClearSeq(ctx context.Context, convID, userID uuid.UUID, seq int) error
	GetReadReceipts(ctx context.Context, convID uuid.UUID) ([]ReadReceipt, error)
	AddRoomMember(ctx context.Context, convID, userID uuid.UUID, role string) error
	RemoveMember(ctx context.Context, convID, userID uuid.UUID) error
	GetMemberRole(ctx context.Context, convID, userID uuid.UUID) (string, error)

	// Rooms
	UpdateRoomPublic(ctx context.Context, convID uuid.UUID, public json.RawMessage) error

	// Pinned messages
	SetPinnedMessage(ctx context.Context, convID uuid.UUID, messageID *uuid.UUID, pinnedBy uuid.UUID) error
	GetPinnedMessageSeq(ctx context.Context, convID uuid.UUID) (*int, error)

	// Disappearing messages
	UpdateConversationDisappearingTTL(ctx context.Context, convID uuid.UUID, ttl *int) error
	GetConversationDisappearingTTL(ctx context.Context, convID uuid.UUID) (*int, error)

	// Messages
	CreateMessage(ctx context.Context, convID, fromUserID uuid.UUID, content []byte, head json.RawMessage) (*Message, error)
	GetMessages(ctx context.Context, convID, userID uuid.UUID, before, limit int, clearSeq int) ([]Message, error)
	GetMessageBySeq(ctx context.Context, convID uuid.UUID, seq int) (*Message, error)
	EditMessage(ctx context.Context, convID uuid.UUID, seq int, content []byte) error
	UnsendMessage(ctx context.Context, convID uuid.UUID, seq int) error
	DeleteMessageForEveryone(ctx context.Context, convID uuid.UUID, seq int) error
	DeleteMessageForUser(ctx context.Context, msgID, userID uuid.UUID) error
	AddReaction(ctx context.Context, convID uuid.UUID, seq int, userID uuid.UUID, emoji string) error
	GetEditCount(ctx context.Context, convID uuid.UUID, seq int) (int, error)

	// View-once and message reads
	CreateMessageWithViewOnce(ctx context.Context, convID, fromUserID uuid.UUID, content []byte, head json.RawMessage, viewOnce bool, viewOnceTTL *int) (*Message, error)
	RecordMessageRead(ctx context.Context, messageID, userID uuid.UUID) (*MessageRead, error)
	GetMessageRead(ctx context.Context, messageID, userID uuid.UUID) (*MessageRead, error)
	GetMessageByID(ctx context.Context, messageID uuid.UUID) (*Message, error)
	IsMessageExpiredForUser(ctx context.Context, messageID, userID uuid.UUID) (bool, error)
	ExpireReadMessages(ctx context.Context) (int64, error)

	// Files
	CreateFile(ctx context.Context, uploaderID uuid.UUID, mimeType string, size int64, location string) (*File, error)
	CreateFileWithHash(ctx context.Context, uploaderID uuid.UUID, mimeType string, size int64, location, hash, originalName string) (*File, error)
	CreateFileWithID(ctx context.Context, fileID, uploaderID uuid.UUID, mimeType string, size int64, location, hash, originalName string) (*File, error)
	GetFileByID(ctx context.Context, id uuid.UUID) (*File, error)
	GetFileByHash(ctx context.Context, hash string) (*File, error)
	GetFileWithMetadata(ctx context.Context, id uuid.UUID) (*FileWithMetadata, error)
	GetFileMetadata(ctx context.Context, fileID uuid.UUID) (*FileMetadata, error)
	CreateFileMetadata(ctx context.Context, fileID uuid.UUID, width, height *int, duration *float64, thumbnail *string, extra json.RawMessage) error
	UpdateFileStatus(ctx context.Context, fileID uuid.UUID, status string) error
	UpdateFileLocation(ctx context.Context, fileID uuid.UUID, location string) error
	DeleteFile(ctx context.Context, fileID uuid.UUID) error
	CanAccessFile(ctx context.Context, fileID, userID uuid.UUID) (bool, error)

	// Invites
	CreateInviteCode(ctx context.Context, inviterID uuid.UUID, code, token, email string, inviteeName *string) (*InviteCode, error)
	GetInviteByID(ctx context.Context, id uuid.UUID) (*InviteCode, error)
	GetInviteByCode(ctx context.Context, code string) (*InviteCode, error)
	GetPendingInviteByUsernames(ctx context.Context, inviterUsername, inviteeEmail string) (*InviteCode, error)
	GetPendingInvitesByEmail(ctx context.Context, email string) ([]InviteCode, error)
	GetUserInvites(ctx context.Context, userID uuid.UUID) ([]*InviteCode, error)
	UseInvite(ctx context.Context, inviteID, usedByID uuid.UUID) (*InviteCode, error)
	RevokeInvite(ctx context.Context, inviteID uuid.UUID, inviterID uuid.UUID) error
	ExpireOldInvites(ctx context.Context) (int64, error)

	// Contacts
	AddContact(ctx context.Context, userID, contactID uuid.UUID, source string, inviteID *uuid.UUID) error
	GetContacts(ctx context.Context, userID uuid.UUID) ([]Contact, error)
	IsContact(ctx context.Context, userID, contactID uuid.UUID) (bool, error)
	UpdateContactNickname(ctx context.Context, userID, contactID uuid.UUID, nickname *string) error
	RemoveContact(ctx context.Context, userID, contactID uuid.UUID) error
}

// Compile-time check that DB implements Store.
var _ Store = (*DB)(nil)
