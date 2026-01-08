# Contacts

> **IMPORTANT**: This documentation is accurate as of January 8th, 2026 at 4:00 AM CST. Review the actual code before assuming anything here is still current.

## Overview

Contacts in mvChat2 are bidirectional relationships between users. They can be created via:
- **Invite codes**: Automatically when someone redeems your invite
- **Manual add**: Searching and adding a user

Contacts are private - Alice cannot see Bob's contacts.

## Fetching Contacts

```typescript
const contacts = await client.getContacts();

// Returns:
// [
//   {
//     user: 'bob-uuid',
//     source: 'invite',        // 'invite' or 'manual'
//     nickname: 'My Lawyer',   // Optional custom name
//     createdAt: '2026-01-08T04:00:00Z',
//     public: { fn: 'Bob Smith' },
//     online: true,
//     lastSeen: '2026-01-08T03:55:00Z',
//   },
// ]
```

## Adding a Contact

```typescript
const contact = await client.addContact('user-uuid');

// Returns:
// { user: 'uuid', public: { fn: 'Cathy' } }
```

## Removing a Contact

```typescript
await client.removeContact('user-uuid');
// Removes bidirectionally
```

## Updating Contact Nickname

```typescript
await client.updateContactNickname('user-uuid', 'My Therapist');

// Clear nickname
await client.updateContactNickname('user-uuid', null);
```

## Searching Users

```typescript
const results = await client.searchUsers('bob');

// Returns:
// [
//   { id: 'uuid', public: { fn: 'Bob Smith' } },
//   { id: 'uuid', public: { fn: 'Bobby Jones' } },
// ]
```

## React Hook

```typescript
import { useContacts } from '@mvchat/react-native-sdk';

function ContactsScreen() {
  const {
    contacts,
    isLoading,
    refresh,
    addContact,
    removeContact,
    updateNickname,
  } = useContacts(client);

  return (
    <FlatList
      data={contacts}
      onRefresh={refresh}
      renderItem={({ item }) => (
        <ContactRow
          contact={item}
          onPress={() => startDMWith(item.user)}
          onLongPress={() => showContactOptions(item)}
        />
      )}
    />
  );
}
```

## Wire Protocol

### Get Contacts
```json
{
  "id": "1",
  "get": {
    "what": "contacts"
  }
}
```

### Add Contact
```json
{
  "id": "2",
  "contact": {
    "add": "user-uuid"
  }
}
```

### Remove Contact
```json
{
  "id": "3",
  "contact": {
    "remove": "user-uuid"
  }
}
```

### Update Nickname
```json
{
  "id": "4",
  "contact": {
    "user": "user-uuid",
    "nickname": "My Lawyer"
  }
}
```

### Search Users
```json
{
  "id": "5",
  "search": {
    "query": "bob",
    "limit": 20
  }
}
```

## Invite Code Flow

When Bob redeems Alice's invite code:
1. DM is created between Alice and Bob
2. Alice and Bob are added as contacts (bidirectional)
3. If Cathy also invited Bob's email, her invite is auto-redeemed too

This ensures all support persons are connected when a survivor joins.
