---
name: rules
description: Project-specific rules and conventions for the Oil Go REST API backend. Load this skill whenever writing, modifying, or reviewing any code in this repository to ensure consistency with established architecture, naming, and patterns.
license: MIT
metadata:
  author: project
  version: "1.0.0"
  domain: project-rules
  triggers: oil, Go API, new domain, new handler, new service, new repository, migration, middleware, chi, wire, repository, service, handler, postgres, redis, kafka, s3, jwt, otel, permissions
  role: enforcer
  scope: implementation
  output-format: code
---

# Oil — Project Rules & Conventions

These rules govern all code written in the Oil repository. They reflect the established architecture, naming conventions, and patterns found throughout the codebase. Follow them for every change — new features, bug fixes, and refactors alike.

---

## 1. Architecture: Three-Layer Domain Model

Every business domain **must** follow the exact same three-layer structure, no exceptions.

```
internal/domains/<name>/
  model/
    model.go          ← DB model struct + table/field constants
    dto/
      dto.go          ← Request/response DTOs, ToModel/FromModel methods
  repository/
    repository.go     ← Repository interface + repositoryImpl (embeds shared generic repo)
  service/
    service.go        ← Service interface + serviceImpl
    service_test.go   ← Unit tests using mocks

internal/handlers/<name>/
  handler.go          ← HTTP handler struct, Router(), and handler methods
  handler_test.go     ← Handler tests
```

- **Handlers** depend on **service interfaces** only — never on repository or infrastructure directly.
- **Services** depend on **repository interfaces** only — never on `*postgres.Connection` directly.
- **Repositories** depend on `*postgres.Connection` and `otel.Otel` — nothing from domain layers above.
- Cross-domain calls go service-to-service, never handler-to-handler or repository-to-repository.

**To scaffold a new domain:**
```bash
make domains name=<domainname>
```
This creates the empty directory structure. Fill in the files following existing domains (todo, gallery, auth).

---

## 2. Model Conventions

### DB Model (`model/model.go`)
```go
package model

import "oil/shared/model"

const (
    TableName  = "tablename"    // exact SQL table name
    EntityName = "entityname"   // singular lowercase, used for OTel spans and error messages

    FieldID    = "id"
    FieldFoo   = "foo"          // one const per queryable column
)

type MyEntity struct {
    ID  string `db:"id"`
    Foo string `db:"foo"`
    model.Metadata              // always embedded — provides created_at, modified_at, created_by, modified_by
}
```

- Always embed `model.Metadata` from `oil/shared/model`.
- Use `db:` struct tags that exactly match SQL column names.
- Declare a `const` for every field that will be used in filters, sorting, or query building.
- Do **not** put JSON tags on DB models — those belong on DTOs.

### Join Field Tag Rules (`column`, `db`, `table`)

When a field comes from a JOINed table, you must use all three tags:

```go
FieldName string `column:"original_column" db:"alias_name" table:"source_table"`
```

- `column:` is actual column name in the joined table  
- `table:` is source table name  
- `db:` is alias used in SQL query (MUST match SELECT alias)

### Join Queries

When a model needs to join with other tables, implement a `GetJoinQuery()` method that returns the JOIN clause:

```go
func (a AIChatContent) GetJoinQuery() string {
	return `
		INNER JOIN ai_chats ON ai_chat_contents.chat_id = ai_chats.id
	`
}
```

- Use this pattern for any model that requires table joins in queries.
- AIChatContent is only an example. Replace it with your actual model struct name when implementing.

---

## 3. DTO Conventions

### Request/Response DTOs (`model/dto/dto.go`)

