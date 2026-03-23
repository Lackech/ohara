# Contributing to Ohara

Thanks for your interest in contributing to Ohara!

## Development Setup

### Prerequisites

- [Bun](https://bun.sh) (latest)
- [Node.js](https://nodejs.org) (v20+)
- [pnpm](https://pnpm.io) (v9+)
- [Go](https://go.dev) (v1.22+ for CLI)
- PostgreSQL (or [Neon](https://neon.tech) account)

### Getting Started

```bash
# Clone the repo
git clone https://github.com/ohara-project/ohara.git
cd ohara

# Install dependencies
pnpm install

# Copy environment variables
cp apps/api/.env.example apps/api/.env
# Edit .env with your database URL

# Push database schema
pnpm db:push

# Start development servers
pnpm dev
```

The API runs at `http://localhost:3001` and the web app at `http://localhost:3000`.

### Building the CLI

```bash
cd apps/cli
go build -o ohara .
./ohara --help
```

## Project Structure

```
apps/
  api/       — Elysia API server (Bun)
  web/       — Next.js frontend
  cli/       — Go CLI
packages/
  shared/    — Shared types, schemas, markdown pipeline
  config/    — Shared tsconfig, ESLint, Prettier
docs/        — Ohara's own documentation (dogfood)
```

## Making Changes

1. Create a branch from `develop`
2. Make your changes
3. Run `pnpm turbo typecheck` and `pnpm turbo build`
4. Submit a PR against `develop`

## Code Style

- TypeScript: Prettier + ESLint (configured in `packages/config`)
- Go: `gofmt`
- Commits: Conventional commits preferred but not enforced

## Key Files

If you're looking to understand the codebase, start with these:

1. `apps/api/src/db/schema.ts` — Database schema
2. `packages/shared/src/markdown/parser.ts` — Markdown pipeline
3. `apps/api/src/modules/git/sync-service.ts` — Git sync engine
4. `apps/api/src/modules/agent/mcp/server.ts` — MCP server
5. `apps/api/src/modules/search/hybrid.ts` — Hybrid search (RRF)
