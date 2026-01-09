import { useState, useEffect, useCallback } from 'react';
import { MVChat2Client } from '../client';
import { ReadReceipt } from '../types';

export interface UseReadReceiptsResult {
  /** Map of user ID to their read/recv sequence numbers */
  receipts: Map<string, { readSeq: number; recvSeq: number }>;
  /** Get all receipts as array */
  receiptList: ReadReceipt[];
  /** Check if a user has read up to a specific seq */
  hasRead: (userId: string, seq: number) => boolean;
  /** Check if a user has received up to a specific seq */
  hasReceived: (userId: string, seq: number) => boolean;
  /** Get users who have read a specific message */
  getReadersForSeq: (seq: number) => string[];
  /** Loading state */
  isLoading: boolean;
  /** Error state */
  error: Error | null;
  /** Refresh receipts */
  refresh: () => Promise<void>;
}

export function useReadReceipts(client: MVChat2Client, conversationId: string): UseReadReceiptsResult {
  const [receipts, setReceipts] = useState<Map<string, { readSeq: number; recvSeq: number }>>(new Map());
  const [receiptList, setReceiptList] = useState<ReadReceipt[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const refresh = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const list = await client.getReceipts(conversationId);
      setReceiptList(list);

      const map = new Map<string, { readSeq: number; recvSeq: number }>();
      for (const r of list) {
        map.set(r.user, { readSeq: r.readSeq, recvSeq: r.recvSeq });
      }
      setReceipts(map);
    } catch (err) {
      setError(err as Error);
    } finally {
      setIsLoading(false);
    }
  }, [client, conversationId]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  // Listen for read receipts
  useEffect(() => {
    const handleRead = (event: { conv: string; user: string; seq: number }) => {
      if (event.conv === conversationId) {
        setReceipts((prev) => {
          const next = new Map(prev);
          const current = next.get(event.user) || { readSeq: 0, recvSeq: 0 };
          next.set(event.user, { ...current, readSeq: Math.max(current.readSeq, event.seq) });
          return next;
        });
        setReceiptList((prev) => {
          const idx = prev.findIndex((r) => r.user === event.user);
          if (idx >= 0) {
            const updated = [...prev];
            updated[idx] = { ...updated[idx], readSeq: Math.max(updated[idx].readSeq, event.seq) };
            return updated;
          }
          return [...prev, { user: event.user, readSeq: event.seq, recvSeq: 0 }];
        });
      }
    };

    const handleRecv = (event: { conv: string; user: string; seq: number }) => {
      if (event.conv === conversationId) {
        setReceipts((prev) => {
          const next = new Map(prev);
          const current = next.get(event.user) || { readSeq: 0, recvSeq: 0 };
          next.set(event.user, { ...current, recvSeq: Math.max(current.recvSeq, event.seq) });
          return next;
        });
        setReceiptList((prev) => {
          const idx = prev.findIndex((r) => r.user === event.user);
          if (idx >= 0) {
            const updated = [...prev];
            updated[idx] = { ...updated[idx], recvSeq: Math.max(updated[idx].recvSeq, event.seq) };
            return updated;
          }
          return [...prev, { user: event.user, readSeq: 0, recvSeq: event.seq }];
        });
      }
    };

    client.on('read', handleRead);
    client.on('recv', handleRecv);

    return () => {
      client.off('read', handleRead);
      client.off('recv', handleRecv);
    };
  }, [client, conversationId]);

  const hasRead = useCallback(
    (userId: string, seq: number) => {
      const r = receipts.get(userId);
      return r ? r.readSeq >= seq : false;
    },
    [receipts]
  );

  const hasReceived = useCallback(
    (userId: string, seq: number) => {
      const r = receipts.get(userId);
      return r ? r.recvSeq >= seq : false;
    },
    [receipts]
  );

  const getReadersForSeq = useCallback(
    (seq: number) => {
      const readers: string[] = [];
      for (const [userId, r] of receipts) {
        if (r.readSeq >= seq) {
          readers.push(userId);
        }
      }
      return readers;
    },
    [receipts]
  );

  return {
    receipts,
    receiptList,
    hasRead,
    hasReceived,
    getReadersForSeq,
    isLoading,
    error,
    refresh,
  };
}
