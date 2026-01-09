// Connection types
export type ConnectionState = 'disconnected' | 'connecting' | 'connected' | 'reconnecting';

export interface MVChat2ClientConfig {
  url: string;
  autoReconnect?: boolean;
  reconnectDelay?: number;
  reconnectMaxDelay?: number;
  reconnectBackoff?: number;
  compression?: boolean;
  timeout?: number;
  // Optional device/client info sent in handshake
  deviceId?: string;
  lang?: string;
  userAgent?: string;
}

// User types
export interface User {
  id: string;
  createdAt: string;
  updatedAt: string;
  public: UserPublic;
  lastSeen?: string;
}

export interface UserPublic {
  fn?: string;
  photo?: string;
  bio?: string;
  [key: string]: any;
}

// Auth types
export interface LoginCredentials {
  username: string;
  password: string;
}

export interface SignupData {
  username: string;
  password: string;
  inviteCode?: string;
  profile?: UserPublic;
  login?: boolean;
}

export interface AuthResult {
  user: string;
  token: string;
  expires: string;
  inviters?: string[];
  mustChangePassword?: boolean;
  emailVerified?: boolean;
  desc?: {
    public?: UserPublic;
  };
}

export interface ChangePasswordData {
  oldPassword: string;
  newPassword: string;
}

// Conversation types
export type ConversationType = 'dm' | 'room';

export interface Conversation {
  id: string;
  type: ConversationType;
  lastSeq: number;
  readSeq: number;
  unread: number;
  favorite?: boolean;
  muted?: boolean;
  lastMsgAt?: string;
  public?: ConversationPublic;
  // Private data for this conversation (only visible to you)
  private?: any;
  // For DMs: the other user (backend returns as 'user')
  user?: {
    id: string;
    public: UserPublic;
    online: boolean;
    lastSeen?: string;
  };
  // Disappearing messages TTL in seconds (undefined = disabled)
  disappearingTTL?: number;
  // Pinned message info
  pinnedSeq?: number;
  pinnedAt?: string;
  pinnedBy?: string;
}

export interface ConversationPublic {
  fn?: string;
  description?: string;
  photo?: string;
  [key: string]: any;
}

// Member info returned from getMembers - note: backend doesn't return role/joinedAt currently
export interface Member {
  id: string;
  public: UserPublic;
  online: boolean;
  lastSeen?: string;
}

// Full member info with role (for future use when backend supports it)
export interface RoomMember extends Member {
  role?: 'owner' | 'admin' | 'member';
  joinedAt?: string;
}

export interface ReadReceipt {
  user: string;
  readSeq: number;
  recvSeq: number;
}

// Message types
export interface Message {
  seq: number;
  from: string;
  ts: string;
  content?: Irido;
  head?: Record<string, any>;
  deleted?: boolean;
  // View-once message indicator
  viewOnce?: boolean;
}

export interface Irido {
  v: 1;
  text?: string;
  media?: IridoMedia[];
  reply?: IridoReply;
  mentions?: IridoMention[];
}

