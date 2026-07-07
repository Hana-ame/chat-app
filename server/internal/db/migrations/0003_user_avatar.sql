CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_uniq ON users(username);
ALTER TABLE users ADD COLUMN avatar_url TEXT NOT NULL DEFAULT '';
