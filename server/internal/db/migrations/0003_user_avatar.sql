DROP INDEX IF EXISTS idx_users_username;
CREATE UNIQUE INDEX idx_users_username_uniq ON users(username);
ALTER TABLE users ADD COLUMN avatar_url TEXT NOT NULL DEFAULT '';
