FROM golang:1.26-alpine AS builder

WORKDIR /src
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN mkdir -p /out \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server ./cmd/server \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/migrate-schema ./cmd/migrate \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/migrate-sql ./cmd/migrate-sql \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/file-transfer-gateway ./cmd/file-transfer-gateway \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/file-worker ./cmd/file-worker \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/sftp-gateway ./cmd/sftp-gateway \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/file-reconciler ./cmd/file-reconciler \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/migrate-knowledge-inventory ./cmd/migrate-knowledge-inventory \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/migrate-knowledge-precopy ./cmd/migrate-knowledge-precopy \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/migrate-knowledge-switch ./cmd/migrate-knowledge-switch

FROM alpine:3.21

WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S sparkops \
    && adduser -S -G sparkops -h /app sparkops \
    && mkdir -p /app/bin /var/lib/sparkops/filesystem /var/lib/sparkops/sftp \
    && chown -R sparkops:sparkops /app /var/lib/sparkops

COPY --from=builder /out/ ./bin/
RUN ln -s /app/bin/migrate-schema /app/migrate-schema \
    && ln -s /app/bin/migrate-sql /app/migrate-sql
COPY deployments/docker/config.docker.yaml ./config.yaml
COPY migrations ./migrations

USER sparkops
EXPOSE 8080 8082 2022
CMD ["/app/bin/server"]
