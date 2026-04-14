FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o proctura-backend ./cmd/main.go

# ── Final stage ───────────────────────────────────────────────────────────────
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata curl

# Install Atlas CLI
RUN curl -sSf https://atlasgo.sh | sh

WORKDIR /app

COPY --from=builder /app/proctura-backend .
COPY --from=builder /app/migrations ./migrations

EXPOSE 8080

CMD ["./proctura-backend"]
