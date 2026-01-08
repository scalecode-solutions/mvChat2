import { useState, useEffect, useCallback } from 'react';
import { MVChat2Client } from '../client';
import { Message, MessageEvent, SendMessageOptions } from '../types';

export interface UseMessagesResult {
  messages: Message[];
  isLoading: boolean;
  hasMore: boolean;
  error: Error | null;
  loadMore: () => Promise<void>;
  sendMessage: (options: SendMessageOptions) => Promise<{ seq: number }>;
  editMessage: (seq: number, text: string) => Promise<void>;
  unsendMessage: (seq: number) => Promise<void>;
  deleteForEveryone: (seq: number) => Promise<void>;
  deleteForMe: (seq: number) => Promise<void>;
  react: (seq: number, emoji: string) => Promise<void>;
}

export function useMessages(client: MVChat2Client, conversationId: string): UseMessagesResult {
  const [messages, setMessages] = useState<Message[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [hasMore, setHasMore] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  // Load initial messages
  useEffect(() => {
    let mounted = true;

    const loadMessages = async () => {
      setIsLoading(true);
      try {
        const msgs = await client.getMessages(conversationId, { limit: 50 });
        if (mounted) {
          setMessages(msgs.reverse());
          setHasMore(msgs.length === 50);
        }
      } catch (err) {
        if (mounted) setError(err as Error);
      } finally {
        if (mounted) setIsLoading(false);
      }
    };

    loadMessages();

    return () => {
      mounted = false;
    };
  }, [client, conversationId]);

  // Listen for new messages
  useEffect(() => {
    const handleMessage = (event: MessageEvent) => {
      if (event.conv === conversationId) {
        const msg: Message = {
          seq: event.seq,
          from: event.from,
          ts: event.ts,
          content: event.content,
          head: event.head,
        };
        setMessages((prev: Message[]) => [...prev, msg]);
      }
    };

    const handleEdit = (event: { conv: string; seq: number; content: any }) => {
      if (event.conv === conversationId) {
        setMessages((prev: Message[]) =>
          prev.map((m: Message) =>
            m.seq === event.seq ? { ...m, content: event.content } : m
          )
        );
      }
    };

    const handleUnsend = (event: { conv: string; seq: number }) => {
      if (event.conv === conversationId) {
        setMessages((prev: Message[]) => prev.filter((m: Message) => m.seq !== event.seq));
      }
    };

    const handleDelete = (event: { conv: string; seq: number }) => {
      if (event.conv === conversationId) {
        setMessages((prev: Message[]) => prev.filter((m: Message) => m.seq !== event.seq));
      }
    };

    client.on('message', handleMessage);
    client.on('edit', handleEdit);
    client.on('unsend', handleUnsend);
    client.on('deleteForEveryone', handleDelete);

    return () => {
      client.off('message', handleMessage);
      client.off('edit', handleEdit);
      client.off('unsend', handleUnsend);
      client.off('deleteForEveryone', handleDelete);
    };
  }, [client, conversationId]);

  const loadMore = useCallback(async () => {
    if (isLoading || !hasMore || messages.length === 0) return;

    setIsLoading(true);
    try {
      const oldestSeq = messages[0]?.seq;
      const olderMsgs = await client.getMessages(conversationId, {
        limit: 50,
        before: oldestSeq,
      });
      setMessages((prev: Message[]) => [...olderMsgs.reverse(), ...prev]);
      setHasMore(olderMsgs.length === 50);
    } catch (err) {
      setError(err as Error);
    } finally {
      setIsLoading(false);
    }
  }, [client, conversationId, isLoading, hasMore, messages]);

  const sendMessage = useCallback(
    async (options: SendMessageOptions) => {
      return client.sendMessage(conversationId, options);
    },
    [client, conversationId]
  );

  const editMessage = useCallback(
    async (seq: number, text: string) => {
      await client.editMessage(conversationId, seq, { text });
    },
    [client, conversationId]
  );

  const unsendMessage = useCallback(
    async (seq: number) => {
      await client.unsendMessage(conversationId, seq);
    },
    [client, conversationId]
  );

  const deleteForEveryone = useCallback(
    async (seq: number) => {
      await client.deleteForEveryone(conversationId, seq);
    },
    [client, conversationId]
  );

  const deleteForMe = useCallback(
    async (seq: number) => {
      await client.deleteForMe(conversationId, seq);
    },
    [client, conversationId]
  );

  const react = useCallback(
    async (seq: number, emoji: string) => {
      await client.react(conversationId, seq, emoji);
    },
    [client, conversationId]
  );

  return {
    messages,
    isLoading,
    hasMore,
    error,
    loadMore,
    sendMessage,
    editMessage,
    unsendMessage,
    deleteForEveryone,
    deleteForMe,
    react,
  };
}
