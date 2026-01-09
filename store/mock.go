package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// MockStore is a mock implementation of Store for testing.
// Each method field can be set to a custom function to control behavior.
type MockStore struct {
	// Users
	CreateUserFn            func(ctx context.Context, public json.RawMessage) (uuid.UUID, error)
	CreateUserWithOptionsFn func(ctx context.Context, public json.RawMessage, mustChangePassword bool, email *string, emailVerified bool) (uuid.UUID, error)
	GetUserByIDFn           func(ctx context.Context, id uuid.UUID) (*User, error)
	GetUserByEmailFn        func(ctx context.Context, email string) (*User, error)
	UpdateUserLastSeenFn    func(ctx context.Context, userID uuid.UUID, userAgent string) error
	UpdateUserPublicFn      func(ctx context.Context, userID uuid.UUID, public json.RawMessage) error
	UpdateUserEmailFn       func(ctx context.Context, userID uuid.UUID, email *string) error
	SearchUsersFn           func(ctx context.Context, query string, limit int) ([]User, error)

	// Auth
	CreateAuthRecordFn        func(ctx context.Context, userID uuid.UUID, scheme, secret string, uname *string) error
	GetAuthByUsernameFn       func(ctx context.Context, username string) (*AuthRecord, error)
	GetAuthByUserIDFn         func(ctx context.Context, userID uuid.UUID) (*AuthRecord, error)
	GetUserUsernameFn         func(ctx context.Context, userID uuid.UUID) (string, error)
	UsernameExistsFn          func(ctx context.Context, username string) (bool, error)
	UpdatePasswordFn          func(ctx context.Context, userID uuid.UUID, hashedPassword string) error
	ClearMustChangePasswordFn func(ctx context.Context, userID uuid.UUID) error

	// Email verification
	SetEmailVerificationTokenFn func(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) error
	VerifyEmailByTokenFn        func(ctx context.Context, token string) (*uuid.UUID, error)
	MarkEmailVerifiedFn         func(ctx context.Context, userID uuid.UUID) error

	// Conversations
	CreateDMFn               func(ctx context.Context, user1ID, user2ID uuid.UUID) (*Conversation, bool, error)
	CreateRoomFn             func(ctx context.Context, ownerID uuid.UUID, public json.RawMessage) (*Conversation, error)
	GetConversationByIDFn    func(ctx context.Context, id uuid.UUID) (*Conversation, error)
	GetUserConversationsFn   func(ctx context.Context, userID uuid.UUID) ([]ConversationWithMember, error)
	GetDMOtherUserFn         func(ctx context.Context, convID, userID uuid.UUID) (*User, error)
	GetConversationMembersFn func(ctx context.Context, convID uuid.UUID) ([]uuid.UUID, error)

	// Members
	GetMemberFn            func(ctx context.Context, convID, userID uuid.UUID) (*Member, error)
	IsMemberFn             func(ctx context.Context, convID, userID uuid.UUID) (bool, error)
	IsBlockedFn            func(ctx context.Context, convID, blockerID, blockedID uuid.UUID) (bool, error)
	UpdateMemberSettingsFn func(ctx context.Context, convID, userID uuid.UUID, settings MemberSettings) error
	UpdateReadSeqFn        func(ctx context.Context, convID, userID uuid.UUID, seq int) error
	UpdateRecvSeqFn        func(ctx context.Context, convID, userID uuid.UUID, seq int) error
	UpdateClearSeqFn       func(ctx context.Context, convID, userID uuid.UUID, seq int) error
	GetReadReceiptsFn      func(ctx context.Context, convID uuid.UUID) ([]ReadReceipt, error)
	AddRoomMemberFn        func(ctx context.Context, convID, userID uuid.UUID, role string) error
	RemoveMemberFn         func(ctx context.Context, convID, userID uuid.UUID) error
	GetMemberRoleFn        func(ctx context.Context, convID, userID uuid.UUID) (string, error)

	// Rooms
	UpdateRoomPublicFn func(ctx context.Context, convID uuid.UUID, public json.RawMessage) error

	// Pinned messages
	SetPinnedMessageFn    func(ctx context.Context, convID uuid.UUID, messageID *uuid.UUID, pinnedBy uuid.UUID) error
	GetPinnedMessageSeqFn func(ctx context.Context, convID uuid.UUID) (*int, error)

	// Disappearing messages
	UpdateConversationDisappearingTTLFn func(ctx context.Context, convID uuid.UUID, ttl *int) error
	GetConversationDisappearingTTLFn    func(ctx context.Context, convID uuid.UUID) (*int, error)

	// Messages
	CreateMessageFn            func(ctx context.Context, convID, fromUserID uuid.UUID, content []byte, head json.RawMessage) (*Message, error)
	GetMessagesFn              func(ctx context.Context, convID, userID uuid.UUID, before, limit int, clearSeq int) ([]Message, error)
	GetMessageBySeqFn          func(ctx context.Context, convID uuid.UUID, seq int) (*Message, error)
	EditMessageFn              func(ctx context.Context, convID uuid.UUID, seq int, content []byte) error
	UnsendMessageFn            func(ctx context.Context, convID uuid.UUID, seq int) error
	DeleteMessageForEveryoneFn func(ctx context.Context, convID uuid.UUID, seq int) error
	DeleteMessageForUserFn     func(ctx context.Context, msgID, userID uuid.UUID) error
	AddReactionFn              func(ctx context.Context, convID uuid.UUID, seq int, userID uuid.UUID, emoji string) error
	GetEditCountFn             func(ctx context.Context, convID uuid.UUID, seq int) (int, error)

	// View-once and message reads
	CreateMessageWithViewOnceFn func(ctx context.Context, convID, fromUserID uuid.UUID, content []byte, head json.RawMessage, viewOnce bool, viewOnceTTL *int) (*Message, error)
	RecordMessageReadFn         func(ctx context.Context, messageID, userID uuid.UUID) (*MessageRead, error)
	GetMessageReadFn            func(ctx context.Context, messageID, userID uuid.UUID) (*MessageRead, error)
	GetMessageByIDFn            func(ctx context.Context, messageID uuid.UUID) (*Message, error)
	IsMessageExpiredForUserFn   func(ctx context.Context, messageID, userID uuid.UUID) (bool, error)
	ExpireReadMessagesFn        func(ctx context.Context) (int64, error)

	// Files
	CreateFileFn          func(ctx context.Context, uploaderID uuid.UUID, mimeType string, size int64, location string) (*File, error)
	CreateFileWithHashFn  func(ctx context.Context, uploaderID uuid.UUID, mimeType string, size int64, location, hash, originalName string) (*File, error)
	CreateFileWithIDFn    func(ctx context.Context, fileID, uploaderID uuid.UUID, mimeType string, size int64, location, hash, originalName string) (*File, error)
	GetFileByIDFn         func(ctx context.Context, id uuid.UUID) (*File, error)
	GetFileByHashFn       func(ctx context.Context, hash string) (*File, error)
	GetFileWithMetadataFn func(ctx context.Context, id uuid.UUID) (*FileWithMetadata, error)
	GetFileMetadataFn     func(ctx context.Context, fileID uuid.UUID) (*FileMetadata, error)
	CreateFileMetadataFn  func(ctx context.Context, fileID uuid.UUID, width, height *int, duration *float64, thumbnail *string, extra json.RawMessage) error
	UpdateFileStatusFn    func(ctx context.Context, fileID uuid.UUID, status string) error
	UpdateFileLocationFn  func(ctx context.Context, fileID uuid.UUID, location string) error
	DeleteFileFn          func(ctx context.Context, fileID uuid.UUID) error
	CanAccessFileFn       func(ctx context.Context, fileID, userID uuid.UUID) (bool, error)

	// Invites
	CreateInviteCodeFn            func(ctx context.Context, inviterID uuid.UUID, code, token, email string, inviteeName *string) (*InviteCode, error)
	GetInviteByIDFn               func(ctx context.Context, id uuid.UUID) (*InviteCode, error)
	GetInviteByCodeFn             func(ctx context.Context, code string) (*InviteCode, error)
	GetPendingInviteByUsernamesFn func(ctx context.Context, inviterUsername, inviteeEmail string) (*InviteCode, error)
	GetPendingInvitesByEmailFn    func(ctx context.Context, email string) ([]InviteCode, error)
	GetUserInvitesFn              func(ctx context.Context, userID uuid.UUID) ([]*InviteCode, error)
	UseInviteFn                   func(ctx context.Context, inviteID, usedByID uuid.UUID) (*InviteCode, error)
	RevokeInviteFn                func(ctx context.Context, inviteID uuid.UUID, inviterID uuid.UUID) error
	ExpireOldInvitesFn            func(ctx context.Context) (int64, error)

	// Contacts
	AddContactFn            func(ctx context.Context, userID, contactID uuid.UUID, source string, inviteID *uuid.UUID) error
	GetContactsFn           func(ctx context.Context, userID uuid.UUID) ([]Contact, error)
	IsContactFn             func(ctx context.Context, userID, contactID uuid.UUID) (bool, error)
	UpdateContactNicknameFn func(ctx context.Context, userID, contactID uuid.UUID, nickname *string) error
	RemoveContactFn         func(ctx context.Context, userID, contactID uuid.UUID) error
}

