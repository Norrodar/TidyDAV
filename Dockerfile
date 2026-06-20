# syntax=docker/dockerfile:1

# ── Stage 1: build the SvelteKit frontend ────────────────────────────────────
FROM node:22-alpine AS frontend
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
# Writes the static SPA to /src/internal/ui/dist (see web/svelte.config.js).
RUN npm run build

# ── Stage 2: build the Go binary (CGO disabled, static) ──────────────────────
FROM golang:1.25-alpine AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Replace the committed placeholder with the freshly built frontend.
COPY --from=frontend /src/internal/ui/dist ./internal/ui/dist
ARG VERSION=docker
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/tidydav ./cmd/tidydav

# ── Stage 3: minimal runtime ─────────────────────────────────────────────────
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata su-exec
COPY --from=backend /out/tidydav /tidydav
COPY deploy/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENV TIDYDAV_DB_PATH=/data/tidydav.db \
    TIDYDAV_LISTEN_ADDR=:8080
EXPOSE 8080
VOLUME ["/data"]

# The entrypoint drops privileges to PUID:PGID; HEALTHCHECK calls the binary's
# built-in probe (no curl/wget needed).
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/tidydav", "-healthcheck"]
ENTRYPOINT ["/entrypoint.sh"]
