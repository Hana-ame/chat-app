# API Endpoints & Architecture

This document describes how the frontend communicates with the backend and handles API endpoints.

## 🌐 Base URL & Environment Handling

The API base URL is dynamically determined in `client/src/api/client.js` to support both local development and production deployments.

```javascript
const IS_PAGES = typeof window !== 'undefined' && window.location.hostname.endsWith('pages.dev');
const API_BASE = IS_PAGES ? 'https://wsl-8080.moonchan.xyz' : '';
```

- **Production (Cloudflare Pages)**: Uses `https://wsl-8080.moonchan.xyz` as the prefix.
- **Local Development**: Uses an empty string `''`, triggering relative requests (e.g., `/api/auth/login`).

## 🛠 Development Proxy (Vite)

In local development, Vite acts as a reverse proxy to avoid CORS issues and simplify request paths.

**Configuration (`client/vite.config.js`):**
- `/api` $\rightarrow$ `http://localhost:8080`
- `/uploads` $\rightarrow$ `http://localhost:8080`

This allows the frontend to call `/api/...` while the request is actually routed to the Go backend running on port 8080.

## 📤 External Upload Service

File uploads are decoupled from the main API server to optimize binary data handling.

- **Upload Base**: `https://upload.moonchan.xyz`
- **Method**: `PUT`
- **Endpoint**: `/api/upload`
- **Payload**: Raw binary stream.
- **Return URL Format**: `https://upload.moonchan.xyz/api/{id}/{filename}`

## 📡 Real-time Events (SSE)

The app uses Server-Sent Events (SSE) for real-time updates (presence, new messages).

- **Endpoint**: `API_BASE + '/api/events?access_token={token}'`
- **Implementation**: Handled via `api.startStreaming` using `fetch` and a `ReadableStream` reader.

## 🔄 Request Workflow

All standard API calls pass through a centralized `request` helper in `client/src/api/client.js`, which manages:

1. **Authentication**: Automatically attaches `Authorization: Bearer {token}` to headers.
2. **Content-Type**: Sets `application/json` for POST/PATCH requests.
3. **Error Handling**: 
   - Intercepts `401 Unauthorized` to trigger the `auth:unauthorized` custom event.
   - Throws a formatted error object containing the status and server response.
4. **Parsing**: Automatically parses JSON responses.
