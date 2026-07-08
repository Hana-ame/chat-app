-- Replace deleted INTEGER with deleted_at TEXT (null = not deleted).
ALTER TABLE messages ADD COLUMN deleted_at TEXT;