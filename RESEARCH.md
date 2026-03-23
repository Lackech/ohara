# Ohara: Agent-Optimized Documentation Platform — Product Research Report

**Date:** March 2026
**Status:** Research Complete

---

## Executive Summary

Ohara is a documentation/knowledge library platform for organizations, designed for both AI agents and human developers. Built on the **Diataxis framework** (Tutorials, How-to Guides, Reference, Explanation), it uses **version-controlled directories** (one per service/repo) with **PR-based workflows** where agents both consume and contribute knowledge.

**The core insight:** Documentation should be a living, agent-maintained knowledge layer — not a static artifact that goes stale. Agents read it to become experts on your org, and they write back to keep it current.

### Key Findings

| Area | Finding |
|------|---------|
| **Market** | $6.3B docs tools market + $7.6B AI agents market, both growing fast. 18-24 month window before incumbents close the gap. |
| **Positioning** | "Context engineering platform" not "documentation tool." The knowledge layer for your agent team. |
| **Business Model** | Open-core + usage-based agent queries. $29-59/user/mo SaaS with metered agent API. |
| **Revenue** | $120K ARR Year 1 → $5.6M ARR Year 3 (conservative). Aggressive: $20M Year 3. |
| **Architecture** | Git monorepo, TypeScript backend (Elysia), Next.js frontend, Go CLI, PostgreSQL + pgvector, MCP server. |
| **MVP** | 16-22 weeks, 2-3 engineers, ~$110/mo infrastructure. |
| **Biggest Risk** | Competition from GitHub Copilot Spaces, Mintlify (acquiring RAG companies), and Confluence + Rovo AI. |
| **Biggest Moat** | Agent-as-contributor flywheel: agents read docs → do better work → contribute back → docs improve → agents get smarter. |

---

## 1. The Diataxis Foundation

### What Is Diataxis?

Diataxis organizes documentation into four types along two axes:

|  | **Study** (Learning) | **Work** (Applying) |
|---|---|---|
| **Action** (Doing) | **Tutorials** — Guided learning experiences | **How-to Guides** — Steps to solve real problems |
| **Cognition** (Understanding) | **Explanation** — Why things work the way they do | **Reference** — Technical facts and specifications |

**Why it matters for agents:** Each type maps to a distinct agent need:

| Diataxis Type | Agent Need |
|---|---|
| Tutorial | Onboarding to an unfamiliar codebase/library |
| How-to Guide | Executing a task with step-by-step instructions |
| Reference | Accessing accurate API signatures, config schemas, types |
| Explanation | Making architectural decisions aligned with project intent |

**Key insight:** Current agent instruction files (CLAUDE.md, AGENTS.md) are flat and untyped. They dump everything into the context window regardless of what task the agent is performing. Diataxis-typed documentation enables **selective retrieval** — serve how-to guides when an agent needs to execute a task, serve explanation when it needs to make a design decision.

### The Agent Instruction Ecosystem Today

| File | Tool | Adoption |
|---|---|---|
| `AGENTS.md` | Universal (60+ tools) | 60,000+ repos, Linux Foundation governed |
| `CLAUDE.md` | Claude Code | Hierarchical (home/project/subdir) |
| `.cursor/rules/*.mdc` | Cursor | Granular activation modes |
| `.github/copilot-instructions.md` | GitHub Copilot | Native GitHub integration |
| `GEMINI.md` | Gemini CLI | Memory inspection support |

**Problem:** These are all flat, static files with no semantic typing, no dynamic retrieval, and context window pressure from loading everything on every interaction.

**Ohara's opportunity:** Auto-generate these files from a structured, Diataxis-typed knowledge base — and provide an MCP server for dynamic, task-specific retrieval.

---

## 2. Architecture: How It Works

### Four-Layer Architecture

```
Layer 4: Human Interface
         Rendered docs site (Next.js/Fumadocs), search, versioning
                              ▲
Layer 3: Agent Interface
         MCP server, llms.txt, AGENTS.md generation, REST API
                              ▲
Layer 2: Semantic Layer
         Knowledge graph from frontmatter, embeddings, hybrid search
                              ▲
Layer 1: Source Layer (Docs-as-Code)
         Markdown/MDX in Git, YAML frontmatter, Diataxis-typed, CI-validated
```

### Storage: Git Monorepo

**Git is the source of truth.** Documentation lives in repositories as Markdown files, organized by service:

```
knowledge-library/
  _meta/
    schema.json              # Document structure schema
    style-guide.md           # Writing standards
    service-registry.yaml    # Catalog of all services
  services/
    auth-service/
      tutorials/
        getting-started.mdx
      guides/
        deploy-to-prod.mdx
      reference/
        api.mdx              # Auto-generated from OpenAPI
        config.mdx
      explanation/
        architecture.mdx
        decisions/
          001-jwt-tokens.md   # ADRs
      manifest.yaml           # Links to code repo, ownership
      changelog.md
    payment-service/
      ...
  shared/
    glossary.md
    architecture-overview.md
```

**Why Git wins** over every alternative (Dolt, CRDTs, S3, Confluence):
- Full version history with commit semantics (what changed, why, by whom)
- Native branching/merging and PR review workflows
- Every clone is a complete, portable copy
- Markdown files are the most agent-friendly format
- Agents interact via REST API — no local clone needed
- Familiar workflow for every developer

**Platform choice:** GitHub (preferred for ecosystem), GitLab (for self-hosting), or Forgejo (lightweight self-hosting). Support all three from day one.

### Agent Interaction: PR-Based Workflow

```
Agent detects need for documentation update
  → Creates feature branch (docs/service-x/update-api-reference)
  → Commits changes via API (no local clone needed)
  → Opens PR with structured description:
      - What changed and why
      - Source reference (link to code change, API spec)
      - Confidence level / areas needing human review
  → CI runs automated validation (lint, links, schema, spelling)
  → Human reviewer approves or requests changes
  → Merge on approval
```

Patterns borrowed from **Dependabot/Renovate**: configuration-driven, scheduled, grouped, with automatic pausing when maintainers ignore PRs.

---

## 3. Technical Stack

### Recommended Stack

```
┌─────────────────────────────────────────────────────┐
│                   OHARA TECH STACK                   │
├─────────────────────────────────────────────────────┤
│  FRONTEND                                           │
│  ├── Next.js 15+ (App Router, RSC)                  │
│  ├── Tailwind CSS v4 + shadcn/ui                    │
│  ├── Fumadocs (documentation UI framework)          │
│  └── MDX + remark/rehype pipeline                   │
│                                                     │
│  BACKEND                                            │
│  ├── Elysia (Bun-native HTTP framework)         │
│  ├── TypeScript end-to-end                          │
│  ├── Drizzle ORM (SQL-first, type-safe)             │
│  ├── Zod (validation + OpenAPI generation)          │
│  ├── BullMQ (background jobs)                       │
│  ├── Vercel AI SDK (LLM abstraction)                │
│  └── MCP TypeScript SDK (agent protocol)            │
│                                                     │
│  CLI                                                │
│  ├── Go + Cobra (single binary, cross-platform)     │
│  └── Embedded preview server                        │
│                                                     │
│  DATA                                               │
│  ├── Git (content source of truth)                  │
│  ├── PostgreSQL via Neon (metadata + vectors)       │
│  ├── pgvector (semantic search, up to ~10M vectors) │
│  ├── Cloudflare R2 (object storage, zero egress)    │
│  └── Upstash Redis (cache + queue)                  │
│                                                     │
│  INFRASTRUCTURE                                     │
│  ├── Vercel (frontend hosting)                      │
│  ├── Fly.io (API server)                            │
│  ├── Cloudflare (CDN + DNS + R2)                    │
│  ├── GitHub Actions (CI/CD)                         │
│  └── Sentry + Axiom (observability)                 │
└─────────────────────────────────────────────────────┘
```

### Key Architectural Decisions

1. **Modular monolith** — Clear domain boundaries (Content, Git, Search, Auth, Billing, Agent, Integrations) in a single deployment. Microservices are overkill for <5 person team.

2. **Elysia on Bun** — End-to-end type safety with automatic OpenAPI generation, Eden Treaty for type-safe client, and Bun's native performance. Production-ready with built-in validation, lifecycle hooks, and plugin ecosystem.

3. **Git is storage, PostgreSQL is serving** — Content is parsed during sync and stored in the database. No disk reads on request. This separates the write path (Git) from the read path (DB + search).

4. **pgvector inside PostgreSQL** — Eliminates a separate vector database. Sufficient for ~10M vectors. Saves $70-200/mo. Upgrade to Qdrant only if exceeding 100+ QPS vector search.

5. **Hybrid search** — Full-text (PostgreSQL `tsvector`) + semantic (pgvector) combined via Reciprocal Rank Fusion. Diataxis-aware boosting (if agent asks "how do I...", boost guides).

