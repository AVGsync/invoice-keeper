-include .env
export

COMPOSE_LOCAL := docker compose -f docker-compose.local.yaml
MIGRATION_DSN := postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@postgres:5432/$(POSTGRES_DB)?sslmode=disable

.PHONY: run build up down logs ps migrate-create migrate-up migrate-down migrate-version test vet swagger

run:
	@cd backend && go run ./cmd/apiserver

build:
	@cd backend && go build ./cmd/apiserver

up:
	@$(COMPOSE_LOCAL) up -d --build

down:
	@$(COMPOSE_LOCAL) down

logs:
	@$(COMPOSE_LOCAL) logs -f backend

ps:
	@$(COMPOSE_LOCAL) ps

migrate-create:
	@if [ -z "$(seq)" ]; then \
		echo "Нужен параметр seq, например: make migrate-create seq=init"; \
		exit 1; \
	fi
	@$(COMPOSE_LOCAL) run --rm migrate create -ext sql -dir /migrations -seq "$(seq)"

migrate-up:
	@$(COMPOSE_LOCAL) run --rm migrate -path /migrations -database "$(MIGRATION_DSN)" up

migrate-down:
	@$(COMPOSE_LOCAL) run --rm migrate -path /migrations -database "$(MIGRATION_DSN)" down $(steps)

migrate-version:
	@$(COMPOSE_LOCAL) run --rm migrate -path /migrations -database "$(MIGRATION_DSN)" version

test:
	@cd backend && go test ./...

vet:
	@cd backend && go vet ./...

swagger:
	@cd backend && go run github.com/swaggo/swag/cmd/swag@v1.16.4 init -g cmd/apiserver/main.go -o docs

.DEFAULT_GOAL := run