```go
package dto

import (
	"oil/internal/domains/myentity/model"
	gModel "oil/shared/model"
	gDto "oil/shared/dto"
	"oil/shared/timezone"
	"time"
)

// CreateFooRequest is the request body for creating a Foo.
type CreateFooRequest struct {
	Name string `json:"name" validate:"required,min=1,max=100"`
}

// ToModel converts the DTO to a DB model.
func (r *CreateFooRequest) ToModel(createdBy string) model.Foo {
	now := timezone.NowUTC()

	return model.Foo{
		ID:   uuid.NewString(),
		Name: r.Name,
		Metadata: gModel.Metadata{
			CreatedBy:  createdBy,
			ModifiedBy: createdBy,
			CreatedAt:  now,
            ModifiedAt: now,
		},
	}
}

// FooResponse is the API response shape for a single Foo.
type FooResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	gDto.Metadata
}

// FromModel populates FooResponse from a DB model.
func (r *FooResponse) FromModel(m model.Foo) {
	r.ID = m.ID
	r.Name = m.Name
	r.Metadata.FromModel(m.Metadata)
}
```

Rules:
- All request fields must carry `validate:` tags (using `go-playground/validator` syntax).
- Every request DTO must have a `ToModel(createdBy string) model.T` method.
- Every response DTO must have a `FromModel(m model.T)` method.
- Never expose raw DB models directly in HTTP responses.
- Update DTOs follow the same pattern; fields should be pointer types or omitempty-marked so zero values are distinguishable from "not provided".

---

## 4. Repository Conventions

### Repository interface + implementation
```go
package repository

//go:generate go run go.uber.org/mock/mockgen -source=./repository.go -destination=../mocks/repository_mock.go -package=mocks

import (
    "context"
    "oil/infras/otel"
    "oil/infras/postgres"
    "oil/internal/domains/myentity/model"
    gDto "oil/shared/dto"
    gRepo "oil/shared/repository"
)

// MyEntity defines the repository contract for the myentity domain.
type MyEntity interface {
    Insert(ctx context.Context, m model.MyEntity) error
    Get(ctx context.Context, filter gDto.FilterGroup, columns ...string) (model.MyEntity, error)
    GetAll(ctx context.Context, params gDto.QueryParams, filter gDto.FilterGroup, columns ...string) ([]model.MyEntity, error)
    Exist(ctx context.Context, filter gDto.FilterGroup) (bool, error)
    Count(ctx context.Context, filter gDto.FilterGroup) (int, error)
    Update(ctx context.Context, req map[string]any, filter gDto.FilterGroup) error
    Delete(ctx context.Context, filter gDto.FilterGroup) error
}

type repositoryImpl struct {
    gRepo.Repository[model.MyEntity]
    db   *postgres.Connection
    otel otel.Otel
}

func New(db *postgres.Connection, otel otel.Otel) MyEntity {
    return &repositoryImpl{
        Repository: gRepo.NewRepository[model.MyEntity](
            model.EntityName, model.TableName, model.FieldID, db, otel,
        ),
        db:   db,
        otel: otel,
    }
}
```

Rules:
- Always include the `//go:generate mockgen` directive at the top of `repository.go`.
- The repository interface name must match the domain name (PascalCase singular).
- Only add methods beyond the generic set (Insert/Get/GetAll/Exist/Count/Update/Delete) when custom SQL is genuinely required.
- Custom queries must use `db.Read` for SELECTs and `db.Write` for mutations.
- Never write raw SQL with positional `$1` placeholders — use named parameters (`:fieldname`) and `NamedExecContext` / `PrepareNamedContext` consistently with the generic repo pattern.
- Transactional variants (InsertTx, UpdateTx, DeleteTx) are available from the embedded generic repo — use them when multiple writes must be atomic.

---

## 5. Service Conventions

