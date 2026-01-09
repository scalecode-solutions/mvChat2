# SDK Gap Analysis

This document identifies gaps between the mvChat2 SDK and what's needed for a complete React Native chat application.

## Critical Gaps (Implemented)

| Feature | Method/Hook | Status |
|---------|-------------|--------|
| Get single user profile | `getUser(userId)` | Added |
| Get single conversation | `getConversation(convId)` | Added |
| Reactive room members | `useMembers` hook | Added |
| Reactive read receipts | `useReadReceipts` hook | Added |

## Medium Priority (Future)

| Missing | Impact | Notes |
|---------|--------|-------|
| Message search | Can't search within chats | Requires backend `search` endpoint for messages |
| Conversation filtering | Can't filter by type/muted/etc | `getConversations(options)` with filters |
| Global user blocking | Only DM-level blocking | Need `blockUser(userId)` endpoint |
| Room role management | Can't promote/demote members | Need backend role update endpoint |
| Explicit reactions API | Reactions buried in message head | `getReactions(convId, seq)` method |
| Batch operations | No multi-select delete/read | `markMultipleRead()`, `deleteMultiple()` |

## Lower Priority / App-Level Concerns

| Feature | Notes |
|---------|-------|
| Upload progress callbacks | App can implement with fetch event listeners |
| Token persistence | App responsibility (use react-native-keychain) |
| Offline message queue | App-level architectural decision |
| E2EE encryption/decryption | Client-side implementation, SDK provides transport |
| Audio/video calls | Backend not ready yet (see docs/audio-calls.md) |
| Rich presence (busy/away/dnd) | Backend only supports online/offline |
| Message threading UI | `replyTo` exists, but no `getThread()` to fetch context |

## Comparison with Stream Chat / SendBird

Features those SDKs have that we don't:

| Feature | Priority | Notes |
|---------|----------|-------|
| User status (busy, away, dnd) | Low | Would require backend changes |
| Message reporting | Medium | `reportMessage(convId, seq, reason)` |
| Shadow banning | Low | Admin feature |
| Multiple pinned messages | Low | Currently one per conversation |
| Resumable uploads | Medium | For large file uploads |
| Connection quality indicators | Low | Nice to have |

## Implementation Status

### Hooks Available

| Hook | Purpose | Complete |
|------|---------|----------|
| `useClient` | Connection management | Yes |
| `useAuth` | Authentication + profile | Yes |
| `useMessages` | Message CRUD + reactions | Yes |
| `useConversations` | Conversations + rooms | Yes |
| `useContacts` | Contact management | Yes |
| `useTyping` | Typing indicators | Yes |
| `useInvites` | Invite management | Yes |
| `usePresence` | Online/offline tracking | Yes |
| `useMembers` | Room member management | Yes |
| `useReadReceipts` | Read/delivery receipts | Yes |

### Client Methods Count

- Authentication: 7 methods
- Conversations: 12 methods
- Messages: 10 methods
- Contacts: 4 methods
- Invites: 4 methods
- Files: 3 methods
- Pins: 2 methods
- Disappearing: 2 methods
- **Total: 44+ methods**

## Conclusion

The SDK provides complete coverage for building a production React Native chat application. The remaining gaps are either:
1. Features requiring backend changes
2. App-level concerns (caching, offline, encryption)
3. Nice-to-have features for v2
