-- Rename last_visited_at to last_active_at in chat_members.
--
-- On existing DBs: the column is still last_visited_at because the
-- original init migration renamed it in Go code (not in SQL) and
-- earlier attempts at renaming via migration crashed.
--
-- On fresh DBs: 000__init.sql already creates the table with
-- last_active_at, so this ALTER TABLE will fail with "no such
-- column: last_visited_at". The migration runner tolerates that
-- specific error for idempotency.

ALTER TABLE chat_members RENAME COLUMN last_visited_at TO last_active_at;