```go
package service

import (
    "context"
    "fmt"
    "oil/config"
    "oil/infras/otel"
    "oil/internal/domains/myentity/model"
    "oil/internal/domains/myentity/model/dto"
    "oil/internal/domains/myentity/repository"
    "oil/shared"
    "oil/shared/cache"
    "oil/shared/constant"
    "oil/shared/errkey"
    "oil/shared/failure"
)

// Cache key prefixes — keep them unique across domains.
const (
    cacheGetFoo    = "foo:get"
    cacheGetAllFoo = "foo:gets"
    cacheCountFoo  = "foo:count"
)

// MyEntity defines the service contract.
type MyEntity interface {
    Create(ctx context.Context, req dto.CreateFooRequest) error
    Get(ctx context.Context, id string) (dto.FooResponse, error)
    // ...
}

type serviceImpl struct {
    repo  repository.MyEntity
    cfg   *config.Config
    cache cache.RedisCache
    otel  otel.Otel
}

func New(repo repository.MyEntity, cfg *config.Config, cache cache.RedisCache, otel otel.Otel) MyEntity {
    return &serviceImpl{repo: repo, cfg: cfg, cache: cache, otel: otel}
}
```

**Every service method must:**
1. Create an OTel scope immediately:
   ```go
   func (s *serviceImpl) Create(ctx context.Context, req dto.CreateRequest) (err error) {
       ctx, scope := s.otel.NewScope(ctx, constant.OtelServiceScopeName, constant.OtelServiceScopeName+".Create")
       defer scope.End()
       defer scope.TraceIfError(err)
       // ...
   }
   ```
2. Use named return `(err error)` when using `defer scope.TraceIfError(err)`.
3. Return `failure.*WithKey(errkey.Err*, message)` typed errors — never raw `fmt.Errorf` to the caller.
4. For read operations: check cache first, hit DB on miss, save to cache asynchronously in a goroutine using `context.WithoutCancel(ctx)`.
5. For write operations: invalidate affected cache keys asynchronously in a goroutine.
6. Extract user identity from context via `ctx.Value(constant.ContextKeyUserID).(string)` — never accept user ID as a parameter.

---

## 6. Handler Conventions

```go
package myentity

import (
    "net/http"
    "oil/infras/otel"
    "oil/internal/domains/myentity/model/dto"
    "oil/internal/domains/myentity/service"
    "oil/shared/constant"
    "oil/shared/validator"
    "oil/transport/http/response"

    "github.com/go-chi/chi/v5"
    "github.com/rs/zerolog/log"
)

type Handler struct {
    service service.MyEntity
    otel    otel.Otel
}

func New(service service.MyEntity, otel otel.Otel) Handler {
    return Handler{service: service, otel: otel}
}

// Router registers all routes for this domain.
func (h *Handler) Router(r chi.Router) {
    r.Route("/myentities", func(r chi.Router) {
        r.Post("/", h.Create)
        r.Get("/", h.GetAll)
        r.Get("/{id}", h.GetByID)
        r.Patch("/{id}", h.Update)
        r.Delete("/{id}", h.Delete)
    })
}
```

**Every handler method must:**
1. Create an OTel scope immediately.
2. Decode and validate input via `validator.Validate(r.Body, &req)`.
3. On any error, call `scope.TraceError(err)` then `response.WithError(w, err)` and return.
4. Use `chi.URLParam(r, constant.RequestParamID)` for path parameters.
5. Have complete Swagger annotations (`@Summary`, `@Description`, `@Tags`, `@Accept`, `@Produce`, `@Param`, `@Success`, `@Failure`, `@Router`, and `@Security BearerAuth` where applicable).
6. Use `response.WithJSON`, `response.WithMessage`, or `response.WithError` exclusively — never write directly to `http.ResponseWriter`.
7. If `response.WithMessage` is used, then must use the key. This allows clients to internationalize messages based on the key instead of hardcoded English strings.

---

## 7. Dependency Injection (Wire)

When adding a new domain, register it in `di/wire.go`:

```go
// 1. Add a domain provider set
var myEntityDomain = wire.NewSet(
    myEntityRepository.New,
    myEntityService.New,
)

// 2. Add the handler
var routing = wire.NewSet(
    wire.Struct(new(router.DomainHandlers), "*"),
    // ... existing handlers ...
    myEntityHandler.New,
    router.New,
)

// 3. Add to the domains set
var domains = wire.NewSet(
    // ... existing domains ...
    myEntityDomain,
)
```

