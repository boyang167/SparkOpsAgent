.PHONY: test backend-test frontend-test helm-lint seed migrate server

test: backend-test frontend-test

backend-test:
	cd backend && go test ./...

frontend-test:
	cd fronted && pnpm --filter web typecheck

helm-lint:
	helm lint deployments/helm/sparkops

migrate:
	cd backend && go run ./cmd/migrate

seed:
	cd backend && go run ./cmd/seed

server:
	cd backend && go run ./cmd/server

dev-web:
	cd fronted && pnpm dev

docker-up:
	cd backend/deployments/docker && docker compose up --build -d

docker-down:
	cd backend/deployments/docker && docker compose down
