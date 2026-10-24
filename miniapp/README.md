# Invoice Keeper — Telegram Mini App

Отдельное приложение (Vue 3 + Vite + TypeScript), раздаётся nginx по `https://<DOMAIN>/tg/`.
Не связано с `frontend/` (Nuxt-сайтом).

## Стек

- Vue 3 + vue-router 4, без Pinia и UI-китов (сторы — обычный `reactive()`).
- Vite 8, TypeScript 6, vitest (node), Playwright (smoke).
- Telegram WebApp SDK подключается тегом `https://telegram.org/js/telegram-web-app.js`;
  типизированная обёртка — `src/telegram/webapp.ts`, dev-мок — `dev/telegram-mock.js`.

## Скрипты

| Команда | Что делает |
|---|---|
| `npm run dev` | дев-сервер на `http://localhost:5173/tg/` |
| `npm run build` | `vue-tsc --noEmit` + `vite build` + постпроверка `dev/check-dist.mjs` |
| `npm run typecheck` | только проверка типов |
| `npm test` | модульные тесты (vitest) |
| `npm run e2e` | Playwright smoke против прод-сборки |

## Локальная разработка

```
cd miniapp
npm ci
npm run dev        # http://localhost:5173/tg/
```

`/api` проксируется на `DEV_API_TARGET` (по умолчанию `http://localhost:8080`).

Переменные dev (в `miniapp/.env.development.local`, без префикса `VITE_`, читает только `vite.config.ts`):

| Переменная | Назначение |
|---|---|
| `DEV_API_TARGET` | адрес бэкенда для прокси `/api` (по умолчанию `http://localhost:8080`) |
| `DEV_TG_BOT_TOKEN` | токен бота — должен совпадать с backend `TELEGRAM_BOT_TOKEN`; бот должен быть включён, иначе `/telegram/miniapp/*` отдаёт 404 |
| `DEV_TG_USER_ID` | id Telegram-пользователя (по умолчанию `100001`) |
| `DEV_TG_FIRST_NAME`, `DEV_TG_USERNAME` | имя и username для dev-initData |
| `DEV_TG_START_PARAM` | startapp-параметр, например `occ_<hex32>` |
| `DEV_TG_COLOR_SCHEME` | `light`\|`dark` для dev-мока |

Без `DEV_TG_BOT_TOKEN` initData пустой — показывается экран «Откройте из Telegram».

Мок Telegram и подписчик initData — только для dev/e2e, в прод-бандл не попадают
(`npm run build` падает, если это нарушено).

## Переменные сборки

- `VITE_APP_TIMEZONE` — таймзона «сегодня» (по умолчанию `Europe/Moscow`), должна совпадать
  с backend `APP_TIMEZONE`. Меняется только пересборкой образа (build-arg).

## Структура

- `src/api` — клиент (токен в памяти), эндпоинты (сверены со swagger), SSE и ассистент.
- `src/lib` — чистые помощники (деньги, даты, статусы, агрегаты, права, расписание) с тестами.
- `src/stores` — реактивные сторы (session, occurrences, recurring, users, assistant).
- `src/telegram` — обёртка WebApp и разбор startapp.
- `src/composables` — MainButton/BackButton/DirtyGuard/useAsync.
- `src/components`, `src/screens` — UI (русский).
- `dev/` — dev-мок, подписчик initData, Vite-плагин, постпроверка сборки.
- `e2e/` — Playwright smoke (прод-сборка).
