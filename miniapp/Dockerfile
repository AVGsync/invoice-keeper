# Telegram Mini App (miniapp/): отдельный статичный образ. Сборка Vite в первом
# этапе, раздача через nginx во втором. Контекст сборки — ./miniapp целиком
# (vite.config.ts импортирует ./dev/*, поэтому нужен весь каталог; лишнее
# отсекает .dockerignore).
FROM node:22.23.2-alpine3.24 AS builder
WORKDIR /app
# Сначала манифесты — слой кэшируется, пока не изменились зависимости.
COPY package.json package-lock.json .npmrc ./
# npm 12.2.0 — тот же, что сгенерировал package-lock.json (lockfileVersion npm 12),
# иначе npm ci ругается на несовместимый формат лока.
RUN npm install -g npm@12.2.0 && npm ci
COPY . .
# Зона приложения вшивается в бандл на этапе сборки (VITE_* доступны только при
# build). Значение по умолчанию совпадает с backend APP_TIMEZONE. Смена зоны —
# только пересборкой образа с другим build-arg.
ARG VITE_APP_TIMEZONE=Europe/Moscow
ENV VITE_APP_TIMEZONE=${VITE_APP_TIMEZONE}
RUN npm run build

FROM nginx:1.27.5-alpine3.21
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
# Статика лежит под /tg, чтобы пути совпадали с base '/tg/' у Vite и с проксей
# главного nginx (/tg/ -> miniapp:8080 без переписывания URI).
COPY --from=builder /app/dist /usr/share/nginx/html/tg
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
