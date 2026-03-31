# Changelog

All notable changes to the AI Readiness Assessment Backend are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
Versioning follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Planned
- Authentication / JWT middleware for multi-tenant deployments
- Webhook callbacks on assessment completion
- Assessment versioning (re-take tracking)
- Bulk export endpoint for all assessments

---

## [1.1.0] — 2024-06-01

### Added
- **Prometheus metrics** (`internal/metrics`) — 9 metrics covering HTTP, scoring, PDF, MongoDB, and business KPIs
- **Grafana dashboard** (`monitoring/grafana-dashboard.json`) — 12 panels, auto-provisioned
- **Alert rules** (`monitoring/alerts.yml`) — 8 alert definitions with PagerDuty + Slack routing
- **Production docker-compose** (`docker-compose.prod.yml`) — full observability stack in one command
- **Structured audit log** (`internal/audit`) — 6 event types emitted on all state-changing operations
- **Per-route rate limiter** (`internal/ratelimit`) — token bucket limiter with separate limits for `/compute` (10 rpm) and `/export/pdf` (5 rpm)
- **Centralised request validator** (`internal/validator`) — structured field errors returned as `errors[]` array
- **Shared repository mock** (`internal/repository/mock.go`) — thread-safe, deep-copying in-memory store for all tests
- **Admin CLI** (`cmd/admin`) — `stats`, `list`, `get`, `recompute`, `delete`, `export`, `validate-bank`
- **k6 load test** (`tests/load/assessment_load_test.js`) — realistic lifecycle scenarios with SLO thresholds
- **Benchmark suite** (`tests/bench`) — 5 benchmarks including parallel throughput
- **PDF service tests** (`internal/service/pdf_test.go`) — 12 tests covering all maturity levels, magic bytes, cleanup
- **Extended scoring tests** (`internal/scoring/engine_extended_test.go`) — helper coverage, weight validation, edge cases
- **Mock repository tests** (`internal/repository/mock_test.go`) — concurrency, deep-copy, pagination
- **Test fixtures** (`tests/fixtures/`) — canonical JSON payloads + loader utility
- **Developer Runbook** (`docs/runbook.md`) — architecture diagram, all make targets, MongoDB queries, troubleshooting
- `QUESTION_BANK_PATH` environment variable for configurable bank location
- `Version` and `BuildTime` injected at build time via `-ldflags`

### Changed
- Handler constructor now accepts `*audit.Logger` for clean audit injection
- Handler now uses `AssessmentServicer` interface (was concrete `*AssessmentService`) — improves testability
- `ValidateObjectID` pre-validates all `{id}` params before hitting the database
- List endpoint uses `validator.ValidatePagination` for structured error responses
- `make test` now runs all unit tests; `make test-integration` for DB tests

### Fixed
- PDF cleanup always runs via `defer CleanupFile(path)` — no more temp file leaks on error paths
- Race condition in concurrent answer saves resolved by MongoDB field-level `$set` (was full document replace)

---

## [1.0.0] — 2024-03-01

### Added
- Initial release
- **Assessment lifecycle**: Create → Answer → Compute → Export
- **Scoring engine** (`internal/scoring`) — exact parity with frontend TypeScript algorithm
  - Weighted per-question normalization `((score-1)/4)*100`
  - Domain weighted averages
  - Overall score with 6 domain weights (Strategic 20%, Technology 20%, Data 20%, Organization 15%, Security 15%, UseCase 10%)
  - 5 maturity levels: Foundational Risk Zone / AI Emerging / AI Structured / AI Advanced / AI-Native
  - 4 risk flags: CRITICAL_GAPS, DATA_HIGH_RISK, SECURITY_HIGH_RISK, MATURITY_CAPPED
- **MongoDB repository** (`internal/repository`) — field-level merge, 4 indexes, `ErrNotFound` sentinel
- **Assessment service** (`internal/service`) — answer validation, question bank loader
- **PDF export** (`internal/service/pdf.go`) — 2-page gofpdf report with domain table, recommendations, risk flags
- **8 HTTP routes** via chi v5 with full CORS, compression, rate limiting, and 30s timeout
- **Structured logging** via uber-go/zap with development/production modes
- **Graceful shutdown** with 10-second drain window
- **Docker + docker-compose** for local development
- **GitHub Actions CI** — lint, test, build, Docker push, Trivy security scan
- **OpenAPI 3.0 spec** (`docs/openapi.yaml`)
- **Migration script** (`scripts/migrate.go`) with optional seed data

---

[Unreleased]: https://github.com/yourorg/ai-readiness-backend/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/yourorg/ai-readiness-backend/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/yourorg/ai-readiness-backend/releases/tag/v1.0.0
