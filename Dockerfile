# Step 1: Modules caching
FROM golang:1.26-alpine3.23 AS modules

COPY go.mod go.sum /modules/

WORKDIR /modules

RUN go mod download

# Step 2: Builder
FROM golang:1.26-alpine3.23 AS builder

ARG TARGETARCH

RUN apk add --no-cache ca-certificates make tzdata

COPY --from=modules /go/pkg /go/pkg
COPY . /app

WORKDIR /app

# Generate swagger docs + the wire DI graph. We deliberately do NOT run the full
# `make generate` here: that target also invokes `npx @scalar/cli` to upgrade the
# spec to OpenAPI 3.1, which needs Node (absent from this image) and is only used
# by the dev-only /docs route (SERVER_ENV=development). Production images skip it.
RUN go generate -skip="mockgen" ./... \
    && if [ ! -f permissions/permissions.json ]; then \
         echo '{"skip": true, "endpoints": []}' > permissions/permissions.json; \
       fi
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o engine ./cmd/app/main.go
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o worker ./cmd/worker/main.go
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o migrate ./cmd/migrate/main.go

# Step 3: Final
FROM scratch

# The HTTP read API (default entrypoint). Run with: CMD ["/app"].
COPY --from=builder /app/engine /app
# The ESPN/NHL ingestion worker (Celery-beat replacement). Run with: CMD ["/worker"].
COPY --from=builder /app/worker /worker
# One-shot DB migrator. Run with: ["/migrate", "up"].
COPY --from=builder /app/migrate /migrate
# Migration SQL is embedded via go:embed (see helper/), but keep the raw files
# available for `migrate` tooling and inspection.
COPY --from=builder /app/migrations /migrations
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

CMD ["/app"]
