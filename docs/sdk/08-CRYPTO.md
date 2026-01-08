# End-to-End Encryption (E2EE)

> **IMPORTANT**: This documentation is accurate as of January 8th, 2026 at 4:00 AM CST. Review the actual code before assuming anything here is still current.

> **NOTE**: E2EE is not yet implemented in mvChat2. This document describes the planned architecture.

## Overview

mvChat2 uses a triple-layer security model:

1. **E2EE (client-to-client)**: Messages encrypted on sender's device, decrypted on recipient's device
2. **Encryption at rest (server-side)**: Server encrypts the already-encrypted ciphertext with AES-GCM
3. **Soft delete**: Nothing is truly deleted, allowing admin restore for evidence

The server NEVER sees plaintext message content.

## Key Management

### Per-Conversation Keys

Each conversation has its own encryption key:

```typescript
// Key is generated when conversation is created
const conversationKey = await crypto.generateKey();

// Key is encrypted for each member using their public key
const encryptedKeyForAlice = await crypto.encryptKey(conversationKey, alicePublicKey);
const encryptedKeyForBob = await crypto.encryptKey(conversationKey, bobPublicKey);
```

### User Key Pair

Each user has an asymmetric key pair:

```typescript
// Generated on signup, stored securely on device
const { publicKey, privateKey } = await crypto.generateKeyPair();

// Public key is uploaded to server
await client.updateProfile({ publicKey });

// Private key is stored locally (never leaves device)
await SecureStore.setItemAsync('privateKey', privateKey);
```

## Encryption Flow

### Sending a Message

```typescript
// 1. Get conversation key (decrypt with your private key)
const conversationKey = await crypto.decryptKey(
  encryptedConversationKey,
  myPrivateKey
);

// 2. Encrypt message content
const encryptedContent = await crypto.encrypt(
  JSON.stringify(iridoContent),
  conversationKey
);

// 3. Send encrypted content
await client.sendMessage(conversationId, {
  encrypted: encryptedContent,
});
```

### Receiving a Message

```typescript
// 1. Get conversation key
const conversationKey = await crypto.decryptKey(
  encryptedConversationKey,
  myPrivateKey
);

// 2. Decrypt message content
const decryptedContent = await crypto.decrypt(
  message.encrypted,
  conversationKey
);

// 3. Parse Irido content
const irido = JSON.parse(decryptedContent);
```

## SDK API

```typescript
// Enable E2EE for client
const client = new MVChat2Client({
  url: 'wss://api2.mvchat.app/v0/ws',
  e2ee: true,
  privateKey: await SecureStore.getItemAsync('privateKey'),
});

// Messages are automatically encrypted/decrypted
await client.sendMessage(conversationId, {
  text: 'This will be encrypted automatically',
});

// Received messages are automatically decrypted
client.on('message', (message) => {
  console.log(message.content.text); // Already decrypted
});
```

## Key Backup & Recovery

For cross-device sync, users need to backup their private key:

```typescript
// Export key (encrypted with password)
const backup = await crypto.exportKey(privateKey, userPassword);

// Store backup on server (encrypted, server can't read it)
await client.storeKeyBackup(backup);

// On new device, recover key
const backup = await client.getKeyBackup();
const privateKey = await crypto.importKey(backup, userPassword);
```

## Algorithm Details

| Purpose | Algorithm |
|---------|-----------|
| Symmetric encryption | AES-256-GCM |
| Asymmetric encryption | X25519 (key exchange) + XChaCha20-Poly1305 |
| Key derivation | Argon2id |
| Hashing | BLAKE3 |

## Security Considerations

- **Forward secrecy**: Future implementation may use Double Ratchet (Signal protocol)
- **Key rotation**: Conversation keys can be rotated when members change
- **Verification**: Users can verify each other's keys via QR code or safety numbers
- **Subpoena-proof**: Server only has double-encrypted blobs, no plaintext

## Evidence Preservation

Even with E2EE, evidence can be preserved:

1. Messages are soft-deleted (never truly deleted)
2. Admin can restore soft-deleted messages
3. User downloads and decrypts with their key
4. Exports plaintext for court evidence

This works because the USER holds the decryption key, not the server.
