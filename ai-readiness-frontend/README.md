# AI Readiness Assessment Tool

A production-ready frontend for assessing organizational AI readiness across 6 domains with weighted questions.

## Quick Start

```bash
npm install
npm run dev
# Open http://localhost:3000
```

## Environment Variables

Create `.env.local` (optional — app works fully client-side without a backend):

```env
NEXT_PUBLIC_API_BASE_URL=https://your-backend.example.com
```

## File Structure

```
src/
├── app/
│   ├── layout.tsx              # Root layout with provider
│   ├── page.tsx                # Welcome / landing
│   ├── globals.css
│   ├── assessment/
│   │   ├── page.tsx            # Domain dashboard
│   │   └── [domain]/
│   │       └── page.tsx        # Individual domain questions
│   ├── review/
│   │   └── page.tsx            # Review & compute
│   └── results/
│       └── page.tsx            # Results dashboard
├── components/
│   ├── Navbar.tsx
│   ├── DomainCard.tsx
│   ├── ProgressHeader.tsx
│   ├── QuestionItem.tsx
│   ├── RadarChart.tsx
│   ├── RecommendationsList.tsx
│   └── RoadmapBoard.tsx
└── lib/
    ├── types.ts                # TypeScript interfaces
    ├── scoring.ts              # Client-side scoring engine
    ├── api.ts                  # Backend API client
    └── store.tsx               # React context + localStorage state
public/
└── question-bank-v1.json       # 72-question bank (6 domains × 12 questions)
```

## Architecture

- **State**: React Context + localStorage (no external state library required)
- **Scoring**: Full client-side fallback when no backend configured
- **Charts**: Recharts for radar visualization
- **Backend API**:  `src/lib/api.ts` handles all backend calls with graceful fallback

## Scoring Algorithm

- Each answer (1–5) normalized: `((score - 1) / 4) × 100`
- Domain score: weighted average by `question.weight`
- Overall: weighted sum: Strategic(20%) + Technology(20%) + Data(20%) + Organization(15%) + Security(15%) + UseCase(10%)

## Maturity Levels

| Score | Level                  |
| ----- | ---------------------- |
| < 40  | Foundational Risk Zone |
| < 60  | AI Emerging            |
| < 75  | AI Structured          |
| < 90  | AI Advanced            |
| ≥ 90 | AI-Native              |

## Risk Flags

- `CRITICAL_GAPS`: Any critical question scored ≤ 2
- `DATA_HIGH_RISK`: Data domain score < 50
- `SECURITY_HIGH_RISK`: Security domain score < 50
- `MATURITY_CAPPED`: Overall ≥ 75 but Data or Security < 50 (caps maturity to AI Structured)
