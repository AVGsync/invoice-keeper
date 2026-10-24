#!/usr/bin/env bash
# Разрезает старый единый .env на .env (общее), .env.backend и .env.nlq. Исходный файл сохраняется в .env.bak.
# Запуск из корня репозитория: ./scripts/split-env.sh
set -euo pipefail
cd "$(dirname "$0")/.."

[ -f .env ] || { echo "нет файла .env"; exit 1; }
if [ -f .env.backend ] && [ -z "${FORCE:-}" ]; then
  echo ".env.backend уже есть: .env уже разделён (FORCE=1 перезапишет)"; exit 0
fi

# Общее для compose и нескольких контейнеров
SHARED='^(POSTGRES_[A-Z]+|APP_TIMEZONE|TZ|DOMAIN|LE_EMAIL|IMAGE_PREFIX|IMAGE_TAG|NLQ_ACTOR_PUBLIC_KEY|NLQ_DB_PASSWORD|NLQ_USER_SCOPE|NLQ_TODAY|NLQ_DEV_SCHEMA|LOCAL_[A-Z_]+|PGADMIN_[A-Z_]+)$'
# Только агент с LLM
NLQ='^(LLM_[A-Z_]+|OPENROUTER_[A-Z_]+|NLQ_MODELS|NLQ_LLM|NLQ_RULES_FALLBACK|NLQ_PUBLIC_URL|NLQ_MAX_LLM_CALLS|NLQ_MAX_TOOL_CALLS_PER_STEP|NLQ_MAX_TOKENS|NLQ_STEP_TIMEOUT|NLQ_TOOL_TIMEOUT|NLQ_TOTAL_TIMEOUT)$'

cp -p .env .env.bak
umask 077
: > .env.shared.tmp; : > .env.backend.tmp; : > .env.nlq.new.tmp

while IFS= read -r line || [ -n "$line" ]; do
  case "$line" in ''|'#'*|' '*) continue ;; esac
  key="${line%%=*}"
  if   [[ "$key" =~ $SHARED ]]; then echo "$line" >> .env.shared.tmp
  elif [[ "$key" =~ $NLQ ]];    then echo "$line" >> .env.nlq.new.tmp
  else                               echo "$line" >> .env.backend.tmp
  fi
done < .env.bak

# .env.nlq: уже существующие строки не затираем, добавляем недостающие. Старое имя ключа переименовывается.
touch .env.nlq
sed -i 's/^OPENROUTER_API_KEY=/LLM_API_KEY=/; s/^OPENROUTER_BASE_URL=/LLM_BASE_URL=/' .env.nlq
sed -i 's/^OPENROUTER_API_KEY=/LLM_API_KEY=/; s/^OPENROUTER_BASE_URL=/LLM_BASE_URL=/; s/^NLQ_LLM=openrouter$/NLQ_LLM=llm/' .env.nlq.new.tmp
while IFS= read -r line; do
  grep -q "^${line%%=*}=" .env.nlq || echo "$line" >> .env.nlq
done < .env.nlq.new.tmp

mv .env.shared.tmp .env
mv .env.backend.tmp .env.backend
rm -f .env.nlq.new.tmp
chmod 600 .env .env.backend .env.nlq

echo "готово: .env ($(wc -l < .env) строк), .env.backend ($(wc -l < .env.backend)), .env.nlq ($(wc -l < .env.nlq)); копия старого файла: .env.bak"
