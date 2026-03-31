# Developer Runbook

Operational reference for the AI Readiness Assessment backend.

---

## Table of Contents

1. [Architecture Overview](#architecture)
2. [Local Development](#local-development)
3. [Running Tests](#running-tests)
4. [Benchmarks](#benchmarks)
5. [Admin CLI](#admin-cli)
6. [Monitoring & Metrics](#monitoring--metrics)
7. [Audit Log](#audit-log)
8. [API Quick Reference](#api-quick-reference)
9. [Database Operations](#database-operations)
10. [Troubleshooting](#troubleshooting)
11. [Adding a New Feature](#adding-a-new-feature)

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        HTTP Layer                           │
│  chi router → CORS → RateLimit → Metrics → RequestID       │
│               Logger → Recoverer → Timeout                  │
└───────────────────┬─────────────────────────────────────────┘
                    │
┌───────────────────▼─────────────────────────────────────────┐
│                     Handlers (8 routes)                     │
│  validators ─── audit logger ─── metrics counters          │
└───────────────────┬─────────────────────────────────────────┘
                    │
┌───────────────────▼─────────────────────────────────────────┐
│                  AssessmentService                          │
│  business logic · question bank · answer validation        │
│                  ┌──────────────┐                           │
│                  │ ScoringEngine│ (pure functions, no I/O) │
│                  └──────────────┘                           │
└───────────────────┬─────────────────────────────────────────┘
                    │
┌───────────────────▼─────────────────────────────────────────┐
│              AssessmentRepository                           │
│  MongoDB · field-level $set merges · idempotent indexes    │
└─────────────────────────────────────────────────────────────┘
```

**Key design decisions:**

- **Scoring is pure**: `scoring.Compute()` has no I/O, no side effects — trivially testable and benchmarkable
- **Interface-driven handlers**: `AssessmentServicer` interface lets tests inject in-memory mocks with zero database setup
- **Field-level answer merges**: `PUT /answers` uses MongoDB `$set` per field, so concurrent domain saves don't stomp each other
- **Validator layer**: All input validation is centralised in `internal/validator` and returns structured `[]FieldError` responses

---

## Local Development

### Prerequisites by approach

| Approach | Requires |
|----------|----------|
| HTML demo (instant) | Just a browser |
| Next.js frontend only | Node.js 18+ |
| Full stack | Go 1.22+, MongoDB 7, Node.js 18+ |
| Docker (easiest full stack) | Docker Desktop |

### Option A — Docker Compose (recommended)

```bash
# Start MongoDB + API server
make docker-up

# Or with Mongo Express UI at http://localhost:8081
make docker-up-debug

# Tail logs
make docker-logs

# Stop everything
make docker-down
```

### Option B — Native Go

```bash
# 1. Copy and configure environment
cp .env.example .env
#    At minimum: MONGO_URI=mongodb://localhost:27017

# 2. Ensure MongoDB is running locally
#    Mac:   brew services start mongodb-community
#    Linux: sudo systemctl start mongod

# 3. Run migrations (creates indexes, optionally seeds sample data)
make migrate MONGO_URI=mongodb://localhost:27017

# 4. Start with hot-reload (requires: go install github.com/air-verse/air@latest)
make dev

# 4. Or start without hot-reload
make run

# Verify it's up
curl http://localhost:8080/health
```

### Option C — Run just the frontend

```bash
cd ../ai-readiness          # next.js project
npm install
npm run dev
# Open http://localhost:3000
# Works fully without backend — scoring is client-side
```

---

## Running Tests

```bash
# All unit tests (no MongoDB required)
make test

# With HTML coverage report
make test-cover

# Individual packages
make test-scoring     # scoring engine only
make test-service     # service layer with mock repo
make test-handlers    # HTTP handlers with mock service
make test-middleware  # middleware
make test-config      # config loading/validation

# Integration tests (requires MONGO_URI)
make test-integration MONGO_URI=mongodb://localhost:27017

# Everything
make test-all MONGO_URI=mongodb://localhost:27017
```

### Test anatomy

```
internal/
├── config/config_test.go              ← env loading, splitTrim edge cases
├── validator/validator_test.go        ← 20 cases: scores, IDs, pagination, comments
├── scoring/engine_test.go             ← normalization, maturity thresholds, all 4 risk flags
├── audit/audit_test.go                ← field presence, request ID propagation
├── metrics/metrics_test.go            ← counter increments, histogram sampling, /metrics endpoint
├── middleware/middleware_test.go      ← request ID, logger, panic recovery
├── service/assessment_test.go        ← full lifecycle with mock repo
├── handlers/assessment_test.go       ← 12 HTTP scenarios with mock service
└── repository/
    └── assessment_integration_test.go ← MongoDB field-merge, pagination, timestamps (needs DB)
tests/
├── e2e/api_test.go                    ← full HTTP + real MongoDB (needs DB)
└── bench/scoring_bench_test.go        ← throughput benchmarks
```

---

## Benchmarks

```bash
# Run all benchmarks
go test -bench=. -benchmem ./tests/bench/...

# Run only scoring benchmarks
go test -bench=BenchmarkScoring -benchmem ./tests/bench/...

# Run with CPU profiling
go test -bench=BenchmarkScoring_Parallel -benchmem -cpuprofile=cpu.prof ./tests/bench/...
go tool pprof cpu.prof

# Run with memory profiling
go test -bench=BenchmarkScoring_MixedAnswers -benchmem -memprofile=mem.prof ./tests/bench/...
go tool pprof mem.prof
```

**Expected throughput** (M1 MacBook Pro, reference):

| Benchmark | ops/sec | ns/op | allocs/op |
|-----------|---------|-------|-----------|
| AllAnswered | ~250,000 | ~4,000 | ~80 |
| MixedAnswers | ~200,000 | ~5,000 | ~95 |
| Parallel (8 cores) | ~1,500,000 | ~650 | ~80 |
| NoAnswers | ~800,000 | ~1,250 | ~45 |

---

## Admin CLI

```bash
# Build the admin binary
go build -o bin/admin ./cmd/admin

# Or run directly
MONGO_URI=mongodb://localhost:27017 go run ./cmd/admin <command>
```

### Commands

```bash
# Database statistics
admin stats

# List 20 most recent assessments
admin list

# Inspect a single assessment in full detail
admin get 6643f1a2c3d4e5f6a7b8c9d0

# Re-run the scoring engine (e.g. after question bank update)
admin recompute 6643f1a2c3d4e5f6a7b8c9d0

# Export result as JSON (pipe to file)
admin export 6643f1a2c3d4e5f6a7b8c9d0 > result.json

# Delete permanently (prompts for confirmation)
admin delete 6643f1a2c3d4e5f6a7b8c9d0

# Validate question bank integrity
admin validate-bank
QUESTION_BANK_PATH=/path/to/custom-bank.json admin validate-bank
```

---

## Monitoring & Metrics

### Prometheus endpoint

```
GET /metrics
```

Scrape this endpoint with Prometheus. All metrics are prefixed `aireadiness_`.

### Key metrics to alert on

| Metric | Alert condition | Meaning |
|--------|-----------------|---------|
| `aireadiness_http_requests_total{status="5xx"}` | rate > 0.01/s | Server errors |
| `aireadiness_http_request_duration_seconds{p99}` | > 2s | Latency SLO |
| `aireadiness_mongo_operation_duration_seconds{p95}` | > 500ms | DB slowness |
| `aireadiness_pdf_exports_total{outcome="failure"}` | rate > 0 | PDF errors |
| `up{job="ai-readiness"}` | == 0 | Service down |

### Grafana dashboard queries

```promql
# Request rate by status class
sum by (status) (rate(aireadiness_http_requests_total[5m]))

# p99 latency per route
histogram_quantile(0.99,
  sum by (route, le) (rate(aireadiness_http_request_duration_seconds_bucket[5m]))
)

# Assessments computed per minute
rate(aireadiness_assessments_computed_total[1m]) * 60

# Maturity distribution (last 24h)
increase(aireadiness_assessments_computed_total[24h])

# Scoring engine p95 latency
histogram_quantile(0.95, rate(aireadiness_scoring_duration_seconds_bucket[5m]))

# Risk flag rates
rate(aireadiness_risk_flags_total[1h])
```

### Prometheus scrape config

```yaml
# prometheus.yml
scrape_configs:
  - job_name: ai-readiness
    static_configs:
      - targets: ['localhost:8080']
    scrape_interval: 15s
    metrics_path: /metrics
```

---

## Audit Log

Every state-changing operation emits a structured JSON audit entry via the `audit` named logger.

### Event types

| Event | Trigger |
|-------|---------|
| `assessment.created` | POST /api/assessment |
| `assessment.answers_saved` | PUT /api/assessment/{id}/answers |
| `assessment.computed` | POST /api/assessment/{id}/compute |
| `assessment.deleted` | DELETE /api/assessment/{id} |
| `assessment.result_fetched` | GET /api/assessment/{id}/results |
| `pdf.exported` | GET /api/assessment/{id}/export/pdf |

### Example log entry

```json
{
  "level": "info",
  "ts": "2024-01-15T10:05:23.142Z",
  "logger": "audit",
  "msg": "audit",
  "event_type": "assessment.computed",
  "assessment_id": "6643f1a2c3d4e5f6a7b8c9d0",
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "occurred_at": "2024-01-15T10:05:23.141Z",
  "meta.maturity": "AI Structured",
  "meta.overall": "63.42",
  "meta.risk.CRITICAL_GAPS": "true"
}
```

### Shipping audit logs

In production, configure zap to use a log shipper:

```bash
# Ship to Loki (via promtail)
# Ship to Datadog (via datadog-agent)
# Ship to CloudWatch (via AWS log driver in ECS/EKS)

# Filter audit events from application logs:
# jq 'select(.logger == "audit")'
```

---

## API Quick Reference

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health + MongoDB status |
| GET | `/metrics` | Prometheus metrics |
| GET | `/api/questions` | Full 72-question bank |
| POST | `/api/assessment` | Create assessment → `{assessmentId}` |
| GET | `/api/assessment` | List (paginated) |
| GET | `/api/assessment/{id}` | Get by ID |
| PUT | `/api/assessment/{id}/answers` | Merge answers (bulk) |
| POST | `/api/assessment/{id}/compute` | Run scoring engine |
| GET | `/api/assessment/{id}/results` | Get result object |
| GET | `/api/assessment/{id}/export/pdf` | Stream PDF |
| DELETE | `/api/assessment/{id}` | Delete permanently |

Full OpenAPI spec: `docs/openapi.yaml`

---

## Database Operations

### Connect to MongoDB shell

```bash
# Local
mongosh mongodb://localhost:27017/ai_readiness

# Docker
docker exec -it ai-readiness-mongo mongosh ai_readiness
```

### Useful queries

```js
// Count by status
db.assessments.aggregate([
  { $group: { _id: "$status", count: { $sum: 1 } } }
])

// Find computed assessments with high overall score
db.assessments.find(
  { status: "computed", "result.overall": { $gte: 75 } },
  { "result.overall": 1, "result.maturity": 1, client_ref: 1, created_at: 1 }
).sort({ created_at: -1 }).limit(10)

// Find assessments with CRITICAL_GAPS risk
db.assessments.find(
  { "result.risks": "CRITICAL_GAPS" },
  { client_ref: 1, "result.overall": 1, created_at: 1 }
)

// Average overall score
db.assessments.aggregate([
  { $match: { status: "computed" } },
  { $group: { _id: null, avg: { $avg: "$result.overall" } } }
])

// Maturity distribution
db.assessments.aggregate([
  { $match: { status: "computed" } },
  { $group: { _id: "$result.maturity", count: { $sum: 1 } } },
  { $sort: { count: -1 } }
])

// Stale in-progress assessments (> 7 days old)
db.assessments.find({
  status: "in_progress",
  updated_at: { $lt: new Date(Date.now() - 7 * 24 * 60 * 60 * 1000) }
})
```

### Index reference

```js
db.assessments.getIndexes()
// idx_created_at      — sort by most recent
// idx_status          — filter by status
// idx_client_ref      — sparse, for multi-tenant lookups
// idx_status_created_at — compound, for status+sort queries
```

---

## Troubleshooting

### Server won't start

```bash
# Check: MONGO_URI is set
echo $MONGO_URI

# Check: question bank exists
ls question-bank-v1.json

# Check: MongoDB is reachable
mongosh "$MONGO_URI" --eval "db.adminCommand('ping')"

# Check: PDF tmp dir is writable
ls -la $PDF_TMP_DIR
```

### `CRITICAL_GAPS` always appearing

The question bank has 17 critical questions. `CRITICAL_GAPS` fires when **any** critical question is scored ≤ 2. Common causes:
- Assessor left critical questions at score 1 (default/unanswered behaviour)
- Genuinely low maturity in that domain

Check which critical questions are low-scored:
```js
db.assessments.findOne({ _id: ObjectId("...") }, { answers: 1 })
// Then cross-reference with question-bank-v1.json isCritical: true entries
```

### PDF export fails

1. Check `PDF_TMP_DIR` exists and is writable: `ls -la $PDF_TMP_DIR`
2. Check disk space: `df -h /tmp`
3. Check the assessment has a computed result: `admin get <id>`
4. Check logs for gofpdf errors: `make docker-logs | grep "pdf"`

### MongoDB field-level merge not working

Ensure you're using `PUT /answers` (not a custom direct DB write). The repository uses `$set: { "answers.<qid>": value }` per question — this is intentional to prevent race conditions when multiple tabs save simultaneously.

### Rate limiting hitting legitimate traffic

Increase `RATE_LIMIT_RPM` in `.env` (default: 60 req/min per IP). For load tests, set to a high value:

```bash
RATE_LIMIT_RPM=10000 go run ./cmd/server
```

---

## Adding a New Feature

### Adding a new API endpoint

1. **Define the route** in `cmd/server/main.go`
2. **Add the handler method** in `internal/handlers/assessment.go`
3. **Add the service method** to `internal/handlers/service.go` (interface) and `internal/service/assessment.go` (implementation)
4. **Add repository method** to `internal/repository/assessment.go` if DB access needed
5. **Add audit event** in `internal/audit/audit.go`
6. **Add metric** in `internal/metrics/metrics.go`
7. **Add handler test** in `internal/handlers/assessment_test.go` (mock service)
8. **Add e2e test** in `tests/e2e/api_test.go`
9. **Update OpenAPI spec** in `docs/openapi.yaml`

### Adding a new risk flag

1. Add detection logic to `internal/scoring/engine.go` → `detectRisks()`
2. Add the flag constant string
3. Add a unit test in `internal/scoring/engine_test.go`
4. Add the flag description to `README.md` and `docs/openapi.yaml`
5. Add the metric label to `metrics.RiskFlagsTotal` (already handles any label value)

### Updating the question bank

1. Edit `question-bank-v1.json` (backend) and `public/question-bank-v1.json` (frontend) in sync
2. Run `make validate-bank` or `admin validate-bank` to check integrity
3. Run `admin recompute <id>` for any existing assessments that need re-scoring
4. The scoring engine reads the bank at startup — restart the server to pick up changes