Then add the handler field to `router.DomainHandlers` in `transport/http/router/router.go` and register its routes in `SetupRoutes`.

After editing `di/wire.go`, always regenerate:
```bash
make generate
```

---

## 8. Routing & Permissions

When adding new endpoints:

1. Register them in the handler's `Router()` method.
2. Add entries to `permissions/permissions.json`:
```json
{
  "path": "/api/myentities",
  "method": "POST",
  "permissions": ["admin", "superadmin"],
  "skip": false
}
```

Permission rules:
- `"skip": true` = no authentication required (public endpoint).
- `"skip": false` + empty `permissions` = authenticated but any role allowed.
- `"skip": false` + non-empty `permissions` = authenticated + role must be in the list.
- Valid role values: `"user"`, `"admin"`.
- If an endpoint is private, no need to specify it in `permissions.json` — only public endpoints must be listed with `"skip": true`.

---

## 9. Error Handling

Always use the typed error constructors from `oil/shared/failure`:

| Situation | Function |
|---|---|
| Resource not found | `failure.NotFoundWithKey(errkey.ErrXxx, "message")` |
| Bad request / invalid input | `failure.BadRequestWithKey(errkey.ErrXxx, "message")` |
| Unauthorized (no/bad token) | `failure.UnauthorizedWithKey(errkey.ErrXxx, "message")` |
| Forbidden (wrong role) | `failure.ForbiddenWithKey(errkey.ErrXxx, "message")` |
| Duplicate resource | `failure.ConflictWithKey(errkey.ErrXxx, "message")` |
| DB / internal errors | `failure.InternalErrorWithKey(errkey.ErrXxx, "message")` |

Define new error keys in `shared/errkey/` — do not use raw string error messages as keys.

Never return `fmt.Errorf(...)` from a service to a handler — that will produce an untyped 500. Always wrap with a `failure.*` constructor before returning up the call stack.

---

## 10. Database Migrations

To create a new migration:
```bash
make migrate.create name=create_myentity_table
```

Migration file conventions:
- All tables must include `id VARCHAR(36) PRIMARY KEY` (UUID string).
- All tables must include the four audit columns: `created_at TIMESTAMPTZ DEFAULT NOW()`, `modified_at TIMESTAMPTZ DEFAULT NOW()`, `created_by VARCHAR(36) NOT NULL`, `modified_by VARCHAR(36) NOT NULL`.
- Use `BEGIN; ... COMMIT;` wrapping when the migration creates multiple tables or indexes.
- Always write the corresponding `.down.sql` that exactly reverses the `.up.sql`.
- Add indexes for any column used as a filter or join key.

---

## 11. Caching Rules

- Cache keys are always built using `shared.BuildCacheKey(prefix, ...parts)` or `shared.BuildCacheKeyWithQuery(prefix, queryParams, filter)`.
- Cache key prefixes follow the pattern `<domain>:<operation>` (e.g., `todo:get`, `todo:gets`, `todo:count`).
- TTL comes from `s.cfg.Cache.TTL` — never hardcode a TTL value.
- Cache saves and invalidations must be done in goroutines so they do not block the HTTP response:
  ```go
  go func() {
      c := context.WithoutCancel(ctx)
      s.cache.Save(c, key, value, s.cfg.Cache.TTL)
  }()
  ```
- On mutation (create/update/delete), invalidate all related list/count caches using `shared.InvalidateCaches(ctx, s.cache, cacheKeyPrefix)`.

---

## 12. OpenTelemetry Tracing

OTel scope names follow these constants from `oil/shared/constant`:
- `constant.OtelHandlerScopeName` — for handlers
- `constant.OtelServiceScopeName` — for services
- `constant.OtelRepositoryScopeName` — for repositories (handled automatically by generic repo)

