FROM node:24-bookworm-slim AS frontend

WORKDIR /src/web

RUN corepack enable && corepack prepare pnpm@11.3.0 --activate

COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

COPY web/ ./
RUN pnpm build

FROM golang:1.25-alpine3.23 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend /src/web/dist ./web/dist

ARG VERSION=v1.0.0
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X github.com/Rehtt/hamster-bin/internal/version.Version=${VERSION}" \
    -trimpath \
    -o /hamster-bin \
    cmd/server/main.go

FROM alpine:3.23

WORKDIR /app

COPY --from=builder /hamster-bin /app/hamster-bin

ENV PORT=8080 \
    DB_DRIVER=sqlite \
    DB_PATH=/app/data/inventory.db \
    IMAGE_DIR=/app/data/images

EXPOSE 8080

ENTRYPOINT ["/app/hamster-bin"]
