-- Image Studio hardening: composite index supporting the stale-pending sweep
-- (FailStaleGenerations): WHERE status = 'pending' AND created_at < cutoff.
-- CONCURRENTLY so it never blocks live image-generation writes; _notx because
-- CONCURRENTLY cannot run inside a transaction.
CREATE INDEX CONCURRENTLY IF NOT EXISTS imagegeneration_status_created_at
    ON image_generations (status, created_at);
