-- Image Studio hardening: add image_generations.error_code, a stable
-- machine-readable failure classifier the frontend maps to a localized message.
-- The raw, human-facing detail stays in `error` for admin diagnostics.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE image_generations
    ADD COLUMN IF NOT EXISTS error_code VARCHAR(40);
