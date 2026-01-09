import { useState, useEffect, useCallback } from 'react';
import { MVChat2Client } from '../client';
import { Member } from '../types';

export interface UseMembersResult {
  members: Member[];
  isLoading: boolean;
  error: Error | null;
  refresh: () => Promise<void>;
}

export function useMembers(client: MVChat2Client, conversationId: string): UseMembersResult {
  const [members, setMembers] = useState<Member[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const refresh = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const list = await client.getMembers(conversationId);
      setMembers(list);
    } catch (err) {
      setError(err as Error);
    } finally {
      setIsLoading(false);
    }
  }, [client, conversationId]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  // Listen for member changes
  useEffect(() => {
    const handleMemberJoined = (event: { conv: string }) => {
      if (event.conv === conversationId) {
        refresh();
      }
    };

    const handleMemberLeft = (event: { conv: string }) => {
      if (event.conv === conversationId) {
        refresh();
      }
    };

    const handleMemberKicked = (event: { conv: string }) => {
      if (event.conv === conversationId) {
        refresh();
      }
    };

    client.on('memberJoined', handleMemberJoined);
    client.on('memberLeft', handleMemberLeft);
    client.on('memberKicked', handleMemberKicked);

    return () => {
      client.off('memberJoined', handleMemberJoined);
      client.off('memberLeft', handleMemberLeft);
      client.off('memberKicked', handleMemberKicked);
    };
  }, [client, conversationId, refresh]);

  // Listen for presence changes to update online status
  useEffect(() => {
    const handlePresence = (event: { user: string; online: boolean; lastSeen?: string }) => {
      setMembers((prev) =>
        prev.map((m) =>
          m.id === event.user
            ? { ...m, online: event.online, lastSeen: event.lastSeen }
            : m
        )
      );
    };

    client.on('presence', handlePresence);

    return () => {
      client.off('presence', handlePresence);
    };
  }, [client]);

  return {
    members,
    isLoading,
    error,
    refresh,
  };
}
