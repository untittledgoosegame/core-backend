COMPOSE = docker compose -f docker/compose.yml

.PHONY: help install dev run build test lint fmt tidy db-up db-down db-logs clean

help:
	@echo "install  - download go modules"
	@echo "dev      - run with hot reload (air)"
	@echo "run      - run the server"
	@echo "build    - compile to bin/server"
	@echo "test     - run tests"
	@echo "lint     - go vet"
	@echo "fmt      - gofmt the codebase"
	@echo "tidy     - go mod tidy"
	@echo "db-up    - start mongo, redis, mailpit (http://localhost:8025) and s3mock (R2 stand-in, :9090)"
	@echo "db-down  - stop the local services"
	@echo "db-logs  - tail mongo logs"
	@echo "clean    - remove build artifacts"

install:
	go mod download

dev:
	air

run:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

test:
	go test ./...

lint:
	go vet ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy

db-up:
	$(COMPOSE) up -d

db-down:
	$(COMPOSE) down

db-logs:
	$(COMPOSE) logs -f mongo

clean:
	rm -rf bin
