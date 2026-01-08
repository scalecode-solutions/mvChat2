import { useState, useEffect, useCallback, useRef } from 'react';
import { MVChat2Client } from '../client';

export interface UseTypingResult {
  typingUsers: string[];
  sendTyping: () => void;
}

export function useTyping(client: MVChat2Client, conversationId: string): UseTypingResult {
  const [typingUsers, setTypingUsers] = useState<string[]>([]);
  const typingTimeouts = useRef<Map<string, NodeJS.Timeout>>(new Map());
  const lastTypingSent = useRef<number>(0);

  useEffect(() => {
    const handleTyping = (event: { conv: string; user: string }) => {
      if (event.conv !== conversationId) return;

      // Clear existing timeout for this user
      const existingTimeout = typingTimeouts.current.get(event.user);
      if (existingTimeout) {
        clearTimeout(existingTimeout);
      }

      // Add user to typing list
      setTypingUsers((prev) => {
        if (prev.includes(event.user)) return prev;
        return [...prev, event.user];
      });

      // Set timeout to remove user after 5 seconds
      const timeout = setTimeout(() => {
        setTypingUsers((prev) => prev.filter((u) => u !== event.user));
        typingTimeouts.current.delete(event.user);
      }, 5000);

      typingTimeouts.current.set(event.user, timeout);
    };

    client.on('typing', handleTyping);

    return () => {
      client.off('typing', handleTyping);
      // Clear all timeouts
      typingTimeouts.current.forEach((timeout) => clearTimeout(timeout));
      typingTimeouts.current.clear();
    };
  }, [client, conversationId]);

  const sendTyping = useCallback(() => {
    const now = Date.now();
    // Debounce: only send every 3 seconds
    if (now - lastTypingSent.current < 3000) return;
    lastTypingSent.current = now;
    client.sendTyping(conversationId);
  }, [client, conversationId]);

  return {
    typingUsers,
    sendTyping,
  };
}
