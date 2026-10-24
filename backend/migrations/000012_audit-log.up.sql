-- Журнал действий пользователей (веб, Mini App, бот, система). Без FK на users:
-- запись живёт дольше пользователя и не мешает его удалению.
CREATE TABLE IF NOT EXISTS audit_log (
    id          BIGSERIAL PRIMARY KEY,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actor_id    UUID,
    actor_role  TEXT CHECK (actor_role IN ('admin','user')),
    channel     TEXT NOT NULL CHECK (channel IN ('web','miniapp','bot','system')),
    action      TEXT NOT NULL CHECK (action ~ '^[a-z_]+(\.[a-z_]+)+$' AND char_length(action) <= 64),
    outcome     TEXT NOT NULL DEFAULT 'success' CHECK (outcome IN ('success','failure')),
    entity_type TEXT CHECK (char_length(entity_type) <= 64),
    entity_id   TEXT CHECK (char_length(entity_id) <= 128),
    changes     JSONB,
    payload     JSONB,
    ip          INET,
    user_agent  TEXT CHECK (char_length(user_agent) <= 512),
    request_id  TEXT CHECK (char_length(request_id) <= 128)
);
CREATE INDEX IF NOT EXISTS idx_audit_log_created ON audit_log (created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_actor   ON audit_log (actor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_entity  ON audit_log (entity_type, entity_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_action  ON audit_log (action, created_at DESC);