// Compile-time check that MockStore implements Store.
var _ Store = (*MockStore)(nil)

func (m *MockStore) Close() {}

func (m *MockStore) CreateUser(ctx context.Context, public json.RawMessage) (uuid.UUID, error) {
	if m.CreateUserFn != nil {
		return m.CreateUserFn(ctx, public)
	}
	return uuid.New(), nil
}

func (m *MockStore) CreateUserWithOptions(ctx context.Context, public json.RawMessage, mustChangePassword bool, email *string, emailVerified bool) (uuid.UUID, error) {
	if m.CreateUserWithOptionsFn != nil {
		return m.CreateUserWithOptionsFn(ctx, public, mustChangePassword, email, emailVerified)
	}
	return uuid.New(), nil
}

func (m *MockStore) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	if m.GetUserByIDFn != nil {
		return m.GetUserByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *MockStore) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	if m.GetUserByEmailFn != nil {
		return m.GetUserByEmailFn(ctx, email)
	}
	return nil, nil
}

func (m *MockStore) UpdateUserLastSeen(ctx context.Context, userID uuid.UUID, userAgent string) error {
	if m.UpdateUserLastSeenFn != nil {
		return m.UpdateUserLastSeenFn(ctx, userID, userAgent)
	}
	return nil
}

func (m *MockStore) UpdateUserPublic(ctx context.Context, userID uuid.UUID, public json.RawMessage) error {
	if m.UpdateUserPublicFn != nil {
		return m.UpdateUserPublicFn(ctx, userID, public)
	}
	return nil
}