Span names follow the pattern: `<ScopeName>.<MethodName>` (e.g., `"service.Create"`, `"handler.CreateTodo"`).

Always use `defer scope.End()` immediately after `NewScope`. For named-return functions, also use `defer scope.TraceIfError(err)`.

---

## 13. Testing Requirements

- Every service must have a `service_test.go` with mocked repository and cache.
- Every handler must have a `handler_test.go` with mocked service.
- Use `go.uber.org/mock/mockgen` — mocks are auto-generated from interfaces via `make generate.mock`.
- Use `github.com/stretchr/testify/assert` and `testify/require` for assertions.
- Use `github.com/DATA-DOG/go-sqlmock` for any tests that touch raw SQL.
- Tests run with: `make test` (which also runs `make generate` and `make generate.mock` first).
- Coverage report: `make coverage` then `make coverage.view`.

Mock generation directive (must be present in every repository and infra interface file):
```go
//go:generate go run go.uber.org/mock/mockgen -source=./repository.go -destination=../mocks/repository_mock.go -package=mocks
```

---

## 14. Logging

Use `github.com/rs/zerolog/log` exclusively. Never use `fmt.Println` or `log` from the standard library in business logic.

Logging patterns:
```go
log.Error().Err(err).Msg("failed to create foo")
log.Warn().Str("email", email).Msg("login attempt with wrong password")
log.Info().Str("cacheKey", key).Msg("cache hit for foo")
```

Always include structured fields (`.Err()`, `.Str()`, `.Int()`, etc.) — never interpolate values into the message string.

---

## 15. Code Generation

The project uses two code generators — always run `make generate` after:
- Modifying `di/wire.go` (regenerates `di/wire_gen.go` via Wire)
- Adding or modifying Swagger annotations (regenerates `docs/` via swag)
- Modifying repository or infra interfaces with `//go:generate mockgen` directives

```bash
make generate       # runs swag init + wire (skips mockgen)
make generate.mock  # runs mockgen only
```

Both are run automatically by `make test` and `make build`.

---

## 16. Configuration

- All new configuration values must be added to the `Config` struct in `config/config.go` with proper `envconfig:` tags.
- All new env vars must be documented in `.env.example` with a sensible default value.
- Never hardcode secrets, hostnames, timeouts, or feature flags — always use config.
- Config is a singleton — access it via `config.Get()` and pass the `*config.Config` pointer into constructors (Wire will inject it automatically).

---

## 17. Kafka

Kafka is wired but **commented out in `di/wire.go`** by default (`// kafka.New`). To enable:
1. Uncomment `kafka.New` in the `infrastructures` provider set in `di/wire.go`.
2. Inject the `kafka.Client` interface into the service(s) that need it.
3. Set `KAFKA_SASL_USERNAME`, `KAFKA_SASL_PASSWORD`, and `KAFKA_BROKERS` in `.env`.

All Kafka operations use the `kafka.Client` interface — never import `kafkaGo` directly in domain code.

---

## 18. S3 / File Storage

- The `infras/s3` package provides the S3 client. Inject it into services that handle file uploads.
- The gallery domain is the reference implementation — consult `internal/domains/gallery/` for patterns.
- Use `shared.GenerateUniqueFilename(originalName)` to generate collision-safe filenames before upload.
- Store only the S3 key or public URL in the database — never the full presigned URL (those expire).

---

## 19. Graceful Shutdown

The HTTP server handles `SIGTERM` with a two-phase shutdown (grace period → cleanup period). During cleanup, new requests receive a `503 Service Unavailable`. These periods are configurable via `SERVER_SHUTDOWN_GRACE_PERIOD_SECONDS` and `SERVER_SHUTDOWN_CLEANUP_PERIOD_SECONDS`. In development mode (`SERVER_ENV=development`) shutdown is immediate.

---
