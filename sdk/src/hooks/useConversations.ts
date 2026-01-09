import { useState, useEffect, useCallback } from 'react';
import { MVChat2Client } from '../client';
import { Conversation, StartDMResult, CreateRoomResult } from '../types';

export interface UseConversationsResult {
  conversations: Conversation[];
  isLoading: boolean;
  error: Error | null;
  refresh: () => Promise<void>;
  startDM: (userId: string) => Promise<StartDMResult>;
  createRoom: (options: { public: any }) => Promise<CreateRoomResult>;
  // Room management
  inviteToRoom: (roomId: string, userId: string) => Promise<void>;
  leaveRoom: (roomId: string) => Promise<void>;
  kickFromRoom: (roomId: string, userId: string) => Promise<void>;
  updateRoom: (roomId: string, options: { public?: any }) => Promise<void>;
  // DM settings
  updateDMSettings: (convId: string, options: { favorite?: boolean; muted?: boolean; blocked?: boolean }) => Promise<void>;
  // Disappearing messages
  setDMDisappearingTTL: (convId: string, ttl: number | null) => Promise<void>;
  setRoomDisappearingTTL: (roomId: string, ttl: number | null) => Promise<void>;
  // Pinned messages
  pinMessage: (convId: string, seq: number) => Promise<void>;
  unpinMessage: (convId: string) => Promise<void>;
  // Clear conversation history
  clearConversation: (convId: string, seq: number) => Promise<void>;
}

export function useConversations(client: MVChat2Client): UseConversationsResult {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const refresh = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const convs = await client.getConversations();
      setConversations(convs);
    } catch (err) {
      setError(err as Error);
    } finally {
      setIsLoading(false);
    }
  }, [client]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  // Listen for events that affect conversation state
  useEffect(() => {
    const handleRefresh = () => refresh();

    client.on('pin', handleRefresh);
    client.on('unpin', handleRefresh);
    client.on('disappearingUpdated', handleRefresh);
    client.on('memberJoined', handleRefresh);
    client.on('memberLeft', handleRefresh);
    client.on('memberKicked', handleRefresh);
    client.on('roomUpdated', handleRefresh);

    return () => {
      client.off('pin', handleRefresh);
      client.off('unpin', handleRefresh);
      client.off('disappearingUpdated', handleRefresh);
      client.off('memberJoined', handleRefresh);
      client.off('memberLeft', handleRefresh);
      client.off('memberKicked', handleRefresh);
      client.off('roomUpdated', handleRefresh);
    };
  }, [client, refresh]);

  const startDM = useCallback(
    async (userId: string) => {
      const result = await client.startDM(userId);
      await refresh();
      return result;
    },
    [client, refresh]
  );

  const createRoom = useCallback(
    async (options: { public: any }) => {
      const result = await client.createRoom(options);
      await refresh();
      return result;
    },
    [client, refresh]
  );

  // Room management
  const inviteToRoom = useCallback(
    async (roomId: string, userId: string) => {
      await client.inviteToRoom(roomId, userId);
    },
    [client]
  );

  const leaveRoom = useCallback(
    async (roomId: string) => {
      await client.leaveRoom(roomId);
      await refresh();
    },
    [client, refresh]
  );

  const kickFromRoom = useCallback(
    async (roomId: string, userId: string) => {
      await client.kickFromRoom(roomId, userId);
    },
    [client]
  );

  const updateRoom = useCallback(
    async (roomId: string, options: { public?: any }) => {
      await client.updateRoom(roomId, options);
      await refresh();
    },
    [client, refresh]
  );

  // DM settings
  const updateDMSettings = useCallback(
    async (convId: string, options: { favorite?: boolean; muted?: boolean; blocked?: boolean }) => {
      await client.updateDMSettings(convId, options);
      await refresh();
    },
    [client, refresh]
  );

  // Disappearing messages
  const setDMDisappearingTTL = useCallback(
    async (convId: string, ttl: number | null) => {
      await client.setDMDisappearingTTL(convId, ttl);
      await refresh();
    },
    [client, refresh]
  );

  const setRoomDisappearingTTL = useCallback(
    async (roomId: string, ttl: number | null) => {
      await client.setRoomDisappearingTTL(roomId, ttl);
      await refresh();
    },
    [client, refresh]
  );

  // Pinned messages
  const pinMessage = useCallback(
    async (convId: string, seq: number) => {
      await client.pinMessage(convId, seq);
      await refresh();
    },
    [client, refresh]
  );

  const unpinMessage = useCallback(
    async (convId: string) => {
      await client.unpinMessage(convId);
      await refresh();
    },
    [client, refresh]
  );

  const clearConversation = useCallback(
    async (convId: string, seq: number) => {
      await client.clearConversation(convId, seq);
      await refresh();
    },
    [client, refresh]
  );

  return {
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
    clearConversation,
  };
}
