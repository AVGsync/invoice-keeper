# invoice-keeper

Напоминания о регулярных платежах (Telegram за 3 дня до срока).

```
backend/    Go API: cmd, internal (app, model, repository, service, infrastructure, transport), migrations, Dockerfile
frontend/   веб-панель (пока пусто)
```

## Локальный запуск

```bash
cp .env.example .env
make up
curl http://localhost:8080/api/v1/ping
```

## Образы

Workflow `.github/workflows/docker-publish.yml` на push в `main` собирает образ
и публикует в GHCR: `ghcr.io/<owner>/invoice-keeper-backend:latest` и `:<sha>`.
