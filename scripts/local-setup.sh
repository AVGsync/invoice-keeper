#!/usr/bin/env bash
# Готовит .env, .env.backend и .env.nlq для локального запуска (docker-compose.local.yaml).
# Безопасно запускать повторно: уже заданные значения не трогает.
#   ./scripts/local-setup.sh
set -euo pipefail
cd "$(dirname "$0")/.."

# Старый единый .env (с секретами backend внутри) разрезаем на три файла
if [ -f .env ] && [ ! -f .env.backend ] && grep -q '^JWT_ACCESS_SECRET=' .env; then
  ./scripts/split-env.sh
fi

for f in .env .env.backend .env.nlq; do
  [ -f "$f" ] || { cp "$f.example" "$f"; echo "создан $f"; }
done

# Пустой .env.nlq (например, после split-env.sh) заполняем примером; ключ из старого файла с опечаткой .env.enlq переносим
if [ ! -s .env.nlq ]; then
  cp .env.nlq.example .env.nlq
  if [ -f .env.enlq ]; then
    key="$(sed -n 's/^\(OPENROUTER_API_KEY\|LLM_API_KEY\)=//p' .env.enlq | head -1)"
    [ -z "$key" ] || { sed -i "s|^LLM_API_KEY=.*|LLM_API_KEY=$key|" .env.nlq; echo "ключ LLM перенесён из .env.enlq в .env.nlq"; }
  fi
fi

# get FILE KEY / set FILE KEY VALUE: значения без кавычек, как в примерах
get() { sed -n "s/^$2=//p" "$1" | head -1; }
set_kv() {
  if grep -q "^$2=" "$1"; then
    sed -i "s|^$2=.*|$2=$3|" "$1"
  else
    printf '%s=%s\n' "$2" "$3" >> "$1"
  fi
}
rand() { openssl rand -hex 24; }

# Старый файл мог остаться без новых настроек: добавляем ключи из примера, которых в нём нет
for f in .env .env.backend .env.nlq; do
  while IFS= read -r line; do
    case "$line" in ''|'#'*|' '*) continue ;; esac
    grep -q "^${line%%=*}=" "$f" || { echo "$line" >> "$f"; echo "$f: добавлено ${line%%=*}"; }
  done < "$f.example"
done

# JWT-секреты: примерные значения заменяем случайными
for k in JWT_ACCESS_SECRET JWT_REFRESH_SECRET; do
  v="$(get .env.backend "$k")"
  if [ -z "$v" ] || [[ "$v" == *change-me* ]]; then set_kv .env.backend "$k" "$(rand)"; echo "сгенерирован $k"; fi
done

# Пароль админа: без него войти нельзя
if [ -z "$(get .env.backend ADMIN_PASSWORD)" ]; then set_kv .env.backend ADMIN_PASSWORD admin123; echo "ADMIN_PASSWORD=admin123 (только для локальной работы)"; fi

# Сайт открывается через локальный nginx
origins="$(get .env.backend ALLOWED_ORIGINS)"
if [[ "$origins" != *"localhost:8000"* ]]; then
  set_kv .env.backend ALLOWED_ORIGINS "${origins:+$origins,}http://localhost:8000,http://127.0.0.1:8000"
fi

# Ассистент: пара ключей Ed25519 (приватный в .env.backend, публичный в .env)
if [ -z "$(get .env.backend NLQ_ACTOR_PRIVATE_KEY)" ] || [ -z "$(get .env NLQ_ACTOR_PUBLIC_KEY)" ]; then
  tmp="$(mktemp)"; trap 'rm -f "$tmp"' EXIT
  openssl genpkey -algorithm ed25519 -outform DER -out "$tmp"
  priv="$(tail -c 32 "$tmp" | base64 -w0)"
  pub="$(openssl pkey -inform DER -in "$tmp" -pubout -outform DER | tail -c 32 | base64 -w0)"
  set_kv .env.backend NLQ_ACTOR_PRIVATE_KEY "$priv"
  set_kv .env NLQ_ACTOR_PUBLIC_KEY "$pub"
  echo "сгенерированы ключи ассистента"
fi
# Отладочная схема из примера в локальном запуске не нужна
[ -n "$(get .env NLQ_DEV_SCHEMA)" ] && set_kv .env NLQ_DEV_SCHEMA ""

chmod 600 .env .env.backend .env.nlq
echo "готово. Запуск: docker compose -f docker-compose.local.yaml up -d --build  (или make up)"
