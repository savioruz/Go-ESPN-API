BINARY=engine
COMPOSE=docker-compose
MIGRATION_PATH=migrations/postgres

.PHONY: help test coverage coverage.view coverage.check dev run worker ingest build clean docker.build docker.start docker.stop lint.prepare lint generate generate.mock migrate.up migrate.down migrate.step-up migrate.drop migrate.create migrate.order setup.githook domains

help: ## Display this help screen
	@if [ -z "$(shell which awk)" ]; then \
		echo "awk is required to display help"; \
		exit 1; \
	fi
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_.-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

test: generate generate.mock ## Run tests
	GO_ENV=test go test -v -cover -covermode=atomic ./...

coverage: generate generate.mock ## Run tests with coverage
	GO_ENV=test go test $(shell go list ./... |  grep -v  mocks) -coverprofile=tmp/cover.out  -coverpkg=./...

coverage.view: ## View coverage report
	go tool cover -html=tmp/cover.out

coverage.check: ## Check coverage
	go tool cover -func tmp/cover.out

dev: setup.githook generate ## Run the application in development mode
	go run github.com/air-verse/air

run: generate ## Run the application
	go run .

worker: generate ## Run the ESPN ingestion worker
	go run cmd/worker/main.go

ingest: ## Run every enabled ingest job once, then exit (back-fill / cutover). Respects SERVICES_ESPN_ENABLED / SERVICES_NHL_ENABLED and INGEST_LEAGUES.
	go run cmd/worker/main.go --once

build: generate ## Build the application
	go build -ldflags '-s -w' -o ${BINARY} ./cmd/app/main.go

clean: ## Clean up the project
	@if [ -f ${BINARY} ] ; then rm ${BINARY} ; fi
	@find . -name *mock* -delete
	@rm -rf docs/ ./di/wire_gen.go tmp/cover.out

docker.build: ## Build the docker image
	$(COMPOSE) build

docker.start: ## Start the docker container
	$(COMPOSE) compose up -d

docker.stop: ## Stop the docker container
	$(COMPOSE) compose down

lint.prepare: ## Prepare the environment for linting
	@echo "Installing golangci-lint"
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin latest

lint: generate generate.mock ## Run linters
	golangci-lint run ./... --fix

generate: ## Generate code
	go generate -skip="mockgen" ./...
	@if [ ! -f permissions/permissions.json ]; then \
		echo '{"skip": true, "endpoints": []}' > permissions/permissions.json; \
	fi
	@echo "Upgrading to OpenAPI 3.1 (optional; @scalar/cli needs Node >=24)..."
	@npx @scalar/cli document upgrade docs/swagger.json --output docs/openapi.json \
		|| echo "  skipped OpenAPI 3.1 upgrade (@scalar/cli unavailable / Node <24) — docs/openapi.json left unchanged; only the dev /docs UI needs it"

generate.mock: ## Generate mock code
	go generate -run="mockgen" ./...

migrate.up: ## Run database migrations up
	go run cmd/migrate/main.go up

migrate.down: ## Run database migrations down
	go run cmd/migrate/main.go down

migrate.step-up: ## Run database migrations step up
	go run cmd/migrate/main.go step-up

migrate.drop: ## Drop all database migrations
	go run cmd/migrate/main.go drop

migrate.create: ## Create a new migration file. Usage: make migrate.create name=<migration name>
	@if [ -z "$(name)" ]; then \
		echo "Please set the name variable"; \
		echo "Example: make migrate.create name=add_users_table"; \
		exit 1; \
	fi
	go run github.com/golang-migrate/migrate/v4/cmd/migrate@latest create -ext sql -dir $(MIGRATION_PATH) -seq $(name)

migrate.order: ## Order migrations. Usage: make migrate-order table=<table name> n=<order number>
	@if [ -z "$(table)" ] || [ -z "$(n)" ]; then \
		echo "Please set the table and n variables"; \
		echo "Example: make migrate.order table=users n=2"; \
		exit 1; \
	fi
	cmd/migrate/order.sh $(table) $(n)

setup.githook: ## Setup git hooks
	git config core.hooksPath .githooks

domains: ## Create simple empty domains file. Usage: make domains name=<domains name>
	@if [ -z "$(name)" ]; then \
    	echo "Please set the name variable"; \
    	echo "Example: make domains name=service"; \
    	exit 1; \
    fi
	@mkdir -p ./internal/domains/$(name)/model/dto
	@echo "package model" > ./internal/domains/$(name)/model/model.go
	@echo "package dto" > ./internal/domains/$(name)/model/dto/dto.go
	@mkdir -p ./internal/domains/$(name)/repository
	@echo "package repository" > ./internal/domains/$(name)/repository/repository.go
	@mkdir -p ./internal/domains/$(name)/service
	@echo "package service" > ./internal/domains/$(name)/service/service.go
	@echo "Domain $(name) created successfully"
