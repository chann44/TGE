SHELL := /bin/sh

GOOSE_DIR := sql/migrations
SQLC_CONFIG := sqlc.yaml
BACKEND_IMAGE ?= chann44/tge-backend:latest
WEB_IMAGE ?= chann44/tge-web:latest

.PHONY: help api-dev worker-dev scheduler-dev web-dev dev test fmt codegen migrate-up migrate-down migrate-status migrate-reset migrate-create docker-build-backend docker-build-web docker-push-backend docker-push-web docker-build docker-push

help:
	@printf "Available targets:\n"
	@printf "  make api-dev           Run Go API server\n"
	@printf "  make web-dev           Run Svelte web dev server\n"
	@printf "  make worker-dev        Run dependency worker\n"
	@printf "  make scheduler-dev     Run scheduled scan dispatcher\n"
	@printf "  make dev               Run API + web + worker dev servers\n"
	@printf "  make test              Run Go tests\n"
	@printf "  make fmt               Format Go code\n"
	@printf "  make codegen           Generate sqlc code\n"
	@printf "  make migrate-up        Apply goose migrations\n"
	@printf "  make migrate-down      Roll back one goose migration\n"
	@printf "  make migrate-status    Show goose migration status\n"
	@printf "  make migrate-reset     Roll back all goose migrations\n"
	@printf "  make migrate-create NAME=create_users  Create a new migration\n"
	@printf "  make docker-build      Build backend + web images\n"
	@printf "  make docker-push       Push backend + web images\n"

api-dev:
	go run ./apps/api

web-dev:
	npm --prefix apps/web run dev

worker-dev:
	go run ./apps/worker

scheduler-dev:
	go run ./apps/scheduler

dev:
	@set -e; \
	trap 'kill 0' INT TERM EXIT; \
	go run ./apps/api & \
	go run ./apps/worker & \
	go run ./apps/scheduler & \
	npm --prefix apps/web run dev & \
	wait

test:
	go test ./...

fmt:
	go fmt ./...

codegen:
	sqlc generate -f $(SQLC_CONFIG)

migrate-up:
	@set -e; \
	DATABASE_URL=$${DATABASE_URL:-$$(python3 scripts/get_database_url.py)}; \
	if [ -z "$$DATABASE_URL" ]; then \
		echo "DATABASE_URL is required (export it or set it in .env)"; \
		exit 1; \
	fi; \
	goose -dir $(GOOSE_DIR) postgres "$$DATABASE_URL?sslmode=disable" up

migrate-down:
	@set -e; \
	DATABASE_URL=$${DATABASE_URL:-$$(python3 scripts/get_database_url.py)}; \
	if [ -z "$$DATABASE_URL" ]; then \
		echo "DATABASE_URL is required (export it or set it in .env)"; \
		exit 1; \
	fi; \
	goose -dir $(GOOSE_DIR) postgres "$$DATABASE_URL?sslmode=disable" down

migrate-status:
	@set -e; \
	DATABASE_URL=$${DATABASE_URL:-$$(python3 scripts/get_database_url.py)}; \
	if [ -z "$$DATABASE_URL" ]; then \
		echo "DATABASE_URL is required (export it or set it in .env)"; \
		exit 1; \
	fi; \
	goose -dir $(GOOSE_DIR) postgres "$$DATABASE_URL?sslmode=disable" status

migrate-reset:
	@set -e; \
	DATABASE_URL=$${DATABASE_URL:-$$(python3 scripts/get_database_url.py)}; \
	if [ -z "$$DATABASE_URL" ]; then \
		echo "DATABASE_URL is required (export it or set it in .env)"; \
		exit 1; \
	fi; \
	goose -dir $(GOOSE_DIR) postgres "$$DATABASE_URL?sslmode=disable" reset

migrate-create:
	@if [ -z "$(NAME)" ]; then \
		echo "Usage: make migrate-create NAME=create_github_tables"; \
		exit 1; \
	fi
	goose -dir $(GOOSE_DIR) create $(NAME) sql

docker-build-backend:
	docker build -f deployments/docker/Dockerfile.backend -t $(BACKEND_IMAGE) .

docker-build-web:
	docker build -f deployments/docker/Dockerfile.web -t $(WEB_IMAGE) .

docker-push-backend:
	docker push $(BACKEND_IMAGE)

docker-push-web:
	docker push $(WEB_IMAGE)

docker-build: docker-build-backend docker-build-web

docker-push: docker-push-backend docker-push-web
