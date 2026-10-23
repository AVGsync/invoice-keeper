-- Персона — должность или зона ответственности пользователя (Бухгалтер, DevOps).
-- На права не влияет: права задаёт только users.role. Список ведёт администратор.
CREATE TABLE IF NOT EXISTS personas (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT personas_name_check CHECK (char_length(name) BETWEEN 1 AND 64),
    CONSTRAINT personas_description_check CHECK (char_length(description) <= 255)
);

-- Название уникально без учёта регистра: «Бухгалтер» и «бухгалтер» — одна персона.
CREATE UNIQUE INDEX idx_personas_name_lower ON personas (lower(name));

-- Удаление персоны не удаляет пользователей: у них просто пропадает персона.
ALTER TABLE users ADD COLUMN persona_id UUID REFERENCES personas(id) ON DELETE SET NULL;
