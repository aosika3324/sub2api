-- Migration: 150_restore_group_sora_billing_columns
-- Restore group-level Sora billing columns that are still part of the current
-- Ent Group schema after migration 090 removed the legacy Sora runtime tables.

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS sora_image_price_360 decimal(20,8),
    ADD COLUMN IF NOT EXISTS sora_image_price_540 decimal(20,8),
    ADD COLUMN IF NOT EXISTS sora_video_price_per_request decimal(20,8),
    ADD COLUMN IF NOT EXISTS sora_video_price_per_request_hd decimal(20,8),
    ADD COLUMN IF NOT EXISTS sora_storage_quota_bytes BIGINT NOT NULL DEFAULT 0;
