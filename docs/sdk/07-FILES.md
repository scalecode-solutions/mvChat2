# Files

> **IMPORTANT**: This documentation is accurate as of January 8th, 2026 at 4:00 AM CST. Review the actual code before assuming anything here is still current.

## Overview

mvChat2 supports file uploads with:
- Automatic thumbnail generation for images/videos
- Encryption at rest (files are encrypted server-side)
- E2EE support (files encrypted client-side before upload)

## Uploading Files

### Simple Upload

```typescript
const file = await client.uploadFile({
  uri: 'file:///path/to/photo.jpg',
  name: 'photo.jpg',
  type: 'image/jpeg',
});

// Returns:
// {
//   ref: 'file-uuid',
//   name: 'photo.jpg',
//   mime: 'image/jpeg',
//   size: 102400,
//   width: 1920,
//   height: 1080,
//   thumb: 'thumbnail-uuid',  // For images/videos
// }
```

### Upload with Progress

```typescript
const file = await client.uploadFile(
  { uri, name, type },
  {
    onProgress: (progress) => {
      console.log(`Upload: ${progress}%`);
    },
  }
);
```

### Upload with E2EE

```typescript
// File is encrypted client-side before upload
const file = await client.uploadFileEncrypted(
  { uri, name, type },
  conversationId, // Uses conversation's encryption key
);
```

## Downloading Files

### Simple Download

```typescript
const localUri = await client.downloadFile(fileRef);
// Returns local file path
```

### Download with Progress

```typescript
const localUri = await client.downloadFile(fileRef, {
  onProgress: (progress) => {
    console.log(`Download: ${progress}%`);
  },
});
```

### Download Encrypted File

```typescript
// File is decrypted client-side after download
const localUri = await client.downloadFileEncrypted(fileRef, conversationId);
```

## Thumbnails

```typescript
// Get thumbnail URL
const thumbUrl = client.getThumbnailUrl(thumbRef);

// Download thumbnail
const thumbUri = await client.downloadThumbnail(thumbRef);
```

## Sending Files in Messages

```typescript
// Upload first
const file = await client.uploadFile({ uri, name, type });

// Then send message with file
await client.sendMessage(conversationId, {
  text: 'Check out this document',
  media: [
    {
      type: 'file',
      ref: file.ref,
      name: file.name,
      mime: file.mime,
      size: file.size,
    },
  ],
});
```

## React Hook

```typescript
import { useFileUpload } from '@mvchat/react-native-sdk';

function AttachmentButton({ conversationId }) {
  const { upload, isUploading, progress } = useFileUpload(client);

  const handlePick = async () => {
    const result = await ImagePicker.launchImageLibraryAsync();
    if (!result.canceled) {
      const file = await upload({
        uri: result.assets[0].uri,
        name: 'photo.jpg',
        type: 'image/jpeg',
      });
      // Use file.ref in message
    }
  };

  return (
    <TouchableOpacity onPress={handlePick} disabled={isUploading}>
      {isUploading ? (
        <ProgressBar progress={progress} />
      ) : (
        <AttachIcon />
      )}
    </TouchableOpacity>
  );
}
```

## HTTP Endpoints

Files use HTTP endpoints, not WebSocket:

```
POST /v0/file/upload
  - Multipart form data
  - Returns file metadata

GET /v0/file/{ref}
  - Downloads file
  - Requires auth header

GET /v0/file/{ref}/thumb
  - Downloads thumbnail
```

## Supported Types

| Type | Extensions | Thumbnail |
|------|------------|-----------|
| Image | jpg, png, gif, webp, heic | ✅ Yes |
| Video | mp4, mov, webm | ✅ Yes (first frame) |
| Audio | mp3, m4a, wav, ogg | ❌ No |
| Document | pdf, doc, docx, txt | ❌ No |

## Size Limits

- Max file size: 100 MB (configurable)
- Max thumbnail size: 256x256 pixels
- Thumbnail quality: 80% JPEG
