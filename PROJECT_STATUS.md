# Ohara Project Status

**Date:** March 2026
**Status:** MVP Built — Local Dev Working — Ready for Beta

---

## What Is Ohara

Ohara is an agent-optimized documentation platform built on the Diataxis framework. It treats AI agents as first-class citizens — agents read docs to become experts on your organization, and write back to keep docs current.

**Core insight:** Documentation should be a living, agent-maintained knowledge layer. Diataxis typing (Tutorial, Guide, Reference, Explanation) enables selective retrieval — serve the right content for the right task.

**Three ways agents consume docs:**
1. **MCP Server** — Tools, resources, and prompts for Claude Code, Cursor, etc.
2. **REST API** — Content negotiation (JSON or raw markdown) with metadata
3. **llms.txt** — Zero-setup text file any LLM can fetch and parse

---

## What We Built

### Architecture

```
LOCAL                                    CLOUD
─────                                    ─────
Developer                               GitHub (webhooks)
    │                                        │
CLI (Go)                                 Elysia API (Bun) ── Fly.io
  ohara init                               ├── Auth (Better Auth + GitHub OAuth)
  ohara generate                           ├── Sync Engine (BullMQ worker)
  ohara validate                           ├── Search (tsvector + pgvector + RRF)
  ohara search                             ├── MCP Server (7 tools, 3 prompts)
  ohara login                              ├── Scaffolding Engine
  ohara status                             └── REST API + llms.txt
    │                                        │
Git Repo                                PostgreSQL + pgvector ── Neon
  ohara.yaml                             Redis ── Upstash
  tutorials/
  guides/                                Next.js + Fumadocs ── Vercel
  reference/                               ├── Docs viewer (Diataxis sidebar)
  explanation/                             ├── Cmd+K search
    │                                      ├── Dashboard
    └── git push ────────────────────────► └── Onboarding wizard
```

### Tech Stack

| Layer | Technology | Purpose |
|-------|-----------|---------|
| API | Elysia (Bun) | End-to-end type safety, built-in OpenAPI |
| Frontend | Next.js 15 + Fumadocs | SSR docs with Diataxis sidebar |
| Database | PostgreSQL + pgvector (Neon) | Data + vectors in one store |
| Queue | BullMQ + Redis (Upstash) | Async sync jobs |
| CLI | Go + Cobra | Single binary, cross-platform |
| Search | tsvector + pgvector + RRF | Hybrid without extra infrastructure |
| Auth | Better Auth | GitHub OAuth, sessions, API keys |
| Embeddings | OpenAI text-embedding-3-small | 1536d, best quality/cost ratio |
| MCP | @modelcontextprotocol/sdk | Agent tool interface |

### Database Schema (9 Tables)

**Auth (Better Auth managed):**
- `user` — Users (GitHub OAuth)
- `session` — Active sessions
- `account` — OAuth provider accounts
- `verification` — Email verification tokens

**Application:**
- `projects` — Documentation projects linked to Git repos
- `documents` — Parsed docs with Diataxis type, HTML, tsvector
- `document_chunks` — RAG chunks with pgvector embeddings (1536d)
- `api_keys` — SHA-256 hashed API keys (`ohara_` prefix)
- `sync_logs` — Sync job history and status

### API Surface (20+ Endpoints)

**Public (no auth):**
```
GET  /health
GET  /api/v1/public/projects/:slug
GET  /api/v1/public/projects/:slug/documents
GET  /:projectSlug/llms.txt
GET  /:projectSlug/llms-full.txt
```

**Protected (session or API key):**
```
GET/POST  /api/v1/projects
GET       /api/v1/projects/:slug
GET       /api/v1/projects/:slug/documents
GET       /api/v1/projects/:slug/documents/:path
GET       /api/v1/projects/:slug/search?q=...&type=...
GET       /api/v1/projects/:slug/search/hybrid?q=...
GET       /api/v1/search?query=...&project=...
GET       /api/v1/documents/:projectSlug
GET       /api/v1/documents/:projectSlug/:path
GET/POST  /api/v1/api-keys
DELETE    /api/v1/api-keys/:id
POST      /api/v1/scaffold/analyze
POST      /api/v1/scaffold/analyze-local
POST      /api/v1/github/setup
GET       /api/v1/github/repos
POST      /api/v1/github/detect
```

