# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# GOARCH=arm64 targets the Ampere A1 (arm64) VPS this is deployed on.
# amd64 is for regular x86 servers (e.g. Railway, most traditional cloud
# hosts) — the wrong target here. Any project with a Dockerfile hosted on
# this VPS needs its cross-compile arch flag (GOARCH, --platform, etc.)
# set to arm64, not amd64.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
    go build -ldflags="-s -w" -o api ./cmd/main.go

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM alpine:3.21

SHELL ["/bin/ash", "-eo", "pipefail", "-c"]

RUN apk add --no-cache ca-certificates tzdata curl

# atlasexec (used by the Go migration runner) calls the `atlas` binary at startup.
# This installs it to /usr/local/bin/atlas so it's in PATH.
RUN curl -sSf https://atlasgo.sh | ATLAS_VERSION=v1.1.0 sh

WORKDIR /app

COPY --from=builder /app/api .
COPY migrations/ ./migrations/

EXPOSE 8080

CMD ["./api"]
