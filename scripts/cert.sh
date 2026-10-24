#!/usr/bin/env bash
# TLS-сертификат Let's Encrypt для nginx. DOMAIN в .env — домен или публичный IP.
#   ./scripts/cert.sh issue        первый выпуск (порт 80 свободен, нужен LE_EMAIL)
#   ./scripts/cert.sh renew        продление без простоя
#   ./scripts/cert.sh self-signed  самоподписанный
# Для IP сертификат живёт 6 дней: renew запускать по cron ежедневно.
set -euo pipefail
cd "$(dirname "$0")/.."

if [ -f .env ]; then set -a; . ./.env; set +a; fi
: "${DOMAIN:?set DOMAIN in .env}"

CERTS=./certs
LE=./letsencrypt
WEBROOT=./certbot-www
CERTBOT_IMAGE=certbot/certbot:v5.8.0

# Для IP вместо -d нужен --ip-address и профиль shortlived.
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
    # standalone занимает :80, поэтому nginx останавливаем.
    docker compose stop nginx 2>/dev/null || true
    docker run --rm -p 80:80 \
      -v "$PWD/letsencrypt:/etc/letsencrypt" \
      "$CERTBOT_IMAGE" certonly --standalone --non-interactive --agree-tos \
      -m "$LE_EMAIL" "${TARGET[@]}"
    install_certs
    docker compose up -d
    ;;
  renew)
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
