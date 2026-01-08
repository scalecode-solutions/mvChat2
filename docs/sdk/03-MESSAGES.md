# Messages

> **IMPORTANT**: This documentation is accurate as of January 8th, 2026 at 4:00 AM CST. Review the actual code before assuming anything here is still current.

## Overview

Messages in mvChat2 use the Irido format - a structured JSON format supporting text, media, replies, and mentions.

## Sending Messages

### Simple Text

```typescript
await client.sendMessage(conversationId, {
  text: 'Hello, world!',
});
```

### With Media

```typescript
await client.sendMessage(conversationId, {
  text: 'Check out this photo',
  media: [
    {
      type: 'image',
      ref: 'file-uuid',
      name: 'photo.jpg',
      mime: 'image/jpeg',
      size: 102400,
      width: 1920,
      height: 1080,
    },
  ],
});
```

### Reply to Message

```typescript
await client.sendMessage(conversationId, {
  text: 'I agree!',
  replyTo: 5, // seq of message being replied to
});
```

### With Mentions

```typescript
await client.sendMessage(conversationId, {
  text: 'Hey @Bob, what do you think?',
  mentions: [
    { userId: 'bob-uuid', username: 'Bob', offset: 4, length: 4 },
  ],
});
```

## Receiving Messages

```typescript
client.on('message', (message: Message) => {
  console.log('New message:', message);
  // {
  //   conv: 'conversation-uuid',
  //   seq: 10,
  //   from: 'alice-uuid',
  //   ts: '2026-01-08T04:00:00Z',
  //   content: { v: 1, text: 'Hello!' },
  // }
});
```

## Fetching Message History

```typescript
const messages = await client.getMessages(conversationId, {
  limit: 50,
  before: 100, // Get messages before seq 100
});
```

## Editing Messages

```typescript
await client.editMessage(conversationId, seq, {
  text: 'Updated message text',
});

// Listen for edits
client.on('edit', (info) => {
  // { conv: 'uuid', seq: 5, from: 'alice-uuid', content: {...} }
});
```

## Unsending Messages (Delete for Everyone)

```typescript
await client.unsendMessage(conversationId, seq);

// Listen for unsends
client.on('unsend', (info) => {
  // { conv: 'uuid', seq: 5, from: 'alice-uuid' }
});
```

## Deleting Messages (Delete for Me)

```typescript
await client.deleteMessages(conversationId, [1, 2, 3]);
```

## Reactions

```typescript
// Add reaction
await client.react(conversationId, seq, '👍');

// Remove reaction
await client.react(conversationId, seq, '👍', true); // remove = true

// Listen for reactions
client.on('react', (info) => {
  // { conv: 'uuid', seq: 5, from: 'alice-uuid', emoji: '👍' }
});
```

## React Hook

```typescript
import { useMessages } from '@mvchat/react-native-sdk';

function ChatScreen({ conversationId }) {
  const {
    messages,
    isLoading,
    hasMore,
    loadMore,
    sendMessage,
    editMessage,
    unsendMessage,
    deleteMessages,
    react,
  } = useMessages(client, conversationId);

  return (
    <FlatList
      data={messages}
      inverted
      onEndReached={loadMore}
      renderItem={({ item }) => <MessageBubble message={item} />}
    />
  );
}
```

## Irido Format Reference

```typescript
interface Irido {
  v: 1;                          // Version
  text?: string;                 // Markdown text
  media?: IridoMedia[];          // Attached media (max 10)
  reply?: IridoReply;            // Reply reference
  mentions?: IridoMention[];     // @mentions
}

interface IridoMedia {
  type: 'image' | 'video' | 'audio' | 'file';
  ref: string;                   // File UUID
  name: string;                  // Filename
  mime: string;                  // MIME type
  size: number;                  // Bytes
  width?: number;                // For images/video
  height?: number;
  duration?: number;             // For audio/video (seconds)
}

interface IridoReply {
  seq: number;                   // Message being replied to
  preview?: string;              // Preview text
}

interface IridoMention {
  userId: string;
  username: string;
  offset: number;                // Character position in text
  length: number;
}
```

## Wire Protocol

### Send Message
```json
{
  "id": "1",
  "send": {
    "conv": "conversation-uuid",
    "content": {
      "v": 1,
      "text": "Hello!"
    },
    "replyTo": 5
  }
}
```

### Edit Message
```json
{
  "id": "2",
  "edit": {
    "conv": "conversation-uuid",
    "seq": 10,
    "content": {
      "v": 1,
      "text": "Updated text"
    }
  }
}
```

### Unsend Message
```json
{
  "id": "3",
  "unsend": {
    "conv": "conversation-uuid",
    "seq": 10
  }
}
```

### React
```json
{
  "id": "4",
  "react": {
    "conv": "conversation-uuid",
    "seq": 10,
    "emoji": "👍"
  }
}
```
