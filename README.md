# Invoice Keeper

Учёт регулярных платежей команды. Напоминания об оплате приходят в Telegram за 3 дня до срока, просрочка считается со следующего дня после срока. Платежи в RUB, USD и EUR никогда не суммируются между собой. В проекте есть веб-панель, Telegram-бот, Mini App и ассистент «спроси про платежи» на естественном языке.

Роли: `admin` (учётка задаётся в `.env`, через API не меняется) и `user`. Платёж всегда закреплён за ответственным; отметить оплату может он сам или админ.

## Состав

| Каталог | Что внутри |
|---|---|
| `backend/` | Go API (chi, PostgreSQL), миграции, Swagger в `backend/docs`, Telegram-бот и движок напоминаний |
| `frontend/` | Веб-панель (Nuxt): тёмная и светлая тема, адаптирована под телефоны и планшеты (меню-шторка, таблицы-карточки, фильтры снизу), талисман проекта |
| `miniapp/` | Telegram Mini App (Vue 3 + Vite), раздаётся по `/tg/` |
| `nlq/` | Python: ассистент (агент с LLM через любой OpenAI-совместимый API и MCP-сервер только для чтения БД) |
| `nginx/` | Образ nginx: TLS и прокси на backend, frontend и miniapp; `local.conf` — тот же прокси без TLS для локального compose |
| `scripts/` | `local-setup.sh` (подготовка локального запуска), `split-env.sh` (разрезать старый `.env`), `cert.sh` (сертификат Let's Encrypt), `seed.py` (демо-данные) |

```
Браузер / Telegram ──> nginx :443 ──┬─ /api/ ──> backend ──> PostgreSQL
                                    ├─ /tg/  ──> miniapp        ^
                                    └─ /     ──> frontend       │ роль nlq_reader (read-only)
                                          backend ──> nlq-agent ──> nlq-mcp ──┘
                                                         └──> LLM API (OpenRouter и др.)
```

Ассистент необязателен: без него (или с `NLQ_ENABLED=false`) всё остальное работает.

## Требования

- Docker с плагином Compose, `make`.
- Для запуска без Docker: Go (версия в `backend/go.mod`), для Mini App Node 22, для `nlq` — `uv`.
- Telegram-бот от @BotFather (для напоминаний и Mini App).
- Для прода: сервер с открытыми портами 80 и 443 и доменом или публичным IP.

## Локальный запуск

```bash
make up
```

`make up` делает два шага:

1. `./scripts/local-setup.sh` готовит конфиги: создаёт `.env`, `.env.backend`, `.env.nlq` из примеров (старый единый `.env` разрезает), дописывает недостающие настройки, генерирует случайные `JWT_*` и пару ключей ассистента, ставит `ALLOWED_ORIGINS` под локальный nginx. Скрипт безопасен при повторном запуске: уже заданные значения не трогает.
2. `docker compose -f docker-compose.local.yaml up -d --build` собирает и поднимает всё: postgres, миграции, backend, веб-панель, Mini App, nginx, ассистента (`nlq-db-init`, `nlq-mcp`, `nlq-agent`) и pgAdmin.

Первая сборка занимает 5–10 минут, потом образы берутся из кэша. Когда `docker compose -f docker-compose.local.yaml ps` показывает `healthy` у backend, `nlq-mcp` и `nlq-agent`, можно открывать сайт.

| Что | Адрес |
|---|---|
| **Сайт** (панель админа и сотрудника, адаптирован под телефон) | `http://localhost:8000` |
| Mini App | `http://localhost:8000/tg/` |
| API | `http://localhost:8000/api/v1/` или напрямую `http://localhost:8080/api/v1/` |
| Swagger (при `DEBUG=true`) | `http://localhost:8080/api/v1/docs/index.html` |
| pgAdmin | `http://localhost:5050` (`PGADMIN_DEFAULT_EMAIL` / `PGADMIN_DEFAULT_PASSWORD`, по умолчанию `ad@ad.com` / `admin`) |
| PostgreSQL | `localhost:5432` |
| MCP Inspector / агент (отладка) | `127.0.0.1:8900`, `127.0.0.1:8090` |

Вход на сайт: `admin` и пароль из `ADMIN_PASSWORD` в `.env.backend` (скрипт ставит `admin123`, только для локальной работы). Демо-данные: `ADMIN_PASSWORD=admin123 python3 scripts/seed.py`.

Что работает локально сразу, а что нужно включить:

| Часть | Состояние |
|---|---|
| Сайт, API, база, миграции, чеки, журнал | работают |
| Ассистент | работает в режиме `rules` без ключа; с `LLM_API_KEY` в `.env.nlq` отвечает через LLM (`mode: llm` в `/assistant/status`) |
| Почта | выключена: пустой `SMTP_HOST` в `.env.backend`. Заполните `SMTP_*`, чтобы слать письма |
| Telegram: вход через бота, напоминания, кнопка «Оплачено» | выключены (`TELEGRAM_MODE=off`). Для проверки поставьте `TELEGRAM_MODE=polling` и **отдельный** токен тестового бота в `TELEGRAM_BOT_TOKEN` и `TELEGRAM_BOT_USERNAME` |
| Mini App внутри Telegram | нужен HTTPS-адрес (например, туннель ngrok или cloudflared на порт 8000) и `MINIAPP_URL` в `.env.backend`; в браузере страница `/tg/` откроется, но войти без Telegram нельзя |

После правки `.env.backend` или `.env.nlq` перезапустите нужный сервис: `docker compose -f docker-compose.local.yaml up -d backend nlq-agent`.

Остановка и сброс:

```bash
make down                                          # остановить, данные сохраняются
docker compose -f docker-compose.local.yaml down -v   # остановить и стереть базу и чеки
```

Другие команды: `make logs` (логи backend), `make ps`, `make run` (backend без Docker, нужен Postgres), `make test`, `make swagger` (перегенерировать `backend/docs`).

Разработка фронта с горячей перезагрузкой: `cd frontend && npm ci && npm run dev` (http://localhost:3000). Запросы к `/api/v1` проксируются на адрес из `nuxt.config.ts` (`nitro.devProxy`, сейчас боевой сервер). Чтобы работать с локальным backend, поменяйте `target` на `http://localhost:8080/api/v1` или пользуйтесь собранным сайтом на `:8000`.

Mini App с горячей перезагрузкой: `cd miniapp && npm ci && npm run dev` (http://localhost:5173/tg/). Запросы к `/api` идут на `DEV_API_TARGET` (по умолчанию `http://localhost:8080`). Для входа вне Telegram создайте `miniapp/.env.development.local` с `DEV_TG_BOT_TOKEN` (как у backend), `DEV_TG_USER_ID`, `DEV_TG_FIRST_NAME`.

## Деплой на сервер

Образы собирает GitHub Actions (`docker-publish.yml`) при push в `main` и публикует в GHCR: `ghcr.io/<owner>/invoice-keeper-{backend,migrations,frontend,miniapp,nginx,nlq}`. Сервер ничего не собирает, только тянет образы.

1. **Дождитесь сборки.** В GitHub → Actions должен завершиться `Publish Docker images`. Если пакеты в GHCR приватные, на сервере выполните `docker login ghcr.io`.

2. **Подготовьте сервер.** Установите Docker и Compose, откройте порты 80 и 443. Склонируйте репозиторий:

   ```bash
   git clone https://github.com/<owner>/invoice-keeper.git && cd invoice-keeper
   ```

3. **Заполните три файла.** Секреты разложены по контейнерам: каждый получает только своё.

   | Файл | Кому | Что внутри |
   |---|---|---|
   | `.env` | compose и общие переменные | домен, база, часовой пояс, публичный ключ ассистента |
   | `.env.backend` | только backend | JWT, админ, SMTP, Telegram, приватный ключ ассистента |
   | `.env.nlq` | только агент | ключ и адрес LLM, модели |

   ```bash
   cp .env.example .env && cp .env.backend.example .env.backend && cp .env.nlq.example .env.nlq
   ```

   Если у вас старый единый `.env`, разрежьте его скриптом: `./scripts/split-env.sh` (копия старого файла останется в `.env.bak`).

   | Переменная | Файл и значение |
   |---|---|
   |---|---|
   | `DOMAIN` | `.env`: домен или публичный IP сервера |
   | `LE_EMAIL` | `.env`: почта для Let's Encrypt |
   | `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` | `.env`: свои значения (обязательны) |
   | `APP_TIMEZONE` | `.env`: например `Europe/Moscow` |
   | `JWT_ACCESS_SECRET`, `JWT_REFRESH_SECRET` | `.env.backend`: случайные строки, `openssl rand -hex 32` |
   | `ADMIN_LOGIN`, `ADMIN_PASSWORD` | `.env.backend`: учётка админа |
   | `ALLOWED_ORIGINS` | `.env.backend`: `https://<DOMAIN>` |
   | `DEBUG` | `.env.backend`: `false` (при `true` открыт Swagger) |
   | `TELEGRAM_*`, `MINIAPP_URL`, `REMINDER_*`, `SMTP_*` | `.env.backend`, см. разделы «Telegram» и «Почта» |
   | `NLQ_*`, `LLM_*` | см. раздел «Ассистент» |

   `APP_ENV` задавать не нужно: compose ставит `production`. В проде `REMINDER_SOURCE` должен быть `db`.

4. **Запустите.** Сначала стянуть образы, затем выпустить сертификат (команда сама остановит nginx на время выпуска и поднимет весь стек):

   ```bash
   docker compose pull
   ./scripts/cert.sh issue
   ```

   Продление сертификата по cron (для IP-адреса сертификат живёт 6 дней, поэтому ежедневно):

   ```bash
   0 4 * * * cd /path/to/invoice-keeper && ./scripts/cert.sh renew >> cert.log 2>&1
   ```

   Самоподписанный сертификат для проверки без интернета: `./scripts/cert.sh self-signed` (Telegram его не примет).

5. **Проверьте:**

   ```bash
   docker compose ps
   curl https://<DOMAIN>/api/v1/ping       # pong
   ```

   Откройте `https://<DOMAIN>/` и войдите под админом. Mini App: `https://<DOMAIN>/tg/`.

6. **Обновление:**

   ```bash
   git pull && docker compose pull && docker compose up -d
   ```

   Если после обновления в Mini App или API внезапно 404, перезапустите nginx: `docker compose restart nginx`.

Логи: `docker compose logs -f backend` (так же `nginx`, `miniapp`, `nlq-agent`).

## Telegram

1. В @BotFather: `/newbot`, получите токен. Для Mini App: **Bot Settings → Configure Mini App → Enable**, URL `https://<DOMAIN>/tg` (не `/api/v1/tg`); при желании Menu Button с тем же адресом.
2. Пропишите в `.env`:

   ```
   TELEGRAM_MODE=webhook
   TELEGRAM_BOT_TOKEN=<токен>
   TELEGRAM_BOT_USERNAME=<username без @>
   TELEGRAM_WEBHOOK_URL=https://<DOMAIN>/api/v1/telegram/webhook
   TELEGRAM_WEBHOOK_SECRET=<openssl rand -hex 32>
   MINIAPP_URL=https://<DOMAIN>/tg
   REMINDER_SOURCE=db
   ```

   Перезапустите: `docker compose up -d backend`. Backend сам вызовет `setWebhook`.
   Локально используйте `TELEGRAM_MODE=polling` и **отдельного** dev-бота: один токен нельзя слушать и вебхуком, и поллингом.
3. **Привяжите аккаунт:** в веб-панели в профиле получите ссылку привязки и откройте её в Telegram (кнопка Start). Админ должен быть привязан, чтобы назначить чат.
4. **Командный чат:** добавьте бота в группу, отправьте `/chatid` (покажет id), затем от админа `/bind`. Напоминания идут только в этот чат. `/unbind` отключает.

Команды бота: `/today`, `/week`, `/overdue`, `/ask <вопрос>` (в личке можно писать вопрос без команды), `/new` (сбросить диалог с ассистентом), `/unlink`, `/help`.

Вход на сайт через бота: на странице входа кнопка «Войти через Telegram» открывает бота по ссылке `t.me/<бот>?start=login_<код>`. Бот спрашивает «Войти как …?» с кнопками «Войти» и «Это не я», сайт тем временем опрашивает `POST /api/v1/auth/telegram/poll` и после подтверждения получает те же куки, что при входе по паролю. Так входят только те, кто уже привязал Telegram; запрос живёт 5 минут и одноразовый. Нужны включённый бот и миграция `000017`.

Как работают напоминания:
- Каждые 30 секунд движок ставит в очередь неоплаченные платежи со сроком в ближайшие `REMINDER_LEAD_DAYS` дней (по умолчанию 3) и просроченные не больше 3 дней назад. Повторов нет: один платёж на один срок и вид получает одно сообщение.
- В сообщении кнопки «Оплачено» и «Открыть» (Mini App на карточке платежа). Оплату отмечает ответственный или админ, сообщение обновляется.
- Если платёж оплатили в вебе или остановили, сообщение в чате правится само.

## Почта

Письма-напоминания работают как второй канал рядом с Telegram: чат команды получает сообщение, а каждый сотрудник, включивший почту, получает ещё и личную сводку по своим платежам.

1. Задайте SMTP в `.env` (подходит любой сервис с STARTTLS на порту 587 или 2525, например Brevo): `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM` (подтверждённый у провайдера адрес без имени). Для ссылки в письме укажите `PUBLIC_URL` или `DOMAIN`.
2. Проверьте отправку: админом выполните `POST /api/v1/notifications/test-email` с телом `{"to": "почта"}` (ответ 202 — ушло, 502 — ошибка SMTP с текстом причины).
3. Сотрудник открывает в Mini App **Профиль → Почта**: вводит адрес, выбирает, за сколько дней и во сколько напоминать, и подтверждает адрес кодом из письма. Через API то же самое: `/notifications/settings`, `/notifications/email/code`, `/notifications/email/verify`.

Рассылка идёт раз в минуту и учитывает часовой пояс и время отправки каждого человека. Письмо не отправляется, пока адрес не подтверждён. Один платёж и срок дают не больше одного письма с «приближается срок» и одного с «просрочено» (первые 3 дня), письма собираются в одну сводку на человека. Валюты в письме не суммируются. Отключить рассылку целиком: `EMAIL_REMINDERS_ENABLED=false`.

## Ассистент

1. Сгенерируйте ключи подписи (один раз): приватный `NLQ_ACTOR_PRIVATE_KEY` пойдёт в `.env.backend`, публичный `NLQ_ACTOR_PUBLIC_KEY` в `.env`:

   ```bash
   docker run --rm ghcr.io/<owner>/invoice-keeper-nlq:latest nlq.keygen
   ```

   `NLQ_ACTOR_PRIVATE_KEY` нужен только backend, `NLQ_ACTOR_PUBLIC_KEY` — агенту и MCP.
2. В `.env.backend` задайте `NLQ_ENABLED=true`, в `.env` пароль роли `NLQ_DB_PASSWORD`.
3. Подключите LLM: подойдёт любой OpenAI-совместимый API (`/chat/completions` с вызовом функций).
   Всё про LLM лежит в `.env.nlq`.
   - `LLM_BASE_URL`: адрес API; пусто означает OpenRouter. Например `https://api.openai.com/v1`, `https://api.groq.com/openai/v1`, `https://api.deepseek.com/v1`, свой vLLM или Ollama (`http://host:11434/v1`).
   - `LLM_API_KEY`: ключ, только в файле `.env.nlq` . Так он не попадает в контейнер backend. В `.env` его класть не нужно.
   - `NLQ_MODELS`: основная и резервная модель через запятую в формате выбранного API (у OpenAI `gpt-4o-mini`, у OpenRouter `vendor/model`). Модель должна уметь tool calling.
   - `LLM_MAX_TOKENS_FIELD=max_completion_tokens`: для новых моделей OpenAI (gpt-5, o-серия).
   - Старые имена `OPENROUTER_API_KEY` и `OPENROUTER_BASE_URL` по-прежнему работают. Поля, которые понимает только OpenRouter (цепочка моделей, запрет на сбор данных), отправляются лишь на его адрес.
   - Без ключа ассистент работает в упрощённом режиме `rules`, без LLM.
4. `docker compose up -d`. Проверка: `GET /api/v1/assistant/status` под авторизованным пользователем.

Агент вызывает инструменты MCP (только чтение под ролью `nlq_reader`), числа в ответе проверяются по данным, валюты не складываются. История диалога хранится на сервере, ассистент помнит последние реплики: `GET` и `DELETE /api/v1/assistant/history`.

## Чеки

К периоду платежа можно прикрепить чек: файл или ссылку. Доступно ответственному за платёж и админу. В Mini App это блок «Чеки» на карточке платежа (кнопки «Прикрепить файл» и «Добавить ссылку»).

- **Форматы файлов:** PDF, JPEG, PNG, GIF, WebP, HEIC, TXT, CSV, DOCX, XLSX, ODT, ODS. Тип проверяется по содержимому, а не по расширению; HTML, SVG и исполняемые файлы отклоняются. Размер до `RECEIPTS_MAX_MB` (по умолчанию 10 МБ), не больше 20 чеков на платёж.
- **Ссылки:** только `http` и `https`, без логина и пароля в адресе; без схемы подставляется `https`.
- **Хранение:** файлы лежат в каталоге `RECEIPTS_DIR` (в Docker это том `receipts-data`, его нужно включать в бэкапы вместе с базой), метаданные в таблице `payment_receipts`.
- **API:** `GET` и `POST /api/v1/payment-occurrences/{id}/receipts` (файл, `multipart/form-data`), `POST …/receipts/link`, `DELETE /api/v1/receipts/{id}`, `GET /api/v1/receipts/{id}/access`. Файл открывается по временному подписанному адресу на 5 минут (`/api/v1/receipt-files/{id}?exp=…&sig=…`), поэтому работает в браузере и внутри Telegram без заголовков авторизации.
- Прикрепление и удаление чеков попадают в журнал действий.

## Демо-данные

```bash
python3 scripts/seed.py                    # локально: http://localhost:8080, admin/admin123
API_URL=https://<DOMAIN>/api/v1 ADMIN_LOGIN=<логин> ADMIN_PASSWORD=<пароль> python3 scripts/seed.py
```

Создаёт персоны, 4 сотрудников (пароль `Demo12345`), 12 регулярных платежей в трёх валютах и отмечает часть прошлых периодов оплаченными. Скрипт можно запускать повторно.

## API

- Swagger: `https://<DOMAIN>/api/v1/docs/index.html` (только при `DEBUG=true`), спецификация в `backend/docs/`.
- Основные группы: `/auth`, `/users`, `/personas`, `/recurring-payments` (+ `/search`), `/payment-occurrences` (+ `/search`), `/payments/summary`, `/notifications/settings`, `/assistant/*`, `/audit-log` и `/me/activity` (журнал действий), `/receipts/*` (чеки), `/telegram/*`. В списке пользователей (`GET /users`, `GET /auth/me`) есть поля `telegram_linked`, `telegram_username` и `telegram_linked_at`: привязан ли Telegram.
- Журнал действий фиксирует всё, что пользователи делают в вебе, Mini App и боте: кто, откуда, что и над чем. Пароли и токены в журнал не попадают.

## Миграции

Лежат в `backend/migrations` и применяются сервисом `migrate` при каждом запуске. Вручную (через локальный compose):

```bash
make migrate-up
make migrate-down steps=1
make migrate-version
make migrate-create seq=add_something
```

## Переменные окружения

Настройки разложены по трём файлам, чтобы каждый контейнер получал только своё: пароль SMTP или токен бота не попадают в контейнеры ассистента, а ключ LLM не попадает в backend. Примеры лежат рядом: `.env.example`, `.env.backend.example`, `.env.nlq.example`.

| Файл | Кто читает | Для чего |
|---|---|---|
| `.env` | compose и несколько контейнеров | база данных, домен, часовой пояс, общие ключи ассистента |
| `.env.backend` | только backend | вход и права, почта, Telegram, напоминания, чеки, ассистент (сторона backend) |
| `.env.nlq` | только агент ассистента | ключ и адрес LLM, модели |

### `.env`: общее

| Переменная | За что отвечает |
|---|---|
| `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` | логин, пароль и имя базы; используются postgres, миграциями, backend и `nlq-db-init` |
| `POSTGRES_HOST`, `POSTGRES_PORT` | только для запуска backend без Docker (`make run`); в контейнерах подставляется `postgres:5432` |
| `APP_TIMEZONE` | часовой пояс бизнес-логики: что значит «сегодня», когда платёж просрочен, окно напоминаний |
| `TZ` | часовой пояс только для системных логов |
| `DOMAIN` | домен или публичный IP сервера: сертификат, nginx, ссылка в письмах |
| `LE_EMAIL` | почта для сертификата Let's Encrypt (`scripts/cert.sh`) |
| `IMAGE_PREFIX`, `IMAGE_TAG` | откуда брать образы (по умолчанию GHCR и тег `latest`) |
| `NLQ_ACTOR_PUBLIC_KEY` | публичный ключ: агент и MCP проверяют им подпись backend |
| `NLQ_DB_PASSWORD` | пароль read-only роли `nlq_reader`, под которой ассистент читает базу |
| `NLQ_USER_SCOPE` | что видит обычный сотрудник в ассистенте: `team` (все платежи) или `own` (только свои) |
| `NLQ_TODAY`, `NLQ_DEV_SCHEMA` | отладка: подменить «сегодня» и тестовая схема; в проде оставьте пустыми |
| `LOCAL_WEB_PORT`, `LOCAL_API_PORT`, `LOCAL_POSTGRES_PORT`, `LOCAL_PGADMIN_PORT`, `PGADMIN_*` | только `docker-compose.local.yaml`: порты (сайт `8000`, API `8080`, база `5432`, pgAdmin `5050`) и вход в pgAdmin |

### `.env.backend`: backend

**Сервер и вход**

| Переменная | За что отвечает |
|---|---|
| `APP_ENV` | `local` или `production`; в прод-compose всегда `production` |
| `BIND_ADDR` | порт HTTP внутри контейнера |
| `LOG_LEVEL` | подробность логов: `debug`, `info`, `warning`, `error` |
| `DEBUG` | `true` открывает Swagger; в проде ставьте `false` |
| `ALLOWED_ORIGINS` | с каких сайтов браузеру разрешено обращаться к API (CORS), например `https://<DOMAIN>` |
| `JWT_ACCESS_SECRET`, `JWT_REFRESH_SECRET` | секреты подписи токенов входа; задайте случайные (`openssl rand -hex 32`), смена разлогинит всех |
| `TTL_ACCESS_TOKEN`, `TTL_REFRESH_TOKEN` | сколько живут токены, секунды (3600 = час, 604800 = неделя) |
| `ADMIN_LOGIN`, `ADMIN_PASSWORD` | учётка администратора, создаётся из этих значений; через API её не изменить |

**Чеки платежей**

| Переменная | За что отвечает |
|---|---|
| `RECEIPTS_DIR` | каталог для загруженных файлов; в Docker это том, задавать не нужно |
| `RECEIPTS_MAX_MB` | максимальный размер файла чека, МБ (по умолчанию 10) |

**Почта**

| Переменная | За что отвечает |
|---|---|
| `SMTP_HOST`, `SMTP_PORT` | почтовый сервер; пустой `SMTP_HOST` выключает почту. Если хостер блокирует 587, используйте 2525 |
| `SMTP_USERNAME`, `SMTP_PASSWORD` | логин и SMTP-ключ провайдера |
| `SMTP_FROM` | адрес отправителя, подтверждённый у провайдера, без имени |
| `EMAIL_REMINDERS_ENABLED` | включает рассылку напоминаний на почту (работает, если задан SMTP) |
| `PUBLIC_URL` | адрес приложения для ссылки в письме; пусто означает `https://<DOMAIN>` |

**Telegram и напоминания**

| Переменная | За что отвечает |
|---|---|
| `TELEGRAM_MODE` | `off` (бот выключен), `polling` (локально, отдельный бот), `webhook` (прод) |
| `TELEGRAM_BOT_TOKEN` | токен бота из BotFather; пустой выключает бота |
| `TELEGRAM_BOT_USERNAME` | имя бота без `@`: для ссылок привязки и кнопки «Открыть» |
| `TELEGRAM_MINIAPP_SHORT_NAME` | короткое имя Mini App, если создавали через `/newapp`; обычно пусто |
| `TELEGRAM_WEBHOOK_URL`, `TELEGRAM_WEBHOOK_SECRET` | адрес вебхука (`https://<DOMAIN>/api/v1/telegram/webhook`) и его секрет; нужны только в режиме `webhook` |
| `TELEGRAM_TEAM_CHAT_ID` | id командного чата заранее; иначе его назначает админ командой `/bind` |
| `MINIAPP_URL` | адрес Mini App (`https://<DOMAIN>/tg`): кнопка в личке бота |
| `TELEGRAM_INITDATA_MAX_AGE` | сколько живёт подпись входа из Mini App (по умолчанию 10 минут) |
| `REMINDER_SOURCE` | откуда брать платежи для напоминаний: `db` (боевой) или `stub` (демо, в проде запрещён) |
| `REMINDER_LEAD_DAYS` | за сколько дней напоминать в Telegram, по кейсу не меньше 3 |
| `REMINDER_TICK` | как часто движок проверяет очередь (по умолчанию 30 секунд) |

**Ассистент (сторона backend)**

| Переменная | За что отвечает |
|---|---|
| `NLQ_ENABLED` | включает ручки ассистента; при `false` они отвечают 503 |
| `NLQ_AGENT_URL` | адрес агента внутри сети compose |
| `NLQ_TIMEOUT` | общий лимит времени на один вопрос |
| `NLQ_RATE_PER_MIN`, `NLQ_DAILY_LIMIT`, `NLQ_MAX_CONCURRENT` | лимиты: вопросов в минуту и в сутки на человека, одновременных вопросов |
| `NLQ_ACTOR_PRIVATE_KEY` | приватный ключ подписи; лежит только здесь, парный публичный в `.env` |
| `NLQ_MEMORY_ENABLED` | включает память диалога, чтобы работали уточнения вроде «а в долларах?» |
| `NLQ_MEMORY_TURNS`, `NLQ_MEMORY_TTL`, `NLQ_MEMORY_MAX_TURNS`, `NLQ_HISTORY_KEEP` | сколько реплик идёт в контекст, сколько они живут, максимум в памяти и сколько хранить в базе |

### `.env.nlq`: агент ассистента

| Переменная | За что отвечает |
|---|---|
| `LLM_BASE_URL` | адрес OpenAI-совместимого API; пусто означает OpenRouter |
| `LLM_API_KEY` | ключ к этому API |
| `LLM_MAX_TOKENS_FIELD` | `max_tokens` или `max_completion_tokens` (нужен для новых моделей OpenAI) |
| `NLQ_MODELS` | основная и резервная модель через запятую, в формате выбранного API |
| `NLQ_LLM` | режим: `llm` (боевой), `rules` (упрощённый, без LLM), `fake` (тесты) |
| `NLQ_RULES_FALLBACK` | при отказе LLM отвечать упрощённым режимом, а не ошибкой |

Если у вас старый единый `.env`, его разрезает `./scripts/split-env.sh`.

## Проблемы

| Симптом | Что делать |
|---|---|
| 404 на `/tg/` или `/api/` после обновления | `docker compose restart nginx` |
| Локально `make up` падает на `.env.backend` | запустите `./scripts/local-setup.sh`: он создаст файл и сгенерирует секреты |
| Локально порт 8000, 8080, 5432 или 5050 занят | поменяйте `LOCAL_WEB_PORT`, `LOCAL_API_PORT`, `LOCAL_POSTGRES_PORT`, `LOCAL_PGADMIN_PORT` в `.env` |
| Локально вход на сайте не держится в Safari | куки `Secure` на `http://` Safari не принимает: откройте сайт в Chrome или Firefox |
| Кнопка «Войти через Telegram» отвечает «недоступен» | бот выключен: `TELEGRAM_MODE` и токен в `.env.backend`; войти так может только тот, кто уже привязал Telegram в профиле |
| Бот молчит, в логах `409 Conflict` | на токене уже стоит webhook или запущен второй поллер; используйте отдельный токен для локали |
| Напоминания не приходят | нет командного чата (`/bind`), `TELEGRAM_MODE=off`, пустой токен или `REMINDER_SOURCE=stub` |
| Ассистент отвечает 503 | `NLQ_ENABLED=false`, не поднят `nlq-agent`, не совпадают ключи в `.env` или устаревший образ: `docker compose pull && docker compose up -d` |
| Mini App пишет «Откройте из Telegram» | страница открыта в браузере, а не из бота |
| Сертификат не выпускается | порт 80 занят или `DOMAIN` не указывает на сервер; проверьте `./scripts/cert.sh issue` |
