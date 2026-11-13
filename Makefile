.PHONY: test backend-test frontend-test helm-lint seed migrate server filesystem-up filesystem-test

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

dev-restart:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev-restart.ps1

dev-stop:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev-restart.ps1 -Stop

docker-up:
	cd backend/deployments/docker && docker compose up --build -d

docker-down:
	cd backend/deployments/docker && docker compose down

filesystem-up:
	docker compose -f backend/deployments/docker/docker-compose.yaml up -d --build

filesystem-test:
	cd backend && go test -tags=integration ./tests/filesystem/... -count=1
