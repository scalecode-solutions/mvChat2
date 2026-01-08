import { useState, useEffect, useCallback } from 'react';
import { MVChat2Client } from '../client';
import { ConnectionState } from '../types';

export interface UseClientResult {
  isConnected: boolean;
  state: ConnectionState;
  error: Error | null;
  connect: () => Promise<void>;
  disconnect: () => void;
}

export function useClient(client: MVChat2Client): UseClientResult {
  const [state, setState] = useState<ConnectionState>(client.connectionState);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    const handleStateChange = (newState: ConnectionState) => {
      setState(newState);
    };

    const handleError = (err: Error) => {
      setError(err);
    };

    const handleConnect = () => {
      setError(null);
    };

    client.on('stateChange', handleStateChange);
    client.on('error', handleError);
    client.on('connect', handleConnect);

    return () => {
      client.off('stateChange', handleStateChange);
      client.off('error', handleError);
      client.off('connect', handleConnect);
    };
  }, [client]);

  const connect = useCallback(async () => {
    setError(null);
    try {
      await client.connect();
    } catch (err) {
      setError(err as Error);
      throw err;
    }
  }, [client]);

  const disconnect = useCallback(() => {
    client.disconnect();
  }, [client]);

  return {
    isConnected: state === 'connected',
    state,
    error,
    connect,
    disconnect,
  };
}
