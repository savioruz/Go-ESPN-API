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

# Generate swagger docs (docs/swagger.json) + the wire DI graph. The OpenAPI 3.1
# upgrade (docs/openapi.json) that the /docs UI reads is produced in the separate
# `docs` stage below, since @scalar/cli needs Node >=24 (absent from this image).
RUN go generate -skip="mockgen" ./... \
    && if [ ! -f permissions/permissions.json ]; then \
         echo '{"skip": true, "endpoints": []}' > permissions/permissions.json; \
       fi
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o engine ./cmd/app/main.go
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o worker ./cmd/worker/main.go
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o migrate ./cmd/migrate/main.go

# Step 2b: OpenAPI 3.1 spec for the /docs (Scalar) UI. @scalar/cli requires
# Node >=24, so upgrade docs/swagger.json -> docs/openapi.json in a dedicated Node
# stage (mirrors the Makefile `generate` scalar step) rather than adding Node to
# the Go builder.
FROM node:24-alpine AS docs

WORKDIR /docs

COPY --from=builder /app/docs/swagger.json ./swagger.json

RUN npx --yes @scalar/cli document upgrade swagger.json --output openapi.json

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
# OpenAPI 3.1 spec for the /docs UI (dev only), produced by the `docs` stage.
# The /docs handler reads docs/openapi.json relative to its CWD (/).
COPY --from=docs /docs/openapi.json /docs/openapi.json
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

CMD ["/app"]
