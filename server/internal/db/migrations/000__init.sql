-- ─────────────────────────────────────────────────────────────────────────────
-- Chat App — Full Schema
--
-- This is a single init migration (all-in-one). The app has never been
-- released, so there is no need for incremental migrations. Keeping a single
-- file makes the schema easy to review, diff, and bootstrap from scratch.
--
-- Conventions:
--   * All IDs are UUID v4 (TEXT).
--   * All timestamps are UTC ISO-8601 with timezone, stored as TEXT.
--   * All FK constraints use ON DELETE CASCADE (or SET NULL for owner_id).
--   * SQLite WAL mode + synchronous NORMAL for write performance.
--   * Foreign keys enforced at the engine level.
-- ─────────────────────────────────────────────────────────────────────────────

PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;
PRAGMA foreign_keys=ON;

-- ── Users ────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    email         TEXT UNIQUE NOT NULL,
    username      TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    avatar_color  TEXT NOT NULL DEFAULT '#5865F2',
    status        TEXT NOT NULL DEFAULT 'offline',
    last_seen     TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    avatar_url    TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_uniq ON users(username);

-- ── Refresh Tokens ───────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT UNIQUE NOT NULL,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX IF NOT EXISTS idx_refresh_user ON refresh_tokens(user_id);

-- ── Chats ────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS chats (
    id              TEXT PRIMARY KEY,
    type            TEXT NOT NULL CHECK (type IN ('dm','group')),
    name            TEXT,
    icon_color      TEXT NOT NULL DEFAULT '#5865F2',
    owner_id        TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    last_message_at TEXT,
    last_message_id TEXT,
    member_count    INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_chats_last_msg ON chats(last_message_at DESC);

ALTER TABLE chats ADD COLUMN visibility TEXT NOT NULL DEFAULT 'private';

-- pinned_message: text content of the pinned message, set by POST/PATCH /pin
-- pinned_updated_at: timestamp when pinned_message was last set or updated
ALTER TABLE chats ADD COLUMN pinned_message TEXT NOT NULL DEFAULT '';
ALTER TABLE chats ADD COLUMN pinned_updated_at TEXT;

-- ── Chat Members ─────────────────────────────────────────────────────────────

-- Chat memberships.
-- role: "owner" for creator, "admin" for promoted members, "" for regular.
-- last_active_at (renamed from last_visited_at): updated when the member sends a message or visits the chat.
CREATE TABLE IF NOT EXISTS chat_members (
    chat_id              TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    user_id              TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role                 TEXT NOT NULL DEFAULT '',
    joined_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    last_active_at       TEXT,
    last_read_message_id TEXT,
    pinned_last_read_at  TEXT,
    pinned               INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (chat_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_chat_members_user ON chat_members(user_id);

-- ── Messages ─────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS messages (
    id         TEXT PRIMARY KEY,
    chat_id    TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content    TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    edited_at  TEXT
);

CREATE INDEX IF NOT EXISTS idx_messages_chat_id ON messages(chat_id, id);
CREATE INDEX IF NOT EXISTS idx_messages_chat_created ON messages(chat_id, created_at DESC);

-- Count columns avoid N+1 subqueries for chat-list previews.
ALTER TABLE messages ADD COLUMN attachment_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE messages ADD COLUMN mention_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE messages ADD COLUMN reaction_count INTEGER NOT NULL DEFAULT 0;

-- Soft delete: deleted_at IS NULL means not deleted; set to timestamp to mark.
ALTER TABLE messages ADD COLUMN deleted_at TEXT;

-- Aggregated reactions JSON array, synced on every add/remove reaction.
-- Format: [{"emoji":"👍","count":2}]
-- Avoids subquery on the reactions table for the common read path.
ALTER TABLE messages ADD COLUMN reactions TEXT NOT NULL DEFAULT '[]';
ALTER TABLE messages ADD COLUMN attachments TEXT NOT NULL DEFAULT '[]';
ALTER TABLE messages ADD COLUMN mentions TEXT NOT NULL DEFAULT '[]';

-- ── Attachments ──────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS attachments (
    id         TEXT PRIMARY KEY,
    message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    filename   TEXT NOT NULL,
    mime_type  TEXT NOT NULL,
    size       INTEGER NOT NULL,
    url        TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_attachments_message ON attachments(message_id);

-- ── Reactions (raw rows) ─────────────────────────────────────────────────────

-- Each row represents one user reacting with one emoji on one message.
-- The messages.reactions column is the pre-aggregated JSON cache of this table.
CREATE TABLE IF NOT EXISTS reactions (
    message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    emoji      TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    PRIMARY KEY (message_id, user_id, emoji)
);

CREATE INDEX IF NOT EXISTS idx_reactions_message ON reactions(message_id);

-- ── Mentions ─────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS mentions (
    message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (message_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_mentions_user ON mentions(user_id);

-- ═══════════════════════════════════════════════════════════════════════════════
-- Design notes
-- ═══════════════════════════════════════════════════════════════════════════════
--
-- Why ALTER TABLE after CREATE?
--   The schema was originally built with incremental migrations during
--   development. Since the app has never shipped, we collapse everything into
--   a single init file. We keep ALTER TABLE for columns added later so that
--   this file stays structurally identical to the migration sequence, making
--   it easy to verify correctness.
--
-- Why a reactions JSON cache column?
--   Reading reactions requires GROUP BY + aggregation across the reactions
--   table. With the JSON cache column we avoid this subquery on every message
--   read. The column is kept in sync by syncReactionsColumn() called from
--   AddReaction / RemoveReaction.
--
-- Why INTEGER count columns?
--   Same N+1 prevention. The counts are set at write time (CreateMessage,
--   AddReaction, RemoveReaction) so chat-list previews need no subqueries.
--
-- Why TEXT timestamps instead of INTEGER epoch?
--   Human-readable in the DB, timezone-safe (always UTC), and Go's
--   time.RFC3339Nano handles both nanosecond and microsecond precision.
-- ═══════════════════════════════════════════════════════════════════════════════