export interface IridoMedia {
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

export interface IridoReply {
  seq: number;
  preview?: string;
}

export interface IridoMention {
  userId: string;
  username: string;
  offset: number;
  length: number;
}

export interface SendMessageOptions {
  text?: string;
  media?: IridoMedia[];
  replyTo?: number;
  mentions?: IridoMention[];
  // View-once message (disappears after recipient views)
  viewOnce?: boolean;
  // View-once TTL in seconds: 10, 30, 60, 300, 3600, 86400, 604800
  viewOnceTTL?: number;
}

// Contact types
export type ContactSource = 'invite' | 'manual';

export interface Contact {
  user: string;
  source: ContactSource;
  nickname?: string;
  createdAt: string;
  public: UserPublic;
  online: boolean;
  lastSeen?: string;
}

// File types
export interface FileUploadOptions {
  uri: string;
  name: string;
  type: string;
}

export interface FileUploadResult {
  ref: string;
  name: string;
  mime: string;
  size: number;
  width?: number;
  height?: number;
  duration?: number;
  thumb?: string;
}

// Event types (for real-time events via WebSocket)
export interface MessageEvent {
  conv: string;
  seq: number;
  from: string;
  ts: string;
  content: Irido;
  head?: Record<string, any>;
}

export interface TypingEvent {
  conv: string;
  user: string;
}

export interface PresenceEvent {
  user: string;
  online: boolean;
  lastSeen?: string;
}

export interface EditEvent {
  conv: string;
  seq: number;
  from: string;
  content: Irido;
  editCount?: number;
}

export interface UnsendEvent {
  conv: string;
  seq: number;
  from: string;
}

export interface DeleteEvent {
  conv: string;
  seq: number;
  from: string;
}

export interface ReactEvent {
  conv: string;
  seq: number;
  from: string;
  emoji: string;
  remove?: boolean;
}

export interface ReadEvent {
  conv: string;
  user: string;
  seq: number;
}

export interface RecvEvent {
  conv: string;
  user: string;
  seq: number;
}

export interface PinEvent {
  conv: string;
  from: string;
  seq: number;
}

export interface UnpinEvent {
  conv: string;
  from: string;
}

export interface DisappearingUpdatedEvent {
  conv: string;
  from: string;
}

// Room management events
export interface MemberJoinedEvent {
  conv: string;
  from: string;
}

export interface MemberLeftEvent {
  conv: string;
  from: string;
}

export interface MemberKickedEvent {
  conv: string;
  from: string;
}

export interface RoomUpdatedEvent {
  conv: string;
  from: string;
  content?: any;
}

// Invite types
export type InviteStatus = 'pending' | 'used' | 'expired' | 'revoked';

export interface Invite {
  id: string;
  email: string;
  name?: string;
  code?: string;
  status: InviteStatus;
  createdAt: string;
  expiresAt: string;
  usedAt?: string;
  usedBy?: string;
}

// Search result (simplified user info)
export interface SearchResult {
  id: string;
  public: UserPublic;
  online: boolean;
  lastSeen?: string;
}

// DM start result
export interface StartDMResult {
  conv: string;
  created: boolean;
  user: {
    id: string;
    public: UserPublic;
    online: boolean;
  };
}

// Room create result
export interface CreateRoomResult {
  conv: string;
  public?: any;
}

// Wire protocol types
export interface ClientMessage {
  id?: string;
  hi?: MsgClientHi;
  login?: MsgClientLogin;
  acc?: MsgClientAcc;
  search?: MsgClientSearch;
  dm?: MsgClientDM;
  room?: MsgClientRoom;
  send?: MsgClientSend;
  get?: MsgClientGet;
  edit?: MsgClientEdit;
  unsend?: MsgClientUnsend;
  delete?: MsgClientDelete;
  react?: MsgClientReact;
  typing?: MsgClientTyping;
  read?: MsgClientRead;
  recv?: MsgClientRecv;
  clear?: MsgClientClear;
  invite?: MsgClientInvite;
  contact?: MsgClientContact;
  pin?: MsgClientPin;
}

export interface MsgClientHi {
  ver: string;
  ua?: string;
  dev?: string;
  lang?: string;
}

export interface MsgClientLogin {
  scheme: 'basic' | 'token';
  secret: string;
}

export interface MsgClientAcc {
  user: 'new' | 'me';
  scheme?: string;
  secret?: string;
  login?: boolean;
  desc?: { public?: any; private?: any };
  inviteCode?: string;
  email?: string;
}

export interface MsgClientSearch {
  query: string;
  limit?: number;
}

export interface MsgClientDM {
  user?: string;
  conv?: string;
  favorite?: boolean;
  muted?: boolean;
  blocked?: boolean;
  // Private data for this conversation (only visible to you)
  private?: any;
  // Disappearing messages TTL in seconds (0 to disable)
  disappearingTTL?: number;
}

export interface MsgClientRoom {
  id?: string;
  action?: 'create' | 'join' | 'leave' | 'invite' | 'kick' | 'update';
  user?: string;
  desc?: { public?: any };
  // Disappearing messages TTL in seconds (0 to disable)
  disappearingTTL?: number;
}

export interface MsgClientSend {
  conv: string;
  content: any;
  replyTo?: number;
  // View-once message
  viewOnce?: boolean;
  viewOnceTTL?: number;
}

export interface MsgClientGet {
  what: 'conversations' | 'messages' | 'members' | 'receipts' | 'contacts';
  conv?: string;
  limit?: number;
  before?: number;
}

export interface MsgClientEdit {
  conv: string;
  seq: number;
  content: any;
}

export interface MsgClientUnsend {
  conv: string;
  seq: number;
}

export interface MsgClientDelete {
  conv: string;
  seq: number;
  forEveryone?: boolean;
}

export interface MsgClientReact {
  conv: string;
  seq: number;
  emoji: string;
}

export interface MsgClientTyping {
  conv: string;
}

export interface MsgClientRead {
  conv: string;
  seq: number;
}

export interface MsgClientRecv {
  conv: string;
  seq: number;
}

export interface MsgClientClear {
  conv: string;
  seq: number;
}

export interface MsgClientInvite {
  create?: { email: string; name?: string };
  list?: boolean;
  revoke?: string;
  redeem?: string;
}

export interface MsgClientContact {
  add?: string;
  remove?: string;
  user?: string;
  nickname?: string;
}

export interface MsgClientPin {
  conv: string;
  // Message seq to pin (0 to unpin)
  seq?: number;
}

// Server message types
export interface ServerMessage {
  ctrl?: MsgServerCtrl;
  data?: MsgServerData;
  info?: MsgServerInfo;
  pres?: MsgServerPres;
}

export interface MsgServerPres {
  user: string;
  what: 'on' | 'off';
  lastSeen?: string;
}

export interface MsgServerCtrl {
  id?: string;
  code: number;
  text?: string;
  params?: Record<string, any>;
}

export interface MsgServerData {
  conv: string;
  seq: number;
  from: string;
  ts: string;
  content: any;
  head?: Record<string, any>;
}

export interface MsgServerInfo {
  conv?: string;
  from?: string;
  what: string;
  seq?: number;
  content?: any;
  emoji?: string;
  ts?: string;
}

// Error types
export class MVChat2Error extends Error {
  code: number;
  text: string;

  constructor(code: number, text: string) {
    super(text);
    this.code = code;
    this.text = text;
    this.name = 'MVChat2Error';
  }
}
