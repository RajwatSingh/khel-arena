# Khel Arena — backend
#
# `make check` is what CI runs and what should pass before you push.

DB_URL      ?= postgres://khel:khel@localhost:5432/khel_arena?sslmode=disable
TEST_DB_URL ?= postgres://khel:khel@localhost:5432/khel_arena_test?sslmode=disable
PG_CONTAINER = khel-pg
PG_IMAGE     = postgres:17-alpine

.PHONY: help
help: ## List the available targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'

# ── Build ───────────────────────────────────────────────────────────────────

.PHONY: build
build: ## Compile every binary into ./bin
	go build -o bin/ ./cmd/...

.PHONY: tidy
tidy: ## Sync go.mod and go.sum
	go mod tidy

# ── Test ────────────────────────────────────────────────────────────────────

.PHONY: test
test: ## Run unit tests (no database needed)
	go test ./... -count=1

.PHONY: test-integration
test-integration: ## Run every test, including those needing a database
	TEST_DATABASE_URL="$(TEST_DB_URL)" go test ./... -count=1

.PHONY: test-race
test-race: ## Run every test under the race detector
	TEST_DATABASE_URL="$(TEST_DB_URL)" go test ./... -count=1 -race

.PHONY: cover
cover: ## Report test coverage per package
	TEST_DATABASE_URL="$(TEST_DB_URL)" go test ./... -count=1 -cover

.PHONY: check
check: tidy-check vet test-race ## Everything CI runs

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: tidy-check
tidy-check: ## Fail if go.mod or go.sum is out of date
	@go mod tidy
	@git diff --exit-code go.mod go.sum || \
		(echo "go.mod/go.sum are out of date — run 'make tidy' and commit"; exit 1)

# ── Database ────────────────────────────────────────────────────────────────

.PHONY: migrate
migrate: ## Apply pending migrations to $(DB_URL)
	DATABASE_URL="$(DB_URL)" JWT_SECRET="$${JWT_SECRET:-placeholder-secret-for-migrations-only}" \
		go run ./cmd/migrate

.PHONY: db-up
db-up: ## Start a local Postgres in Docker and create both databases
	docker run -d --name $(PG_CONTAINER) \
		-e POSTGRES_USER=khel -e POSTGRES_PASSWORD=khel -e POSTGRES_DB=khel_arena \
		-p 5432:5432 $(PG_IMAGE)
	@echo "waiting for postgres..."
	@until docker exec $(PG_CONTAINER) pg_isready -U khel >/dev/null 2>&1; do sleep 1; done
	@docker exec $(PG_CONTAINER) createdb -U khel khel_arena_test
	@echo "postgres ready on :5432 (databases: khel_arena, khel_arena_test)"

.PHONY: db-down
db-down: ## Stop and remove the local Postgres container
	-docker rm -f $(PG_CONTAINER)

.PHONY: db-reset
db-reset: ## Drop and recreate both databases, then migrate
	docker exec $(PG_CONTAINER) psql -U khel -d postgres -q \
		-c 'drop database if exists khel_arena with (force)' \
		-c 'drop database if exists khel_arena_test with (force)' \
		-c 'create database khel_arena' \
		-c 'create database khel_arena_test'
	$(MAKE) migrate

.PHONY: psql
psql: ## Open a psql shell on the development database
	docker exec -it $(PG_CONTAINER) psql -U khel -d khel_arena
