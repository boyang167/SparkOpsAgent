# SparkOps 仓库根入口。
# 后端与 Compose 目标转发到 backend/Makefile，避免两处命令分叉。

.DEFAULT_GOAL := help

BACKEND_DIR := backend
FRONTEND_DIR := fronted

API_IMAGE ?= sparkops/api
WEB_IMAGE ?= sparkops/web
IMAGE_TAG ?= dev

.PHONY: help \
	run run-dev test build seed migrate migrate-up check-deps \
	web-install web-dev web-build web-lint web-typecheck web-test \
	docker-up docker-down docker-logs \
	docker-build docker-build-api docker-build-web \
	dev dev-backend dev-frontend dev-stop

define HELP_TEXT
SparkOps
--- Backend ---
  make run              启动 API，端口 8080
  make run-dev          启动 devserver
  make test             go test ./...
  make build            编译 backend/bin/server
  make migrate          执行数据库迁移
  make seed             写入演示数据
  make check-deps       检查 Go 依赖
--- Frontend ---
  make web-install      pnpm install
  make web-dev          启动前端，端口 3000
  make web-build        构建前端
  make web-lint         前端 lint
  make web-typecheck    前端类型检查
  make web-test         前端 vitest（单次）
--- Local ---
  make dev              重启后端和前端
  make dev-backend      只重启后端
  make dev-frontend     只重启前端
  make dev-stop         停止本地开发服务
--- Docker ---
  make docker-up        Compose 构建并启动
  make docker-logs      跟踪 API 日志
  make docker-down      停止 Compose
  make docker-build     构建 API 与 Web 镜像，tag=$(IMAGE_TAG)
endef

help:
	$(info $(HELP_TEXT))
	@:


run run-dev test build seed migrate migrate-up check-deps docker-up docker-down docker-logs:
	$(MAKE) -C $(BACKEND_DIR) $@

web-install:
	pnpm --dir $(FRONTEND_DIR) install

web-dev:
	pnpm --dir $(FRONTEND_DIR) dev

web-build:
	pnpm --dir $(FRONTEND_DIR) build

web-lint:
	pnpm --dir $(FRONTEND_DIR) lint

web-typecheck:
	pnpm --dir $(FRONTEND_DIR) typecheck

web-test:
	pnpm --dir $(FRONTEND_DIR) exec vitest run

docker-build: docker-build-api docker-build-web

docker-build-api:
	docker build -t $(API_IMAGE):$(IMAGE_TAG) -f backend/deployments/docker/Dockerfile backend

docker-build-web:
	docker build -t $(WEB_IMAGE):$(IMAGE_TAG) -f fronted/deployments/docker/Dockerfile fronted

ifeq ($(OS),Windows_NT)
dev:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev-restart.ps1

dev-backend:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev-restart.ps1 -Backend

dev-frontend:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev-restart.ps1 -Frontend

dev-stop:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev-restart.ps1 -Stop
else
dev:
	./scripts/dev-restart.sh

dev-backend:
	./scripts/dev-restart.sh --backend

dev-frontend:
	./scripts/dev-restart.sh --frontend

dev-stop:
	./scripts/dev-restart.sh --stop
endif
