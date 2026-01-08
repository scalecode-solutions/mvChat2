# mvChat2 React Native SDK

> **IMPORTANT**: This documentation is accurate as of January 8th, 2026 at 4:00 AM CST. Things may have changed since then. Always review the actual SDK code and mvChat2 backend before assuming anything documented here is still current.

## Overview

The mvChat2 React Native SDK provides a simple, type-safe way to integrate mvChat2 messaging into React Native applications. It handles WebSocket connections, authentication, reconnection, message parsing, and E2EE.

## Package Structure

```
@mvchat/react-native-sdk/
├── src/
│   ├── index.ts              # Main exports
│   ├── client.ts             # MVChat2Client class
│   ├── connection.ts         # WebSocket connection manager
│   ├── auth.ts               # Authentication helpers
│   ├── messages.ts           # Message types and handlers
│   ├── conversations.ts      # Conversation management
│   ├── contacts.ts           # Contact management
│   ├── files.ts              # File upload/download
│   ├── presence.ts           # Online/offline, typing
│   ├── crypto.ts             # E2EE encryption/decryption
│   ├── hooks/
│   │   ├── useClient.ts      # Main client hook
│   │   ├── useConversations.ts
│   │   ├── useMessages.ts
│   │   ├── useContacts.ts
│   │   ├── usePresence.ts
│   │   └── useTyping.ts
│   └── types/
│       ├── client.ts
│       ├── messages.ts
│       ├── conversations.ts
│       ├── contacts.ts
│       └── irido.ts          # Irido message format types
├── package.json
├── tsconfig.json
└── README.md
```

## Installation

```bash
npm install @mvchat/react-native-sdk
# or
yarn add @mvchat/react-native-sdk
```

## Quick Start

```typescript
import { MVChat2Client, useClient, useMessages } from '@mvchat/react-native-sdk';

// Initialize client
const client = new MVChat2Client({
  url: 'wss://api2.mvchat.app/v0/ws',
  autoReconnect: true,
  compression: true,
});

// In your component
function ChatScreen({ conversationId }) {
  const { isConnected, user } = useClient(client);
  const { messages, sendMessage, loadMore } = useMessages(client, conversationId);

  const handleSend = (text: string) => {
    sendMessage({ text });
  };

  return (
    <ChatView
      messages={messages}
      onSend={handleSend}
      onLoadMore={loadMore}
    />
  );
}
```

## Documentation

- [01-CONNECTION.md](./01-CONNECTION.md) - WebSocket connection and reconnection
- [02-AUTHENTICATION.md](./02-AUTHENTICATION.md) - Login, signup, token management
- [03-MESSAGES.md](./03-MESSAGES.md) - Sending, receiving, editing, deleting messages
- [04-CONVERSATIONS.md](./04-CONVERSATIONS.md) - DMs and rooms
- [05-CONTACTS.md](./05-CONTACTS.md) - Contact management
- [06-PRESENCE.md](./06-PRESENCE.md) - Online status, typing indicators
- [07-FILES.md](./07-FILES.md) - File upload/download
- [08-CRYPTO.md](./08-CRYPTO.md) - E2EE implementation
- [09-HOOKS.md](./09-HOOKS.md) - React hooks reference
- [10-TYPES.md](./10-TYPES.md) - TypeScript type definitions

## Versioning

The SDK follows semantic versioning:
- **Major** (1.0.0 → 2.0.0): Breaking changes to API
- **Minor** (1.0.0 → 1.1.0): New features, backward compatible
- **Patch** (1.0.0 → 1.0.1): Bug fixes

When mvChat2 backend changes:
1. SDK is updated to match
2. Version is bumped appropriately
3. Changelog documents what changed
4. Apps update SDK: `npm update @mvchat/react-native-sdk`

## License

MIT