6. **MCP server as primary agent interface** — One implementation reaches Claude Code, Cursor, GitHub Copilot, Windsurf, OpenAI ChatGPT, and any MCP-compatible client.

7. **Go CLI** — Single binary, ~10ms startup, cross-compiles to all platforms. Cobra framework (same as `gh`, `docker`, `kubectl`).

### MCP Server Design

```
Tools:
  search_documentation(query, filters?)   → Search across all docs
  get_document(path, project?)            → Retrieve a specific document
  list_documents(project?, type?)         → List available documents
  get_api_reference(service, endpoint?)   → Get API reference details
  get_changelog(service, since?)          → Recent changes
  create_doc_pr(service, content, title)  → Create a PR with doc changes

Resources:
  docs://{project}/{path}                 → Individual document content
  docs://{project}/llms.txt               → LLM-optimized summary
  docs://{project}/reference/             → All reference docs

Prompts:
  troubleshoot(service, error_message)    → Guided troubleshooting
  explain_concept(topic, level)           → Conceptual explanation
```

### RAG Pipeline

```
INGESTION: Git Sync → Parse MDX → Chunk (Diataxis-aware) → Embed → Store
                                    ↓
                        Each chunk carries:
                        - Diataxis type
                        - Heading hierarchy
                        - Service name
                        - Version
                        - Content hash (for incremental re-embedding)

RETRIEVAL: Agent Query → Embed → Hybrid Search → Rank (RRF) → Return
                                   ↓         ↓
                             Vector Search  Full-Text
                             (pgvector)     (PostgreSQL)
```

---

## 4. Commercial Viability

### Market Size

| Metric | Estimate | Rationale |
|--------|----------|-----------|
| **TAM** | $8-12B by 2028 | Intersection of docs tools ($12B) and AI agent tooling ($50B+) |
| **SAM** | $1.5-3B by 2028 | Engineering teams using AI agents needing structured knowledge |
| **SOM** | $50-150M by 2030 | Achievable with strong positioning in the agent-native docs niche |

Key market signals:
- Documentation tools market: **$6.32B** (2024), growing at 8.12% CAGR
- AI agents market: **$7.63B** (2025), growing at **49.6% CAGR**
- 57% of companies have AI agents in production
- 95% of developers use AI tools weekly
- 64% of developers use AI for documentation (Google DORA 2025)
- Enterprises plan $10-50M for agentic AI infrastructure

### Competitive Landscape

| Competitor | Strengths | Weakness vs Ohara |
|---|---|---|
| **Mintlify** ($10M ARR, $88M valuation) | AI features, acquired Trieve (RAG) + Helicone | No local-first workflow, no Diataxis enforcement |
| **GitBook** | Polished editor, Git sync | Limited AI/agent support |
| **GitHub Copilot Spaces** | Native GitHub integration, Microsoft enterprise sales | Locked to GitHub ecosystem |
| **Confluence + Rovo AI** | Massive installed base, $6-11/user | Terrible DX, no agent-native design |
| **Dust.tt** ($6M ARR) | Agent builder + knowledge layer | Not docs-focused, no Diataxis structure |
| **Glean** ($100M+ ARR, $7.2B valuation) | Enterprise AI search | Not developer-focused, proprietary |

**Competitive gap:** No existing platform combines Diataxis structure enforcement, full Git-native storage, local-first CLI workflow, and agent-optimized APIs with MCP support.

### Recommended Business Model: Open-Core + Usage-Based

**Stream 1: SaaS Subscriptions**

| Tier | Price | Target | Key Features |
|------|-------|--------|--------------|
| **Community** | Free / OSS | Individuals, OSS | Core engine, Diataxis templates, public repos, basic MCP server, CLI |
| **Team** | $29/user/mo | 2-20 people | Private repos, PR workflows, 5 agent integrations, basic analytics |
| **Business** | $59/user/mo | 20-100 people | SSO, advanced permissions, unlimited integrations, agent analytics |
| **Enterprise** | Custom ($15K-80K/yr) | 100+ | SLA, dedicated instance, audit logs, SCIM/SAML, data residency |

**Stream 2: Usage-Based Agent Queries (the unique revenue stream)**

| Metric | Price |
|--------|-------|
| Agent queries | First 10K/mo included in paid plans; $0.002-0.005/query after |
| Context retrieval volume | Tiered by tokens retrieved |
| Webhook/sync events | First 50K/mo included; overages billed |

**Stream 3: Marketplace (longer-term)**
- Diataxis documentation templates ($10-50 each)
- Pre-built integration connectors
- Professional migration services

