import {
  MVChat2ClientConfig,
  ConnectionState,
  ClientMessage,
  ServerMessage,
  MsgServerCtrl,
  AuthResult,
  LoginCredentials,
  SignupData,
  ChangePasswordData,
  User,
  Conversation,
  Message,
  Contact,
  Member,
  ReadReceipt,
  SendMessageOptions,
  Irido,
  MVChat2Error,
  Invite,
  SearchResult,
  StartDMResult,
  CreateRoomResult,
} from './types';

type EventCallback = (...args: any[]) => void;

export class MVChat2Client {
  private config: MVChat2ClientConfig;
  private ws: WebSocket | null = null;
  private state: ConnectionState = 'disconnected';
  private messageId = 0;
  private pendingRequests: Map<string, { resolve: Function; reject: Function; timeout: NodeJS.Timeout }> = new Map();
  private eventListeners: Map<string, Set<EventCallback>> = new Map();
  private reconnectAttempts = 0;
  private reconnectTimer: NodeJS.Timeout | null = null;

  // Auth state
  private _user: User | null = null;
  private _token: string | null = null;
  private _userID: string | null = null;
  private _mustChangePassword: boolean = false;
  private _emailVerified: boolean = true;

  constructor(config: MVChat2ClientConfig) {
    this.config = {
      autoReconnect: true,
      reconnectDelay: 1000,
      reconnectMaxDelay: 30000,
      reconnectBackoff: 1.5,
      compression: true,
      timeout: 10000,
      ...config,
    };
  }

  // Getters
  get isConnected(): boolean {
    return this.state === 'connected';
  }

  get connectionState(): ConnectionState {
    return this.state;
  }

  get isAuthenticated(): boolean {
    return this._userID !== null;
  }

  get user(): User | null {
    return this._user;
  }

  get userID(): string | null {
    return this._userID;
  }

  get token(): string | null {
    return this._token;
  }

  get mustChangePassword(): boolean {
    return this._mustChangePassword;
  }

  get emailVerified(): boolean {
    return this._emailVerified;
  }

  // Connection management
  async connect(): Promise<void> {
    if (this.state === 'connected' || this.state === 'connecting') {
      return;
    }

    this.setState('connecting');

    return new Promise((resolve, reject) => {
      try {
        this.ws = new WebSocket(this.config.url);

        this.ws.onopen = async () => {
          this.reconnectAttempts = 0;
          try {
            await this.sendHi();
            this.setState('connected');
            this.emit('connect');
            resolve();
          } catch (err) {
            reject(err);
          }
        };

        this.ws.onmessage = (event) => {
          this.handleMessage(event.data);
        };

        this.ws.onclose = (event) => {
          this.handleClose(event);
        };

        this.ws.onerror = (error) => {
          this.emit('error', error);
          if (this.state === 'connecting') {
            reject(error);
          }
        };
      } catch (err) {
        this.setState('disconnected');
        reject(err);
      }
    });
  }

