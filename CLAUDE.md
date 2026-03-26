# Ohara

Local-first documentation hub for AI agent teams. Built on Diataxis.

## Project Structure

```
apps/cli/          Go CLI (Cobra) — the entire product
  cmd/             Commands: init, add, generate, build, validate, view, run, serve, upgrade
  web/             Embedded HTMX viewer (fallback, Starlight is primary)
docs/              Ohara's own documentation (dogfood)
```

## Development

```bash
cd apps/cli
go build -o ohara .
./ohara --help
```

## Adding a CLI Command

1. Create `apps/cli/cmd/{name}.go`
2. Define a `cobra.Command`
3. Call `rootCmd.AddCommand()` in `init()`
4. Run `go build -o ohara .` to test

## Key Files

- `cmd/init.go` — Creates hub + agents + skills + hooks + MCP config
- `cmd/skills.go` — Defines all subagents and skills
- `cmd/upgrade.go` — Updates agent infra without touching docs
- `cmd/buildsite.go` — Generates Starlight content from hub
- `cmd/hooks.go` — CLI commands called by Claude Code hooks
- `cmd/playbooks.go` — Starter playbook definitions
- `cmd/hub.go` — Hub config types and helpers
- `cmd/serve.go` — Local MCP server (stdio)

## Releasing

```bash
git tag v0.X.0
git push origin v0.X.0
cd apps/cli && GITHUB_TOKEN=$(gh auth token) goreleaser release --clean
```

Homebrew tap at github.com/Lackech/homebrew-ohara (public).
