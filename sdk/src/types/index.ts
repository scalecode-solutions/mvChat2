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
  // For DMs: the other user (backend returns as 'user')
  user?: {
    id: string;
    public: UserPublic;
    online: boolean;
    lastSeen?: string;
  };
}

export interface ConversationPublic {
  fn?: string;
  description?: string;
  photo?: string;
  [key: string]: any;
}

export interface Member {
  userId: string;
  role: 'owner' | 'admin' | 'member';
  joinedAt: string;
  public: UserPublic;
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
  invite?: MsgClientInvite;
  contact?: MsgClientContact;
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
}

export interface MsgClientRoom {
  id?: string;
  action?: 'create' | 'join' | 'leave' | 'invite' | 'kick' | 'update';
  user?: string;
  desc?: { public?: any };
}

export interface MsgClientSend {
  conv: string;
  content: any;
  replyTo?: number;
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

// Server message types
export interface ServerMessage {
  ctrl?: MsgServerCtrl;
  data?: MsgServerData;
  info?: MsgServerInfo;
  pres?: MsgServerPres;
  meta?: MsgServerMeta;
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

export interface MsgServerMeta {
  conv?: string;
  what?: string;
  data?: any;
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
