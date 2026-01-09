# In-App Audio Calls

## Overview

WebRTC-based audio calls without CallKit integration. Calls only work when the app is open (stealth mode for DV survivor safety).

## Architecture

```
┌─────────┐         ┌─────────────┐         ┌─────────┐
│ Caller  │◄───────►│  mvChat2    │◄───────►│ Callee  │
│ (WebRTC)│   WS    │ (Signaling) │   WS    │ (WebRTC)│
└────┬────┘         └─────────────┘         └────┬────┘
     │                                           │
     │              ┌─────────────┐              │
     └─────────────►│   coturn    │◄─────────────┘
          STUN/TURN │ (STUN/TURN) │ STUN/TURN
                    └─────────────┘
```

- **Signaling**: WebSocket messages through mvChat2 for call setup
- **Media**: Peer-to-peer audio via WebRTC (or relayed through TURN if needed)
- **NAT Traversal**: coturn provides STUN (discovery) and TURN (relay fallback)

## Wire Protocol

### Initiate Call
```json
{"id":"1","call":{"action":"initiate","user":"callee-uuid"}}
```
Response:
```json
{"ctrl":{"id":"1","code":200,"params":{"call_id":"call-uuid"}}}
```

### Incoming Call (to callee)
```json
{"info":{"what":"incoming_call","call_id":"call-uuid","from":"caller-uuid","fromPublic":{"fn":"Alice"}}}
```

### Answer Call
```json
{"id":"2","call":{"action":"answer","call_id":"call-uuid"}}
```

### Reject Call
```json
{"id":"3","call":{"action":"reject","call_id":"call-uuid"}}
```

### End Call
```json
{"id":"4","call":{"action":"end","call_id":"call-uuid"}}
```

### SDP Offer/Answer Exchange
```json
{"id":"5","call":{"action":"sdp","call_id":"call-uuid","sdp":{"type":"offer","sdp":"v=0\r\n..."}}}
{"id":"6","call":{"action":"sdp","call_id":"call-uuid","sdp":{"type":"answer","sdp":"v=0\r\n..."}}}
```

### ICE Candidate Exchange
```json
{"id":"7","call":{"action":"ice","call_id":"call-uuid","candidate":{"candidate":"candidate:...","sdpMid":"0","sdpMLineIndex":0}}}
```

### Call State Changes (broadcast to both parties)
```json
{"info":{"what":"call_ringing","call_id":"call-uuid"}}
{"info":{"what":"call_answered","call_id":"call-uuid"}}
{"info":{"what":"call_ended","call_id":"call-uuid","reason":"hangup|rejected|timeout|error"}}
```

## Backend Implementation

### New Types (types.go)

```go
type MsgClientCall struct {
    Action   string          `json:"action"`   // initiate, answer, reject, end, sdp, ice
    CallID   string          `json:"call_id,omitempty"`
    User     string          `json:"user,omitempty"`     // for initiate
    SDP      json.RawMessage `json:"sdp,omitempty"`      // for sdp action
    Candidate json.RawMessage `json:"candidate,omitempty"` // for ice action
}
```

### Call State Management

```go
type CallState struct {
    ID        uuid.UUID
    CallerID  uuid.UUID
    CalleeID  uuid.UUID
    Status    string    // "ringing", "connected", "ended"
    StartedAt time.Time
    EndedAt   *time.Time
}

// In-memory map (calls are ephemeral)
var activeCalls = sync.Map{} // map[uuid.UUID]*CallState
```

### Handler Functions

```go
func (h *Hub) handleCall(ctx context.Context, s *Session, msg *ClientMessage, call *MsgClientCall) {
    switch call.Action {
    case "initiate":
        h.handleCallInitiate(ctx, s, msg, call)
    case "answer":
        h.handleCallAnswer(ctx, s, msg, call)
    case "reject":
        h.handleCallReject(ctx, s, msg, call)
    case "end":
        h.handleCallEnd(ctx, s, msg, call)
    case "sdp":
        h.handleCallSDP(ctx, s, msg, call)
    case "ice":
        h.handleCallICE(ctx, s, msg, call)
    }
}
```

### Permission Rules

- Users can only call their contacts
- Callee must be online (no voicemail/offline calls)
- One active call per user at a time
- Call timeout: 30 seconds if not answered

## coturn Setup

### Docker Compose Addition

```yaml
services:
  coturn:
    image: coturn/coturn:latest
    container_name: mvchat-coturn
    network_mode: host
    volumes:
      - ./coturn/turnserver.conf:/etc/coturn/turnserver.conf:ro
    restart: unless-stopped
```

### turnserver.conf

