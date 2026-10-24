CREATE TABLE user_notification_settings (
    user_id UUID PRIMARY KEY,

    email VARCHAR(320) NOT NULL,
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    email_enabled BOOLEAN NOT NULL DEFAULT TRUE,

    days_before_due INTEGER NOT NULL DEFAULT 1
        CHECK (days_before_due BETWEEN 0 AND 365),

    send_time TIME NOT NULL DEFAULT '09:00:00',
    timezone VARCHAR(64) NOT NULL DEFAULT 'UTC',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_user_notification_settings_user
        FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE
);

CREATE UNIQUE INDEX idx_user_notification_settings_email
    ON user_notification_settings (LOWER(email));