# Test Users

> **Server:** api2.mvchat.app (mvchat2-test deployment)
> **Last Updated:** January 9, 2026

## Current Test Users

| Username | Password | Name | User ID |
|----------|----------|------|---------|
| tsrlegends@gmail.com | Echelon1! | Travis Marquis | 22eb3c1a-541f-493e-ba44-55915515576d |
| shelby.cottrell@yahoo.com | DrPepper91 | Shelby Rogers | 1557f447-949f-44c2-b52b-8f5c77a0fe3b |

## Creating New Test Users

### Prerequisites

1. SSH access to `root@scalecode.dev`
2. Go installed at `/usr/local/go/bin/go`
3. Password hasher at `/root/hashpw/hashpw.go`

### Step 1: Generate Password Hash

```bash
# SSH to server
ssh root@scalecode.dev

# Update password in hasher and run
cd /root/hashpw
sed -i 's/password := ".*"/password := "YOUR_PASSWORD_HERE"/' hashpw.go
/usr/local/go/bin/go run hashpw.go
```

This outputs an Argon2id hash like:
```
$argon2id$SALT$HASH
```

### Step 2: Create User Record

```bash
docker exec mvchat2-test-db psql -U mvchat2 -d mvchat2 -c "
INSERT INTO users (public, email, email_verified) 
VALUES ('{\"fn\": \"FIRST_NAME\", \"ln\": \"LAST_NAME\"}', 'EMAIL@DOMAIN.COM', true) 
RETURNING id;"
```

Save the returned UUID.

### Step 3: Create Auth Record

```bash
docker exec mvchat2-test-db psql -U mvchat2 -d mvchat2 -c "
INSERT INTO auth (user_id, scheme, secret, uname) 
VALUES ('USER_UUID_HERE', 'basic', '\$argon2id\$SALT\$HASH', 'EMAIL@DOMAIN.COM');"
```

**Note:** The `$` signs in the hash need to be escaped as `\$` in the shell command.

### Step 4: Verify Login

```bash
# Base64 encode credentials
echo -n 'username:password' | base64

# Test WebSocket login
timeout 10 websocat -n wss://api2.mvchat.app/v0/ws << 'EOF'
{"id":"1","hi":{"ver":"0.1.0","ua":"TestClient/1.0"}}
{"id":"2","login":{"scheme":"basic","secret":"BASE64_ENCODED_CREDENTIALS"}}
EOF
```

A successful login returns `code: 200` with a JWT token.

## Password Hasher Source

Located at `/root/hashpw/hashpw.go` on the server:

```go
package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/argon2"
)

func main() {
	password := "YOUR_PASSWORD_HERE"
	salt := make([]byte, 16)
	rand.Read(salt)
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	fmt.Printf("$argon2id$%s$%s\n",
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash))
}
```

First-time setup:
```bash
mkdir -p /root/hashpw && cd /root/hashpw
# Create hashpw.go with content above
/usr/local/go/bin/go mod init hashpw
/usr/local/go/bin/go mod tidy
```

## Quick One-Liner (Full User Creation)

```bash
# Variables
EMAIL="newuser@example.com"
PASSWORD="SecurePass123"
FIRST="John"
LAST="Doe"

# Generate hash
ssh root@scalecode.dev "cd /root/hashpw && sed -i 's/password := \".*\"/password := \"$PASSWORD\"/' hashpw.go && /usr/local/go/bin/go run hashpw.go"

# Then manually run the INSERT statements with the hash
```

## Database Schema Reference

**users table:**
- `id` UUID (auto-generated)
- `public` JSONB - `{"fn": "First", "ln": "Last"}`
- `email` VARCHAR(255)
- `email_verified` BOOLEAN (set to true for test users)
- `user_agent` VARCHAR(255) NOT NULL DEFAULT ''
- `must_change_password` BOOLEAN DEFAULT FALSE

**auth table:**
- `user_id` UUID (FK to users)
- `scheme` VARCHAR - always 'basic' for password auth
- `secret` VARCHAR - Argon2id hash
- `uname` VARCHAR - username (usually email)
