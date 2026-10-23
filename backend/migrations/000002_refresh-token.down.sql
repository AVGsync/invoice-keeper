DROP INDEX IF EXISTS idx_refresh_tokens_expires_at;
DROP INDEX IF EXISTS idx_refresh_tokens_revoked_at;

DROP TABLE IF EXISTS refresh_tokens;
