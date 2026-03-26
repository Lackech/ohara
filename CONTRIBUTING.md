# Contributing to Ohara

## Prerequisites

- [Go](https://go.dev) (v1.22+)
- [Node.js](https://nodejs.org) (v20+ for Starlight viewer)

## Getting Started

```bash
git clone https://github.com/Lackech/ohara.git
cd ohara/apps/cli
go build -o ohara .
./ohara --help
```

## Making Changes

1. Create a branch from `main`
2. Make your changes in `apps/cli/`
3. Run `go build -o ohara .` to verify
4. Test: `./ohara init`, `./ohara generate`, `./ohara view`
5. Submit a PR

## Code Style

- Go: `gofmt`
- Commits: Conventional commits preferred

## Key Files

| File | Purpose |
|------|---------|
| `cmd/init.go` | Hub creation + agent/skill/hook setup |
| `cmd/skills.go` | All subagent and skill definitions |
| `cmd/upgrade.go` | CLAUDE.md builder + settings.json with hooks |
| `cmd/buildsite.go` | Starlight content generation from hub |
| `cmd/hooks.go` | CLI hook commands (gate, watch-hook, session-summary) |
| `cmd/playbooks.go` | Starter playbook definitions |
| `cmd/serve.go` | Local MCP server (stdio, 7 tools) |
| `cmd/hub.go` | Hub config types and helpers |

## Releasing

```bash
git tag v0.X.0
git push origin v0.X.0
cd apps/cli && GITHUB_TOKEN=$(gh auth token) goreleaser release --clean
```
