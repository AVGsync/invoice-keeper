FROM node:22.23.2-alpine3.24 AS builder
WORKDIR /app
COPY package.json package-lock.json .npmrc ./
# npm 12.2.0 — версия, которой сгенерирован package-lock.json.
RUN npm install -g npm@12.2.0 && npm ci
COPY . .
# Часовой пояс вшивается в бандл при сборке и должен совпадать с APP_TIMEZONE бэкенда.
ARG VITE_APP_TIMEZONE=Europe/Moscow
ENV VITE_APP_TIMEZONE=${VITE_APP_TIMEZONE}
RUN npm run build

FROM nginx:1.27.5-alpine3.21
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
# Статика под /tg: совпадает с base Vite и с проксей главного nginx.
COPY --from=builder /app/dist /usr/share/nginx/html/tg
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
