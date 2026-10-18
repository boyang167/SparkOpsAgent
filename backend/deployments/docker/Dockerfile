FROM golang:1.26-alpine AS builder

WORKDIR /src
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /server ./cmd/server \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /migrate-schema ./cmd/migrate \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /migrate-sql ./cmd/migrate-sql

FROM alpine:3.21

WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata wget

COPY --from=builder /server ./server
COPY --from=builder /migrate-schema ./migrate-schema
COPY --from=builder /migrate-sql ./migrate-sql
COPY deployments/docker/config.docker.yaml ./config.yaml
COPY migrations ./migrations

EXPOSE 8080
CMD ["./server"]
