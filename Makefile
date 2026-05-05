.PHONY: help dev down logs build api-build web-build migrate migrate-down sqlc seed test test-go test-web lint lint-go lint-web fmt fmt-go fmt-web clean prod-up prod-down prod-logs prod-migrate

# Use deploy/.env for compose
COMPOSE := docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.dev.yml --env-file deploy/.env
COMPOSE_PROD := docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.caddy.yml --env-file deploy/.env

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

dev: ## Start full dev stack (postgres, redis, mailhog, adminer, api, web)
	$(COMPOSE) up -d --build

down: ## Stop dev stack
	$(COMPOSE) down

logs: ## Tail logs from all services
	$(COMPOSE) logs -f --tail=100

build: api-build web-build ## Build all production images

api-build:
	docker build -t lunarleague/api:latest -f apps/api/Dockerfile apps/api

web-build:
	docker build -t lunarleague/web:latest -f apps/web/Dockerfile apps/web

migrate: ## Run all up migrations
	$(COMPOSE) run --rm api migrate up

migrate-down: ## Roll back the latest migration
	$(COMPOSE) run --rm api migrate down

sqlc: ## Regenerate sqlc bindings
	cd apps/api && sqlc generate

seed: ## Seed sports + a demo league
	$(COMPOSE) run --rm api seed

test: test-go test-web ## Run all tests

test-go:
	cd apps/api && go test ./...

test-web:
	cd apps/web && pnpm test

lint: lint-go lint-web

lint-go:
	cd apps/api && golangci-lint run ./...

lint-web:
	cd apps/web && pnpm lint

fmt: fmt-go fmt-web

fmt-go:
	cd apps/api && gofmt -w . && go vet ./...

fmt-web:
	cd apps/web && pnpm format

clean:
	$(COMPOSE) down -v
	rm -rf apps/api/bin apps/api/tmp apps/web/.next apps/web/node_modules

prod-up: ## Production stack with TLS (Caddy); needs deploy/.env (see deploy/.env.production.example)
	$(COMPOSE_PROD) up -d --build --remove-orphans

prod-down: ## Stop production stack (TLS overlay)
	$(COMPOSE_PROD) down

prod-logs: ## Tail production logs (api, worker, web, caddy)
	$(COMPOSE_PROD) logs -f --tail=100

prod-migrate: ## Run DB migrations against production compose stack
	$(COMPOSE_PROD) run --rm api migrate up
