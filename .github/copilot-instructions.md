# Copilot Instructions — DeltaCast

## Project Overview

DeltaCast is a **one-in, multi-out live streaming relay** system. A streamer pushes via Agora RTC; the Golang backend orchestrates relay to YouTube (RTMP) and Google Live Stream API (HLS via Cloud CDN). The system uses a **two-phase flow**: `prepare` pre-warms GCP/YouTube resources (30-60s), then `start` returns an Agora token for immediate streaming. Agora NCS webhook triggers Media Push upon detecting the stream. Read [doc/spec.md](../doc/spec.md) and [doc/instruction.md](../doc/instruction.md) for the full system design before making changes.

## Architecture

- **`server/`** — Golang orchestrator (Go 1.24+). Strict 3-layer architecture:
  - `internal/handler/` → HTTP request/response only, no business logic
  - `internal/middleware/` → JWT auth, logging, and other cross-cutting concerns
  - `internal/service/` → business logic & session state machine (`idle → preparing → ready → live → stopping → idle`)
  - `internal/provider/` → all third-party API calls (Agora, GCP Live Stream, YouTube Data API)
  - `internal/model/` → shared data structures and types
  - `internal/config/` → environment variable loading and configuration
  - **Never skip layers** — handlers must not call providers directly; services must not import `net/http`.
- **`web/`** — Next.js 16 + Tailwind CSS. App Router (`app/` for pages, `components/` for shared UI). Player: video.js (GCP HLS) + react-player (YouTube).
- **`mobile/`** — Future iOS (Swift) / Android (Kotlin) clients.

## Key API Endpoints

| Method | Path                | Purpose                                         |
| ------ | ------------------- | ----------------------------------------------- |
| POST   | `/v1/live/prepare`  | Pre-warm GCP + YouTube resources (30-60s)       |
| POST   | `/v1/live/start`    | Return Agora token, begin streaming             |
| POST   | `/v1/live/stop`     | Stop relay, clean up all resources              |
| GET    | `/v1/live/status`   | Poll session state                              |
| POST   | `/v1/webhook/agora` | Agora NCS callback (no JWT, signature-verified) |

## Development Workflow

```bash
# Backend
cd server && go run ./cmd/

# Frontend
cd web && pnpm install && pnpm dev

# Full stack
docker-compose up
```

## Conventions

- **Go**: `gofmt`/`goimports`, snake_case filenames (`live_service.go`), GoDoc on public functions, `if err != nil` error handling (no panic), secrets from env vars only.
- **TypeScript/React**: ESLint + Prettier, kebab-case filenames (`live-player.tsx`), functional components + hooks only.
- **Git**: Adhere to the Conventional Commits specification. Use English for the commit type (e.g., feat:, fix:, refactor:, doc:, chore:), but write the descriptive message in Traditional Chinese. Ensure each commit represents exactly one logical change.
- **Tests**: Co-located with source (`_test.go` / `.test.ts`). Cover happy path + at least one error path. Required for new service/provider functions.

## Critical Rules

1. **Read before write** — always read `doc/spec.md`, `doc/instruction.md`, `doc/api.md`, and the target file fully before editing.
2. **Minimal changes** — one concern per edit; no drive-by refactors.
3. **No hardcoded secrets** — all API keys/secrets via environment variables (see `server/.env.example`, `script/.env.example` and `web/.env.example`), and the secrets must never be committed to version control.
4. **Resource cleanup is critical** — GCP resources are billed per-time. The stop flow must attempt every cleanup step even if earlier steps fail (log error, continue).
5. **Idempotent webhook handling** — Agora NCS may send duplicate events; guard with session state checks.
6. **Single active session** — POC supports one session at a time. Duplicate `start` returns existing session, not a new one.
7. **Task tracking** — when completing items related to [doc/task-tracking.md](../doc/task-tracking.md), update the checkboxes there.
8. **Documentation** — update `doc/spec.md`, `doc/instruction.md` and `doc/api.md` with any design changes or new implementation details.
9. **Accuracy** — if you have questions or need clarification, ask in the project chat or create an issue. Do not make assumptions without confirming.
10. **Language Consistency** — When the user provides prompts or questions in Traditional Chinese, always respond in Traditional Chinese; otherwise, respond in English. This ensures clear communication and maintains consistency throughout the project.
