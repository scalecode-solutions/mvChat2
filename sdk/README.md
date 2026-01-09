# mvChat2 React Native SDK

TypeScript SDK for mvChat2, a secure chat backend. Works with React Native and web applications.

## Installation

```bash
npm install @mvchat/react-native-sdk
# or
yarn add @mvchat/react-native-sdk
```

## Quick Start

```typescript
import { MVChat2Client } from '@mvchat/react-native-sdk';

// Create client
const client = new MVChat2Client({
  url: 'wss://api.example.com/v0/channels',
});

// Connect and authenticate
await client.connect();
const auth = await client.login({ username: 'alice', password: 'secret' });
console.log('Logged in as:', auth.user);

// Get conversations
const conversations = await client.getConversations();

// Send a message
await client.sendMessage(conversationId, { text: 'Hello!' });
```

## Configuration

```typescript
interface MVChat2ClientConfig {
  url: string;                    // WebSocket URL (required)
  autoReconnect?: boolean;        // Auto-reconnect on disconnect (default: true)
  reconnectDelay?: number;        // Initial reconnect delay ms (default: 1000)
  reconnectMaxDelay?: number;     // Max reconnect delay ms (default: 30000)
  reconnectBackoff?: number;      // Backoff multiplier (default: 1.5)
  compression?: boolean;          // Enable compression (default: true)
  timeout?: number;               // Request timeout ms (default: 10000)
  // Optional device/client info sent in handshake
  deviceId?: string;              // Unique device identifier
  lang?: string;                  // Language code (e.g., 'en', 'es')
  userAgent?: string;             // Custom user agent string
}
```

## Authentication

### Login with Username/Password

```typescript
const result = await client.login({
  username: 'alice',
  password: 'secret',
});

// Result includes:
// - user: string (user ID)
// - token: string (JWT for reconnection)
// - expires: string (token expiration)
// - mustChangePassword?: boolean (true if using temp password)
// - emailVerified?: boolean
```

### Login with Token (Reconnection)

```typescript
// Store token from initial login
const token = result.token;

// Later, reconnect with token
await client.connect();
await client.loginWithToken(token);
```

### Signup

```typescript
const result = await client.signup({
  username: 'bob',
  password: 'secret123',
  inviteCode: '1234567890',        // Optional invite code
  profile: { fn: 'Bob Smith' },    // Optional profile
  login: true,                     // Auto-login after signup (default: true)
});
```

### Change Password

```typescript
await client.changePassword({
  oldPassword: 'currentPassword',
  newPassword: 'newSecurePassword',
});
```

### Update Profile

```typescript
await client.updateProfile({
  fn: 'Alice Johnson',
  bio: 'Software developer',
  photo: 'https://example.com/photo.jpg',
});
```

## Conversations

### Get All Conversations

```typescript
const conversations = await client.getConversations();

// Each conversation includes:
// - id: string
// - type: 'dm' | 'room'
// - lastSeq: number
// - readSeq: number
// - unread: number
// - favorite?: boolean
// - muted?: boolean
// - lastMsgAt?: string
// - disappearingTTL?: number (seconds)
// - pinnedSeq?: number
// - pinnedAt?: string
// - pinnedBy?: string
// For DMs: user: { id, public, online, lastSeen }
// For rooms: public: { fn, description, photo }
```

### Start a DM

```typescript
const result = await client.startDM(userId);
// Returns:
// - conv: string (conversation ID)
// - created: boolean (true if new DM)
// - user: { id, public, online }
```

### Create a Room

```typescript
const result = await client.createRoom({
  public: {
    fn: 'Support Group',
    description: 'A safe space',
  },
});
// Returns: { conv: string, public: any }
```

### Room Management

```typescript
// Invite user to room (owner/admin only)
await client.inviteToRoom(roomId, userId);

// Leave room (members only, owner cannot leave)
await client.leaveRoom(roomId);

// Kick user from room (owner/admin only)
await client.kickFromRoom(roomId, userId);

// Update room info (owner/admin only)
await client.updateRoom(roomId, {
  public: { fn: 'New Name', description: 'Updated description' },
});
```

### DM Settings

