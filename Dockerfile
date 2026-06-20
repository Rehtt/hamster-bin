FROM node:20-alpine AS frontend

WORKDIR /src/web

COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS builder

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

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /hamster-bin /app/hamster-bin

ENV PORT=8080 \
    DB_PATH=/app/data/inventory.db \
    IMAGE_DIR=/app/data/images

EXPOSE 8080

VOLUME ["/app/data"]

ENTRYPOINT ["/app/hamster-bin"]
