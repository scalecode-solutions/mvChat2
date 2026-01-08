# Connection Management

> **IMPORTANT**: This documentation is accurate as of January 8th, 2026 at 4:00 AM CST. Review the actual code before assuming anything here is still current.

## Overview

The SDK manages WebSocket connections to mvChat2 with automatic reconnection, compression, and state management.

## Client Configuration

```typescript
import { MVChat2Client } from '@mvchat/react-native-sdk';

const client = new MVChat2Client({
  // Required
  url: 'wss://api2.mvchat.app/v0/ws',
  
  // Optional
  autoReconnect: true,           // Auto-reconnect on disconnect (default: true)
  reconnectDelay: 1000,          // Initial reconnect delay in ms (default: 1000)
  reconnectMaxDelay: 30000,      // Max reconnect delay in ms (default: 30000)
  reconnectBackoff: 1.5,         // Backoff multiplier (default: 1.5)
  compression: true,             // Enable per-message compression (default: true)
  timeout: 10000,                // Request timeout in ms (default: 10000)
});
```

## Connection Lifecycle

```typescript
// Connect
await client.connect();

// Disconnect
client.disconnect();

// Check status
client.isConnected;  // boolean
client.state;        // 'disconnected' | 'connecting' | 'connected' | 'reconnecting'
```

## Events

```typescript
// Connection events
client.on('connect', () => {
  console.log('Connected to mvChat2');
});

client.on('disconnect', (reason: string) => {
  console.log('Disconnected:', reason);
});

client.on('reconnecting', (attempt: number) => {
  console.log('Reconnecting, attempt:', attempt);
});

client.on('error', (error: Error) => {
  console.error('Connection error:', error);
});
```

## Reconnection Strategy

The SDK uses exponential backoff for reconnection:

1. First attempt: 1 second delay
2. Second attempt: 1.5 seconds
3. Third attempt: 2.25 seconds
4. ...continues until max delay (30 seconds)

After successful reconnection:
- Re-authenticates with stored token
- Resumes subscriptions
- Fetches missed messages (history recovery)

## React Hook

```typescript
import { useClient } from '@mvchat/react-native-sdk';

function App() {
  const { 
    isConnected, 
    state, 
    error,
    connect,
    disconnect 
  } = useClient(client);

  if (state === 'connecting') {
    return <LoadingSpinner />;
  }

  if (state === 'reconnecting') {
    return <ReconnectingBanner />;
  }

  if (error) {
    return <ErrorScreen error={error} onRetry={connect} />;
  }

  return <MainApp />;
}
```

## Wire Protocol

### Handshake

Client sends:
```json
{"id":"1","hi":{"ver":"0.1.0","ua":"Clingy/1.0 (iOS)"}}
```

Server responds:
```json
{
  "ctrl": {
    "id": "1",
    "code": 200,
    "params": {
      "ver": "0.1.0",
      "build": "20260108T040000Z",
      "sid": "session-uuid"
    }
  }
}
```

### Session Resumption (Future)

For reconnection without re-auth:
```json
{"id":"1","hi":{"ver":"0.1.0","sid":"previous-session-uuid"}}
```
