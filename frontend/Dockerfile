FROM node:22.23.2-alpine3.24 AS builder

WORKDIR /app
# postinstall (nuxt prepare) нужны исходники конфигов, поэтому копируем их до npm ci.
COPY package.json package-lock.json nuxt.config.ts tsconfig.json ./
RUN npm install -g npm@12.2.0
RUN npm ci

COPY app ./app
COPY public ./public
RUN npm run build

FROM node:22.23.2-alpine3.24

ENV NODE_ENV=production \
    NITRO_HOST=0.0.0.0 \
    NITRO_PORT=3000

WORKDIR /app
# Nitro кладёт в .output всё нужное для запуска, node_modules не требуется.
COPY --from=builder /app/.output ./.output

USER node
EXPOSE 3000

CMD ["node", ".output/server/index.mjs"]
