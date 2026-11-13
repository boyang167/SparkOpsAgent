.PHONY: run test build seed migrate migrate-up docker-up docker-down docker-logs check-deps run-dev filesystem-up filesystem-test

run:
	go run ./cmd/server

run-dev:
	go run ./cmd/devserver

test:
	go test ./...

build:
	go build -o bin/server ./cmd/server

seed:
	go run ./cmd/seed

migrate:
	go run ./cmd/migrate

migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

check-deps:
	go run ./scripts/check-deps.go

docker-up:
	docker compose -f deployments/docker/docker-compose.yaml up -d --build

docker-down:
	docker compose -f deployments/docker/docker-compose.yaml down

docker-logs:
	docker compose -f deployments/docker/docker-compose.yaml logs -f api

filesystem-up:
	docker compose -f deployments/docker/docker-compose.yaml up -d --build

filesystem-test:
	go test -tags=integration ./tests/filesystem/... -count=1
