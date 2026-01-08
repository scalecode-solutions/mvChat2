import { useState, useEffect, useCallback } from 'react';
import { MVChat2Client } from '../client';
import { Conversation } from '../types';

export interface UseConversationsResult {
  conversations: Conversation[];
  isLoading: boolean;
  error: Error | null;
  refresh: () => Promise<void>;
  startDM: (userId: string) => Promise<{ conv: string; created: boolean }>;
  createRoom: (options: { public: any }) => Promise<{ conv: string }>;
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

  return {
    conversations,
    isLoading,
    error,
    refresh,
    startDM,
    createRoom,
  };
}