**Webhooks:**
```
POST /webhooks/github          (signature verified)
```

**MCP:**
```
POST /mcp                      (API key auth, JSON-RPC)
```

**Auth (Better Auth):**
```
POST /api/auth/*
```

### MCP Server (The Key Differentiator)

**7 Tools:**
| Tool | Purpose |
|------|---------|
| `search_documentation` | Hybrid full-text + semantic search |
| `get_document` | Full doc content + related docs |
| `list_documents` | List all docs grouped by Diataxis type |
| `analyze_project` | Step 1 of scaffolding — returns generation plan |
| `create_document` | Write-back — create new doc (parsed, chunked, searchable immediately) |
| `update_document` | Write-back — update existing doc |
| `validate_project` | Check Diataxis coverage + quality |

**3 Prompts:**
| Prompt | Purpose |
|--------|---------|
| `troubleshoot` | Guided troubleshooting from guide docs |
| `explain_concept` | Concept explanation from explanation docs |
| `generate_documentation` | Full scaffolding prompt for the agent |

**1 Resource:**
| Resource | Purpose |
|----------|---------|
| `docs://{project}/{path}` | Raw markdown content |

### Scaffolding Engine (Zero-Docs Solution)

The scaffolding engine analyzes a codebase and generates a documentation plan:

1. **Analyzer** — Walks file tree, extracts signals (routes, configs, types, CI, Docker, tests)
2. **Planner** — Maps signals to Diataxis types, generates outlines + LLM prompts
3. **Adapters** — Same engine, 4 interfaces:
   - CLI: `ohara generate`
   - MCP: `analyze_project` + `create_document` tools
   - API: `POST /api/v1/scaffold/analyze`
   - GitHub Action: (spec ready)

### Sync Pipeline

```
GitHub push → Webhook (HMAC-SHA256 verified)
    → BullMQ queue (Redis)
    → Sync worker (concurrency=2)
        → Shallow clone to temp dir
        → Walk file tree (detect ohara.yaml, Diataxis dirs)
        → Parse markdown (unified/remark/rehype)
        → Content-hash diffing (skip unchanged docs)
        → Generate HTML
        → Chunk at heading boundaries (500-800 tokens)
        → Upsert to PostgreSQL
        → tsvector auto-populated via trigger
        → Cache invalidation
        → Cleanup temp dir
    → Sync lock prevents concurrent syncs per project
```

### Search Architecture

**Hybrid search** combining two methods with Reciprocal Rank Fusion:
- **Full-text:** PostgreSQL tsvector with weighted ranking (title=A, headings=B, body=C)
- **Semantic:** pgvector cosine similarity on document chunks (HNSW index)
- **RRF:** Merges results with k=60, consistently outperforms either method alone
- **Diataxis boosting:** "How do I..." → boosts guides, "Why does..." → boosts explanations

### Frontend

- **Fumadocs** layout with custom source adapter
- **Diataxis sidebar** — docs grouped by type (Tutorial, Guide, Reference, Explanation)
- **Cmd+K search** — debounced, type filter chips, highlighted snippets
- **Landing page** — hero, how-it-works, Diataxis explainer, CTA
- **Dashboard** — project list, sync status
- **Onboarding wizard** — GitHub App install → repo select → structure detect → sync

### CLI (Go)

```
ohara init [name]              Create ohara.yaml + Diataxis directories
ohara generate [directory]     Analyze code, generate doc scaffolding
ohara validate                 Check structure, frontmatter, coverage
ohara login                    Authenticate with API key
ohara status                   Show project sync status
ohara search <query>           Search docs via API
```

### Infrastructure

