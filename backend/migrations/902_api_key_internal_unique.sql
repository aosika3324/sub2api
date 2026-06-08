-- Image Studio: enforce at most one live internal (studio-managed) API key per
-- (user_id, group_id, name). EnsureStudioAPIKey relies on a unique violation to
-- make its concurrent-create recovery branch fire; without this index a race can
-- silently create duplicate hidden keys and split a (user, group)'s
-- billing/usage/rate-limit accounting across multiple synthetic keys.
--
-- Runs inside a transaction (regular, non-_notx migration): the duplicate
-- cleanup and the unique index creation are applied atomically.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

-- 1) Soft-delete any pre-existing duplicate live internal keys, keeping the
--    lowest id per (user_id, group_id, name). Matches the partial index scope
--    (internal = true AND deleted_at IS NULL); group_id is compared by equality
--    so NULL group_ids are treated as distinct, exactly like the unique index.
UPDATE api_keys a
SET deleted_at = NOW()
WHERE a.internal = TRUE
  AND a.deleted_at IS NULL
  AND EXISTS (
      SELECT 1
      FROM api_keys b
      WHERE b.internal = TRUE
        AND b.deleted_at IS NULL
        AND b.user_id = a.user_id
        AND b.group_id = a.group_id
        AND b.name = a.name
        AND b.id < a.id
  );

-- 2) Partial unique index covering only live internal keys, so normal user keys
--    (which may legitimately repeat a name across a user's groups) are unaffected.
CREATE UNIQUE INDEX IF NOT EXISTS api_keys_internal_user_group_name_unique
    ON api_keys (user_id, group_id, name)
    WHERE internal = TRUE AND deleted_at IS NULL;
