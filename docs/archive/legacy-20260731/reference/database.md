# Database Schema (SQLite)

## Tables

### users

| Column | Type | Constraints |
|---|---|---|
| id | TEXT | PK (UUID v4) |
| email | TEXT | UNIQUE NOT NULL |
| username | TEXT | NOT NULL, UNIQUE INDEX |
| password_hash | TEXT | NOT NULL (bcrypt) |
| avatar_color | TEXT | DEFAULT '#5865F2' |
| avatar_url | TEXT | DEFAULT '' |
| status | TEXT | DEFAULT 'offline' |
| last_seen | TEXT | DEFAULT '' (ISO-8601 UTC) |
| created_at | TEXT | DEFAULT now |

Indexes: `idx_users_username(username)`, `idx_users_username_uniq(username)`.

### refresh_tokens

| Column | Type | Constraints |
|---|---|---|
| id | TEXT | PK |
| user_id | TEXT | FK → users(id) ON DELETE CASCADE |
| token_hash | TEXT | UNIQUE NOT NULL (SHA256 hex) |
| expires_at | TEXT | NOT NULL |
| created_at | TEXT | DEFAULT now |

Indexes: `idx_refresh_user(user_id)`.

### chats

| Column | Type | Constraints |
|---|---|---|
| id | TEXT | PK |
| type | TEXT | CHECK IN ('dm','group') |
| name | TEXT | nullable |
| icon_color | TEXT | DEFAULT '#5865F2' |
| owner_id | TEXT | FK → users(id) ON DELETE SET NULL |
| created_at | TEXT | DEFAULT now |
| last_message_at | TEXT | nullable |
| last_message_id | TEXT | nullable |
| member_count | INTEGER | DEFAULT 0 |
| visibility | TEXT | DEFAULT 'private' |
| pinned_message | TEXT | DEFAULT '' (JSON) |
| pinned_updated_at | TEXT | nullable |

Indexes: `idx_chats_last_msg(last_message_at DESC)`.

### chat_members

| Column | Type | Constraints |
|---|---|---|
| chat_id | TEXT | FK → chats(id) ON DELETE CASCADE |
| user_id | TEXT | FK → users(id) ON DELETE CASCADE |
| role | TEXT | DEFAULT '' ('owner', 'admin', '') |
| joined_at | TEXT | DEFAULT now |
| last_active_at | TEXT | nullable (updated on visit/send) |
| last_read_message_id | TEXT | nullable (deprecated) |
| pinned_last_read_at | TEXT | nullable |
| pinned | INTEGER | DEFAULT 0 (boolean) |

PK: `(chat_id, user_id)`. Index: `idx_chat_members_user(user_id)`.

### messages

| Column | Type | Constraints |
|---|---|---|
| id | TEXT | PK |
| chat_id | TEXT | FK → chats(id) ON DELETE CASCADE |
| user_id | TEXT | FK → users(id) ON DELETE CASCADE |
| content | TEXT | DEFAULT '' |
| created_at | TEXT | DEFAULT now |
| edited_at | TEXT | nullable |
| attachment_count | INTEGER | DEFAULT 0 |
| mention_count | INTEGER | DEFAULT 0 |
| reaction_count | INTEGER | DEFAULT 0 |
| deleted_at | TEXT | nullable (soft delete) |
| reactions | TEXT | DEFAULT '[]' (JSON cache) |
| attachments | TEXT | DEFAULT '[]' (JSON) |
| mentions | TEXT | DEFAULT '[]' (JSON) |

Indexes: `idx_messages_chat_id(chat_id, id)`, `idx_messages_chat_created(chat_id, created_at DESC)`.

### attachments

| Column | Type | Constraints |
|---|---|---|
| id | TEXT | PK |
| message_id | TEXT | FK → messages(id) ON DELETE CASCADE |
| filename | TEXT | NOT NULL |
| mime_type | TEXT | NOT NULL |
| size | INTEGER | NOT NULL |
| url | TEXT | NOT NULL |

Index: `idx_attachments_message(message_id)`.

### reactions

| Column | Type | Constraints |
|---|---|---|
| message_id | TEXT | FK → messages(id) ON DELETE CASCADE |
| user_id | TEXT | FK → users(id) ON DELETE CASCADE |
| emoji | TEXT | NOT NULL |
| created_at | TEXT | DEFAULT now |

PK: `(message_id, user_id, emoji)`. Index: `idx_reactions_message(message_id)`.

### mentions

| Column | Type | Constraints |
|---|---|---|
| message_id | TEXT | FK → messages(id) ON DELETE CASCADE |
| user_id | TEXT | FK → users(id) ON DELETE CASCADE |

PK: `(message_id, user_id)`. Index: `idx_mentions_user(user_id)`.

## Design Notes

- **All timestamps**: ISO-8601 UTC TEXT (not epoch), human-readable in DB
- **All IDs**: UUID v4 TEXT
- **Foreign keys**: ON DELETE CASCADE (SET NULL for `chats.owner_id`)
- **Pragmas**: WAL mode + synchronous NORMAL
- **Reactions JSON cache**: `messages.reactions` is a pre-aggregated cache, synced on add/remove to avoid GROUP BY on every read
- **Count columns**: `attachment_count`, `mention_count`, `reaction_count` are write-time counters, preventing N+1 on list queries
