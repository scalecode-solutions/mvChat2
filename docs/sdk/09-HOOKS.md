# React Hooks Reference

> **IMPORTANT**: This documentation is accurate as of January 8th, 2026 at 4:00 AM CST. Review the actual code before assuming anything here is still current.

## Overview

The SDK provides React hooks for easy integration with React Native components.

## useClient

Main client hook for connection state.

```typescript
import { useClient } from '@mvchat/react-native-sdk';

const {
  isConnected,      // boolean
  state,            // 'disconnected' | 'connecting' | 'connected' | 'reconnecting'
  error,            // Error | null
  connect,          // () => Promise<void>
  disconnect,       // () => void
} = useClient(client);
```

## useAuth

Authentication state and actions.

```typescript
import { useAuth } from '@mvchat/react-native-sdk';

const {
  isAuthenticated,  // boolean
  user,             // User | null
  isLoading,        // boolean
  error,            // Error | null
  login,            // (credentials) => Promise<void>
  signup,           // (data) => Promise<void>
  logout,           // () => Promise<void>
  loginWithToken,   // (token) => Promise<void>
} = useAuth(client);
```

## useConversations

Conversation list management.

```typescript
import { useConversations } from '@mvchat/react-native-sdk';

const {
  conversations,    // Conversation[]
  isLoading,        // boolean
  error,            // Error | null
  refresh,          // () => Promise<void>
  startDM,          // (userId) => Promise<Conversation>
  createRoom,       // (options) => Promise<Conversation>
} = useConversations(client);
```

## useMessages

Messages for a specific conversation.

```typescript
import { useMessages } from '@mvchat/react-native-sdk';

const {
  messages,         // Message[]
  isLoading,        // boolean
  hasMore,          // boolean
  error,            // Error | null
  loadMore,         // () => Promise<void>
  sendMessage,      // (content) => Promise<void>
  editMessage,      // (seq, content) => Promise<void>
  unsendMessage,    // (seq) => Promise<void>
  deleteMessages,   // (seqs) => Promise<void>
  react,            // (seq, emoji, remove?) => Promise<void>
} = useMessages(client, conversationId);
```

## useContacts

Contact list management.

```typescript
import { useContacts } from '@mvchat/react-native-sdk';

const {
  contacts,         // Contact[]
  isLoading,        // boolean
  error,            // Error | null
  refresh,          // () => Promise<void>
  addContact,       // (userId) => Promise<void>
  removeContact,    // (userId) => Promise<void>
  updateNickname,   // (userId, nickname) => Promise<void>
} = useContacts(client);
```

## usePresence

Online status for a user.

```typescript
import { usePresence } from '@mvchat/react-native-sdk';

const {
  isOnline,         // boolean
  lastSeen,         // Date | null
} = usePresence(client, userId);
```

## useTyping

Typing indicators for a conversation.

```typescript
import { useTyping } from '@mvchat/react-native-sdk';

const {
  typingUsers,      // string[] (user IDs)
  sendTyping,       // () => void (debounced)
} = useTyping(client, conversationId);
```

## useFileUpload

File upload with progress.

```typescript
import { useFileUpload } from '@mvchat/react-native-sdk';

const {
  upload,           // (file) => Promise<FileRef>
  isUploading,      // boolean
  progress,         // number (0-100)
  error,            // Error | null
  cancel,           // () => void
} = useFileUpload(client);
```

## useSearch

User search.

```typescript
import { useSearch } from '@mvchat/react-native-sdk';

const {
  results,          // User[]
  isSearching,      // boolean
  search,           // (query) => Promise<void>
  clear,            // () => void
} = useSearch(client);
```

## Context Provider

Wrap your app with the provider for global access:

```typescript
import { MVChat2Provider, useClient } from '@mvchat/react-native-sdk';

function App() {
  return (
    <MVChat2Provider client={client}>
      <MainApp />
    </MVChat2Provider>
  );
}

// In any component
function SomeComponent() {
  const { isConnected } = useClient();
  // ...
}
```

## Hook Dependencies

All hooks automatically:
- Subscribe to relevant events
- Update when data changes
- Clean up on unmount
- Handle reconnection

No manual subscription management needed.