### Revenue Projections

| Metric | Year 1 | Year 2 | Year 3 |
|--------|--------|--------|--------|
| Free users | 2,000 | 10,000 | 30,000 |
| Paying teams | 50 | 250 | 800 |
| Avg revenue/team/mo | $200 | $350 | $500 |
| Team ARR | $120K | $1.05M | $4.8M |
| Enterprise deals | 0 | 5 | 20 |
| Enterprise ARR | $0 | $125K | $800K |
| **Total ARR** | **$120K** | **$1.175M** | **$5.6M** |

**Aggressive scenario** (strong PMF, viral MCP adoption): $2M → $8M → $20M.

**Comparable:** Mintlify went from $0 to $10M ARR in ~3 years with an $88.4M valuation.

### The Pitch

- **For investors:** "We're building the knowledge infrastructure that AI agents need to do their jobs — a $50B market growing at 46% CAGR where no one owns the 'context layer' yet."
- **For developers:** "Your AI agents are hallucinating about your codebase because they lack context. We fix that with structured, version-controlled documentation agents can query via MCP."
- **For enterprise:** "Reduce the 3-10 hours/developer/week lost to searching for information, and make your $10M+ AI agent investment actually work."

---

## 5. Integrations & CI/CD

### Priority Integration Roadmap

**Phase 1 — Foundation (Weeks 1-4):**
1. GitHub Actions CI/CD — Validation, build, deploy
2. MCP Server (read-only) — `search_docs`, `get_service_docs`, `get_changelog`
3. llms.txt / llms-full.txt generation
4. Conventional Commits + release-please — Changelog pipeline

**Phase 2 — Core (Weeks 5-8):**
5. Slack notifications — Doc changes, staleness warnings, changelog summaries
6. Confluence one-way sync — Using `mark` or `md2conf` in CI/CD
7. MCP Server (write tools) — `create_doc_pr`, `update_doc_page`
8. OpenAPI/Swagger integration — Auto-generate API docs

**Phase 3 — Ecosystem (Weeks 9-12):**
9. Jira integration — Remote links, automated doc tasks, coverage tracking
10. Backstage TechDocs compatibility — MkDocs format, `catalog-info.yaml`
11. Agent context file generation — Auto-generate AGENTS.md, CLAUDE.md per service
12. Notion importer — One-time migration tool

**Phase 4 — Advanced (Weeks 13+):**
13. VS Code extension, 14. Linear/Shortcut, 15. PagerDuty/Datadog, 16. Teams notifications, 17. Dual-format changelogs, 18. Documentation scorecard dashboard

### Confluence Strategy

Position as **"source of truth that syncs TO Confluence"**, not a replacement:
- One-way sync: Markdown in Git → rendered in Confluence (read-only mirror)
- Synced pages carry a banner: "Auto-generated from [repo]. Edit there, not here."
- Selective sync: Only push certain spaces/directories to Confluence

### Changelog Generation

**Recommended:** `release-please` (creates human-reviewable Release PRs) + `git-cliff` (custom templates for dual-format output).

Dual-format changelog — YAML frontmatter for machines, Markdown body for humans:

```markdown
---
version: 2.4.0
date: 2026-03-15
service: payments-api
breaking: false
changes:
  - type: feat
    scope: webhooks
    description: Add retry logic for failed webhook deliveries
    pr: "#423"
---

## [2.4.0] - 2026-03-15

### Features
- **webhooks**: Add retry logic for failed webhook deliveries (#423)
```

---

## 6. Developer Experience & Onboarding

### The 5-Minute Golden Path

| Minute | Action |
|--------|--------|
| 0-1 | `npm install -g @ohara/cli && ohara login` — Install CLI, GitHub SSO in browser |
| 1-2 | `ohara init` — Auto-discovers existing docs, README, OpenAPI specs |
| 2-3 | `ohara import --auto` — Pulls docs into Diataxis structure with AI classification |
| 3-4 | `ohara preview` — Local dev server with hot reload |
| 4-5 | `ohara publish` — Live at `yourorg.ohara.dev/service-name` |

**The "aha moment":** Seeing scattered documentation transformed into a polished, searchable, AI-queryable site in under 5 minutes.

### CLI Design

