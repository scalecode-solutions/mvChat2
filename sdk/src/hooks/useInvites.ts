import { useState, useEffect, useCallback } from 'react';
import { MVChat2Client } from '../client';
import { Invite } from '../types';

export interface UseInvitesResult {
  invites: Invite[];
  isLoading: boolean;
  error: Error | null;
  refresh: () => Promise<void>;
  createInvite: (email: string, name?: string) => Promise<{ id: string; code: string; expiresAt: string }>;
  revokeInvite: (inviteId: string) => Promise<void>;
  redeemInvite: (code: string) => Promise<{ inviter: string; inviterPublic: any; conv: string }>;
}

export function useInvites(client: MVChat2Client): UseInvitesResult {
  const [invites, setInvites] = useState<Invite[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const refresh = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const list = await client.listInvites();
      setInvites(list);
    } catch (err) {
      setError(err as Error);
    } finally {
      setIsLoading(false);
    }
  }, [client]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const createInvite = useCallback(
    async (email: string, name?: string) => {
      const result = await client.createInvite(email, name);
      await refresh();
      return result;
    },
    [client, refresh]
  );

  const revokeInvite = useCallback(
    async (inviteId: string) => {
      await client.revokeInvite(inviteId);
      await refresh();
    },
    [client, refresh]
  );

  const redeemInvite = useCallback(
    async (code: string) => {
      return client.redeemInvite(code);
    },
    [client]
  );

  return {
    invites,
    isLoading,
    error,
    refresh,
    createInvite,
    revokeInvite,
    redeemInvite,
  };
}