  disconnect(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }

    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }

    this.setState('disconnected');
    this._userID = null;
  }

  private setState(state: ConnectionState): void {
    this.state = state;
    this.emit('stateChange', state);
  }

  private async sendHi(): Promise<MsgServerCtrl> {
    return this.request({
      hi: {
        ver: '0.1.0',
        ua: this.config.userAgent || 'MVChat2SDK/0.1.0',
        dev: this.config.deviceId,
        lang: this.config.lang,
      },
    });
  }

  private handleMessage(data: string): void {
    try {
      const msg: ServerMessage = JSON.parse(data);

      if (msg.ctrl) {
        this.handleCtrl(msg.ctrl);
      }

      if (msg.data) {
        this.emit('message', {
          conv: msg.data.conv,
          seq: msg.data.seq,
          from: msg.data.from,
          ts: msg.data.ts,
          content: msg.data.content,
          head: msg.data.head,
        });
      }

      if (msg.info) {
        this.handleInfo(msg.info);
      }

      if (msg.pres) {
        this.emit('presence', {
          user: msg.pres.user,
          online: msg.pres.what === 'on',
          lastSeen: msg.pres.lastSeen,
        });
      }
    } catch (err) {
      this.emit('error', err);
    }
  }

  private handleCtrl(ctrl: MsgServerCtrl): void {
    if (ctrl.id) {
      const pending = this.pendingRequests.get(ctrl.id);
      if (pending) {
        clearTimeout(pending.timeout);
        this.pendingRequests.delete(ctrl.id);

        if (ctrl.code >= 200 && ctrl.code < 400) {
          pending.resolve(ctrl);
        } else {
          pending.reject(new MVChat2Error(ctrl.code, ctrl.text || 'Unknown error'));
        }
      }
    }
  }

  private handleInfo(info: any): void {
    switch (info.what) {
      case 'typing':
        this.emit('typing', { conv: info.conv, user: info.from });
        break;
      case 'read':
        this.emit('read', { conv: info.conv, user: info.from, seq: info.seq });
        break;
      case 'recv':
        this.emit('recv', { conv: info.conv, user: info.from, seq: info.seq });
        break;
      case 'edit':
        this.emit('edit', { conv: info.conv, seq: info.seq, from: info.from, content: info.content });
        break;
      case 'unsend':
        this.emit('unsend', { conv: info.conv, seq: info.seq, from: info.from });
        break;
      case 'delete':
        this.emit('deleteForEveryone', { conv: info.conv, seq: info.seq, from: info.from });
        break;
      case 'react':
        this.emit('react', { conv: info.conv, seq: info.seq, from: info.from, emoji: info.emoji });
        break;
      case 'pin':
        this.emit('pin', { conv: info.conv, from: info.from, seq: info.seq });
        break;
      case 'unpin':
        this.emit('unpin', { conv: info.conv, from: info.from });
        break;
      case 'disappearing_updated':
        this.emit('disappearingUpdated', { conv: info.conv, from: info.from });
        break;
      case 'member_joined':
        this.emit('memberJoined', { conv: info.conv, from: info.from });
        break;
      case 'member_left':
        this.emit('memberLeft', { conv: info.conv, from: info.from });
        break;
      case 'member_kicked':
        this.emit('memberKicked', { conv: info.conv, from: info.from });
        break;
      case 'room_updated':
        this.emit('roomUpdated', { conv: info.conv, from: info.from, content: info.content });
        break;
    }
  }

  private handleClose(event: { reason?: string }): void {
    this.ws = null;

    // Reject all pending requests
    for (const [id, pending] of this.pendingRequests) {
      clearTimeout(pending.timeout);
      pending.reject(new MVChat2Error(0, 'Connection closed'));
    }
    this.pendingRequests.clear();

    this.emit('disconnect', event.reason);

    if (this.config.autoReconnect && this.state !== 'disconnected') {
      this.scheduleReconnect();
    } else {
      this.setState('disconnected');
    }
  }

  private scheduleReconnect(): void {
    this.setState('reconnecting');
    this.reconnectAttempts++;

    const delay = Math.min(
      this.config.reconnectDelay! * Math.pow(this.config.reconnectBackoff!, this.reconnectAttempts - 1),
      this.config.reconnectMaxDelay!
    );

    this.emit('reconnecting', this.reconnectAttempts);

    this.reconnectTimer = setTimeout(async () => {
      try {
        await this.connect();
        // Re-authenticate if we have a token
        if (this._token) {
          await this.loginWithToken(this._token);
        }
      } catch (err) {
        // Will retry via handleClose
      }
    }, delay);
  }

  // Request/response
  private async request(msg: ClientMessage): Promise<MsgServerCtrl> {
    return new Promise((resolve, reject) => {
      if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
        reject(new MVChat2Error(0, 'Not connected'));
        return;
      }

      const id = String(++this.messageId);
      msg.id = id;

      const timeout = setTimeout(() => {
        this.pendingRequests.delete(id);
        reject(new MVChat2Error(0, 'Request timeout'));
      }, this.config.timeout);

      this.pendingRequests.set(id, { resolve, reject, timeout });

      this.ws.send(JSON.stringify(msg));
    });
  }

  // Event emitter
  on(event: string, callback: EventCallback): void {
    if (!this.eventListeners.has(event)) {
      this.eventListeners.set(event, new Set());
    }
    this.eventListeners.get(event)!.add(callback);
  }

  off(event: string, callback: EventCallback): void {
    this.eventListeners.get(event)?.delete(callback);
  }

  private emit(event: string, ...args: any[]): void {
    this.eventListeners.get(event)?.forEach((cb) => cb(...args));
  }

  // Authentication
  async login(credentials: LoginCredentials): Promise<AuthResult> {
    const secret = btoa(`${credentials.username}:${credentials.password}`);
    const ctrl = await this.request({
      login: {
        scheme: 'basic',
        secret,
      },
    });

    this._userID = ctrl.params?.user;
    this._token = ctrl.params?.token;
    this._mustChangePassword = ctrl.params?.mustChangePassword ?? false;
    this._emailVerified = ctrl.params?.emailVerified ?? true;

    return {
      user: ctrl.params?.user,
      token: ctrl.params?.token,
      expires: ctrl.params?.expires,
      mustChangePassword: ctrl.params?.mustChangePassword,
      emailVerified: ctrl.params?.emailVerified,
      desc: ctrl.params?.desc,
    };
  }

  async loginWithToken(token: string): Promise<AuthResult> {
    const ctrl = await this.request({
      login: {
        scheme: 'token',
        secret: token,
      },
    });

    this._userID = ctrl.params?.user;
    this._token = token;
    this._mustChangePassword = ctrl.params?.mustChangePassword ?? false;
    this._emailVerified = ctrl.params?.emailVerified ?? true;

    return {
      user: ctrl.params?.user,
      token,
      expires: ctrl.params?.expires,
      mustChangePassword: ctrl.params?.mustChangePassword,
      emailVerified: ctrl.params?.emailVerified,
      desc: ctrl.params?.desc,
    };
  }

  async signup(data: SignupData): Promise<AuthResult> {
    const secret = btoa(`${data.username}:${data.password}`);
    const ctrl = await this.request({
      acc: {
        user: 'new',
        scheme: 'basic',
        secret,
        login: data.login ?? true,
        desc: data.profile ? { public: data.profile } : undefined,
        inviteCode: data.inviteCode,
      },
    });

    if (data.login !== false) {
      this._userID = ctrl.params?.user;
      this._token = ctrl.params?.token;
      this._mustChangePassword = ctrl.params?.mustChangePassword ?? false;
      this._emailVerified = ctrl.params?.emailVerified ?? true;
    }

    return {
      user: ctrl.params?.user,
      token: ctrl.params?.token,
      expires: ctrl.params?.expires,
      inviters: ctrl.params?.inviters,
      mustChangePassword: ctrl.params?.mustChangePassword,
      emailVerified: ctrl.params?.emailVerified,
      desc: ctrl.params?.desc,
    };
  }

  async changePassword(data: ChangePasswordData): Promise<void> {
    const secret = btoa(`${data.oldPassword}:${data.newPassword}`);
    await this.request({
      acc: {
        user: 'me',
        secret,
      },
    });
    // Clear the flag after successful password change
    this._mustChangePassword = false;
  }

  async updateProfile(profile: any): Promise<void> {
    await this.request({
      acc: {
        user: 'me',
        desc: { public: profile },
      },
    });
  }

  /**
   * Update the current user's private data.
   * This data is only visible to the user themselves.
   * @param privateData - Any JSON-serializable private data
   */
  async updatePrivateData(privateData: any): Promise<void> {
    await this.request({
      acc: {
        user: 'me',
        desc: { private: privateData },
      },
    });
  }

  async updateEmail(email: string): Promise<void> {
    await this.request({
      acc: {
        user: 'me',
        email,
      },
    });
  }

  logout(): void {
    this._userID = null;
    this._token = null;
    this._user = null;
    this._mustChangePassword = false;
    this._emailVerified = true;
  }

  // Conversations
  async getConversations(): Promise<Conversation[]> {
    const ctrl = await this.request({
      get: { what: 'conversations' },
    });
    return ctrl.params?.conversations || [];
  }

  async startDM(userId: string): Promise<StartDMResult> {
    const ctrl = await this.request({
      dm: { user: userId },
    });
    return {
      conv: ctrl.params?.conv,
      created: ctrl.params?.created,
      user: ctrl.params?.user,
    };
  }

  async createRoom(options: { public: any }): Promise<CreateRoomResult> {
    const ctrl = await this.request({
      room: {
        id: 'new',
        action: 'create',
        desc: { public: options.public },
      },
    });
    return {
      conv: ctrl.params?.conv,
      public: ctrl.params?.public,
    };
  }

  async getMembers(convId: string): Promise<Member[]> {
    const ctrl = await this.request({
      get: { what: 'members', conv: convId },
    });
    return ctrl.params?.members || [];
  }

  async getReceipts(convId: string): Promise<ReadReceipt[]> {
    const ctrl = await this.request({
      get: { what: 'receipts', conv: convId },
    });
    return ctrl.params?.receipts || [];
  }

  // Messages
  async getMessages(convId: string, options?: { limit?: number; before?: number }): Promise<Message[]> {
    const ctrl = await this.request({
      get: {
        what: 'messages',
        conv: convId,
        limit: options?.limit,
        before: options?.before,
      },
    });
    return ctrl.params?.messages || [];
  }

  async sendMessage(convId: string, options: SendMessageOptions): Promise<{ seq: number }> {
    const content: Irido = {
      v: 1,
      text: options.text,
      media: options.media,
      mentions: options.mentions,
    };

    const ctrl = await this.request({
      send: {
        conv: convId,
        content,
        replyTo: options.replyTo,
        viewOnce: options.viewOnce,
        viewOnceTTL: options.viewOnceTTL,
      },
    });

    return { seq: ctrl.params?.seq };
  }

  async editMessage(convId: string, seq: number, options: { text?: string }): Promise<void> {
    const content: Irido = {
      v: 1,
      text: options.text,
    };

    await this.request({
      edit: {
        conv: convId,
        seq,
        content,
      },
    });
  }

  async unsendMessage(convId: string, seq: number): Promise<void> {
    await this.request({
      unsend: { conv: convId, seq },
    });
  }

  async deleteForEveryone(convId: string, seq: number): Promise<void> {
    await this.request({
      delete: { conv: convId, seq, forEveryone: true },
    });
  }

  async deleteForMe(convId: string, seq: number): Promise<void> {
    await this.request({
      delete: { conv: convId, seq, forEveryone: false },
    });
  }

  async react(convId: string, seq: number, emoji: string): Promise<void> {
    await this.request({
      react: { conv: convId, seq, emoji },
    });
  }

  // Presence
  sendTyping(convId: string): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ typing: { conv: convId } }));
    }
  }

  async markRead(convId: string, seq: number): Promise<void> {
    await this.request({
      read: { conv: convId, seq },
    });
  }

  /**
   * Mark messages as received (delivered to device).
   * This sends a delivery receipt to the conversation.
   * @param convId - Conversation ID
   * @param seq - Sequence number of the last received message
   */
  async markReceived(convId: string, seq: number): Promise<void> {
    await this.request({
      recv: { conv: convId, seq },
    });
  }

  /**
   * Clear conversation history up to a sequence number.
   * Messages with seq <= the given value will be hidden from this user.
   * This is a soft delete - other users are not affected.
   * @param convId - Conversation ID
   * @param seq - Clear messages up to and including this sequence number
   */
  async clearConversation(convId: string, seq: number): Promise<void> {
    await this.request({
      clear: { conv: convId, seq },
    });
  }

  // Contacts
  async getContacts(): Promise<Contact[]> {
    const ctrl = await this.request({
      get: { what: 'contacts' },
    });
    return ctrl.params?.contacts || [];
  }

  async addContact(userId: string): Promise<void> {
    await this.request({
      contact: { add: userId },
    });
  }

  async removeContact(userId: string): Promise<void> {
    await this.request({
      contact: { remove: userId },
    });
  }

  async updateContactNickname(userId: string, nickname: string | null): Promise<void> {
    await this.request({
      contact: { user: userId, nickname: nickname || undefined },
    });
  }

  // Search
  async searchUsers(query: string, limit?: number): Promise<SearchResult[]> {
    const ctrl = await this.request({
      search: { query, limit },
    });
    return ctrl.params?.users || [];
  }

  // Invites
  async createInvite(email: string, name?: string): Promise<{ id: string; code: string; expiresAt: string }> {
    const ctrl = await this.request({
      invite: { create: { email, name } },
    });
    return {
      id: ctrl.params?.id,
      code: ctrl.params?.code,
      expiresAt: ctrl.params?.expiresAt,
    };
  }

  async listInvites(): Promise<Invite[]> {
    const ctrl = await this.request({
      invite: { list: true },
    });
    return ctrl.params?.invites || [];
  }

  async revokeInvite(inviteId: string): Promise<void> {
    await this.request({
      invite: { revoke: inviteId },
    });
  }

  async redeemInvite(code: string): Promise<{ inviter: string; inviterPublic: any; conv: string }> {
    const ctrl = await this.request({
      invite: { redeem: code },
    });
    return {
      inviter: ctrl.params?.inviter,
      inviterPublic: ctrl.params?.inviterPublic,
      conv: ctrl.params?.conv,
    };
  }

  // Pinned messages
  async pinMessage(convId: string, seq: number): Promise<void> {
    await this.request({
      pin: { conv: convId, seq },
    });
  }

  async unpinMessage(convId: string): Promise<void> {
    await this.request({
      pin: { conv: convId, seq: 0 },
    });
  }

  // Disappearing messages
  async setDMDisappearingTTL(convId: string, ttl: number | null): Promise<void> {
    await this.request({
      dm: { conv: convId, disappearingTTL: ttl ?? 0 },
    });
  }

  async setRoomDisappearingTTL(roomId: string, ttl: number | null): Promise<void> {
    await this.request({
      room: { id: roomId, action: 'update', disappearingTTL: ttl ?? 0 },
    });
  }

  // Room management
  async inviteToRoom(roomId: string, userId: string): Promise<void> {
    await this.request({
      room: { id: roomId, action: 'invite', user: userId },
    });
  }

  async leaveRoom(roomId: string): Promise<void> {
    await this.request({
      room: { id: roomId, action: 'leave' },
    });
  }

  async kickFromRoom(roomId: string, userId: string): Promise<void> {
    await this.request({
      room: { id: roomId, action: 'kick', user: userId },
    });
  }

  async updateRoom(roomId: string, options: { public?: any }): Promise<void> {
    await this.request({
      room: { id: roomId, action: 'update', desc: { public: options.public } },
    });
  }

  // DM settings
  async updateDMSettings(convId: string, options: { favorite?: boolean; muted?: boolean; blocked?: boolean; private?: any }): Promise<void> {
    await this.request({
      dm: {
        conv: convId,
        favorite: options.favorite,
        muted: options.muted,
        blocked: options.blocked,
        private: options.private,
      },
    });
  }

  // File upload/download (HTTP API)
  // Note: These use HTTP endpoints, not WebSocket

  /**
   * Get the base URL for HTTP file operations.
   * Converts ws:// to http:// or wss:// to https://
   */
  private getHttpBaseUrl(): string {
    return this.config.url
      .replace(/^ws:\/\//, 'http://')
      .replace(/^wss:\/\//, 'https://')
      .replace(/\/v0\/channels$/, ''); // Remove WebSocket path
  }

  /**
   * Upload a file. Returns file info including the ref to use in messages.
   * @param file - File blob to upload
   * @param filename - Original filename
   */
  async uploadFile(file: Blob, filename: string): Promise<{ id: string; mime: string; size: number; deduplicated: boolean }> {
    if (!this._token) {
      throw new MVChat2Error(401, 'Not authenticated');
    }

    const formData = new FormData();
    formData.append('file', file, filename);

    const response = await fetch(`${this.getHttpBaseUrl()}/v0/file/upload`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${this._token}`,
      },
      body: formData,
    });

    if (!response.ok) {
      throw new MVChat2Error(response.status, await response.text());
    }

    return response.json() as Promise<{ id: string; mime: string; size: number; deduplicated: boolean }>;
  }

  /**
   * Get the URL for downloading a file.
   * @param fileId - File ID (ref from message media)
   * @param thumbnail - If true, get thumbnail URL instead
   */
  getFileUrl(fileId: string, thumbnail: boolean = false): string {
    const path = thumbnail ? `${fileId}/thumb` : fileId;
    return `${this.getHttpBaseUrl()}/v0/file/${path}?token=${this._token}`;
  }

  /**
   * Download a file as a blob.
   * @param fileId - File ID
   * @param thumbnail - If true, download thumbnail instead
   */
  async downloadFile(fileId: string, thumbnail: boolean = false): Promise<Blob> {
    if (!this._token) {
      throw new MVChat2Error(401, 'Not authenticated');
    }

    const url = this.getFileUrl(fileId, thumbnail);
    const response = await fetch(url);

    if (!response.ok) {
      throw new MVChat2Error(response.status, await response.text());
    }

    return response.blob();
  }
}