```
ohara init                      # Initialize docs for current repo
ohara discover                  # Scan for existing documentation
ohara import                    # Import from external sources
  ohara import confluence --space KEY --url URL
  ohara import notion --workspace ID
  ohara import openapi --spec ./openapi.yaml
ohara add tutorial "Getting Started"  # Add a Diataxis-typed page
ohara preview                   # Local dev server with hot reload
ohara validate                  # Lint, check links, verify completeness
ohara analyze                   # AI-powered Diataxis classification + gap analysis
ohara publish                   # Deploy to production URL
ohara mcp serve                 # Start MCP server for agent access
ohara agent-config              # Generate CLAUDE.md, AGENTS.md, .cursorrules
ohara health                    # Documentation health score
```

### Import from Existing Sources

| Source | Method |
|--------|--------|
| **Confluence** | REST API v2 crawling → HTML to Markdown conversion, preserving hierarchy |
| **Notion** | API (structured JSON blocks → Markdown) or ZIP export processing |
| **Markdown in repos** | Auto-discovery of README, /docs, /doc, ADRs, existing framework configs |
| **Google Docs** | Drive API export to Markdown with batch processing |
| **OpenAPI/Swagger** | Auto-generate reference docs, pre-populate how-to stubs |

After import, `ohara analyze` uses an LLM to classify all docs against Diataxis and produce a gap report.

### Why Documentation Tools Fail (And How Ohara Avoids It)

| Failure Pattern | Ohara's Solution |
|---|---|
| Docs go stale (only 6% of engineers update daily) | Agent-generated PRs keep docs in sync with code changes |
| No immediate personal value | Agent-queryable docs save devs time answering questions |
| Documentation is a separate task | Doc creation integrated into service creation workflow |
| Poor discoverability | Structured directories + semantic search + MCP |
| Requires dedicated maintenance team (Backstage trap) | Everything is automatable, CI-validated, agent-maintained |

---

## 7. Risks & Mitigations

### Risk Matrix

| Risk | Severity | Likelihood | Mitigation |
|------|----------|-----------|------------|
| **Big player competition** (GitHub Spaces, Mintlify acquisitions, Confluence+Rovo) | HIGH | HIGH | Niche focus, open-source moat, agent-contributor flywheel, multi-platform support |
| **Context windows eliminate need** (models getting bigger) | MEDIUM | LOW | Larger windows make RIGHT context selection MORE important; RAG still 30-60x faster and cheaper than filling 1M token window |
| **Adoption friction** (changing doc habits) | MEDIUM-HIGH | HIGH | AI migration tools, "overlay" approach on existing docs, agents write the docs |
| **Platform dependency** (GitHub, MCP, AI providers) | HIGH | MEDIUM | Multi-platform from day one, REST API fallback, provider-agnostic AI SDK |
| **MCP protocol evolution** | MEDIUM | MEDIUM | Official SDK tracks changes, abstracted behind interface, REST API as stable fallback |
| **Search quality** | MEDIUM | MEDIUM | Curated eval set, RRF tuning, Diataxis-aware boosting, re-ranking with cross-encoder |

### The 18-24 Month Window

The opportunity window is real but closing:
- Mintlify acquired Trieve (RAG) and Helicone (observability) — building exactly this stack
- GitHub Copilot Spaces provides "good enough" for GitHub-native teams
- Confluence + Rovo AI bundles agents into existing $6-11/user plans

**Fast execution is critical.** The competitive moat is the **agent-as-contributor flywheel** (agents read → do better work → contribute back → docs improve → agents get smarter) — this is hard to replicate and creates compounding value.

---

## 8. MVP Scope

### What to Build (16-22 weeks, 2-3 engineers)

| Feature | Complexity | Time |
|---------|-----------|------|
| Git content ingestion (GitHub webhook + sync) | Medium | 2-3 weeks |
| MDX parsing and rendering pipeline | Medium | 2-3 weeks |
| Diataxis directory structure validation | Low | 1 week |
| Documentation web UI (Fumadocs-based) | Medium | 2-3 weeks |
| Full-text search (PostgreSQL) | Low | 1 week |
| Semantic search (pgvector) | Medium | 1-2 weeks |
| MCP server (search, get_document, list) | Medium | 2 weeks |
| llms.txt auto-generation | Low | 2-3 days |
| REST API for agents | Medium | 2 weeks |
| CLI: init, dev, validate, login | Medium | 3-4 weeks |
| Auth (GitHub OAuth) | Low | 1 week |
| Project dashboard + onboarding | Low | 2 weeks |

### What to Defer

