# Components

This directory contains the UI components for the chat application, built with React 19.

## 🧩 Component Map

### Main Views
- `ChatList.jsx`: The sidebar containing the search bar, chat list, and user profile settings.
- `ChatView.jsx`: The primary message display area with smart auto-scrolling and pinned messages.
- `WelcomeView.jsx`: The landing view shown when no conversation is active.

### Chat Elements
- `ChatListItem.jsx`: Individual chat entry in the sidebar with unread badges and context menus.
- `MessageItem.jsx`: A single message bubble supporting Markdown, reactions, attachments, and editing/deleting.
- `Composer.jsx`: The message input area supporting text, file attachments, and AI mock triggers.

### Panels & Modals
- `MemberPanel.jsx`: Right-side panel for managing group members and tracking online status.
- `SettingsModal.jsx`: User profile settings for changing display name and avatar.
- `CreateGroupForm.jsx`: Form for creating new group chats with visibility settings.
- `DmSearchPanel.jsx`: User search interface for starting new direct messages.

### Specialized UI
- `PublicChannelList.jsx`: List of discoverable public channels.
- `ScrollArea.jsx`: A reusable scrollable container ensuring correct flex-box behavior.
- `EmptyState.jsx`: Standardized "No data" display.

## 🛠 Implementation Details
- **Styling**: All components use CSS classes defined in `styles/global.css`.
- **State**: Components primarily consume state from `useChatStore` and `useAuthStore` (Zustand).
- **Markdown**: `MessageItem` uses `react-markdown` and `remark-gfm` for rich text rendering.
