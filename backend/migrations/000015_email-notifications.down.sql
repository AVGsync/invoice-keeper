DROP TABLE IF EXISTS email_notifications;
ALTER TABLE user_notification_settings ALTER COLUMN days_before_due SET DEFAULT 1;
ALTER TABLE user_notification_settings
    DROP COLUMN IF EXISTS verify_sent_at,
    DROP COLUMN IF EXISTS verify_attempts,
    DROP COLUMN IF EXISTS verify_code_expires_at,
    DROP COLUMN IF EXISTS verify_code_hash;