| Feature | Phase | Reason |
|---------|-------|--------|
| WYSIWYG web editor | Phase 2 | Complex, users can edit in IDE |
| Real-time collaboration | Phase 3 | Very complex (CRDTs), not core |
| Billing/subscription | Phase 2 | Free until PMF |
| GitLab integration | Phase 2 | Start GitHub-only |
| Self-hosted option | Phase 3 | Enterprise requirement |
| AI writing assistant | Phase 3 | Focus on consumption first |

### Infrastructure Cost

| Phase | Monthly | Annual |
|-------|---------|--------|
| Development (months 1-4) | ~$50 | ~$200 |
| Beta (months 5-7) | ~$80 | ~$240 |
| Launch (months 8-12) | ~$200 | ~$1,000 |
| **Year 1 Total** | | **~$1,440** |

---

## 9. Go-to-Market Strategy

### Phase 1: Foundation (Months 0-6) — Target: 1,000 stars, 500 users, 50 paying teams

- Launch open-source CLI and core engine on GitHub
- Ship MCP server as first-class integration (distribution channel — agents discover you)
- Diataxis-structured template library
- "Migration wizard" for Notion/Confluence/Markdown → Diataxis conversion
- Content marketing: *"Why your AI agents are hallucinating about your codebase"*

### Phase 2: Growth (Months 6-18) — Target: 5,000 users, 200 teams, $500K ARR

- Team and Business tiers
- GitHub integration (PR contributions), VS Code extension, Cursor integration
- Partnerships with AI coding tool vendors
- Agent query analytics dashboard
- Developer advocacy: conferences, blogs, case studies

### Phase 3: Scale (Months 18-36) — Target: 1,000 teams, $5M+ ARR, Enterprise deals

- Enterprise tier (SOC2, SSO, SCIM)
- Marketplace for templates and integrations
- Non-engineering expansion (product docs, support KB)
- Platform partnerships (Anthropic, OpenAI, GitHub)

### Strategic Positioning

**DO:** Position as a context engineering platform. "The knowledge layer for your agent team."

**DON'T:** Compete as a general documentation tool. That market is crowded.

**Win on three differentiators:**
1. **Agent teams as first-class citizens** — Not "docs with AI bolted on" but a platform where agent teams are the primary contributors and consumers. Your team of agents matures naturally as the knowledge base grows — like raising plants to their full potential. Every task they complete, every PR they merge, feeds back into the library making the whole team smarter.
2. **Diataxis-native structure** — The only tool that enforces a framework designed for both human comprehension and machine retrieval. Agents can request exactly the type of knowledge they need for the task at hand.
3. **Agent-as-contributor flywheel** — Agents submit PRs to improve documentation, creating a self-improving knowledge base. The more your agents work, the more expert they become on your org. This compounds — Mintlify and others focus on humans writing docs for agents to read. We focus on agents writing AND reading, with humans as reviewers and curators.

---

## 10. Key Strategic Decisions to Make

1. **Name:** "Ohara" — nice ring to it. Check trademark availability.

2. **Open source vs proprietary core:** Strong recommendation for open-source core + commercial features. OSS is the distribution strategy.

3. **GitHub-first vs multi-platform:** Start GitHub-only for MVP (80%+ of target users), add GitLab in Phase 2.

4. **CLI-first vs web-first:** CLI-first for developers, web UI for non-technical contributors. Both needed, CLI ships first.

5. **Import-first vs create-first onboarding:** Import-first. Most orgs have existing docs. The value is transforming and structuring them, not starting from scratch.

6. **Self-hosted vs cloud-only:** Cloud-only for MVP. Self-hosted in Phase 3 for enterprise.

---

## Sources

*Full citations are available in the individual research reports. Key sources include:*

- Diataxis Framework — diataxis.fr
- AGENTS.md Standard — agents.md (Linux Foundation)
- MCP Protocol — modelcontextprotocol.io (Anthropic → Linux Foundation)
- Software Documentation Tools Market — Verified Market Reports (2024)
- AI Agents Market — Grand View Research, Fortune Business Insights (2025)
- Mintlify Financials — Sacra ($10M ARR, $88.4M valuation)
- Glean Financials — $150M Series F at $7.2B valuation
- Dust.tt — $6M ARR, open-source agent workspace
- llms.txt Standard — llmstxt.org (Jeremy Howard, 2024)
- DORA 2025 Report — Google Cloud (64% of devs use AI for docs)
- Context Engineering — Anthropic engineering blog
- Backstage — backstage.io (Spotify)
- Fumadocs — Next.js documentation framework
- Elysia — Bun-native HTTP framework
- Drizzle ORM, pgvector, Neon, Vercel AI SDK, MCP TypeScript SDK
