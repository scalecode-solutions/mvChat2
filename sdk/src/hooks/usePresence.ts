import { useState, useEffect, useCallback } from 'react';
import { MVChat2Client } from '../client';

export interface UserPresence {
  online: boolean;
  lastSeen?: string;
}

export interface UsePresenceResult {
  /** Map of user ID to presence state */
  presence: Map<string, UserPresence>;
  /** Get presence for a specific user */
  getPresence: (userId: string) => UserPresence | undefined;
  /** Check if a user is online */
  isOnline: (userId: string) => boolean;
}

export function usePresence(client: MVChat2Client): UsePresenceResult {
  const [presence, setPresence] = useState<Map<string, UserPresence>>(new Map());

  useEffect(() => {
    const handlePresence = (event: { user: string; online: boolean; lastSeen?: string }) => {
      setPresence((prev) => {
        const next = new Map(prev);
        next.set(event.user, {
          online: event.online,
          lastSeen: event.lastSeen,
        });
        return next;
      });
    };

    client.on('presence', handlePresence);

    return () => {
      client.off('presence', handlePresence);
    };
  }, [client]);

  const getPresence = useCallback(
    (userId: string) => presence.get(userId),
    [presence]
  );

  const isOnline = useCallback(
    (userId: string) => presence.get(userId)?.online ?? false,
    [presence]
  );

  return {
    presence,
    getPresence,
    isOnline,
  };
}
