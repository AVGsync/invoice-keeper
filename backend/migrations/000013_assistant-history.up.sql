-- История диалогов с ассистентом: память для уточняющих вопросов (web, miniapp, бот).
CREATE TABLE IF NOT EXISTS assistant_turns (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel      TEXT NOT NULL CHECK (channel IN ('web','miniapp','bot')),
    conversation TEXT NOT NULL CHECK (conversation ~ '^(web|miniapp|tg:dm|tg:-?[0-9]{1,20})$'),
    answer_id    TEXT NOT NULL DEFAULT '' CHECK (char_length(answer_id) <= 128),
    question     TEXT NOT NULL CHECK (char_length(question) BETWEEN 1 AND 500),
    answer_text  TEXT NOT NULL DEFAULT '' CHECK (char_length(answer_text) <= 1200),
    status       TEXT NOT NULL CHECK (status IN ('ok','partial','error')),
    error_code   TEXT CHECK (char_length(error_code) <= 64),
    mode         TEXT NOT NULL DEFAULT '' CHECK (char_length(mode) <= 16),
    memory       JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_assistant_turns_conv
    ON assistant_turns (user_id, conversation, created_at DESC);
