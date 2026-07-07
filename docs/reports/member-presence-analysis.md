# Technical Analysis: Member Management & Presence System

## 1. Member Management ("Add Member") Logic
The member addition process in `MemberPanel.jsx` is governed by the following rules:

### Visibility & Access
- **DM Restriction**: The `+ Add member` button is only rendered if the chat type is NOT a direct message (`chat?.type !== 'dm'`).
- **Ownership**: The "Remove" (X) button is only shown if the current user is the owner of the chat (`chat?.owner_id === user.id`) and is not removing themselves.

### Adding Process
1. **Discovery**: Users are found via `api.searchUsers`.
2. **Filtering**: The search results are filtered to exclude users who are already members of the current chat:
   ```javascript
   (data.users || []).filter(u => !members.find(m => m.id === u.id))
   ```
3. **Execution**: The `addUser` function calls `api.addMember(accessToken, chatId, userId)`, which notifies the backend to update the chat membership.

---

## 2. Online Status (Presence) Update Mechanism
The online status is handled as a real-time reactive system integrated into the global store.

### State Management
- The `useChatStore` maintains a list of `onlineUserIds` (an array of strings).

### Real-time Synchronization
Presence updates are pushed from the server via WebSocket or SSE:
- **Event**: `presence_update`
- **Logic**:
  - If `payload.status === 'online'`: The user ID is added to the `onlineUserIds` set.
  - If `payload.status === 'offline'`: The user ID is removed from the set.

### UI Implementation
- **Presence Indicator**: The `isOnline(uid)` helper checks if a user's ID exists in the `onlineUserIds` array to determine whether to show a green (`online`) or grey (`offline`) dot.
- **Priority Sorting**: To improve UX, the member list is dynamically sorted so that **online users always appear at the top** of the list:
  ```javascript
  [...members].sort((a, b) => {
    const oa = onlineUserIds.includes(a.id) ? 0 : 1;
    const ob = onlineUserIds.includes(b.id) ? 0 : 1;
    return oa - ob;
  })
  ```
