# Codebase Audit 2026-07-15

Base: `https://github.com/Hana-ame/chat-app/blob/main/`

## Critical

### 1. BroadcastUserUpdate data race

- **File**: [`server/internal/ws/hub.go:254`](https://github.com/Hana-ame/chat-app/blob/main/server/internal/ws/hub.go#L254)
- **Problem**: `BroadcastUserUpdate` reads `c.subs` under `h.mu.RLock()`, but `subs` is protected by `c.mu` (`sync.RWMutex` in [`client.go:19-20`](https://github.com/Hana-ame/chat-app/blob/main/server/internal/ws/client.go#L19-L20)). Concurrent map iteration and write can cause panic.
- **Fix**: Lock `c.mu.RLock()` before iterating `c.subs`.

## High

### 2. Reactions handler bypasses service layer

- **File**: [`server/internal/handlers/reactions.go:31,81,118`](https://github.com/Hana-ame/chat-app/blob/main/server/internal/handlers/reactions.go#L31)
- **Problem**: Three identical `IsChatMember` + DB direct call blocks. Duplicates [`service/member.go`](https://github.com/Hana-ame/chat-app/blob/main/server/internal/service/member.go)'s `MustBeMember`.
- **Fix**: Route through `s.Services.Chat.MustBeMember()`.

### 3. Auth/Users/SSE handlers call DB directly

- **Files**: [`auth.go`](https://github.com/Hana-ame/chat-app/blob/main/server/internal/handlers/auth.go), [`users.go`](https://github.com/Hana-ame/chat-app/blob/main/server/internal/handlers/users.go), [`sse.go`](https://github.com/Hana-ame/chat-app/blob/main/server/internal/handlers/sse.go), [`chat.go`](https://github.com/Hana-ame/chat-app/blob/main/server/internal/handlers/chat.go) (CreateOrGetDM)
- **Problem**: All call `s.DB.*` instead of `s.Services.*`, undermining the layered architecture.

### 4. Reconnect fires after explicit disconnect

- **File**: [`client/src/realtime/coordinator.js:52-60`](https://github.com/Hana-ame/chat-app/blob/main/client/src/realtime/coordinator.js#L52-L60)
- **Problem**: `disconnect()` sets `_closeGuard=true`, but a pending `setTimeout` from a previous `onClose` still fires and checks `this._state === STATE.IDLE` — which is true after disconnect — reconnecting unintentionally.
- **Fix**: Check `_closeGuard` inside the reconnect timeout callback, not just `onClose`.

### 5. CONNECTED state set before WS actually opens

- **File**: [`client/src/realtime/coordinator.js:64-84`](https://github.com/Hana-ame/chat-app/blob/main/client/src/realtime/coordinator.js#L64-L84)
- **Problem**: State transitions to CONNECTED immediately, before WebSocket `onopen` fires. `wsReady` is true while connection may still be pending.
- **Fix**: Transition to CONNECTED only after transport reports ready.

### 6. removeUser has no try/catch

- **File**: [`client/src/components/MemberPanel.jsx:38-41`](https://github.com/Hana-ame/chat-app/blob/main/client/src/components/MemberPanel.jsx#L38-L41)
- **Problem**: `api.removeMember` called without error handling. Network error causes unhandled promise rejection.
- **Fix**: Wrap in try/catch with user-facing error toast.

## Medium

### 7. Service layer fragile error string matching

- **File**: [`server/internal/service/authz.go:55-65`](https://github.com/Hana-ame/chat-app/blob/main/server/internal/service/authz.go#L55-L65)
- **Problem**: `err.Error() == "not found"` — breaks when error is wrapped with `fmt.Errorf("...: %w", err)`.
- **Fix**: Use `errors.Is(err, sentinelErr)`.

### 8. Gateway silently discards DB error

- **File**: [`server/internal/ws/gateway.go:80`](https://github.com/Hana-ame/chat-app/blob/main/server/internal/ws/gateway.go#L80)
- **Problem**: `chats, _ := g.db.ListUserChats(...)` — error dropped, ready payload sends `"chats": null`.
- **Fix**: Log the error, send empty array instead of null.

### 9. Dual loadMessages on chat navigation

- **File**: [`client/src/routes/ChatPage.jsx:45`](https://github.com/Hana-ame/chat-app/blob/main/client/src/routes/ChatPage.jsx#L45), [`client/src/components/ChatView.jsx:37`](https://github.com/Hana-ame/chat-app/blob/main/client/src/components/ChatView.jsx#L37)
- **Problem**: Both trigger `loadMessages` on mount, wasting one request.
- **Fix**: Only trigger from ChatView (the component that renders messages).

### 10. ErrContentTooLong mapped to 403

- **File**: [`server/internal/handlers/handler.go:81`](https://github.com/Hana-ame/chat-app/blob/main/server/internal/handlers/handler.go#L81)
- **Problem**: `content_too_long` returns `StatusForbidden(403)`, should be `StatusRequestEntityTooLarge(413)`.
- **Fix**: Change to 413.

### 11. Poll continues after disconnect

- **File**: [`client/src/realtime/transports/poll.js:6-21`](https://github.com/Hana-ame/chat-app/blob/main/client/src/realtime/transports/poll.js#L6-L21)
- **Problem**: No `cancelled` flag; API response callback schedules next poll even after disconnect.
- **Fix**: Add `cancelled` flag, check before scheduling.

### 12. message_create double sort

- **File**: [`client/src/store/chat.js:157,197`](https://github.com/Hana-ame/chat-app/blob/main/client/src/store/chat.js#L157)
- **Problem**: Chat list sorted inside `.map()` and again in the returned object.
- **Fix**: Remove the inner sort, keep only the final sort.

### 13. Avatar rendering duplicated 8+ times

- **Files**: Multiple components (ChatList, ChatListItem, MemberPanel, MessageItem, UserProfileModal, SettingsModal, ChatInfoModal, DmSearchPanel)
- **Fix**: Extract `<UserAvatar user size />` component.

### 14. Modals lack role/keyboard

- **Files**: ImagePreviewModal, ChatInfoModal, SettingsModal, UserProfileModal
- **Fix**: Add `role="dialog"`, `aria-modal="true"`, Escape key handler.

## Low

- [`ChatView.jsx:110`](https://github.com/Hana-ame/chat-app/blob/main/client/src/components/ChatView.jsx#L110) — `userMap` created as always-empty `{}`, useless
- [`ChatList.jsx:172`](https://github.com/Hana-ame/chat-app/blob/main/client/src/components/ChatList.jsx#L172) — `filteredChats` not memoized
- [`Composer.jsx:117`](https://github.com/Hana-ame/chat-app/blob/main/client/src/components/Composer.jsx#L117) — attachment key uses array index
- Multiple components — hardcoded colors instead of CSS variables
- [`realtime/transports/ws.js:8`](https://github.com/Hana-ame/chat-app/blob/main/client/src/realtime/transports/ws.js#L8) — token in WebSocket URL query param
- [`ChatList.jsx:295`](https://github.com/Hana-ame/chat-app/blob/main/client/src/components/ChatList.jsx#L295) — monolithic component, should split
