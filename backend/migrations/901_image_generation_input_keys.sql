-- Image Studio image-to-image: add image_generations.input_storage_keys column
-- to persist the user-provided reference image(s) for an edits generation.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE image_generations
    ADD COLUMN IF NOT EXISTS input_storage_keys JSONB;
