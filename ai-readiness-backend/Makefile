.PHONY: dev build test lint clean docker-up docker-down tidy

# ── Configuration ────────────────────────────────────────────
BINARY     := bin/server
CMD        := ./cmd/server
BANK       := question-bank-v1.json

# ── Development ──────────────────────────────────────────────

## dev: run the server with hot-reload (requires air: go install github.com/air-verse/air@latest)
dev:
	air -c .air.toml

## run: build and run without hot-reload
run: build
	QUESTION_BANK_PATH=$(BANK) ./$(BINARY)

## build: compile the server binary
build:
	@mkdir -p bin
	go build -ldflags="-w -s" -o $(BINARY) $(CMD)

## tidy: tidy and verify go modules
tidy:
	go mod tidy
	go mod verify

# ── Testing ──────────────────────────────────────────────────

## test: run all unit tests with race detector
test:
	go test -race -v ./...

## test-cover: run tests and show coverage
test-cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## test-scoring: run only scoring engine tests
test-scoring:
	go test -race -v ./internal/scoring/...

# ── Linting ──────────────────────────────────────────────────

## lint: run golangci-lint (requires: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
lint:
	golangci-lint run ./...

# ── Docker ───────────────────────────────────────────────────

## docker-up: start MongoDB + API via docker-compose
docker-up:
	docker-compose up -d

## docker-up-debug: start with mongo-express UI on :8081
docker-up-debug:
	docker-compose --profile debug up -d

## docker-down: stop all containers
docker-down:
	docker-compose down

## docker-logs: tail API logs
docker-logs:
	docker-compose logs -f api

## docker-build: rebuild the API image
docker-build:
	docker-compose build api

# ── Utilities ────────────────────────────────────────────────

## clean: remove build artifacts
clean:
	rm -rf bin/ coverage.out coverage.html

## bank-check: validate question bank JSON
bank-check:
	@python3 -c "import json,sys; d=json.load(open('$(BANK)')); \
	  qs=len(d['questions']); domains=set(q['domain'] for q in d['questions']); \
	  print(f'✓ {qs} questions across {len(domains)} domains: {sorted(domains)}')"

## seed: copy question bank from frontend (if both repos are siblings)
seed:
	@if [ -f ../ai-readiness/public/question-bank-v1.json ]; then \
	  cp ../ai-readiness/public/question-bank-v1.json ./question-bank-v1.json; \
	  echo "✓ question-bank-v1.json copied from frontend"; \
	else \
	  echo "⚠ ../ai-readiness/public/question-bank-v1.json not found — copy it manually"; \
	fi

## help: list all targets
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'

## test-service: run service-layer unit tests
test-service:
	go test -race -v ./internal/service/...

## test-handlers: run handler unit tests
test-handlers:
	go test -race -v ./internal/handlers/...

## test-middleware: run middleware unit tests
test-middleware:
	go test -race -v ./internal/middleware/...

## test-config: run config unit tests
test-config:
	go test -race -v ./internal/config/...

## test-integration: run DB integration tests (requires MONGO_URI)
test-integration:
	@if [ -z "$(MONGO_URI)" ]; then \
	  echo "⚠  MONGO_URI is required. Example: make test-integration MONGO_URI=mongodb://localhost:27017"; \
	  exit 1; \
	fi
	MONGO_URI=$(MONGO_URI) MONGO_DB=ai_readiness_test \
	QUESTION_BANK_PATH=$(BANK) \
	go test -race -tags=integration -v -timeout=120s -count=1 \
	  ./internal/repository/... \
	  ./tests/e2e/...

## test-all: run ALL tests including integration (requires MONGO_URI)
test-all: test test-integration

## migrate: run DB migrations
migrate:
	MONGO_URI=$(MONGO_URI) MONGO_DB=$(or $(MONGO_DB),ai_readiness) \
	QUESTION_BANK_PATH=$(BANK) \
	go run scripts/migrate.go

## migrate-seed: run DB migrations and seed sample data
migrate-seed:
	MONGO_URI=$(MONGO_URI) MONGO_DB=$(or $(MONGO_DB),ai_readiness) \
	QUESTION_BANK_PATH=$(BANK) SEED=true \
	go run scripts/migrate.go

## docs: serve OpenAPI docs locally (requires npx)
docs:
	npx @redocly/cli preview-docs docs/openapi.yaml

## bench: run all benchmarks with memory stats
bench:
	go test -bench=. -benchmem -count=3 ./tests/bench/...

## bench-cpu: run benchmarks with CPU profiling
bench-cpu:
	go test -bench=BenchmarkScoring_Parallel -benchmem -cpuprofile=cpu.prof ./tests/bench/...
	go tool pprof -top cpu.prof

## test-ratelimit: run rate limiter tests
test-ratelimit:
	go test -race -v ./internal/ratelimit/...

## test-validator: run validator tests
test-validator:
	go test -race -v ./internal/validator/...

## test-audit: run audit log tests
test-audit:
	go test -race -v ./internal/audit/...

## test-metrics: run metrics tests
test-metrics:
	go test -race -v ./internal/metrics/...

## load-test: run k6 load test (requires k6 installed)
load-test:
	@which k6 || (echo "Install k6: https://k6.io/docs/getting-started/installation/"; exit 1)
	k6 run -e BASE_URL=http://localhost:8080 tests/load/assessment_load_test.js

## load-test-smoke: quick 10-second smoke test with k6
load-test-smoke:
	k6 run --duration 10s --vus 2 -e BASE_URL=http://localhost:8080 tests/load/assessment_load_test.js

## prod-up: start full production stack (API + MongoDB + Prometheus + Grafana)
prod-up:
	docker compose -f docker-compose.prod.yml up -d
	@echo "API:        http://localhost:8080"
	@echo "Grafana:    http://localhost:3001  (admin/admin)"
	@echo "Prometheus: http://localhost:9090"

## prod-down: stop production stack
prod-down:
	docker compose -f docker-compose.prod.yml down

## admin-stats: show database stats via admin CLI
admin-stats:
	MONGO_URI=$(or $(MONGO_URI),mongodb://localhost:27017) \
	QUESTION_BANK_PATH=$(BANK) go run ./cmd/admin stats

## admin-list: list recent assessments via admin CLI
admin-list:
	MONGO_URI=$(or $(MONGO_URI),mongodb://localhost:27017) \
	QUESTION_BANK_PATH=$(BANK) go run ./cmd/admin list

## validate-bank: validate question bank integrity
validate-bank:
	QUESTION_BANK_PATH=$(BANK) go run ./cmd/admin validate-bank