- **CI:** GitHub Actions — lint, typecheck, build, format check
- **Staging:** Auto-deploy on `develop` merge (Fly.io + Vercel)
- **Production:** CI gate → deploy on `main` merge + health check
- **Security:** Rate limiting (10/min auth, 100/min API), security headers, HMAC webhook verification
- **Caching:** In-memory LRU with TTL (Redis-ready interface), invalidation on sync
- **Observability:** Pino structured logging, request timing

---

## How to Work With It

### Local Development

**Prerequisites:** Bun, Node.js 20+, pnpm 9+, Go 1.22+ (for CLI)

```bash
# Install dependencies
pnpm install

# Configure environment
cp apps/api/.env.example apps/api/.env
# Edit .env with your DATABASE_URL, auth secrets, etc.

# Push database schema + create indexes
pnpm db:push
cd apps/api && bun run db:migrate

# Start everything
# Terminal 1: API
cd apps/api && bun run dev

# Terminal 2: Web
cd apps/web && pnpm dev

# Terminal 3: Webhook proxy (optional)
cd apps/api && bun run webhook:proxy
```

**URLs:**
- Web: http://localhost:3000
- API: http://localhost:3001
- Health: http://localhost:3001/health

### Required Environment Variables

| Variable | Required | Where to get it |
|----------|----------|----------------|
| `DATABASE_URL` | Yes | [neon.tech](https://neon.tech) — create project, copy connection string |
| `BETTER_AUTH_SECRET` | Yes | `openssl rand -hex 32` |
| `GITHUB_CLIENT_ID` | For login | GitHub OAuth App → [github.com/settings/developers](https://github.com/settings/developers) |
| `GITHUB_CLIENT_SECRET` | For login | Same OAuth App |
| `GITHUB_APP_ID` | For sync | GitHub App → [github.com/settings/apps](https://github.com/settings/apps) |
| `GITHUB_APP_PRIVATE_KEY` | For sync | GitHub App → generate private key (.pem file path) |
| `GITHUB_WEBHOOK_SECRET` | For sync | `openssl rand -hex 20`, set in GitHub App settings |
| `REDIS_URL` | For queue | [upstash.com](https://upstash.com) — use `rediss://` (TLS) |
| `OPENAI_API_KEY` | For semantic search | [platform.openai.com](https://platform.openai.com/api-keys) |

### Testing the MCP Server

```bash
# Set your API key
API_KEY="ohara_your_key_here"

# List available tools
curl -X POST http://localhost:3001/mcp \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

# Analyze a project
curl -X POST http://localhost:3001/mcp \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"analyze_project","arguments":{"project":"your-project-slug"}}}'

# Create a document (write-back)
curl -X POST http://localhost:3001/mcp \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"create_document","arguments":{"project":"your-project-slug","path":"guides/example.md","title":"Example Guide","diataxis_type":"guide","content":"# Example Guide\n\nYour content here."}}}'

# Validate
curl -X POST http://localhost:3001/mcp \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"validate_project","arguments":{"project":"your-project-slug"}}}'
```

### Using with Claude Code (MCP Client)

Add to your Claude Code MCP configuration:

```json
{
  "mcpServers": {
    "ohara": {
      "type": "url",
      "url": "http://localhost:3001/mcp",
      "headers": {
        "Authorization": "Bearer ohara_your_key_here"
      }
    }
  }
}
```

Then in Claude Code:
- *"Search the bun-tale docs for deployment instructions"*
- *"Analyze the bun-tale project and tell me what docs are missing"*
- *"Generate a getting-started tutorial for bun-tale based on the README"*
- *"Validate the bun-tale project documentation"*

### Using the CLI

```bash
# Build the CLI
cd apps/cli && go build -o ohara .

# Initialize a new docs project in any repo
./ohara init my-project

# Analyze code and scaffold docs
./ohara generate /path/to/your/repo

# Validate structure
./ohara validate

# Authenticate and check status
./ohara login
./ohara status
./ohara search "authentication" --project my-project
```

### Key Files (Start Here to Understand the Code)

| File | What it does |
|------|-------------|
| `apps/api/src/db/schema.ts` | Database schema — all 9 tables + pgvector |
| `apps/api/src/modules/agent/mcp/server.ts` | MCP server — 7 tools, 3 prompts, 1 resource |
| `apps/api/src/modules/git/sync-service.ts` | Git → DB sync pipeline |
| `apps/api/src/modules/search/hybrid.ts` | Hybrid search (RRF) |
| `packages/shared/src/scaffold/analyzer.ts` | Code analysis engine |
| `packages/shared/src/scaffold/planner.ts` | Doc generation planner |
| `packages/shared/src/markdown/parser.ts` | Markdown pipeline (unified/remark/rehype) |
| `packages/shared/src/markdown/chunker.ts` | Heading-based document chunker |
| `apps/api/src/middleware/auth.ts` | Auth guard (session + API key) |
| `apps/api/src/index.ts` | API entry point — all modules wired here |

---

## What's Next

### Immediate (High Priority)

1. **Rotate credentials** — DB URL, GitHub secrets, Upstash key were exposed in dev session. Regenerate all of them.

2. **GitHub OAuth flow** — Login page is built but the callback flow needs testing end-to-end. Verify the callback URL in your GitHub OAuth App matches `http://localhost:3001/api/auth/callback/github`.

3. **Webhook-triggered sync** — The smee.io proxy is set up. Push to a connected repo and verify the full flow: push → webhook → sync → docs appear on web.

4. **OpenAI embeddings** — Add your `OPENAI_API_KEY` to `.env` to enable semantic search. Without it, search is full-text only (still works well).

### Short-Term (Product Polish)

5. **Scaffolding agent improvements** — The `analyze_project` MCP tool currently works from DB metadata. Enhance it to clone the repo and run the full code analyzer for richer signals.

6. **Staleness detection** — Evaluator-optimizer pattern: when code changes in a PR, detect which docs may be outdated and flag them.

7. **PR review GitHub Action** — `ohara/docs-check` action that comments on PRs when code changes affect documented areas.

8. **Cross-project search** — Allow searching across all projects for the Hub pattern.

### Medium-Term (Growth)

9. **Knowledge Hub mode** — One central repo with subdirectories per service. Multi-project navigation and cross-service search.

10. **Embeddings on sync** — Automatically embed chunks after sync (currently requires separate trigger).

11. **Real-time doc preview** — WebSocket connection for live preview while editing docs locally.

12. **CLI `ohara generate --execute`** — Full LLM-powered doc generation from the CLI (currently generates scaffolding + prompts).

### Enterprise

13. **Teams and permissions** — Multi-user orgs with role-based access.

14. **Custom domains** — `docs.yourcompany.com` for each project.

15. **Audit logs** — Track who changed what docs and when.

16. **SSO/SAML** — Enterprise authentication.

---

## Agent Patterns (Design Reference)

Based on [Anthropic's "Building Effective Agents"](https://www.anthropic.com/engineering/building-effective-agents) and [Google Cloud's agentic AI architecture](https://docs.cloud.google.com/architecture/choose-design-pattern-agentic-ai-system).

### Principle: Start Simple

Most Ohara features are **workflows** (predefined code paths), not autonomous agents. Only upgrade to agent patterns when workflows demonstrably fall short.

### Pattern 1: Prompt Chaining (Scaffolding)

```
Analyze (no LLM) → Plan (no LLM) → [Gate: user reviews] → Generate (LLM) → Validate (no LLM)
```

Used for: `ohara generate`, initial doc scaffolding. LLM only in the generation step. Gates between steps for quality control.

### Pattern 2: Evaluator-Optimizer (Maintenance)

```
Code diff → Staleness detector → [Gate: above threshold?] → Doc updater → Validator loop
```

Used for: PR review, "your code changed but docs didn't". Detects stale docs and generates updates.

### Pattern 3: Orchestrator-Workers (Interactive Writing)

```
Planner decomposes task → Worker 1 (tutorial) → Worker 2 (reference) → Worker 3 (guide) → Synthesize
```

Used for: MCP `generate_documentation` prompt, dashboard "Generate" button. In Claude Code, Claude itself is the orchestrator.

### Pattern 4: Parallelization with Review (Quality)

```
Generate docs in parallel → Critic reviews for consistency → Fix issues → Done
```

Used for: Bulk generation for large repos, enterprise Hub pattern.

### Core Design Decision

Every Ohara capability follows: **one core engine, many adapters**. The engine is interface-agnostic. Adapters expose it via CLI, MCP, API, web UI, and GitHub Actions. Different developers use different interfaces at different moments — Ohara adapts to all of them.

---

## Project Structure

```
ohara-project/
├── apps/
│   ├── api/                          Elysia API (Bun)
│   │   ├── src/
│   │   │   ├── db/                   Schema, migrations, setup
│   │   │   ├── lib/                  Auth, API keys, queue, cache, logger
│   │   │   ├── middleware/           Auth guard, rate limit, security, logging
│   │   │   ├── modules/
│   │   │   │   ├── health/
│   │   │   │   ├── projects/
│   │   │   │   ├── documents/
│   │   │   │   ├── api-keys/
│   │   │   │   ├── public/           Public endpoints (no auth)
│   │   │   │   ├── webhooks/         GitHub webhook handler
│   │   │   │   ├── git/              Sync service, worker, file walker
│   │   │   │   ├── search/           Full-text, semantic, hybrid, embeddings
│   │   │   │   ├── agent/            REST API + MCP server
│   │   │   │   ├── scaffold/         Code analysis API
│   │   │   │   ├── llms-txt/         Diataxis-typed llms.txt
│   │   │   │   └── github-app/       GitHub App installation flow
│   │   │   └── test/                 E2E sync tests
│   │   ├── scripts/                  webhook-proxy.sh
│   │   ├── Dockerfile
│   │   ├── fly.toml                  Production config
│   │   └── fly.staging.toml          Staging config
│   │
│   ├── web/                          Next.js Frontend
│   │   └── src/
│   │       ├── app/
│   │       │   ├── page.tsx          Landing page
│   │       │   ├── login/            GitHub OAuth sign-in
│   │       │   ├── dashboard/        Project list + onboarding wizard
│   │       │   └── [project-slug]/   Docs viewer (Fumadocs layout)
│   │       ├── components/           Search dialog (cmd+k)
│   │       └── lib/                  API client, source adapter
│   │
│   └── cli/                          Go CLI
│       ├── main.go
│       ├── cmd/                      init, generate, validate, login, status, search
│       └── .goreleaser.yaml          Cross-platform build config
│
├── packages/
│   ├── shared/                       Shared TypeScript packages
│   │   └── src/
│   │       ├── diataxis/             Types, labels, query patterns
│   │       ├── schemas/              Frontmatter + ohara.yaml Zod schemas
│   │       ├── markdown/             Parser (unified) + Chunker
│   │       ├── scaffold/             Code analyzer + generation planner
│   │       └── constants.ts
│   │
│   └── config/                       Shared configs
│       ├── tsconfig.base.json
│       ├── tsconfig.nextjs.json
│       ├── tsconfig.library.json
│       └── eslint.config.mjs
│
├── docs/                             Ohara's own docs (dogfood)
│   ├── ohara.yaml
│   ├── tutorials/
│   ├── guides/
│   ├── reference/
│   └── explanation/
│
├── .github/workflows/
│   ├── ci.yml                        Lint, typecheck, build
│   ├── deploy-staging.yml            Auto-deploy on develop
│   └── deploy-production.yml         CI gate → deploy on main
│
├── CLAUDE.md                         Dev workflow reference
├── CONTRIBUTING.md
├── LICENSE                           MIT
├── ARCHITECTURE.md                   Original architecture doc
└── RESEARCH.md                       Original research doc
```
