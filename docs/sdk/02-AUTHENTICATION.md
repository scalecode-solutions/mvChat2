# Authentication

> **IMPORTANT**: This documentation is accurate as of January 8th, 2026 at 4:00 AM CST. Review the actual code before assuming anything here is still current.

## Overview

mvChat2 supports two authentication methods:
- **Basic auth**: Username/password (Argon2id hashed server-side)
- **Token auth**: JWT tokens (2-week expiry)

## Signup

### With Invite Code (Recommended)

```typescript
const result = await client.signup({
  username: 'alice',
  password: 'securepassword123',
  inviteCode: '6793336885',
  profile: {
    fn: 'Alice Smith',
    // other public profile fields
  },
  login: true, // Auto-login after signup
});

// Result
{
  user: 'alice-uuid',
  token: 'jwt...',
  expires: '2026-01-22T04:00:00Z',
  inviters: ['bob-uuid', 'cathy-uuid'], // All who invited this email
}
```

### Without Invite Code

```typescript
const result = await client.signup({
  username: 'alice',
  password: 'securepassword123',
  profile: { fn: 'Alice Smith' },
  login: true,
});
```

## Login

### With Password

```typescript
const result = await client.login({
  username: 'alice',
  password: 'securepassword123',
});

// Result
{
  user: 'alice-uuid',
  token: 'jwt...',
  expires: '2026-01-22T04:00:00Z',
}
```

### With Token

```typescript
// Store token after login
await SecureStore.setItemAsync('mvchat_token', result.token);

// Later, login with token
const token = await SecureStore.getItemAsync('mvchat_token');
const result = await client.loginWithToken(token);
```

## Token Management

```typescript
// Check if token is valid
client.isAuthenticated;  // boolean
client.user;             // User object or null
client.token;            // Current token or null

// Token refresh (automatic)
// SDK automatically refreshes token before expiry

// Logout
await client.logout();
```

## Password Change

```typescript
await client.changePassword({
  oldPassword: 'currentPassword123',
  newPassword: 'newSecurePassword456',
});

// Throws error if:
// - Old password is incorrect (403)
// - New password is too short (400)
```

## React Hook

```typescript
import { useAuth } from '@mvchat/react-native-sdk';

function LoginScreen() {
  const { 
    isAuthenticated, 
    user, 
    login, 
    signup, 
    logout,
    isLoading,
    error 
  } = useAuth(client);

  const handleLogin = async () => {
    try {
      await login({ username, password });
      navigation.navigate('Home');
    } catch (err) {
      Alert.alert('Login failed', err.message);
    }
  };

  return (
    <View>
      <TextInput value={username} onChangeText={setUsername} />
      <TextInput value={password} onChangeText={setPassword} secureTextEntry />
      <Button onPress={handleLogin} disabled={isLoading}>
        {isLoading ? 'Logging in...' : 'Login'}
      </Button>
    </View>
  );
}
```

## Invite Code Redemption (Existing User)

If a user already has an account and receives an invite code:

```typescript
const result = await client.redeemInvite('0987654321');

// Result
{
  inviter: 'cathy-uuid',
  inviterPublic: { fn: 'Cathy' },
  conv: 'dm-uuid', // DM created with inviter
}
```

## Wire Protocol

### Signup
```json
{
  "id": "1",
  "acc": {
    "user": "new",
    "scheme": "basic",
    "secret": "base64(username:password)",
    "login": true,
    "inviteCode": "6793336885",
    "desc": {
      "public": { "fn": "Alice Smith" }
    }
  }
}
```

### Login
```json
{
  "id": "2",
  "login": {
    "scheme": "basic",
    "secret": "base64(username:password)"
  }
}
```

### Token Login
```json
{
  "id": "3",
  "login": {
    "scheme": "token",
    "secret": "jwt..."
  }
}
```

### Redeem Invite (Existing User)
```json
{
  "id": "4",
  "invite": {
    "redeem": "0987654321"
  }
}
```

### Password Change
```json
{
  "id": "5",
  "acc": {
    "user": "me",
    "secret": "base64(oldPassword:newPassword)"
  }
}
```

## Security Notes

- Passwords are hashed with Argon2id server-side
- Tokens are JWTs with 2-week expiry
- Store tokens securely (use expo-secure-store or similar)
- Never log or expose tokens
