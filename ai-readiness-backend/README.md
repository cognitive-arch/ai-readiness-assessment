# AI Readiness Assessment — Go Backend

Production-ready REST API in Go, backed by MongoDB, serving the AI Readiness Assessment frontend.

---

## Stack

| Layer        | Technology                          |
|--------------|-------------------------------------|
| Language     | Go 1.22                             |
| HTTP Router  | [chi v5](https://github.com/go-chi/chi) |
| Database     | MongoDB 7 via `mongo-driver`        |
| PDF Export   | `gofpdf`                            |
| Logging      | `uber-go/zap` (structured JSON)     |
| Rate Limiting| `go-chi/httprate`                   |
| Config       | `godotenv` + env vars               |

---

## Quick Start

### 1. Prerequisites

- Go 1.22+
- MongoDB 7 (local or Atlas)
- `question-bank-v1.json` in the project root (copy from the frontend `public/` folder)

```bash
cp ../ai-readiness/public/question-bank-v1.json .
# or: make seed
```

### 2. Configure environment

```bash
cp .env.example .env
# Edit .env — at minimum set MONGO_URI
```

### 3. Run

```bash
# Option A — directly
go run ./cmd/server

# Option B — with hot-reload (requires: go install github.com/air-verse/air@latest)
make dev

# Option C — Docker (MongoDB included)
make docker-up
# API → http://localhost:8080
# (optional Mongo Express UI) → make docker-up-debug → http://localhost:8081
```

---

## Project Structure

```
ai-readiness-backend/
├── cmd/
│   └── server/
│       └── main.go              ← Entry point, DI wiring, router, graceful shutdown
├── internal/
│   ├── config/
│   │   └── config.go            ← Env-based configuration
│   ├── models/
│   │   └── models.go            ← Assessment, Answer, Result, Question structs
│   ├── repository/
│   │   └── assessment.go        ← MongoDB CRUD (interface + implementation)
│   ├── scoring/
│   │   ├── engine.go            ← Scoring algorithm (mirrors frontend exactly)
│   │   └── engine_test.go       ← Unit tests for all scoring logic
│   ├── service/
│   │   ├── assessment.go        ← Business logic layer
│   │   └── pdf.go               ← PDF report generation
│   ├── handlers/
│   │   ├── dto.go               ← Request/response types
│   │   ├── assessment.go        ← HTTP handlers
│   │   └── assessment_test.go   ← Handler-level integration tests
│   └── middleware/
│       └── middleware.go        ← RequestID, Logger, Recoverer
├── scripts/
│   └── mongo-init.js            ← MongoDB collection + index bootstrap
├── question-bank-v1.json        ← 72-question bank (required at runtime)
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── .air.toml                    ← Hot-reload config
├── .env.example
└── go.mod
```

---

## API Reference

Base URL: `http://localhost:8080`

All endpoints return:
```json
{
  "success": true,
  "data": { ... },
  "meta": { ... }   // pagination only
}
```
Errors:
```json
{ "success": false, "error": "message" }
```

---

### Health

```
GET /health
```
Returns MongoDB connectivity status.

```json
{ "status": "ok", "mongo": true, "time": "2024-01-15T10:00:00Z" }
```

---

### Create Assessment

```
POST /api/assessment
Content-Type: application/json

{ "client_ref": "acme-corp" }   ← optional
```

**Response 201:**
```json
{
  "success": true,
  "data": {
    "assessmentId": "6643f1a2c3d4e5f6a7b8c9d0",
    "status": "draft",
    "answers": {},
    "createdAt": "2024-01-15T10:00:00Z",
    "updatedAt": "2024-01-15T10:00:00Z"
  }
}
```

---

### Get Assessment

```
GET /api/assessment/{id}
```

---

### Save Answers (bulk merge)

```
PUT /api/assessment/{id}/answers
Content-Type: application/json

{
  "answers": {
    "s1": { "score": 4, "comment": "We have a documented strategy" },
    "s2": { "score": 3 },
    "t1": { "score": 5, "comment": "AWS SageMaker deployed" }
  }
}
```

- Scores must be integers 1–5
- Question IDs are validated against the question bank
- Answers are **merged** (existing answers not in this payload are preserved)

**Response 200:** Updated assessment document.

---

### Compute Results

```
POST /api/assessment/{id}/compute
```

Runs the full scoring engine against all saved answers. Stores and returns the result.

**Response 200:**
```json
{
  "success": true,
  "data": {
    "assessmentId": "...",
    "status": "computed",
    "result": {
      "overall": 63.42,
      "domainScores": {
        "Strategic":    { "score": 71.25, "answered": 10, "total": 12 },
        "Technology":   { "score": 58.00, "answered": 12, "total": 12 },
        "Data":         { "score": 45.50, "answered": 11, "total": 12 },
        "Organization": { "score": 68.75, "answered": 12, "total": 12 },
        "Security":     { "score": 52.00, "answered": 12, "total": 12 },
        "UseCase":      { "score": 66.67, "answered": 10, "total": 12 }
      },
      "maturity": "AI Structured",
      "confidence": 94.44,
      "risks": ["CRITICAL_GAPS", "DATA_HIGH_RISK"],
      "recommendations": [
        {
          "domain": "Data",
          "text": "Implement a unified data catalog with lineage and quality tracking",
          "priority": "Critical",
          "phase": "Phase 1"
        }
      ],
      "totalAnswered": 67,
      "totalQ": 72,
      "computedAt": "2024-01-15T10:05:00Z"
    }
  }
}
```

---

### Get Results

```
GET /api/assessment/{id}/results
```

Returns only the `result` object. Returns `409 Conflict` if not yet computed.

---

### Export PDF

```
GET /api/assessment/{id}/export/pdf
```

Streams a PDF file. Returns `409 Conflict` if results not yet computed.

**Response headers:**
```
Content-Type: application/pdf
Content-Disposition: attachment; filename="ai-readiness-{id}.pdf"
```

---

### List Assessments

```
GET /api/assessment?limit=20&offset=0
```

**Response 200:**
```json
{
  "success": true,
  "data": [ ... ],
  "meta": { "total": 42, "limit": 20, "offset": 0 }
}
```

---

### Delete Assessment

```
DELETE /api/assessment/{id}
```

---

### Get Question Bank

```
GET /api/questions
```

Returns the full 72-question bank as loaded from `question-bank-v1.json`.

---

## Scoring Algorithm

Mirrors the frontend implementation exactly so client-side fallback and server results are identical.

```
normalized_score = ((raw_score - 1) / 4) × 100      // 1→0, 5→100

domain_score = Σ(normalized_score × weight) / Σ(weight)

overall = Σ(domain_score × domain_weight)
  where: Strategic=0.20, Technology=0.20, Data=0.20,
         Organization=0.15, Security=0.15, UseCase=0.10

confidence = (answered / total_questions) × 100
```

**Maturity levels:**

| Score Range | Level                  |
|-------------|------------------------|
| < 40        | Foundational Risk Zone |
| 40 – 59     | AI Emerging            |
| 60 – 74     | AI Structured          |
| 75 – 89     | AI Advanced            |
| ≥ 90        | AI-Native              |

**Risk flags:**

| Flag               | Condition                                        |
|--------------------|--------------------------------------------------|
| `CRITICAL_GAPS`    | Any critical question scored ≤ 2                |
| `DATA_HIGH_RISK`   | Data domain score < 50                           |
| `SECURITY_HIGH_RISK` | Security domain score < 50                    |
| `MATURITY_CAPPED`  | Overall ≥ 75 but Data or Security < 50 → capped to AI Structured |

---

## Environment Variables

| Variable            | Required | Default                  | Description                          |
|---------------------|----------|--------------------------|--------------------------------------|
| `MONGO_URI`         | ✅       | —                        | MongoDB connection string            |
| `MONGO_DB`          |          | `ai_readiness`           | Database name                        |
| `PORT`              |          | `8080`                   | HTTP listen port                     |
| `ENV`               |          | `development`            | `development` or `production`        |
| `CORS_ORIGINS`      |          | `http://localhost:3000`  | Comma-separated allowed origins      |
| `RATE_LIMIT_RPM`    |          | `60`                     | Requests per minute per IP           |
| `PDF_TMP_DIR`       |          | `/tmp/ai-readiness-pdfs` | Temp directory for generated PDFs    |
| `LOG_LEVEL`         |          | `info`                   | `debug`, `info`, `warn`, `error`     |
| `QUESTION_BANK_PATH`|          | `question-bank-v1.json`  | Path to the question bank JSON file  |

---

## Running Tests

```bash
# All tests
make test

# With coverage report
make test-cover

# Scoring engine only
make test-scoring
```

---

## MongoDB Document Schema

```
assessments collection:
{
  _id:        ObjectID,
  status:     "draft" | "in_progress" | "completed" | "computed",
  answers: {
    "<questionId>": { score: int (1-5), comment: string }
  },
  result: {
    overall:       float64,
    domain_scores: { "<domain>": { score, answered, total } },
    maturity:      string,
    confidence:    float64,
    risks:         [string],
    recommendations: [{ domain, text, priority, phase }],
    total_answered: int,
    total_q:        int,
    computed_at:    ISODate
  },
  client_ref:  string (optional),
  created_at:  ISODate,
  updated_at:  ISODate
}
```

Indexes: `created_at` (desc), `status`, `client_ref` (sparse).
