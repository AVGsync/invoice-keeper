#!/usr/bin/env bash
# Выпуск и продление TLS-сертификата Let's Encrypt для nginx из docker-compose.yaml.
#
#   ./scripts/cert.sh issue        первый выпуск (порт 80 должен быть свободен)
#   ./scripts/cert.sh renew        продление без простоя (nginx должен работать)
#   ./scripts/cert.sh self-signed  самоподписанный, для проверки без интернета
#
# DOMAIN в .env — это домен или публичный IP сервера (IPv4/IPv6). Для IP
# сертификат выпускается с профилем shortlived: он живёт 6 дней, поэтому renew
# нужно запускать по cron каждый день (для домена тоже подойдёт).
# Для issue нужен ещё LE_EMAIL. Результат: ./certs/fullchain.pem и
# ./certs/key.pem — их монтирует nginx.
#
# Автопродление, crontab сервера:
#   0 4 * * *  cd /opt/invoice-keeper && ./scripts/cert.sh renew >> cert.log 2>&1
set -euo pipefail
cd "$(dirname "$0")/.."

if [ -f .env ]; then set -a; . ./.env; set +a; fi
: "${DOMAIN:?set DOMAIN in .env}"

CERTS=./certs
LE=./letsencrypt
WEBROOT=./certbot-www
# IP-сертификаты: --ip-address с 5.3, webroot для них — с 5.4.
CERTBOT_IMAGE=certbot/certbot:v5.8.0

# Для IP вместо -d certbot принимает --ip-address и требует профиль shortlived.
if [[ "$DOMAIN" =~ ^[0-9.]+$ || "$DOMAIN" == *:* ]]; then
  TARGET=(--ip-address "$DOMAIN" --preferred-profile shortlived)
  SAN="IP:$DOMAIN"
else
  TARGET=(-d "$DOMAIN")
  SAN="DNS:$DOMAIN"
fi

mkdir -p "$CERTS" "$LE" "$WEBROOT"

install_certs() {
  cp -L "$LE/live/$DOMAIN/fullchain.pem" "$CERTS/fullchain.pem"
  cp -L "$LE/live/$DOMAIN/privkey.pem" "$CERTS/key.pem"
  chmod 644 "$CERTS/fullchain.pem"
  chmod 640 "$CERTS/key.pem"
}

case "${1:-}" in
  issue)
    : "${LE_EMAIL:?set LE_EMAIL in .env}"
    # standalone сам поднимает временный сервер на :80, поэтому nginx стопаем.
    docker compose stop nginx 2>/dev/null || true
    docker run --rm -p 80:80 \
      -v "$PWD/letsencrypt:/etc/letsencrypt" \
      "$CERTBOT_IMAGE" certonly --standalone --non-interactive --agree-tos \
      -m "$LE_EMAIL" "${TARGET[@]}"
    install_certs
    docker compose up -d
    ;;
  renew)
    # webroot: nginx отдаёт /.well-known/ из ./certbot-www, останавливать не нужно.
    docker run --rm \
      -v "$PWD/letsencrypt:/etc/letsencrypt" \
      -v "$PWD/certbot-www:/var/www/certbot" \
      "$CERTBOT_IMAGE" renew --webroot -w /var/www/certbot --non-interactive
    install_certs
    docker compose exec nginx nginx -s reload
    ;;
  self-signed)
    openssl req -x509 -nodes -newkey rsa:2048 -days 30 \
      -keyout "$CERTS/key.pem" -out "$CERTS/fullchain.pem" \
      -subj "/CN=$DOMAIN" -addext "subjectAltName=$SAN"
    chmod 644 "$CERTS/fullchain.pem"; chmod 640 "$CERTS/key.pem"
    ;;
  *)
    echo "usage: $0 {issue|renew|self-signed}" >&2
    exit 1
    ;;
esac
