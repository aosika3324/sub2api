-- Migration: 905_add_veo_video_pricing
-- 新增 groups 表的 Veo 视频按秒计费字段
-- 背景：接入 Veo 视频生成（Gemini predictLongRunning 异步透传），按视频秒数计费

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS veo_video_price_per_second decimal(20,8);
