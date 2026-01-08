import { useState, useEffect, useCallback } from 'react';
import { MVChat2Client } from '../client';
import { Contact } from '../types';

export interface UseContactsResult {
  contacts: Contact[];
  isLoading: boolean;
  error: Error | null;
  refresh: () => Promise<void>;
  addContact: (userId: string) => Promise<void>;
  removeContact: (userId: string) => Promise<void>;
  updateNickname: (userId: string, nickname: string | null) => Promise<void>;
}

export function useContacts(client: MVChat2Client): UseContactsResult {
  const [contacts, setContacts] = useState<Contact[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const refresh = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const result = await client.getContacts();
      setContacts(result);
    } catch (err) {
      setError(err as Error);
    } finally {
      setIsLoading(false);
    }
  }, [client]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const addContact = useCallback(
    async (userId: string) => {
      await client.addContact(userId);
      await refresh();
    },
    [client, refresh]
  );

  const removeContact = useCallback(
    async (userId: string) => {
      await client.removeContact(userId);
      await refresh();
    },
    [client, refresh]
  );

  const updateNickname = useCallback(
    async (userId: string, nickname: string | null) => {
      await client.updateContactNickname(userId, nickname);
      await refresh();
    },
    [client, refresh]
  );

  return {
    contacts,
    isLoading,
    error,
    refresh,
    addContact,
    removeContact,
    updateNickname,
  };
}
