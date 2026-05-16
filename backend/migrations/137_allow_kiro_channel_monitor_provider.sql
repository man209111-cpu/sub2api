-- Migration: 137_allow_kiro_channel_monitor_provider
-- 渠道监控新增 Kiro 平台。Kiro 网关使用 Anthropic Messages 兼容协议，
-- 但监控配置和模板需要独立 provider 归属，便于筛选和复用。

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE table_name = 'channel_monitors'
          AND constraint_name = 'channel_monitors_provider_check'
    ) THEN
        ALTER TABLE channel_monitors
            DROP CONSTRAINT channel_monitors_provider_check;
    END IF;

    ALTER TABLE channel_monitors
        ADD CONSTRAINT channel_monitors_provider_check
        CHECK (provider IN ('openai', 'anthropic', 'gemini', 'kiro'));
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE table_name = 'channel_monitor_request_templates'
          AND constraint_name = 'channel_monitor_request_templates_provider_check'
    ) THEN
        ALTER TABLE channel_monitor_request_templates
            DROP CONSTRAINT channel_monitor_request_templates_provider_check;
    END IF;

    ALTER TABLE channel_monitor_request_templates
        ADD CONSTRAINT channel_monitor_request_templates_provider_check
        CHECK (provider IN ('openai', 'anthropic', 'gemini', 'kiro'));
END $$;
