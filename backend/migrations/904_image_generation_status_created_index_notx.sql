-- Image Studio hardening: composite index for the stale-pending sweep
-- FailStaleGenerations filters on status = pending AND created_at < cutoff.
-- CONCURRENTLY avoids blocking live image-generation writes, which is why this
-- lives in a _notx file (CONCURRENTLY cannot run inside a transaction).
CREATE INDEX CONCURRENTLY IF NOT EXISTS imagegeneration_status_created_at
    ON image_generations (status, created_at);