```typescript
await client.updateDMSettings(convId, {
  favorite: true,
  muted: false,
  blocked: false,
  private: { notes: 'My private notes about this conversation' },
});
```

The `private` field stores data only visible to you (not the other user).

### Get Members

```typescript
const members = await client.getMembers(convId);
// Returns: Array<{ id, public, online, lastSeen }>
```

### Get Read Receipts

```typescript
const receipts = await client.getReceipts(convId);
// Returns: Array<{ user, readSeq, recvSeq }>
```

## Messages

### Get Messages

```typescript
// Get recent messages
const messages = await client.getMessages(convId);

// Pagination
const olderMessages = await client.getMessages(convId, {
  limit: 50,
  before: lastSeq, // Get messages before this seq
});

// Message format:
// - seq: number
// - from: string (user ID)
// - ts: string (timestamp)
// - content?: Irido (message content)
// - head?: object (metadata like reply_to)
// - deleted?: boolean
// - viewOnce?: boolean
```

### Send Message

```typescript
// Simple text
await client.sendMessage(convId, { text: 'Hello!' });

// With reply
await client.sendMessage(convId, {
  text: 'Great point!',
  replyTo: 42, // seq of message being replied to
});

// With media (see File Upload section)
await client.sendMessage(convId, {
  text: 'Check this out',
  media: [{
    type: 'image',
    ref: fileId,
    name: 'photo.jpg',
    mime: 'image/jpeg',
    size: 12345,
    width: 800,
    height: 600,
  }],
});

// View-once message (disappears after viewing)
await client.sendMessage(convId, {
  text: 'Secret message',
  viewOnce: true,
  viewOnceTTL: 30, // seconds: 10, 30, 60, 300, 3600, 86400, 604800
});

// With mentions
await client.sendMessage(convId, {
  text: 'Hey @alice check this',
  mentions: [{
    userId: 'alice-uuid',
    username: 'alice',
    offset: 4,
    length: 6,
  }],
});
```

### Edit Message

```typescript
// Within 15 minutes, max 10 edits per message
await client.editMessage(convId, seq, { text: 'Updated text' });
```

### Unsend Message

```typescript
// Within 5 minutes of sending
await client.unsendMessage(convId, seq);
```

### Delete Message

```typescript
// Delete for everyone (sender only)
await client.deleteForEveryone(convId, seq);

// Delete for me only
await client.deleteForMe(convId, seq);
```

### React to Message

```typescript
await client.react(convId, seq, '👍');
```

### Mark as Read

```typescript
await client.markRead(convId, seq);
```

## Disappearing Messages

```typescript
// Set disappearing timer for DM (both parties affected)
// TTL options: 10, 30, 60, 300, 3600, 86400, 604800 seconds (or null to disable)
await client.setDMDisappearingTTL(convId, 86400); // 24 hours
await client.setDMDisappearingTTL(convId, null);  // Disable

// Set for room (owner/admin only)
await client.setRoomDisappearingTTL(roomId, 3600); // 1 hour
```

## Pinned Messages

```typescript
// Pin a message (any member in DM, owner/admin in rooms)
await client.pinMessage(convId, seq);

// Unpin
await client.unpinMessage(convId);
```

## Typing Indicators

```typescript
// Send typing indicator (debounced, fire-and-forget)
client.sendTyping(convId);
```

## Contacts

```typescript
// Get contacts
const contacts = await client.getContacts();

// Add contact
await client.addContact(userId);

// Remove contact
await client.removeContact(userId);

// Set nickname
await client.updateContactNickname(userId, 'My Lawyer');
await client.updateContactNickname(userId, null); // Clear nickname
```

## Search

```typescript
const results = await client.searchUsers('alice', 10);
// Returns: Array<{ id, public, online, lastSeen }>
```

## Invites

```typescript
// Create invite
const invite = await client.createInvite('bob@email.com', 'Bob');
// Returns: { id, code, expiresAt }

// List my invites
const invites = await client.listInvites();
// Returns: Array<Invite>

// Revoke invite
await client.revokeInvite(inviteId);

// Redeem invite (existing user)
const result = await client.redeemInvite('1234567890');
// Returns: { inviter, inviterPublic, conv }
```

## File Upload/Download

