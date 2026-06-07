-- Image Studio Phase 1: add image_conversations, image_generations tables
-- and api_keys.internal column for synthetic studio-managed keys.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

CREATE TABLE IF NOT EXISTS image_conversations (
    id          BIGSERIAL PRIMARY KEY,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,
    user_id     BIGINT NOT NULL,
    title       VARCHAR(255) NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS imageconversation_user_id    ON image_conversations (user_id);
CREATE INDEX IF NOT EXISTS imageconversation_deleted_at ON image_conversations (deleted_at);

CREATE TABLE IF NOT EXISTS image_generations (
    id              BIGSERIAL PRIMARY KEY,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,
    user_id         BIGINT NOT NULL,
    conversation_id BIGINT NOT NULL,
    group_id        BIGINT NOT NULL,
    prompt          TEXT NOT NULL DEFAULT '',
    model           VARCHAR(100) NOT NULL DEFAULT '',
    size            VARCHAR(30) NOT NULL DEFAULT '',
    quality         VARCHAR(30) NOT NULL DEFAULT '',
    n               INT NOT NULL DEFAULT 1,
    image_count     INT NOT NULL DEFAULT 0,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    cost            DECIMAL(20, 10) NOT NULL DEFAULT 0,
    storage_keys    JSONB,
    width           INT,
    height          INT,
    error           TEXT
);

CREATE INDEX IF NOT EXISTS imagegeneration_user_id         ON image_generations (user_id);
CREATE INDEX IF NOT EXISTS imagegeneration_conversation_id ON image_generations (conversation_id);
CREATE INDEX IF NOT EXISTS imagegeneration_deleted_at      ON image_generations (deleted_at);

ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS internal BOOLEAN NOT NULL DEFAULT false;
