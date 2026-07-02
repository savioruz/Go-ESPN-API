# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased] - 2026-06-08

- **Observability Infrastructure**
  - Replaced Elasticsearch and Kibana with Badger as the Jaeger storage backend
  - Jaeger now uses embedded Badger storage (self-contained, no external service needed)

- **Docker Compose Fixes**
  - Replaced `es_data` volume with `badger_data`
  - Added `prepare-badger-dir` init container to set Badger directory permissions for non-root Jaeger

- **Environment Configuration**
  - Parametrized all compose files (`docker-compose.yml`, `deployments/app.yml`, `deployments/services.yml`) with `${VAR:-DEFAULT}` env var interpolation
  - Added compose-level overrides to `.env.example` (Jaeger ports, Kafka cluster ID, Kafka UI auth, Unleash admin credentials)
  - Cleaned up duplicate env vars in `.env.example`

- **Kafka UI**
  - Fixed advertised listeners to use internal Docker hostname (`kafka:9092`) instead of `localhost`
  - Added form-based authentication with configurable admin credentials

## [Unreleased] - 2026-03-29

- **Kafka Configuration**
  - SASL/SCRAM authentication enabled by default
  - Requires `kafka_jaas.conf` file (mounted to `/etc/kafka/kafka_jaas.conf`)
  - **Important**: Never commit `kafka_jaas.conf` with credentials to version control

- **Observability Infrastructure**
  - Added Elasticsearch and Kibana to `deployments/services.yml` and `docker-compose.yml`
  - Jaeger now stores spans in Elasticsearch (instead of in-memory)

- **Docker Compose Fixes**
  - Added missing volume definitions (`postgres_data`, `redis_data`, `es_data`) to `deployments/services.yml`
  - Removed obsolete `version` attribute from compose files

## [Unreleased] - 2026-03-24

### Removed

- **Vercel Serverless Support**
  - Archived: `api/`, `vercel.json`
  - Tag created: `archive/legacy-vercel` for reference
  - Now uses trunk-based deployment (main branch only)

### Changed

- **CI/CD**
  - Removed `next` branch from CI triggers (trunk-based)
  - Removed auto-generate workflow (generate.yaml)
  - Developers must run `go generate` locally; generated files are not committed

- **Pre-commit Hook**
  - Now blocks committing: `*_gen.go`, `wire_gen.go`, `*mock.go`, `docs.go`, `swagger.json`, `swagger.yaml`, `permissions.json`

### Added

- **API Documentation**
  - Replaced swagger-ui with [Scalar](https://scalar.com/) for API docs
  - Generate OpenAPI 3.1 via `npx @scalar/cli document upgrade`
  - Added `@scalar/cli` requirement (install via `npm install -g @scalar/cli`)
  - Added `github.com/bdpiprava/scalar-go` for server-side Scalar rendering

- **External Auth Service Integration**
  - JWT validation via JWKS from external auth service (`AUTH_SERVICE_URL`)
  - JWT signing: EdDSA/Ed25519 with OKP key type
  - Issuer validation: accepts `AUTH_SERVICE_URL` or `https://auth.svrz.xyz`
  - Audience validation: accepts `APP_NAME` or `oil-local`
  - One-time JWKS fetch at startup using `sync.Once`

- **User Profile Domain**
  - New domain: `internal/domains/userprofile/`
  - Lazy sync: First API call creates/links profile from JWT claims (`sub` → `auth_user_id`)
  - Endpoints: `GET /api/users`, `PATCH /api/users`, `POST /api/users/presigned-url`

- **S3 Presigned URL Support**
  - New method: `GetPresignedUploadURL` in `infras/s3/s3.go`
  - Used for avatar uploads (max 1MB)

- **Feature Flags with Unleash**
  - Added `infras/unleash` package with `FeatureFlag` interface
  - Fail-open behavior when Unleash is unreachable
  - Config via `UNLEASH_URL`, `UNLEASH_APP_NAME`, `UNLEASH_INSTANCE_ID`, `UNLEASH_SECRET`, `UNLEASH_ENVIRONMENT`

### Removed

- **Local Authentication**
  - Removed `internal/domains/auth/` (service, model, dto)
  - Removed auth handler: `internal/handlers/auth/handler.go`
  - Removed cookie-based token storage (now uses Bearer token only)
  - Removed: `email_verifications` table, `password_resets` table

- **JWT Configuration**
  - Removed: `JWT_ACCESS_SECRET`, `JWT_REFRESH_SECRET`, `JWT_ACCESS_EXPIRE_MIN`, `JWT_REFRESH_EXPIRE_MIN`

- **Users Table**
  - Replaced with `user_profiles` table
  - Removed columns: `password`, `google_id`
  - Renamed domain: `internal/domains/user/` → `internal/domains/userprofile/`

### Changed

- **JWT-based Authentication**
  - Bearer token in Authorization header only
  - Token validation: EdDSA/Ed25519

- **Array-Based Validation Error Response**
  - Validation errors (422) return all field errors at once
  - Each error contains `field` and `message` (error key)
  - Example: `{"errors": [{"field": "title", "message": "validation.required"}]}`

- **Unified Structured Logging**
  - Both development and production use structured JSON logging
  - Enhanced OpenTelemetry spans with additional request/response attributes

## [Unreleased] - 2025-07-31

### Added

- Initial project setup
- Todo management with CRUD operations
- Gallery management with image uploads
- JWT-based authentication
- Role-based access control (RBAC)
- Rate limiting
- OpenAPI/Swagger documentation