```typescript
// Upload file
const file = /* File or Blob */;
const result = await client.uploadFile(file, 'photo.jpg');
// Returns: { id, mime, size, deduplicated }

// Use in message
await client.sendMessage(convId, {
  media: [{
    type: 'image',
    ref: result.id,
    name: 'photo.jpg',
    mime: result.mime,
    size: result.size,
  }],
});

// Get file URL (for display)
const url = client.getFileUrl(fileId);
const thumbUrl = client.getFileUrl(fileId, true); // thumbnail

// Download file
const blob = await client.downloadFile(fileId);
const thumbBlob = await client.downloadFile(fileId, true);
```

## Real-time Events

```typescript
// New message
client.on('message', (event) => {
  // event: { conv, seq, from, ts, content, head }
});

// Typing indicator
client.on('typing', (event) => {
  // event: { conv, user }
});

// Presence (online/offline)
client.on('presence', (event) => {
  // event: { user, online, lastSeen }
});

// Message edited
client.on('edit', (event) => {
  // event: { conv, seq, from, content }
});

// Message unsent
client.on('unsend', (event) => {
  // event: { conv, seq, from }
});

// Message deleted for everyone
client.on('deleteForEveryone', (event) => {
  // event: { conv, seq, from }
});

// Reaction
client.on('react', (event) => {
  // event: { conv, seq, from, emoji }
});

// Read receipt
client.on('read', (event) => {
  // event: { conv, user, seq }
});

// Message pinned
client.on('pin', (event) => {
  // event: { conv, from, seq }
});

// Message unpinned
client.on('unpin', (event) => {
  // event: { conv, from }
});

// Disappearing messages setting changed
client.on('disappearingUpdated', (event) => {
  // event: { conv, from }
});

// Room management events
client.on('memberJoined', (event) => {
  // event: { conv, from }
});

client.on('memberLeft', (event) => {
  // event: { conv, from }
});

client.on('memberKicked', (event) => {
  // event: { conv, from }
});

client.on('roomUpdated', (event) => {
  // event: { conv, from, content }
});

// Connection events
client.on('connect', () => { /* Connected */ });
client.on('disconnect', (reason) => { /* Disconnected */ });
client.on('reconnecting', (attempt) => { /* Reconnecting... */ });
client.on('stateChange', (state) => { /* 'disconnected' | 'connecting' | 'connected' | 'reconnecting' */ });
client.on('error', (error) => { /* Error occurred */ });
```

## React Hooks

### useClient

```typescript
import { useClient } from '@mvchat/react-native-sdk';

function App() {
  const { isConnected, state, error, connect, disconnect } = useClient(client);

  useEffect(() => {
    connect();
    return () => disconnect();
  }, []);

  return <Text>{isConnected ? 'Connected' : state}</Text>;
}
```

### useAuth

```typescript
import { useAuth } from '@mvchat/react-native-sdk';

function LoginScreen() {
  const {
    isAuthenticated,
    user,
    userID,
    mustChangePassword,
    isLoading,
    error,
    login,
    loginWithToken,
    signup,
    logout,
    changePassword,
  } = useAuth(client);

  const handleLogin = async () => {
    await login({ username, password });
  };

  if (mustChangePassword) {
    return <ChangePasswordScreen />;
  }

  return isAuthenticated ? <ChatScreen /> : <LoginForm />;
}
```

### useConversations

```typescript
import { useConversations } from '@mvchat/react-native-sdk';

function ConversationList() {
  const {
    conversations,
    isLoading,
    error,
    refresh,
    startDM,
    createRoom,
    inviteToRoom,
    leaveRoom,
    kickFromRoom,
    updateRoom,
    updateDMSettings,
    setDMDisappearingTTL,
    setRoomDisappearingTTL,
    pinMessage,
    unpinMessage,
  } = useConversations(client);

  return (
    <FlatList
      data={conversations}
      renderItem={({ item }) => <ConversationItem conv={item} />}
      refreshing={isLoading}
      onRefresh={refresh}
    />
  );
}
```

### useMessages

