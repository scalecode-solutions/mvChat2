# Conversations

> **IMPORTANT**: This documentation is accurate as of January 8th, 2026 at 4:00 AM CST. Review the actual code before assuming anything here is still current.

## Overview

mvChat2 supports two conversation types:
- **DMs**: 1:1 direct messages
- **Rooms**: Multi-user chat rooms (private by default)

## Fetching Conversations

```typescript
const conversations = await client.getConversations();

// Returns array of:
// {
//   id: 'conversation-uuid',
//   type: 'dm' | 'room',
//   createdAt: '2026-01-08T04:00:00Z',
//   updatedAt: '2026-01-08T04:00:00Z',
//   lastSeq: 50,
//   lastMsgAt: '2026-01-08T04:00:00Z',
//   public: { fn: 'Room Name' },  // For rooms
//   readSeq: 45,
//   recvSeq: 50,
//   unread: 5,
//   // For DMs, includes other user info
//   otherUser: { id: 'uuid', public: { fn: 'Bob' } },
// }
```

## Starting a DM

```typescript
const dm = await client.startDM('other-user-uuid');

// Returns conversation object
// If DM already exists, returns existing conversation
```

## Creating a Room

```typescript
const room = await client.createRoom({
  public: {
    fn: 'Support Team',
    description: 'A room for support discussions',
  },
});

// Returns:
// { conv: 'room-uuid', public: { fn: 'Support Team', ... } }
```

## Managing DM Settings

```typescript
await client.updateDM(conversationId, {
  favorite: true,
  muted: false,
  blocked: false,
});
```

## Room Actions (Future)

```typescript
// Invite user to room
await client.inviteToRoom(roomId, 'user-uuid');

// Leave room
await client.leaveRoom(roomId);

// Kick user (admin only)
await client.kickFromRoom(roomId, 'user-uuid');

// Update room info (owner only)
await client.updateRoom(roomId, {
  public: { fn: 'New Room Name' },
});
```

## Getting Room Members

```typescript
const members = await client.getMembers(conversationId);

// Returns:
// [
//   { userId: 'uuid', role: 'owner', joinedAt: '...', public: {...} },
//   { userId: 'uuid', role: 'admin', joinedAt: '...', public: {...} },
//   { userId: 'uuid', role: 'member', joinedAt: '...', public: {...} },
// ]
```

## Read Receipts

```typescript
// Mark as read
await client.markRead(conversationId, seq);

// Get read receipts for all members
const receipts = await client.getReceipts(conversationId);

// Returns:
// [
//   { user: 'uuid', readSeq: 45, recvSeq: 50 },
//   { user: 'uuid', readSeq: 50, recvSeq: 50 },
// ]
```

## React Hook

```typescript
import { useConversations } from '@mvchat/react-native-sdk';

function ConversationList() {
  const {
    conversations,
    isLoading,
    refresh,
    startDM,
    createRoom,
  } = useConversations(client);

  return (
    <FlatList
      data={conversations}
      onRefresh={refresh}
      renderItem={({ item }) => (
        <ConversationRow
          conversation={item}
          onPress={() => navigation.navigate('Chat', { id: item.id })}
        />
      )}
    />
  );
}
```

## Wire Protocol

### Get Conversations
```json
{
  "id": "1",
  "get": {
    "what": "conversations"
  }
}
```

### Start DM
```json
{
  "id": "2",
  "dm": {
    "user": "other-user-uuid"
  }
}
```

### Create Room
```json
{
  "id": "3",
  "room": {
    "id": "new",
    "action": "create",
    "desc": {
      "public": { "fn": "Room Name" }
    }
  }
}
```

### Get Members
```json
{
  "id": "4",
  "get": {
    "what": "members",
    "conv": "conversation-uuid"
  }
}
```

### Get Read Receipts
```json
{
  "id": "5",
  "get": {
    "what": "receipts",
    "conv": "conversation-uuid"
  }
}
```

### Mark Read
```json
{
  "id": "6",
  "read": {
    "conv": "conversation-uuid",
    "seq": 50
  }
}
```