func (m *MockStore) UpdateUserEmail(ctx context.Context, userID uuid.UUID, email *string) error {
	if m.UpdateUserEmailFn != nil {
		return m.UpdateUserEmailFn(ctx, userID, email)
	}
	return nil
}

func (m *MockStore) SearchUsers(ctx context.Context, query string, limit int) ([]User, error) {
	if m.SearchUsersFn != nil {
		return m.SearchUsersFn(ctx, query, limit)
	}
	return nil, nil
}

func (m *MockStore) CreateAuthRecord(ctx context.Context, userID uuid.UUID, scheme, secret string, uname *string) error {
	if m.CreateAuthRecordFn != nil {
		return m.CreateAuthRecordFn(ctx, userID, scheme, secret, uname)
	}
	return nil
}

func (m *MockStore) GetAuthByUsername(ctx context.Context, username string) (*AuthRecord, error) {
	if m.GetAuthByUsernameFn != nil {
		return m.GetAuthByUsernameFn(ctx, username)
	}
	return nil, nil
}

func (m *MockStore) GetAuthByUserID(ctx context.Context, userID uuid.UUID) (*AuthRecord, error) {
	if m.GetAuthByUserIDFn != nil {
		return m.GetAuthByUserIDFn(ctx, userID)
	}
	return nil, nil
}

