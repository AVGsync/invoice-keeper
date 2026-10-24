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

## Темы

Приложение поддерживает светлую и тёмную темы. Источник — `colorScheme` Telegram
(событие `themeChanged`); вне Telegram берётся системная настройка
`prefers-color-scheme`. Логика в `src/telegram/theme.ts`: `applyTheme()` вычисляет тему,
ставит `html[data-theme]`, мета `theme-color`/`color-scheme` и красит хром Telegram
(шапку, фон, нижнюю панель) цветом `CHROME[theme]`. Реактивный `theme` переприменяет
цвета `MainButton` при смене темы. Ранний inline-скрипт в `index.html` выставляет
`data-theme` до загрузки бандла, чтобы не было вспышки.

Токены темы — в `src/styles/base.css`: блок `:root` держит тёмные значения (Dimension +
акценты фронтенда), блок `:root[data-theme='light']` их переопределяет светлыми токенами,
зеркалящими веб-фронтенд. Семантические токены: `--fill`/`--on-fill` (залитый контрол),
`--on-horizon` (текст на hero-градиенте), `--accent`/`--accent-ink`/`--accent-bg`,
`--red|green|amber` с вариантами `-ink` (текст) и `-bg` (заливка бейджа). Цветные
hero-поверхности (`TodayScreen`, `Horizon`) не зависят от темы. Контраст текста ≥ 4.5:1 в
обеих темах (в тёмной акцент как текст — всегда `--accent-ink`, не `--accent`).

## Композер «Спросить»

Экран «Спросить» использует не `MainButton`, а composer как в ChatGPT
(`src/components/Composer.vue`): поле ввода с круглой кнопкой-стрелкой. `Enter` отправляет,
`Shift+Enter` — перевод строки (чистая логика `src/lib/composer.ts`), во время стрима кнопка
показывает «стоп» и кольцо-спиннер. Композер фиксирован над плавающей панелью вкладок; при
фокусе (флаг `src/stores/ui.ts`) панель уезжает вниз, чтобы не висеть над клавиатурой.
Высота композера пробрасывается в `--composer-h` (ResizeObserver) для отступа фида. История
диалога (память ассистента) подтягивается `GET /assistant/history`, кнопка «Новый диалог» —
`DELETE /assistant/history`.

## Структура

- `src/api` — клиент (токен в памяти, заголовок `X-Client: miniapp`), эндпоинты (сверены со swagger), SSE и ассистент.
- `src/lib` — чистые помощники (деньги, даты, статусы, агрегаты, права, расписание) с тестами.
- `src/stores` — реактивные сторы (session, occurrences, recurring, users, assistant).
- `src/telegram` — обёртка WebApp и разбор startapp.
- `src/composables` — MainButton/BackButton/DirtyGuard/useAsync.
- `src/components`, `src/screens` — UI (русский).
- `dev/` — dev-мок, подписчик initData, Vite-плагин, постпроверка сборки.
- `e2e/` — Playwright smoke (прод-сборка).
