# Presence

> **IMPORTANT**: This documentation is accurate as of January 8th, 2026 at 4:00 AM CST. Review the actual code before assuming anything here is still current.

## Overview

Presence in mvChat2 includes:
- **Online/offline status**: Is user currently connected?
- **Last seen**: When was user last online?
- **Typing indicators**: Is user typing in a conversation?

## Online Status

```typescript
// Check if a user is online
const isOnline = await client.isUserOnline('user-uuid');

// Get online status for multiple users
const statuses = await client.getOnlineStatus(['uuid1', 'uuid2', 'uuid3']);
// Returns: { 'uuid1': true, 'uuid2': false, 'uuid3': true }
```

## Listening for Presence Changes

```typescript
client.on('presence', (event) => {
  // { user: 'uuid', online: true }
  // or
  // { user: 'uuid', online: false, lastSeen: '2026-01-08T04:00:00Z' }
});
```

## Typing Indicators

### Sending Typing Status

```typescript
// Start typing
client.sendTyping(conversationId);

// SDK automatically sends typing every 3 seconds while user is typing
// and stops when they stop typing or send a message
```

### Receiving Typing Status

```typescript
client.on('typing', (event) => {
  // { conv: 'conversation-uuid', user: 'alice-uuid' }
});
```

## React Hooks

### usePresence

```typescript
import { usePresence } from '@mvchat/react-native-sdk';

function UserAvatar({ userId }) {
  const { isOnline, lastSeen } = usePresence(client, userId);

  return (
    <View>
      <Avatar userId={userId} />
      {isOnline ? (
        <OnlineDot />
      ) : (
        <Text>Last seen {formatLastSeen(lastSeen)}</Text>
      )}
    </View>
  );
}
```

### useTyping

```typescript
import { useTyping } from '@mvchat/react-native-sdk';

function ChatInput({ conversationId }) {
  const { typingUsers, sendTyping } = useTyping(client, conversationId);
  const [text, setText] = useState('');

  const handleTextChange = (newText) => {
    setText(newText);
    sendTyping(); // Debounced internally
  };

  return (
    <View>
      {typingUsers.length > 0 && (
        <TypingIndicator users={typingUsers} />
      )}
      <TextInput value={text} onChangeText={handleTextChange} />
    </View>
  );
}
```

## Wire Protocol

### Typing Indicator
```json
{
  "id": "1",
  "typing": {
    "conv": "conversation-uuid"
  }
}
```

### Presence Info (received)
```json
{
  "info": {
    "what": "presence",
    "user": "alice-uuid",
    "online": true
  }
}
```

### Typing Info (received)
```json
{
  "info": {
    "what": "typing",
    "conv": "conversation-uuid",
    "from": "alice-uuid"
  }
}
```

## Notes

- Typing indicators expire after 5 seconds of no updates
- Last seen is updated when user disconnects
- Online status is real-time via WebSocket