func (m *MockStore) GetUserUsername(ctx context.Context, userID uuid.UUID) (string, error) {
	if m.GetUserUsernameFn != nil {
		return m.GetUserUsernameFn(ctx, userID)
	}
	return "", nil
}

func (m *MockStore) UsernameExists(ctx context.Context, username string) (bool, error) {
	if m.UsernameExistsFn != nil {
		return m.UsernameExistsFn(ctx, username)
	}
	return false, nil
}

func (m *MockStore) UpdatePassword(ctx context.Context, userID uuid.UUID, hashedPassword string) error {
	if m.UpdatePasswordFn != nil {
		return m.UpdatePasswordFn(ctx, userID, hashedPassword)
	}
	return nil
}

func (m *MockStore) ClearMustChangePassword(ctx context.Context, userID uuid.UUID) error {
	if m.ClearMustChangePasswordFn != nil {
		return m.ClearMustChangePasswordFn(ctx, userID)
	}
	return nil
}

func (m *MockStore) SetEmailVerificationToken(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) error {
	if m.SetEmailVerificationTokenFn != nil {
		return m.SetEmailVerificationTokenFn(ctx, userID, token, expiresAt)
	}
	return nil
}

func (m *MockStore) VerifyEmailByToken(ctx context.Context, token string) (*uuid.UUID, error) {
	if m.VerifyEmailByTokenFn != nil {
		return m.VerifyEmailByTokenFn(ctx, token)
	}
	return nil, nil
}

func (m *MockStore) MarkEmailVerified(ctx context.Context, userID uuid.UUID) error {
	if m.MarkEmailVerifiedFn != nil {
		return m.MarkEmailVerifiedFn(ctx, userID)
	}
	return nil
}

func (m *MockStore) CreateDM(ctx context.Context, user1ID, user2ID uuid.UUID) (*Conversation, bool, error) {
	if m.CreateDMFn != nil {
		return m.CreateDMFn(ctx, user1ID, user2ID)
	}
	return &Conversation{ID: uuid.New(), Type: "dm"}, true, nil
}

func (m *MockStore) CreateRoom(ctx context.Context, ownerID uuid.UUID, public json.RawMessage) (*Conversation, error) {
	if m.CreateRoomFn != nil {
		return m.CreateRoomFn(ctx, ownerID, public)
	}
	return &Conversation{ID: uuid.New(), Type: "room", OwnerID: &ownerID}, nil
}

