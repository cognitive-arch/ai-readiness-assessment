# AI Readiness Assessment

A full-stack enterprise tool for benchmarking organizational AI readiness. Assess your organization across **6 capability domains** with **weighted questions**, and receive a maturity score, automated risk flags, and a prioritized transformation roadmap.

> **Live at:** [aitransformation.app
> ](https://aitransformation.app)


---

## Repository Structure

```
AI-Readiness-Assessment/
├── ai-readiness-frontend/    ← Next.js 14 app (TypeScript + Tailwind)
├── ai-readiness-backend/     ← Go REST API (MongoDB)
├── .gitignore
└── README.md                 ← you are here
```

- **[Frontend Documentation](./ai-readiness-frontend/README.md)** — Next.js setup, component architecture, scoring algorithm
- **[Backend Documentation](./ai-readiness-backend/README.md)** — Go API reference, MongoDB schema, deployment guide

---

## Quick Start

### Option 1 — Docker Compose (recommended, runs everything)

```bash
# Clone the repo
git clone https://github.com/yourorg/AI-Readiness-Assessment.git
cd AI-Readiness-Assessment

# Start the full stack: MongoDB + API + Frontend
cd ai-readiness-frontend
NEXT_PUBLIC_API_BASE_URL=http://localhost:8080 docker compose up -d
```

| Service    | URL                          |
| ---------- | ---------------------------- |
| Frontend   | http://localhost:3000        |
| API        | http://localhost:8080        |
| API Health | http://localhost:8080/health |

---

### Option 2 — Run services separately

**1. Start MongoDB**

```bash
# Docker
docker run -d --name mongo -p 27017:27017 mongo:7

# or Homebrew (Mac)
brew services start mongodb-community
```

**2. Start the backend**

```bash
cd ai-readiness-backend

cp .env.example .env
# Edit .env — set MONGO_URI=mongodb://localhost:27017

go run ./cmd/server
# API running at http://localhost:8080
```

**3. Start the frontend**

```bash
cd ai-readiness-frontend

echo "NEXT_PUBLIC_API_BASE_URL=http://localhost:8080" > .env.local

npm install
npm run dev
# App running at http://localhost:3000
```

---

## Tech Stack

| Layer          | Technology                                                    |
| -------------- | ------------------------------------------------------------- |
| Frontend       | Next.js 14 (App Router), TypeScript, Tailwind CSS, Recharts   |
| Backend        | Go 1.22, chi router, uber-go/zap, gofpdf                      |
| Database       | MongoDB 7                                                     |
| Observability  | Prometheus metrics, Grafana dashboards, structured audit logs |
| Infrastructure | Docker, Docker Compose, Kubernetes manifests                  |

---

## Assessment Framework

### 6 Capability Domains

| Domain       | Weight | Focus                                        |
| ------------ | ------ | -------------------------------------------- |
| Strategic    | 20%    | AI vision, governance, executive sponsorship |
| Technology   | 20%    | MLOps, infrastructure, model lifecycle       |
| Data         | 20%    | Governance, quality, lineage, privacy        |
| Organization | 15%    | Talent, culture, change management           |
| Security     | 15%    | Threat modeling, compliance, ethics          |
| Use Cases    | 10%    | Prioritization, production deployments, ROI  |

### Maturity Levels

| Score     | Level                  |
| --------- | ---------------------- |
| 0 – 39   | Foundational Risk Zone |
| 40 – 59  | AI Emerging            |
| 60 – 74  | AI Structured          |
| 75 – 89  | AI Advanced            |
| 90 – 100 | AI-Native              |

### Risk Flags

| Flag                   | Condition                               |
| ---------------------- | --------------------------------------- |
| `CRITICAL_GAPS`      | Any critical question scored ≤ 2       |
| `DATA_HIGH_RISK`     | Data domain score < 50                  |
| `SECURITY_HIGH_RISK` | Security domain score < 50              |
| `MATURITY_CAPPED`    | Overall ≥ 75 but Data or Security < 50 |

---

## Environment Variables

### Frontend (`ai-readiness-frontend/.env.local`)

```env
# Required — backend API base URL (must be reachable from the user's browser)
NEXT_PUBLIC_API_BASE_URL=http://localhost:8080
```

> **Note:** `NEXT_PUBLIC_*` variables are baked into the JavaScript bundle at build time. When building a Docker image, pass this as `--build-arg NEXT_PUBLIC_API_BASE_URL=https://api.yourdomain.com`.

### Backend (`ai-readiness-backend/.env`)

```env
MONGO_URI=mongodb://localhost:27017
MONGO_DB=ai_readiness
PORT=8080
CORS_ORIGINS=http://localhost:3000
```

See [`ai-readiness-backend/.env.example`](./ai-readiness-backend/.env.example) for the full list.

---

## Contributing

- **[Developer Runbook Documentation](./ai-readiness-backend/README.md)** — Operational reference for the backend developers.

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/your-feature`
3. Commit your changes: `git commit -m 'feat: add your feature'`
4. Push to the branch: `git push origin feature/your-feature`
5. Open a Pull Request

Please read the individual project READMEs before contributing — each has specific setup steps, test commands, and architecture notes.

---

## License

MIT — see [LICENSE](./LICENSE) for details.
