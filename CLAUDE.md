# Ohara Project

Agent-optimized documentation platform built on Diataxis.

## Quick Start

```bash
pnpm install          # Install dependencies
pnpm turbo build      # Build all packages
pnpm turbo typecheck  # Type-check all packages
pnpm dev              # Start dev servers (API :3001, Web :3000)
```

## Project Structure

- `apps/api/` — Elysia API (Bun). Entry: `src/index.ts`
- `apps/web/` — Next.js 15 frontend. Entry: `src/app/`
- `apps/cli/` — Go CLI (Cobra). Entry: `main.go`
- `packages/shared/` — Diataxis types, Zod schemas, markdown parser/chunker
- `packages/config/` — Shared tsconfig, ESLint, Prettier
- `docs/` — Ohara's own documentation (dogfood)

## Key Architectural Decisions

- **Elysia on Bun** for the API — end-to-end type safety, built-in OpenAPI
- **Better Auth** for authentication — GitHub OAuth, mounted via `mount(auth.handler)`
- **Auth guard** uses `{ as: 'scoped' }` derive for cross-plugin type propagation in Elysia
- **Drizzle ORM** with PostgreSQL (Neon) + pgvector for vectors in the same DB
- **Shared package** uses extensionless imports (no `.js` suffixes) for Next.js webpack compat
- **BullMQ** for async sync jobs; passes Redis URL string directly (not ioredis instance) to avoid version conflicts
- **Hybrid search** combines tsvector full-text + pgvector semantic via Reciprocal Rank Fusion (k=60)
- **MCP server** uses `@modelcontextprotocol/sdk` with Zod for tool/resource/prompt schemas
- **Go CLI** is independent — communicates with the API via REST, no shared types

## Database

Schema at `apps/api/src/db/schema.ts`. 9 tables:
- Auth: `user`, `session`, `account`, `verification` (Better Auth)
- App: `projects`, `documents`, `document_chunks`, `api_keys`, `sync_logs`

Custom indexes (GIN on tsvector, HNSW on pgvector) are in `apps/api/src/db/migrate.ts`.

## Common Tasks

- Add a new API module: Create `apps/api/src/modules/{name}/index.ts`, export Elysia instance, `.use()` it in `src/index.ts`
- Add auth to a module: `.use(withAuth())` from `middleware/auth.ts`
- Add a new CLI command: Create `apps/cli/cmd/{name}.go`, call `rootCmd.AddCommand()` in `init()`
- Run DB migrations: `pnpm db:push` (Drizzle) then `bun run apps/api/src/db/migrate.ts` (custom indexes)

## Environment

API requires `DATABASE_URL`. Optional: `REDIS_URL` (queues/cache), `OPENAI_API_KEY` (embeddings), GitHub OAuth/App credentials. See `apps/api/.env.example`.