func (m *MockStore) GetConversationByID(ctx context.Context, id uuid.UUID) (*Conversation, error) {
	if m.GetConversationByIDFn != nil {
		return m.GetConversationByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *MockStore) GetUserConversations(ctx context.Context, userID uuid.UUID) ([]ConversationWithMember, error) {
	if m.GetUserConversationsFn != nil {
		return m.GetUserConversationsFn(ctx, userID)
	}
	return nil, nil
}

func (m *MockStore) GetDMOtherUser(ctx context.Context, convID, userID uuid.UUID) (*User, error) {
	if m.GetDMOtherUserFn != nil {
		return m.GetDMOtherUserFn(ctx, convID, userID)
	}
	return nil, nil
}

func (m *MockStore) GetConversationMembers(ctx context.Context, convID uuid.UUID) ([]uuid.UUID, error) {
	if m.GetConversationMembersFn != nil {
		return m.GetConversationMembersFn(ctx, convID)
	}
	return nil, nil
}

func (m *MockStore) GetMember(ctx context.Context, convID, userID uuid.UUID) (*Member, error) {
	if m.GetMemberFn != nil {
		return m.GetMemberFn(ctx, convID, userID)
	}
	return nil, nil
}

func (m *MockStore) IsMember(ctx context.Context, convID, userID uuid.UUID) (bool, error) {
	if m.IsMemberFn != nil {
		return m.IsMemberFn(ctx, convID, userID)
	}
	return true, nil
}

func (m *MockStore) IsBlocked(ctx context.Context, convID, blockerID, blockedID uuid.UUID) (bool, error) {
	if m.IsBlockedFn != nil {
		return m.IsBlockedFn(ctx, convID, blockerID, blockedID)
	}
	return false, nil
}

func (m *MockStore) UpdateMemberSettings(ctx context.Context, convID, userID uuid.UUID, settings MemberSettings) error {
	if m.UpdateMemberSettingsFn != nil {
		return m.UpdateMemberSettingsFn(ctx, convID, userID, settings)
	}
	return nil
}

func (m *MockStore) UpdateReadSeq(ctx context.Context, convID, userID uuid.UUID, seq int) error {
	if m.UpdateReadSeqFn != nil {
		return m.UpdateReadSeqFn(ctx, convID, userID, seq)
	}
	return nil
}

func (m *MockStore) UpdateRecvSeq(ctx context.Context, convID, userID uuid.UUID, seq int) error {
	if m.UpdateRecvSeqFn != nil {
		return m.UpdateRecvSeqFn(ctx, convID, userID, seq)
	}
	return nil
}

func (m *MockStore) UpdateClearSeq(ctx context.Context, convID, userID uuid.UUID, seq int) error {
	if m.UpdateClearSeqFn != nil {
		return m.UpdateClearSeqFn(ctx, convID, userID, seq)
	}
	return nil
}

func (m *MockStore) GetReadReceipts(ctx context.Context, convID uuid.UUID) ([]ReadReceipt, error) {
	if m.GetReadReceiptsFn != nil {
		return m.GetReadReceiptsFn(ctx, convID)
	}
	return nil, nil
}

func (m *MockStore) AddRoomMember(ctx context.Context, convID, userID uuid.UUID, role string) error {
	if m.AddRoomMemberFn != nil {
		return m.AddRoomMemberFn(ctx, convID, userID, role)
	}
	return nil
}

func (m *MockStore) RemoveMember(ctx context.Context, convID, userID uuid.UUID) error {
	if m.RemoveMemberFn != nil {
		return m.RemoveMemberFn(ctx, convID, userID)
	}
	return nil
}

func (m *MockStore) GetMemberRole(ctx context.Context, convID, userID uuid.UUID) (string, error) {
	if m.GetMemberRoleFn != nil {
		return m.GetMemberRoleFn(ctx, convID, userID)
	}
	return "member", nil
}

func (m *MockStore) UpdateRoomPublic(ctx context.Context, convID uuid.UUID, public json.RawMessage) error {
	if m.UpdateRoomPublicFn != nil {
		return m.UpdateRoomPublicFn(ctx, convID, public)
	}
	return nil
}

func (m *MockStore) SetPinnedMessage(ctx context.Context, convID uuid.UUID, messageID *uuid.UUID, pinnedBy uuid.UUID) error {
	if m.SetPinnedMessageFn != nil {
		return m.SetPinnedMessageFn(ctx, convID, messageID, pinnedBy)
	}
	return nil
}

func (m *MockStore) GetPinnedMessageSeq(ctx context.Context, convID uuid.UUID) (*int, error) {
	if m.GetPinnedMessageSeqFn != nil {
		return m.GetPinnedMessageSeqFn(ctx, convID)
	}
	return nil, nil
}

func (m *MockStore) UpdateConversationDisappearingTTL(ctx context.Context, convID uuid.UUID, ttl *int) error {
	if m.UpdateConversationDisappearingTTLFn != nil {
		return m.UpdateConversationDisappearingTTLFn(ctx, convID, ttl)
	}
	return nil
}

func (m *MockStore) GetConversationDisappearingTTL(ctx context.Context, convID uuid.UUID) (*int, error) {
	if m.GetConversationDisappearingTTLFn != nil {
		return m.GetConversationDisappearingTTLFn(ctx, convID)
	}
	return nil, nil
}

func (m *MockStore) CreateMessage(ctx context.Context, convID, fromUserID uuid.UUID, content []byte, head json.RawMessage) (*Message, error) {
	if m.CreateMessageFn != nil {
		return m.CreateMessageFn(ctx, convID, fromUserID, content, head)
	}
	return &Message{ID: uuid.New(), ConversationID: convID, FromUserID: fromUserID, Seq: 1}, nil
}

func (m *MockStore) GetMessages(ctx context.Context, convID, userID uuid.UUID, before, limit int, clearSeq int) ([]Message, error) {
	if m.GetMessagesFn != nil {
		return m.GetMessagesFn(ctx, convID, userID, before, limit, clearSeq)
	}
	return nil, nil
}

func (m *MockStore) GetMessageBySeq(ctx context.Context, convID uuid.UUID, seq int) (*Message, error) {
	if m.GetMessageBySeqFn != nil {
		return m.GetMessageBySeqFn(ctx, convID, seq)
	}
	return nil, nil
}

func (m *MockStore) EditMessage(ctx context.Context, convID uuid.UUID, seq int, content []byte) error {
	if m.EditMessageFn != nil {
		return m.EditMessageFn(ctx, convID, seq, content)
	}
	return nil
}

func (m *MockStore) UnsendMessage(ctx context.Context, convID uuid.UUID, seq int) error {
	if m.UnsendMessageFn != nil {
		return m.UnsendMessageFn(ctx, convID, seq)
	}
	return nil
}

func (m *MockStore) DeleteMessageForEveryone(ctx context.Context, convID uuid.UUID, seq int) error {
	if m.DeleteMessageForEveryoneFn != nil {
		return m.DeleteMessageForEveryoneFn(ctx, convID, seq)
	}
	return nil
}

func (m *MockStore) DeleteMessageForUser(ctx context.Context, msgID, userID uuid.UUID) error {
	if m.DeleteMessageForUserFn != nil {
		return m.DeleteMessageForUserFn(ctx, msgID, userID)
	}
	return nil
}

func (m *MockStore) AddReaction(ctx context.Context, convID uuid.UUID, seq int, userID uuid.UUID, emoji string) error {
	if m.AddReactionFn != nil {
		return m.AddReactionFn(ctx, convID, seq, userID, emoji)
	}
	return nil
}

func (m *MockStore) GetEditCount(ctx context.Context, convID uuid.UUID, seq int) (int, error) {
	if m.GetEditCountFn != nil {
		return m.GetEditCountFn(ctx, convID, seq)
	}
	return 0, nil
}

func (m *MockStore) CreateMessageWithViewOnce(ctx context.Context, convID, fromUserID uuid.UUID, content []byte, head json.RawMessage, viewOnce bool, viewOnceTTL *int) (*Message, error) {
	if m.CreateMessageWithViewOnceFn != nil {
		return m.CreateMessageWithViewOnceFn(ctx, convID, fromUserID, content, head, viewOnce, viewOnceTTL)
	}
	return &Message{ID: uuid.New(), ConversationID: convID, FromUserID: fromUserID, Seq: 1, ViewOnce: viewOnce, ViewOnceTTL: viewOnceTTL}, nil
}

func (m *MockStore) RecordMessageRead(ctx context.Context, messageID, userID uuid.UUID) (*MessageRead, error) {
	if m.RecordMessageReadFn != nil {
		return m.RecordMessageReadFn(ctx, messageID, userID)
	}
	return nil, nil
}

func (m *MockStore) GetMessageRead(ctx context.Context, messageID, userID uuid.UUID) (*MessageRead, error) {
	if m.GetMessageReadFn != nil {
		return m.GetMessageReadFn(ctx, messageID, userID)
	}
	return nil, nil
}

func (m *MockStore) GetMessageByID(ctx context.Context, messageID uuid.UUID) (*Message, error) {
	if m.GetMessageByIDFn != nil {
		return m.GetMessageByIDFn(ctx, messageID)
	}
	return nil, nil
}

func (m *MockStore) IsMessageExpiredForUser(ctx context.Context, messageID, userID uuid.UUID) (bool, error) {
	if m.IsMessageExpiredForUserFn != nil {
		return m.IsMessageExpiredForUserFn(ctx, messageID, userID)
	}
	return false, nil
}

func (m *MockStore) ExpireReadMessages(ctx context.Context) (int64, error) {
	if m.ExpireReadMessagesFn != nil {
		return m.ExpireReadMessagesFn(ctx)
	}
	return 0, nil
}

func (m *MockStore) CreateFile(ctx context.Context, uploaderID uuid.UUID, mimeType string, size int64, location string) (*File, error) {
	if m.CreateFileFn != nil {
		return m.CreateFileFn(ctx, uploaderID, mimeType, size, location)
	}
	return &File{ID: uuid.New(), UploaderID: uploaderID, MimeType: mimeType, Size: size}, nil
}

func (m *MockStore) CreateFileWithHash(ctx context.Context, uploaderID uuid.UUID, mimeType string, size int64, location, hash, originalName string) (*File, error) {
	if m.CreateFileWithHashFn != nil {
		return m.CreateFileWithHashFn(ctx, uploaderID, mimeType, size, location, hash, originalName)
	}
	return &File{ID: uuid.New(), UploaderID: uploaderID, MimeType: mimeType, Size: size}, nil
}

func (m *MockStore) CreateFileWithID(ctx context.Context, fileID, uploaderID uuid.UUID, mimeType string, size int64, location, hash, originalName string) (*File, error) {
	if m.CreateFileWithIDFn != nil {
		return m.CreateFileWithIDFn(ctx, fileID, uploaderID, mimeType, size, location, hash, originalName)
	}
	return &File{ID: fileID, UploaderID: uploaderID, MimeType: mimeType, Size: size}, nil
}

func (m *MockStore) GetFileByID(ctx context.Context, id uuid.UUID) (*File, error) {
	if m.GetFileByIDFn != nil {
		return m.GetFileByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *MockStore) GetFileByHash(ctx context.Context, hash string) (*File, error) {
	if m.GetFileByHashFn != nil {
		return m.GetFileByHashFn(ctx, hash)
	}
	return nil, nil
}

func (m *MockStore) GetFileWithMetadata(ctx context.Context, id uuid.UUID) (*FileWithMetadata, error) {
	if m.GetFileWithMetadataFn != nil {
		return m.GetFileWithMetadataFn(ctx, id)
	}
	return nil, nil
}

func (m *MockStore) GetFileMetadata(ctx context.Context, fileID uuid.UUID) (*FileMetadata, error) {
	if m.GetFileMetadataFn != nil {
		return m.GetFileMetadataFn(ctx, fileID)
	}
	return nil, nil
}

func (m *MockStore) CreateFileMetadata(ctx context.Context, fileID uuid.UUID, width, height *int, duration *float64, thumbnail *string, extra json.RawMessage) error {
	if m.CreateFileMetadataFn != nil {
		return m.CreateFileMetadataFn(ctx, fileID, width, height, duration, thumbnail, extra)
	}
	return nil
}

func (m *MockStore) UpdateFileStatus(ctx context.Context, fileID uuid.UUID, status string) error {
	if m.UpdateFileStatusFn != nil {
		return m.UpdateFileStatusFn(ctx, fileID, status)
	}
	return nil
}

func (m *MockStore) UpdateFileLocation(ctx context.Context, fileID uuid.UUID, location string) error {
	if m.UpdateFileLocationFn != nil {
		return m.UpdateFileLocationFn(ctx, fileID, location)
	}
	return nil
}

func (m *MockStore) DeleteFile(ctx context.Context, fileID uuid.UUID) error {
	if m.DeleteFileFn != nil {
		return m.DeleteFileFn(ctx, fileID)
	}
	return nil
}

func (m *MockStore) CanAccessFile(ctx context.Context, fileID, userID uuid.UUID) (bool, error) {
	if m.CanAccessFileFn != nil {
		return m.CanAccessFileFn(ctx, fileID, userID)
	}
	return true, nil
}

func (m *MockStore) CreateInviteCode(ctx context.Context, inviterID uuid.UUID, code, token, email string, inviteeName *string) (*InviteCode, error) {
	if m.CreateInviteCodeFn != nil {
		return m.CreateInviteCodeFn(ctx, inviterID, code, token, email, inviteeName)
	}
	return &InviteCode{ID: uuid.New(), InviterID: inviterID, Code: code, Email: email}, nil
}

func (m *MockStore) GetInviteByID(ctx context.Context, id uuid.UUID) (*InviteCode, error) {
	if m.GetInviteByIDFn != nil {
		return m.GetInviteByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *MockStore) GetInviteByCode(ctx context.Context, code string) (*InviteCode, error) {
	if m.GetInviteByCodeFn != nil {
		return m.GetInviteByCodeFn(ctx, code)
	}
	return nil, nil
}

func (m *MockStore) GetPendingInviteByUsernames(ctx context.Context, inviterUsername, inviteeEmail string) (*InviteCode, error) {
	if m.GetPendingInviteByUsernamesFn != nil {
		return m.GetPendingInviteByUsernamesFn(ctx, inviterUsername, inviteeEmail)
	}
	return nil, nil
}

func (m *MockStore) GetPendingInvitesByEmail(ctx context.Context, email string) ([]InviteCode, error) {
	if m.GetPendingInvitesByEmailFn != nil {
		return m.GetPendingInvitesByEmailFn(ctx, email)
	}
	return nil, nil
}

func (m *MockStore) GetUserInvites(ctx context.Context, userID uuid.UUID) ([]*InviteCode, error) {
	if m.GetUserInvitesFn != nil {
		return m.GetUserInvitesFn(ctx, userID)
	}
	return nil, nil
}

func (m *MockStore) UseInvite(ctx context.Context, inviteID, usedByID uuid.UUID) (*InviteCode, error) {
	if m.UseInviteFn != nil {
		return m.UseInviteFn(ctx, inviteID, usedByID)
	}
	return nil, nil
}

func (m *MockStore) RevokeInvite(ctx context.Context, inviteID uuid.UUID, inviterID uuid.UUID) error {
	if m.RevokeInviteFn != nil {
		return m.RevokeInviteFn(ctx, inviteID, inviterID)
	}
	return nil
}

func (m *MockStore) ExpireOldInvites(ctx context.Context) (int64, error) {
	if m.ExpireOldInvitesFn != nil {
		return m.ExpireOldInvitesFn(ctx)
	}
	return 0, nil
}

func (m *MockStore) AddContact(ctx context.Context, userID, contactID uuid.UUID, source string, inviteID *uuid.UUID) error {
	if m.AddContactFn != nil {
		return m.AddContactFn(ctx, userID, contactID, source, inviteID)
	}
	return nil
}

func (m *MockStore) GetContacts(ctx context.Context, userID uuid.UUID) ([]Contact, error) {
	if m.GetContactsFn != nil {
		return m.GetContactsFn(ctx, userID)
	}
	return nil, nil
}

func (m *MockStore) IsContact(ctx context.Context, userID, contactID uuid.UUID) (bool, error) {
	if m.IsContactFn != nil {
		return m.IsContactFn(ctx, userID, contactID)
	}
	return false, nil
}

func (m *MockStore) UpdateContactNickname(ctx context.Context, userID, contactID uuid.UUID, nickname *string) error {
	if m.UpdateContactNicknameFn != nil {
		return m.UpdateContactNicknameFn(ctx, userID, contactID, nickname)
	}
	return nil
}

func (m *MockStore) RemoveContact(ctx context.Context, userID, contactID uuid.UUID) error {
	if m.RemoveContactFn != nil {
		return m.RemoveContactFn(ctx, userID, contactID)
	}
	return nil
}