```typescript
import { useMessages } from '@mvchat/react-native-sdk';

function ChatScreen({ conversationId }) {
  const {
    messages,
    isLoading,
    hasMore,
    loadMore,
    send,
    edit,
    unsend,
    deleteForEveryone,
    deleteForMe,
    react,
    markRead,
  } = useMessages(client, conversationId);

  const handleSend = async (text) => {
    await send({ text });
  };

  return (
    <FlatList
      data={messages}
      renderItem={({ item }) => <MessageBubble message={item} />}
      onEndReached={loadMore}
    />
  );
}
```

### useContacts

```typescript
import { useContacts } from '@mvchat/react-native-sdk';

function ContactList() {
  const {
    contacts,
    isLoading,
    error,
    refresh,
    addContact,
    removeContact,
    updateNickname,
  } = useContacts(client);

  return (
    <FlatList
      data={contacts}
      renderItem={({ item }) => <ContactItem contact={item} />}
    />
  );
}
```

### useTyping

```typescript
import { useTyping } from '@mvchat/react-native-sdk';

function ChatInput({ conversationId }) {
  const { typingUsers, sendTyping } = useTyping(client, conversationId);

  const handleTextChange = (text) => {
    setText(text);
    sendTyping(); // Debounced automatically
  };

  return (
    <View>
      {typingUsers.length > 0 && (
        <Text>{typingUsers.join(', ')} typing...</Text>
      )}
      <TextInput onChangeText={handleTextChange} />
    </View>
  );
}
```

## Error Handling

```typescript
import { MVChat2Error } from '@mvchat/react-native-sdk';

try {
  await client.login(credentials);
} catch (err) {
  if (err instanceof MVChat2Error) {
    switch (err.code) {
      case 401: // Unauthorized
        console.log('Invalid credentials');
        break;
      case 403: // Forbidden
        console.log('Access denied');
        break;
      case 404: // Not found
        console.log('Not found');
        break;
      case 429: // Rate limited
        console.log('Too many requests');
        break;
      default:
        console.log('Error:', err.text);
    }
  }
}
```

## Message Content Format (Irido)

Messages use the Irido format:

```typescript
interface Irido {
  v: 1;                        // Version (always 1)
  text?: string;               // Text content
  media?: IridoMedia[];        // Attached media
  reply?: IridoReply;          // Reply info
  mentions?: IridoMention[];   // User mentions
}

interface IridoMedia {
  type: 'image' | 'video' | 'audio' | 'file';
  ref: string;                 // File ID from upload
  name: string;                // Original filename
  mime: string;                // MIME type
  size: number;                // Size in bytes
  width?: number;              // For images/video
  height?: number;             // For images/video
  duration?: number;           // For audio/video (seconds)
  thumb?: string;              // Base64 thumbnail
}

interface IridoReply {
  seq: number;                 // Seq of replied message
  preview?: string;            // Text preview of original
}

interface IridoMention {
  userId: string;
  username: string;
  offset: number;              // Start position in text
  length: number;              // Length of mention
}
```

## TypeScript Types

All types are exported:

```typescript
import {
  // Client config
  MVChat2ClientConfig,
  ConnectionState,

  // User types
  User,
  UserPublic,

  // Auth types
  LoginCredentials,
  SignupData,
  AuthResult,
  ChangePasswordData,

  // Conversation types
  Conversation,
  ConversationType,
  ConversationPublic,
  Member,
  RoomMember,
  ReadReceipt,
  StartDMResult,
  CreateRoomResult,

  // Message types
  Message,
  Irido,
  IridoMedia,
  IridoReply,
  IridoMention,
  SendMessageOptions,

  // Contact types
  Contact,
  ContactSource,

  // Invite types
  Invite,
  InviteStatus,

  // Search types
  SearchResult,

  // Event types
  MessageEvent,
  TypingEvent,
  PresenceEvent,
  EditEvent,
  UnsendEvent,
  DeleteEvent,
  ReactEvent,
  ReadEvent,
  PinEvent,
  UnpinEvent,
  DisappearingUpdatedEvent,
  MemberJoinedEvent,
  MemberLeftEvent,
  MemberKickedEvent,
  RoomUpdatedEvent,

  // Error
  MVChat2Error,
} from '@mvchat/react-native-sdk';
```

## License

MIT
