# TypeScript Types Reference

> **IMPORTANT**: This documentation is accurate as of January 8th, 2026 at 4:00 AM CST. Review the actual code before assuming anything here is still current.

## Client Types

```typescript
interface MVChat2ClientConfig {
  url: string;
  autoReconnect?: boolean;
  reconnectDelay?: number;
  reconnectMaxDelay?: number;
  reconnectBackoff?: number;
  compression?: boolean;
  timeout?: number;
  e2ee?: boolean;
  privateKey?: string;
}

type ConnectionState = 'disconnected' | 'connecting' | 'connected' | 'reconnecting';

interface User {
  id: string;
  createdAt: string;
  updatedAt: string;
  public: UserPublic;
  lastSeen?: string;
}

interface UserPublic {
  fn?: string;           // Full name
  photo?: string;        // Avatar file ref
  bio?: string;
  [key: string]: any;    // Custom fields
}
```

## Authentication Types

```typescript
interface LoginCredentials {
  username: string;
  password: string;
}

interface SignupData {
  username: string;
  password: string;
  inviteCode?: string;
  profile?: UserPublic;
  login?: boolean;
}

interface AuthResult {
  user: string;
  token: string;
  expires: string;
  inviters?: string[];
}

interface RedeemResult {
  inviter: string;
  inviterPublic: UserPublic;
  conv: string;
}
```

## Conversation Types

```typescript
type ConversationType = 'dm' | 'room';

interface Conversation {
  id: string;
  type: ConversationType;
  createdAt: string;
  updatedAt: string;
  lastSeq: number;
  lastMsgAt?: string;
  public?: ConversationPublic;
  readSeq: number;
  recvSeq: number;
  unread: number;
  otherUser?: User;      // For DMs
}

interface ConversationPublic {
  fn?: string;           // Room name
  description?: string;
  photo?: string;        // Room avatar
  [key: string]: any;
}

interface Member {
  userId: string;
  role: 'owner' | 'admin' | 'member';
  joinedAt: string;
  public: UserPublic;
}

interface ReadReceipt {
  user: string;
  readSeq: number;
  recvSeq: number;
}
```

## Message Types

```typescript
interface Message {
  conv: string;
  seq: number;
  from: string;
  ts: string;
  content: Irido;
  editedAt?: string;
  reactions?: Record<string, string[]>;  // emoji -> userIds
}

interface Irido {
  v: 1;
  text?: string;
  media?: IridoMedia[];
  reply?: IridoReply;
  mentions?: IridoMention[];
}

interface IridoMedia {
  type: 'image' | 'video' | 'audio' | 'file';
  ref: string;
  name: string;
  mime: string;
  size: number;
  width?: number;
  height?: number;
  duration?: number;
  thumb?: string;
}

interface IridoReply {
  seq: number;
  preview?: string;
}

interface IridoMention {
  userId: string;
  username: string;
  offset: number;
  length: number;
}

interface SendMessageOptions {
  text?: string;
  media?: IridoMedia[];
  replyTo?: number;
  mentions?: IridoMention[];
}
```

## Contact Types

```typescript
type ContactSource = 'invite' | 'manual';

interface Contact {
  user: string;
  source: ContactSource;
  nickname?: string;
  createdAt: string;
  public: UserPublic;
  online: boolean;
  lastSeen?: string;
}
```

## File Types

```typescript
interface FileUploadOptions {
  uri: string;
  name: string;
  type: string;
}

interface FileUploadResult {
  ref: string;
  name: string;
  mime: string;
  size: number;
  width?: number;
  height?: number;
  duration?: number;
  thumb?: string;
}

interface UploadProgress {
  loaded: number;
  total: number;
  percent: number;
}
```

## Event Types

```typescript
interface MessageEvent {
  conv: string;
  seq: number;
  from: string;
  ts: string;
  content: Irido;
}

interface TypingEvent {
  conv: string;
  user: string;
}

interface PresenceEvent {
  user: string;
  online: boolean;
  lastSeen?: string;
}

interface EditEvent {
  conv: string;
  seq: number;
  from: string;
  content: Irido;
}

interface UnsendEvent {
  conv: string;
  seq: number;
  from: string;
}

interface ReactEvent {
  conv: string;
  seq: number;
  from: string;
  emoji: string;
  remove?: boolean;
}

interface ReadEvent {
  conv: string;
  user: string;
  seq: number;
}
```

## Invite Types

```typescript
interface InviteCreateOptions {
  email: string;
  name?: string;
}

interface Invite {
  id: string;
  code: string;
  email: string;
  inviteeName?: string;
  status: 'pending' | 'used' | 'expired' | 'revoked';
  createdAt: string;
  expiresAt: string;
  usedAt?: string;
  usedBy?: string;
}
```

## Error Types

```typescript
interface MVChat2Error extends Error {
  code: number;
  text: string;
}

// Error codes
const ErrorCodes = {
  BadRequest: 400,
  Unauthorized: 401,
  Forbidden: 403,
  NotFound: 404,
  Conflict: 409,
  InternalError: 500,
} as const;
```
