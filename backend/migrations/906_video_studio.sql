-- Migration: 906_video_studio
-- Video Studio (in-app JWT Veo video generation): add video_generations table.
--
-- Background: the API-gateway Veo path (/v1beta predictLongRunning passthrough)
-- is stateless. The in-app video studio gives logged-in users a task-based
-- workbench, so each submitted Veo job is persisted as a row and tracked through
-- read-time polling (the status endpoint re-polls the bound upstream account,
-- bills once on completion, and proxy-streams the produced video on demand).
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

CREATE TABLE IF NOT EXISTS video_generations (
    id               BIGSERIAL PRIMARY KEY,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMPTZ,
    user_id          BIGINT NOT NULL,
    group_id         BIGINT NOT NULL,
    prompt           TEXT NOT NULL DEFAULT '',
    model            VARCHAR(100) NOT NULL DEFAULT '',
    operation_name   VARCHAR(512) NOT NULL DEFAULT '',
    account_id       BIGINT NOT NULL DEFAULT 0,
    status           VARCHAR(20) NOT NULL DEFAULT 'pending',
    sample_count     INT NOT NULL DEFAULT 0,
    duration_seconds DOUBLE PRECISION NOT NULL DEFAULT 0,
    cost             DECIMAL(20, 10) NOT NULL DEFAULT 0,
    billed           BOOLEAN NOT NULL DEFAULT false,
    error            TEXT,
    error_code       VARCHAR(40)
);

CREATE INDEX IF NOT EXISTS videogeneration_user_id            ON video_generations (user_id);
CREATE INDEX IF NOT EXISTS videogeneration_deleted_at         ON video_generations (deleted_at);
CREATE INDEX IF NOT EXISTS videogeneration_status_created_at  ON video_generations (status, created_at);
