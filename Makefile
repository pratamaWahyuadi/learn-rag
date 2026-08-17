.PHONY: deps db-up db-down migrate-up migrate-down sqlc build test test-integration

# Install / tidy project dependencies
deps:
	go mod tidy

# Start local Postgres + pgvector
db-up:
	docker compose up -d db

# Stop local Postgres + pgvector
db-down:
	docker compose down

# Apply all pending migrations via goose
migrate-up:
	go run github.com/pressly/goose/v3/cmd/goose -dir migrations postgres "$$DATABASE_URL" up

# Roll back the last migration via goose
migrate-down:
	go run github.com/pressly/goose/v3/cmd/goose -dir migrations postgres "$$DATABASE_URL" down

# Generate type-safe query code with sqlc
sqlc:
	sqlc generate

# Build both server and worker binaries
build:
	go build -o bin/rag-server ./cmd/server
	go build -o bin/rag-worker ./cmd/worker

# Run the Go test suite (no external dependencies)
test:
	go test ./...

# Run integration tests that need a reachable Postgres; requires DATABASE_URL
test-integration:
	go test -tags integration ./...
