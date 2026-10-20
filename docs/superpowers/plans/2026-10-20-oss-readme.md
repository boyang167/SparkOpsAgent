# OSS README Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish a Chinese and English repository homepage plus an Apache-2.0 license, and remove the misspelled root note that contains live credentials.

**Architecture:** `README.md` and `README_EN.md` share one section order and the same commands, versions, ports, and demo account. `LICENSE` is the Apache License 2.0 text with the approved copyright line. Deeper API, RBAC, and Helm material stays in the existing documents.

**Tech Stack:** Markdown, Apache License 2.0, Go 1.26, Node.js 20+, pnpm 9.15.9, PostgreSQL 15, Redis 7.

## Global Constraints

- Copyright line is exactly `Copyright 2026 SparkOps contributors`.
- Keep the directory name `fronted/`.
- Demo account is only `demo@sparkops.local` / `Demo1234!`.
- Do not copy live GitLab, Jenkins, Harbor, RabbitMQ, cluster, or token values out of `READEMD.md`.
- Do not edit `backend/README.md`, `fronted/README.md`, Helm docs, historical specs, or the `../READEMD.md` test fixture.
- Do not rewrite Git history.

---

### Task 1: Chinese and English homepages

**Files:**
- Create: `README.md`
- Create: `README_EN.md`

**Interfaces:**
- Consumes: `docs/superpowers/specs/2026-10-20-oss-readme-design.md`
- Produces: linked homepages with identical facts

- [x] **Step 1: Write both READMEs**

Use the 15-section order in the spec. Commands:

```bash
make docker-up
make docker-down
cd backend
cp config.example.yaml config.yaml
go run ./cmd/migrate
go run ./cmd/seed
go run ./cmd/server
cd fronted
pnpm install
cp apps/web/.env.example apps/web/.env.local
pnpm dev
make test
make backend-test
make frontend-test
docker build -t your-registry/sparkops/api:1.0.0 -f backend/deployments/docker/Dockerfile backend
docker build -t your-registry/sparkops/web:1.0.0 -f fronted/deployments/docker/Dockerfile fronted
```

Ports: UI `http://localhost:3000`, API `http://localhost:8080`, Swagger `http://localhost:8080/api/docs`.

- [x] **Step 2: Check the section order and cross links**

Expected: `README.md` links to `README_EN.md` at the top, and the English file links back.

### Task 2: License and credential cleanup

**Files:**
- Create: `LICENSE`
- Delete: `READEMD.md`

- [x] **Step 1: Save the Apache License 2.0 text**

Set the appendix copyright to `Copyright 2026 SparkOps contributors`.

- [x] **Step 2: Delete `READEMD.md`**

- [x] **Step 3: Scan the new files**

Run a search for `glpat-`, `192.168.`, `Harbor12345`, `BdsDevops`, `asdf!`, `kubekey`, `11523e6d`, and `SuperAdmin1234` in `README.md`, `README_EN.md`, and `LICENSE`.

Expected: no matches. `Demo1234!` remains only as the local seed password.
