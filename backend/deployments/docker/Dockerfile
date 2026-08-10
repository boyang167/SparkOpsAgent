FROM golang:1.25-alpine AS builder

WORKDIR /src
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /server ./cmd/server

FROM alpine:3.21

WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata wget

COPY --from=builder /server ./server
COPY deployments/docker/config.docker.yaml ./config.yaml
COPY migrations ./migrations

EXPOSE 8080
CMD ["./server"]
