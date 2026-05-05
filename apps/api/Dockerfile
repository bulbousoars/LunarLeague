# syntax=docker/dockerfile:1.7

# ---------- dev image (used by docker-compose.dev.yml with air hot reload) ----------
FROM golang:1.23-alpine AS dev
RUN apk add --no-cache git ca-certificates make
WORKDIR /src
RUN go install github.com/air-verse/air@latest \
    && go install github.com/pressly/goose/v3/cmd/goose@latest
COPY go.mod go.sum* ./
RUN go mod download || true
EXPOSE 8000
CMD ["air", "-c", ".air.toml"]

# ---------- build stage ----------
FROM golang:1.23-alpine AS build
RUN apk add --no-cache git ca-certificates
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/lunarleague ./cmd/server

# ---------- runtime ----------
FROM alpine:3.20 AS runtime
RUN apk add --no-cache ca-certificates wget tzdata && \
    addgroup -S lunarleague && adduser -S -G lunarleague lunarleague
WORKDIR /app
COPY --from=build /out/lunarleague /app/bin/lunarleague
COPY db/migrations /app/db/migrations
USER lunarleague
EXPOSE 8000
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD wget -qO- http://localhost:8000/healthz || exit 1
ENTRYPOINT ["/app/bin/lunarleague"]
CMD ["serve"]