```conf
# STUN/TURN server configuration
listening-port=3478
tls-listening-port=5349

# Server identification
realm=mvchat.app
server-name=turn.mvchat.app

# Authentication
lt-cred-mech
# Static user (or use REST API for dynamic credentials)
user=mvchat:CHANGE_THIS_SECRET

# Security
fingerprint
no-multicast-peers
no-cli

# Logging
log-file=/var/log/turnserver.log
simple-log

# Performance
total-quota=100
stale-nonce=600

# TURN relay ports (for media relay when p2p fails)
min-port=49152
max-port=65535

# Optional: TLS (recommended for production)
# cert=/etc/ssl/certs/turn.pem
# pkey=/etc/ssl/private/turn.key
```

### Firewall Rules

```bash
# STUN/TURN signaling
ufw allow 3478/tcp
ufw allow 3478/udp
ufw allow 5349/tcp  # TLS

# TURN relay range
ufw allow 49152:65535/udp
```

### Dynamic TURN Credentials (Optional)

Instead of static credentials, generate short-lived tokens:

```go
func generateTURNCredentials(userID uuid.UUID) (username, credential string, ttl int) {
    ttl = 86400 // 24 hours
    timestamp := time.Now().Unix() + int64(ttl)
    username = fmt.Sprintf("%d:%s", timestamp, userID.String())

    h := hmac.New(sha1.New, []byte(turnSecret))
    h.Write([]byte(username))
    credential = base64.StdEncoding.EncodeToString(h.Sum(nil))

    return username, credential, ttl
}
```

Client receives TURN credentials via:
```json
{"ctrl":{"code":200,"params":{"ice_servers":[
    {"urls":"stun:turn.mvchat.app:3478"},
    {"urls":"turn:turn.mvchat.app:3478","username":"1234567890:user-uuid","credential":"base64..."}
]}}}
```

## Client Implementation (React Native)

### Dependencies

```bash
npm install react-native-webrtc
cd ios && pod install
```

### Basic Call Hook

```typescript
import {
  RTCPeerConnection,
  RTCSessionDescription,
  RTCIceCandidate,
  mediaDevices,
} from 'react-native-webrtc';

const useCall = () => {
  const [callState, setCallState] = useState<'idle' | 'ringing' | 'connected'>('idle');
  const peerConnection = useRef<RTCPeerConnection | null>(null);

  const startCall = async (userId: string) => {
    // Get ICE servers from backend
    const iceServers = await getICEServers();

    peerConnection.current = new RTCPeerConnection({ iceServers });

    // Add local audio track
    const stream = await mediaDevices.getUserMedia({ audio: true, video: false });
    stream.getTracks().forEach(track => {
      peerConnection.current?.addTrack(track, stream);
    });

    // Handle ICE candidates
    peerConnection.current.onicecandidate = (event) => {
      if (event.candidate) {
        sendICECandidate(callId, event.candidate);
      }
    };

    // Create and send offer
    const offer = await peerConnection.current.createOffer();
    await peerConnection.current.setLocalDescription(offer);
    sendSDP(callId, offer);
  };

  // ... answer, hangup, etc.
};
```

## Stealth Mode Considerations

### No CallKit
- Calls don't appear in iOS call history
- No incoming call UI when app is backgrounded
- No integration with car Bluetooth "recent calls"

### No Push Notifications for Calls
- User must have app open to receive calls
- Consider: "User is available" indicator in contacts list

### Audio Session (iOS)
```typescript
// Configure audio for VoIP without CallKit
import { AudioSession } from 'react-native-webrtc';

AudioSession.configure({
  category: 'PlayAndRecord',
  mode: 'VoiceChat',
  options: ['DefaultToSpeaker', 'AllowBluetooth'],
});
```

## Database Schema (Optional)

For call history (if needed):

```sql
CREATE TABLE call_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    caller_id UUID NOT NULL REFERENCES users(id),
    callee_id UUID NOT NULL REFERENCES users(id),
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    answered_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    duration_seconds INT,
    end_reason VARCHAR(32), -- 'completed', 'rejected', 'missed', 'failed'
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_call_history_caller ON call_history(caller_id, created_at DESC);
CREATE INDEX idx_call_history_callee ON call_history(callee_id, created_at DESC);
```

## Implementation Order

1. **Backend signaling** - WebSocket handlers for call flow
2. **In-memory call state** - Track active calls
3. **coturn setup** - Docker + firewall
4. **Client WebRTC** - react-native-webrtc integration
5. **Call UI** - Ring screen, in-call screen, end call
6. **Testing** - Cross-device, NAT scenarios
7. **Call history** (optional) - Database + API

## Security Notes

- TURN credentials should be short-lived (24h max)
- Validate caller/callee are contacts before allowing call
- Rate limit call initiation (prevent harassment)
- No call recording server-side (privacy)
- Media is encrypted by WebRTC (SRTP)
