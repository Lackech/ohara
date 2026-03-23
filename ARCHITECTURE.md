# Ohara Project: Technical Architecture Document

## Agent-Optimized Documentation Platform

**Version:** 1.0 — March 2026
**Status:** Architecture Proposal

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Competitive Landscape Analysis](#2-competitive-landscape-analysis)
3. [Core Architecture](#3-core-architecture)
4. [Backend Stack](#4-backend-stack)
5. [Frontend Stack](#5-frontend-stack)
6. [Storage and Database](#6-storage-and-database)
7. [AI/Agent Infrastructure](#7-aiagent-infrastructure)
8. [Search and Indexing](#8-search-and-indexing)
9. [CLI Tool Architecture](#9-cli-tool-architecture)
10. [Infrastructure and Deployment](#10-infrastructure-and-deployment)
11. [Recommended Stack Summary](#11-recommended-stack-summary)
12. [Complexity Assessment and MVP Scope](#12-complexity-assessment-and-mvp-scope)
13. [Phased Implementation Plan](#13-phased-implementation-plan)
14. [Key Technical Risks and Mitigations](#14-key-technical-risks-and-mitigations)
15. [Cost Projections](#15-cost-projections)

---

## 1. Executive Summary

Ohara is a documentation/knowledge library platform grounded in the Diataxis framework, designed to serve both AI agents and human developers. It uses version-controlled directories (one per service/repo), supports local and cloud usage, PR-based workflows, and integrations with dev tools.

### Diataxis Foundation

The platform structures all documentation around four types:
- **Tutorials** — Learning-oriented, guided experiences
- **How-to Guides** — Task-oriented, practical steps
- **Reference** — Information-oriented, precise technical descriptions
- **Explanation** — Understanding-oriented, conceptual discussions

This framework is not just a content organizing principle — it shapes the data model, the API surface for agents, and the search/retrieval strategy.

### Core Differentiators
1. **Agent-first architecture** — Every API endpoint, data structure, and content format is designed for machine consumption alongside human readability
2. **Git-native storage** — Documentation lives in repositories alongside code, not in a proprietary database
3. **Diataxis-structured** — Enforced content taxonomy enables precise retrieval (agents can ask for "the tutorial on X" vs. "the reference for Y")
4. **Local-first workflow** — Full preview and authoring from CLI, with cloud sync for collaboration

---

## 2. Competitive Landscape Analysis

### GitBook
- **Model:** Cloud-hosted documentation platform with Git sync
- **Stack:** React frontend, proprietary backend
- **Strengths:** Polished editor, Git integration, spaces/collections model
- **Weaknesses:** Limited AI agent support, no CLI authoring, no Diataxis structure
- **Pricing:** Free tier available, Team plan at ~$8/user/month

### Mintlify
- **Model:** Developer documentation with AI-native features
- **Stack:** Next.js + Astro, Cloudflare Workers for edge caching, ClickHouse for analytics
- **AI Features:** Acquired Trieve for RAG search, auto-generates llms.txt and MCP servers, skill.md standard
- **Strengths:** Fastest-growing in the space, strong AI story, eliminated cold starts for 72M monthly page views via edge caching
- **Weaknesses:** Cloud-only, limited local workflow, no Diataxis enforcement
- **Pricing:** Free hobby tier, $250/month Pro, custom Enterprise

### ReadMe
- **Model:** API documentation platform with interactive features
- **Stack:** API-first with bi-directional Git sync (GitHub/GitLab)
- **AI Features:** AI Linter, semantic search, Ask AI chat, MCP server generation from API docs
- **Strengths:** Best-in-class API reference rendering, interactive API explorer
- **Weaknesses:** API-doc focused (not general knowledge), expensive, limited agent access
- **Pricing:** Premium pricing, enterprise-focused

### Key Takeaway

The competitive gap Ohara can exploit: **no existing platform combines Diataxis structure, full Git-native storage, local-first CLI workflow, and agent-optimized APIs with MCP support.** Mintlify is the closest competitor on the AI axis, but lacks local-first workflow and structural enforcement.

---

## 3. Core Architecture

### 3.1 Architecture Pattern: Modular Monolith

**Recommendation: Modular monolith with clear domain boundaries, designed for eventual extraction.**

| Approach | Pros | Cons | Verdict |
|----------|------|------|---------|
| Monolith | Simple deployment, easy debugging, fast development | Harder to scale independently, tech debt risk | Too unstructured |
| Modular Monolith | Clear boundaries, single deployment, easy refactoring, team-size appropriate | Requires discipline in module boundaries | **Recommended** |
| Microservices | Independent scaling, tech diversity | Massive operational overhead, premature for <5 person team | Overkill for MVP |

The modular monolith should be structured around these domain modules:
- **Content** — Parsing, rendering, validation, Diataxis classification
- **Git** — Repository management, sync, PR workflows
- **Search** — Full-text and semantic search, indexing pipeline
- **Auth** — Authentication, authorization, team management
- **Billing** — Subscription management, usage tracking
- **Agent** — MCP server, agent API, RAG pipeline
- **Integrations** — Webhooks, GitHub/GitLab apps, Slack notifications

### 3.2 Server Architecture: Hybrid (Server + Serverless Edge)

**Recommendation: Long-running server for core API + edge functions for documentation delivery.**

```
                                    ┌─────────────────┐
                                    │   CDN / Edge     │
                                    │  (Doc Delivery)  │
                                    └────────┬────────┘
                                             │
┌──────────┐     ┌──────────────┐    ┌───────┴────────┐     ┌──────────────┐
│  CLI      │────▶│  API Gateway  │───▶│  Core Server   │────▶│  PostgreSQL   │
│  Tool     │     │  (Elysia/Bun) │    │  (Node.js)     │     │  + pgvector   │
└──────────┘     └──────────────┘    └───────┬────────┘     └──────────────┘
                                             │
┌──────────┐     ┌──────────────┐    ┌───────┴────────┐     ┌──────────────┐
│  Web UI   │────▶│  Next.js      │    │  Background     │────▶│  Redis/Valkey │
│  (Browser)│     │  (App Router) │    │  Workers        │     │  (Cache/Queue)│
└──────────┘     └──────────────┘    └───────┬────────┘     └──────────────┘
                                             │
┌──────────┐     ┌──────────────┐    ┌───────┴────────┐     ┌──────────────┐
│  AI Agent │────▶│  MCP Server   │    │  Git Worker     │────▶│  Object Store │
│  (Claude) │     │  (Streamable  │    │  (Clone/Sync)   │     │  (R2/S3)      │
└──────────┘     │   HTTP)       │    └────────────────┘     └──────────────┘
                 └──────────────┘
```

**Rationale:**
- Long-running Bun/Elysia server handles Git operations, WebSocket connections, and background processing (these are poorly suited to serverless cold starts)
- Edge functions handle documentation page delivery (read-heavy, cacheable, globally distributed)
- MCP server runs as a Streamable HTTP endpoint on the Elysia server (agents need stateful connections)

### 3.3 Event-Driven Integration Layer

For integrations (GitHub webhooks, CI/CD triggers, Slack notifications), use an event-driven pattern:

```
Webhook/Event ──▶ Event Queue (Redis Streams / BullMQ) ──▶ Event Handlers
```

This decouples integration processing from the request cycle and provides retry semantics, dead-letter queues, and observability. At MVP scale, BullMQ on Redis is sufficient (no need for Kafka or dedicated message brokers).

---

## 4. Backend Stack

### 4.1 Primary Language: TypeScript (Bun)

**Recommendation: TypeScript end-to-end, with Elysia as the HTTP framework on Bun.**

| Option | Strengths | Weaknesses | Verdict |
|--------|-----------|------------|---------|
| Bun/TypeScript + Elysia | End-to-end type safety, Eden Treaty (type-safe client), built-in OpenAPI, Bun-native performance, lifecycle hooks, plugin ecosystem | Bun ecosystem still maturing | **Primary choice** |
| Node.js/TypeScript + Hono | Runs everywhere (edge, Node, Bun, Deno), large middleware ecosystem | Less type-safe than Elysia, no Eden Treaty | Strong alternative |
| Node.js/TypeScript + Fastify | Mature, battle-tested, great plugin system | Heavier, less modern DX | Fallback option |
| Go | Excellent for CLI and concurrent workloads, great binary distribution | Two-language overhead, different type system | **Use for CLI only** |
| Rust | Peak performance for parsing/indexing | High development cost, small team can't afford | Defer to post-MVP |
| Python | Best AI/ML ecosystem | Third language adds overhead, slower runtime | Use only if AI needs demand it |

**Why Elysia over Hono/Express/Fastify/Next.js API routes:**
- **End-to-end type safety** — Elysia infers types from route definitions through to the client via Eden Treaty, eliminating an entire class of runtime errors
- **Bun-native performance** — Built specifically for Bun, leveraging its HTTP server for superior throughput
- **Built-in OpenAPI/Swagger** — Auto-generates OpenAPI spec from route schemas without additional plugins
- **Lifecycle hooks** — `onRequest`, `onParse`, `onBeforeHandle`, `onAfterHandle`, `onError` provide fine-grained control over the request pipeline
- **Plugin ecosystem** — Auth, CORS, rate limiting, JWT, and more via first-party and community plugins
- **Eden Treaty** — Type-safe HTTP client generated from server routes, perfect for the CLI and frontend to call the API with full type inference

**Why not Next.js API routes for the core API:**
- Next.js API routes are excellent for BFF (Backend for Frontend) patterns but insufficient for a standalone API that serves CLI tools, MCP servers, and third-party integrations
- Coupling the API lifecycle to the frontend deployment creates scaling and versioning problems
- Use Next.js API routes only for frontend-specific concerns (session management, UI data fetching)

### 4.2 Runtime: Bun

- **Production and Development:** Bun — fast startup, built-in TypeScript support, native test runner, fast package management
- Elysia is Bun-native and takes full advantage of Bun's HTTP server internals
- For any library that requires Node.js compatibility, Bun's Node.js compatibility layer handles it

### 4.3 Key Backend Libraries

| Concern | Library | Rationale |
|---------|---------|-----------|
| HTTP Framework | Elysia | Bun-native, end-to-end type safety, Eden Treaty, built-in OpenAPI |
| Type-safe Client | Eden Treaty (Elysia) | Auto-generated type-safe HTTP client from server routes |
| Validation | Elysia's built-in (TypeBox) + Zod | Elysia uses TypeBox natively; Zod for shared schemas with frontend |
| ORM | Drizzle ORM | TypeScript-native, SQL-first, lighter than Prisma, supports pgvector |
| Auth | Better Auth or Auth.js v5 | OAuth, magic links, passkeys, session management |
| Background Jobs | BullMQ | Redis-backed, reliable, retries, cron, rate limiting |
| Git Operations | isomorphic-git + simple-git | isomorphic-git for browser/programmatic, simple-git for server-side |
| Markdown Processing | unified/remark/rehype | Industry standard, extensible pipeline |
| AI SDK | Vercel AI SDK | Multi-provider (OpenAI, Anthropic, etc.), streaming, tool calling, embeddings |
| Rate Limiting | @upstash/ratelimit or custom | Token bucket for API, sliding window for auth |
| Logging | Pino | Fast structured JSON logging |

---

## 5. Frontend Stack

### 5.1 Framework: Next.js (App Router)

**Recommendation: Next.js 15+ with App Router, React Server Components.**

Next.js is the clear choice because:
- Server Components for fast initial page loads (critical for documentation)
- Streaming SSR for progressive loading
- File-system routing maps naturally to documentation structure
- Static generation (ISR) for documentation pages
- Strong MDX ecosystem integration
- Mature deployment on Vercel or self-hosted

### 5.2 Documentation Rendering Pipeline

```
Markdown/MDX Source (Git)
        │
        ▼
┌─────────────────┐
│  remark plugins  │  ← GFM, frontmatter, math, admonitions
│  (parse + transform) │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  rehype plugins  │  ← syntax highlighting (Shiki), heading IDs, link validation
│  (HTML transform)│
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  MDX compilation │  ← JSX components, interactive elements
│  (React render)  │
└────────┬────────┘
         │
         ▼
   Rendered Page
```

**Key remark/rehype plugins:**
- `remark-gfm` — GitHub Flavored Markdown (tables, task lists)
- `remark-frontmatter` + `remark-mdx-frontmatter` — YAML metadata
- `remark-math` + `rehype-katex` — Mathematical notation
- `rehype-shiki` — Syntax highlighting with theme support
- `rehype-slug` + `rehype-autolink-headings` — Heading anchors
- Custom Diataxis validation plugin — Validates content structure against Diataxis rules

**Recommended approach:** Use Fumadocs as the foundation. It is a Next.js-native documentation framework that provides MDX processing, search (Orama built-in, Algolia optional), navigation, and component library. It saves 2-3 months of building basic documentation UI from scratch. Customize and extend it rather than building from zero.

### 5.3 UI Component Library

| Option | Recommendation |
|--------|---------------|
| Component primitives | shadcn/ui (copy-paste, customizable, Tailwind-based) |
| Styling | Tailwind CSS v4 |
| Icons | Lucide React |
| Code editor (for inline editing) | CodeMirror 6 or Monaco (if full editor needed) |
| Rich text editor | Tiptap or Plate (for WYSIWYG markdown editing) |

### 5.4 Search UI

For the frontend search experience:
- **Instant search** with keyboard navigation (`cmdk` or custom)
- **Faceted search** by Diataxis type, service, version
- **AI-powered "Ask" mode** for natural language queries against docs

### 5.5 Real-time Collaboration (Post-MVP)

Real-time collaboration is complex and should be deferred. When implemented:
- **Yjs** or **Automerge** for CRDT-based collaborative editing
- **PartyKit** or **Liveblocks** for managed real-time infrastructure
- WebSocket transport via the core server

---

## 6. Storage and Database

### 6.1 Git as Primary Content Storage

**Git is the source of truth for all documentation content.** This is a core architectural principle.

```
┌─────────────────────────────────┐
│  GitHub/GitLab (Remote)         │
│  ┌───────────────────────────┐  │
│  │  repo-name/               │  │
│  │  ├── ohara.yaml           │  │  ← Project config
│  │  ├── tutorials/           │  │  ← Diataxis: Tutorials
│  │  │   ├── getting-started.mdx│ │
│  │  │   └── advanced-setup.mdx │ │
│  │  ├── guides/              │  │  ← Diataxis: How-to Guides
│  │  │   ├── deploy.mdx       │  │
│  │  │   └── configure-ci.mdx │  │
│  │  ├── reference/           │  │  ← Diataxis: Reference
│  │  │   ├── api.mdx          │  │
│  │  │   └── config.mdx       │  │
│  │  ├── explanation/         │  │  ← Diataxis: Explanation
│  │  │   └── architecture.mdx │  │
│  │  └── assets/              │  │  ← Images, diagrams
│  └───────────────────────────┘  │
└─────────────────────────────────┘
```

**Git integration approach:**
- Server-side: `simple-git` wrapping native Git for clone, pull, push operations
- Worker process clones/syncs repositories on schedule or webhook trigger
- Content is parsed and indexed into PostgreSQL + search index on each sync
- GitHub/GitLab App for PR webhooks, status checks, preview deployments

**Important design decision:** The server does NOT require a local clone to serve documentation. Content is parsed during sync and stored in the database/search index. Git is the storage layer; the database is the serving layer. This avoids the latency and complexity of reading from disk on every request.

### 6.2 PostgreSQL + pgvector

**Recommendation: Neon (serverless PostgreSQL) with pgvector extension.**

PostgreSQL handles:
- User accounts, teams, organizations
- Project/repository metadata
- Billing and subscription data
- Parsed document metadata (title, path, Diataxis type, version, last updated)
- Document chunks for vector search (via pgvector)
- API keys and permissions
- Webhook delivery logs
- Search analytics

**Why Neon over self-managed Postgres:**
- Scale-to-zero reduces costs during low traffic
- Database branching for preview environments (maps to PR-based workflows)
- Autoscaling handles traffic spikes
- Built-in connection pooling
- pgvector extension supported
- Free tier: 0.5 GB storage, always-available compute

**Why pgvector over dedicated vector databases (Pinecone, Weaviate):**
- Eliminates an entire infrastructure component
- Vectors live alongside the metadata they describe (no cross-system joins)
- HNSW indexes provide excellent recall/speed tradeoff for document-scale data
- Sufficient for up to ~10M vectors (well beyond MVP needs)
- Saves $70-200/month on a separate vector DB service

**When to add a dedicated vector DB:** Only if query volume exceeds 100+ QPS on vector search or vector count exceeds 10M. At that point, evaluate Qdrant (open-source, Rust-based, excellent performance).

### 6.3 Object Storage: Cloudflare R2

**Recommendation: Cloudflare R2 for all binary assets.**

- **Zero egress fees** — Documentation platforms are read-heavy; egress costs on S3 can be significant
- S3-compatible API — Use existing AWS SDK tooling
- $0.015/GB-month storage (with 10 GB free)
- Global distribution via Cloudflare's edge network
- Serves images, PDFs, diagrams, and any binary assets referenced in documentation

### 6.4 Cache and Queue: Redis (Upstash or self-hosted)

**Recommendation: Upstash Redis for serverless, or self-hosted Valkey on Fly.io/Railway for cost optimization.**

Redis handles:
- Response caching (rendered pages, API responses, search results)
- Session storage
- Rate limiting (API keys, auth endpoints)
- Background job queue (BullMQ)
- Real-time features (pub/sub for collaborative editing, later)
- Webhook event queue

**Upstash advantages:** Pay-per-request pricing, zero management, global replication, REST API (works from edge). Free tier: 10K commands/day.

**Self-hosted advantages:** Predictable cost at scale, no per-request charges, full Redis feature set.

**MVP recommendation:** Start with Upstash for simplicity; migrate to self-hosted when monthly costs exceed $50.

---

## 7. AI/Agent Infrastructure

This is the primary differentiator. The architecture must treat agents as first-class consumers.

### 7.1 RAG Architecture

```
┌────────────────────────────────────────────────────┐
│                  INGESTION PIPELINE                 │
│                                                     │
│  Git Sync ──▶ Parse MDX ──▶ Chunk ──▶ Embed ──▶ Store │
│                   │            │         │          │
│                   ▼            ▼         ▼          │
│              Metadata     Chunks    Vectors         │
│              (Postgres)  (Postgres) (pgvector)      │
└────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────┐
│                  RETRIEVAL PIPELINE                  │
│                                                     │
│  Agent Query ──▶ Embed Query ──▶ Hybrid Search      │
│                                    │    │           │
│                          ┌─────────┘    └──────┐    │
│                          ▼                     ▼    │
│                     Vector Search         Full-Text │
│                     (pgvector)           (Postgres)  │
│                          │                     │    │
│                          └──────┬──────────────┘    │
│                                 ▼                   │
│                          Rank + Rerank              │
│                                 │                   │
│                                 ▼                   │
│                     Return Structured Response      │
└────────────────────────────────────────────────────┘
```

### 7.2 Document Chunking Strategy

Chunking is critical for retrieval quality. The strategy must be Diataxis-aware:

1. **Structural chunking** — Split on headings (h2/h3 boundaries), preserving hierarchy
2. **Semantic chunking** — Within long sections, split on paragraph boundaries at ~500-800 tokens
3. **Metadata enrichment** — Each chunk carries:
   - Document path, title, and Diataxis type
   - Heading hierarchy (breadcrumb)
   - Service/repository name
   - Version identifier
   - Code block language tags (if any)

```typescript
interface DocumentChunk {
  id: string;
  content: string;           // Raw text content
  contentHtml: string;       // Rendered HTML (for display)
  embedding: number[];       // Vector embedding (1536-dim for OpenAI, 1024 for Cohere)
  metadata: {
    projectId: string;
    documentPath: string;
    diataxisType: 'tutorial' | 'guide' | 'reference' | 'explanation';
    headingHierarchy: string[];  // ["API Reference", "Authentication", "OAuth"]
    serviceName: string;
    version: string;
    language?: string;        // For code blocks
    lastUpdated: Date;
  };
}
```

### 7.3 Embedding Generation

**Recommendation: Vercel AI SDK with OpenAI `text-embedding-3-small` as default.**

- 1536 dimensions, excellent quality-to-cost ratio
- $0.02 per 1M tokens — approximately $0.01 to embed 500 documentation pages
- Support swappable providers via AI SDK (Cohere, Voyage AI for specialized needs)
- Batch embedding during sync, incremental on document change (diff-based)

**Embedding update strategy:**
- On Git sync, compute content hash per chunk
- Only re-embed chunks whose content hash changed
- Store content hash alongside embedding to avoid unnecessary recomputation

### 7.4 MCP Server Implementation

**This is the highest-value agent integration.** An MCP server makes documentation directly accessible to AI agents (Claude, ChatGPT, Cursor, VS Code Copilot, etc.).

**Transport:** Streamable HTTP (for remote access by any MCP client)

**MCP Primitives to expose:**

**Tools:**
```
search_documentation(query, filters?)      → Search across all docs
get_document(path, project?)               → Retrieve a specific document
list_documents(project?, diataxis_type?)   → List available documents
get_api_reference(service, endpoint?)      → Get API reference details
ask_question(question, context?)           → RAG-powered Q&A
```

**Resources:**
```
docs://{project}/{path}                    → Individual document content
docs://{project}/llms.txt                  → LLM-optimized project summary
docs://{project}/reference/                → All reference docs for a service
```

**Prompts:**
```
troubleshoot(service, error_message)       → Guided troubleshooting
explain_concept(topic, audience_level)     → Conceptual explanation
generate_guide(task_description)           → How-to guide template
```

**Implementation:** Use the official MCP TypeScript SDK (`@modelcontextprotocol/sdk`). The MCP server runs as an endpoint on the core API server (e.g., `POST /mcp`), handling the Streamable HTTP transport.

### 7.5 llms.txt Generation

Auto-generate `llms.txt` for every project following the specification:
- H1: Project name
- Blockquote: Project summary
- Sections organized by Diataxis type
- Links to each document with concise descriptions
- Served at `/{project}/llms.txt`

This enables LLMs to discover and navigate documentation even without MCP.

### 7.6 Agent-Friendly API Design

Every API response should be optimized for agent consumption:

```json
{
  "data": {
    "documents": [...],
    "pagination": {
      "cursor": "abc123",
      "hasMore": true,
      "totalCount": 47
    }
  },
  "metadata": {
    "project": "payment-service",
    "diataxisType": "reference",
    "version": "2.1.0",
    "lastSynced": "2026-03-20T10:00:00Z"
  },
  "context": {
    "relatedDocuments": [...],
    "suggestedQueries": [...]
  }
}
```

Key principles:
- **Cursor-based pagination** (not offset) — Agents process sequentially
- **Structured metadata** in every response — Agents need context
- **Related content suggestions** — Helps agents explore without many round trips
- **Content negotiation** — Support `Accept: text/markdown` for raw content, `application/json` for structured data
- **Consistent error format** with actionable messages

### 7.7 LLM Integration Layer

Use the Vercel AI SDK as the abstraction layer:

```typescript
import { generateText, embed } from 'ai';
import { openai } from '@ai-sdk/openai';
import { anthropic } from '@ai-sdk/anthropic';

// Provider-agnostic embedding
const { embedding } = await embed({
  model: openai.embedding('text-embedding-3-small'),
  value: chunkContent,
});

// Provider-agnostic generation (for RAG responses)
const { text } = await generateText({
  model: anthropic('claude-sonnet-4-20250514'),
  system: 'You are a documentation assistant...',
  prompt: buildRAGPrompt(query, retrievedChunks),
});
```

This allows customers to bring their own API keys and choose providers.

---

## 8. Search and Indexing

### 8.1 Search Strategy: Hybrid (Full-Text + Semantic)

**Recommendation: PostgreSQL full-text search + pgvector semantic search, combined with reciprocal rank fusion.**

This avoids adding a separate search infrastructure component at MVP.

| Approach | Use Case | Implementation |
|----------|----------|----------------|
| Full-text search | Exact term matching, code symbols, config keys | PostgreSQL `tsvector` + GIN index |
| Semantic search | Natural language queries, conceptual questions | pgvector HNSW index |
| Hybrid | Best of both — handles exact and fuzzy queries | Reciprocal Rank Fusion (RRF) |

### 8.2 Full-Text Search with PostgreSQL

```sql
-- Document search with tsvector
CREATE INDEX idx_chunks_fts ON document_chunks
  USING GIN (to_tsvector('english', content));

-- Query with ranking
SELECT *, ts_rank(to_tsvector('english', content), query) AS rank
FROM document_chunks, plainto_tsquery('english', 'authentication oauth') query
WHERE to_tsvector('english', content) @@ query
ORDER BY rank DESC
LIMIT 20;
```

### 8.3 Semantic Search with pgvector

```sql
-- HNSW index for fast approximate nearest neighbor
CREATE INDEX idx_chunks_embedding ON document_chunks
  USING hnsw (embedding vector_cosine_ops)
  WITH (m = 16, ef_construction = 64);

-- Semantic search
SELECT *, 1 - (embedding <=> $1) AS similarity
FROM document_chunks
WHERE project_id = $2
ORDER BY embedding <=> $1
LIMIT 20;
```

### 8.4 Hybrid Search with Reciprocal Rank Fusion

```typescript
async function hybridSearch(query: string, projectId: string) {
  const queryEmbedding = await embed(query);

  // Run both searches in parallel
  const [fullTextResults, semanticResults] = await Promise.all([
    fullTextSearch(query, projectId, 20),
    semanticSearch(queryEmbedding, projectId, 20),
  ]);

  // Reciprocal Rank Fusion
  const k = 60; // RRF constant
  const scores = new Map<string, number>();

  fullTextResults.forEach((doc, rank) => {
    scores.set(doc.id, (scores.get(doc.id) || 0) + 1 / (k + rank + 1));
  });

  semanticResults.forEach((doc, rank) => {
    scores.set(doc.id, (scores.get(doc.id) || 0) + 1 / (k + rank + 1));
  });

  return Array.from(scores.entries())
    .sort(([, a], [, b]) => b - a)
    .slice(0, 10);
}
```

### 8.5 Index Management

- **Incremental indexing:** On Git sync, diff changed files, re-index only modified chunks
- **Batch re-indexing:** Nightly full re-index to catch any drift
- **Index versioning:** Support multiple versions of documentation with version-scoped search
- **Diataxis-aware filtering:** All search queries can be filtered by Diataxis type

### 8.6 When to Add a Dedicated Search Engine

**Trigger:** When PostgreSQL search latency exceeds 100ms at the 95th percentile, or when you need features like typo tolerance, faceted search, or geolocation search.

**Recommended upgrade path:** Typesense (open-source, sub-50ms, built-in vector search, typo-tolerant). Self-host on Fly.io or use Typesense Cloud.

**Alternative:** Meilisearch (Rust-based, excellent developer experience, but vector search is less mature).

---

## 9. CLI Tool Architecture

### 9.1 Language: Go

**Recommendation: Go with Cobra framework.**

| Option | Pros | Cons | Verdict |
|--------|------|------|---------|
| Go + Cobra | Single binary, fast startup, cross-compilation, used by Docker/GitHub/K8s CLIs | Different language from backend | **Recommended** |
| Rust + Clap | Fastest binary, smallest size | Steep learning curve, slower iteration | Overkill for CLI |
| Node.js (oclif) | Same language as backend, shared types | Requires Node.js runtime, slow startup | Poor UX for CLI |

**Why Go wins for CLI:**
- Single binary distribution (no runtime dependency)
- Cross-compiles to macOS, Linux, Windows (ARM + x86)
- Cobra is the industry standard (used by `gh`, `docker`, `kubectl`)
- Fast startup time (~10ms vs ~200ms for Node.js)
- Excellent file system and Git operations
- Can distribute via Homebrew, apt, npm (as binary wrapper), and direct download

### 9.2 CLI Features (MVP)

```
ohara init                          # Initialize a new docs project
ohara dev                           # Start local preview server
ohara build                         # Build static docs for validation
ohara push                          # Push changes to cloud
ohara pull                          # Pull latest from cloud
ohara validate                      # Validate Diataxis structure + links
ohara search <query>                # Search across local docs
ohara login                         # Authenticate with Ohara cloud
ohara status                        # Show sync status
```

### 9.3 Local Preview Server

The CLI embeds a lightweight HTTP server for local preview:
- Serves rendered documentation on `localhost:3000`
- Hot-reloads on file change (using `fsnotify`)
- Renders MDX using a bundled WASM module or by calling out to a Node.js subprocess
- Validates Diataxis structure in real-time
- Shows broken links, missing references

**Implementation note:** For MDX rendering in the CLI, the pragmatic approach is to bundle a small Node.js-based renderer as a subprocess (or use the esbuild-based MDX compiler compiled to a Go-callable binary). Full MDX rendering in pure Go is not practical due to the JSX/React dependency.

### 9.4 Git Integration

The CLI operates directly on the user's Git repository:
- Detects `.ohara.yaml` or `ohara.yaml` configuration file
- Reads content from the working directory (no separate clone)
- `ohara push` commits and pushes via the user's Git credentials
- `ohara pull` pulls latest changes
- PR creation via `gh` CLI integration or direct GitHub API calls

---

## 10. Infrastructure and Deployment

### 10.1 Recommended Infrastructure Stack

| Component | Service | Rationale |
|-----------|---------|-----------|
| Frontend (Next.js) | Vercel | Best-in-class Next.js hosting, global CDN, preview deployments |
| Core API Server | Fly.io or Railway | Long-running server with global deployment, affordable |
| PostgreSQL | Neon | Serverless, branching, autoscaling, pgvector support |
| Redis | Upstash | Serverless, pay-per-request, global |
| Object Storage | Cloudflare R2 | Zero egress, S3-compatible, global CDN |
| Doc CDN | Cloudflare (via R2 or Pages) | Fastest edge network, generous free tier |
| DNS | Cloudflare | Free, fast, integrates with R2 and CDN |
| Monitoring | Sentry + Axiom | Error tracking + log aggregation |
| CI/CD | GitHub Actions | Free for public repos, tight GitHub integration |

### 10.2 Deployment Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     Cloudflare (Edge)                     │
│  ┌─────────────┐  ┌──────────────┐  ┌───────────────┐   │
│  │ DNS          │  │ CDN / Cache   │  │ R2 Storage     │   │
│  └─────────────┘  └──────────────┘  └───────────────┘   │
└─────────────────────────┬───────────────────────────────┘
                          │
         ┌────────────────┼────────────────┐
         ▼                ▼                ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│  Vercel       │  │  Fly.io       │  │  Neon         │
│  (Next.js)    │  │  (API Server) │  │  (PostgreSQL)  │
│               │  │  (MCP Server) │  │  (pgvector)    │
│               │  │  (Git Workers)│  │               │
└──────────────┘  └──────────────┘  └──────────────┘
                          │
                   ┌──────┴──────┐
                   ▼             ▼
            ┌──────────┐  ┌──────────┐
            │ Upstash   │  │ GitHub   │
            │ (Redis)   │  │ (Git     │
            │           │  │  Remote) │
            └──────────┘  └──────────┘
```

### 10.3 Cost Optimization Strategy

**Phase 1 (MVP, 0-100 users):** ~$50-100/month
- Vercel Pro: $20/month
- Fly.io: $5-15/month (shared CPU)
- Neon: Free tier (0.5 GB)
- Upstash: Free tier
- Cloudflare R2: Free tier (10 GB)
- Domain: ~$15/year
- **Total: ~$50-60/month**

**Phase 2 (Growth, 100-1000 users):** ~$200-400/month
- Vercel Pro: $20/month + usage
- Fly.io: $30-60/month (dedicated CPU)
- Neon: $19/month (Launch tier)
- Upstash: $10-30/month
- R2: $5-15/month
- Sentry: $26/month
- **Total: ~$200-300/month**

**Phase 3 (Scale, 1000+ users):** Evaluate based on bottlenecks
- Consider Railway or self-hosted K8s for API
- Neon Scale or self-hosted Postgres
- Dedicated Typesense instance for search
- Dedicated Redis/Valkey instance

### 10.4 CDN Strategy for Documentation Delivery

Documentation pages are the most frequently accessed resource. Cache strategy:

1. **Static pages:** Cached at edge indefinitely, purged on Git sync
2. **API responses:** Short TTL (60s) with stale-while-revalidate
3. **Search results:** No cache (dynamic)
4. **Assets (images):** Long TTL (1 year) with content-hash URLs
5. **llms.txt:** Cached at edge, purged on sync, `Cache-Control: public, max-age=3600`

---

## 11. Recommended Stack Summary

```
┌─────────────────────────────────────────────────────┐
│                   OHARA TECH STACK                    │
├─────────────────────────────────────────────────────┤
│                                                       │
│  FRONTEND                                             │
│  ├── Next.js 15+ (App Router, RSC)                   │
│  ├── Tailwind CSS v4 + shadcn/ui                     │
│  ├── Fumadocs (documentation UI framework)           │
│  ├── MDX + remark/rehype pipeline                    │
│  └── cmdk (search UI)                                │
│                                                       │
│  BACKEND                                              │
│  ├── Elysia (Bun-native HTTP framework)              │
│  ├── TypeScript (end-to-end)                         │
│  ├── Drizzle ORM (SQL-first, type-safe)              │
│  ├── Zod (validation + OpenAPI generation)           │
│  ├── BullMQ (background jobs)                        │
│  ├── Vercel AI SDK (LLM abstraction)                 │
│  ├── MCP TypeScript SDK (agent protocol)             │
│  └── Auth.js v5 or Better Auth (authentication)     │
│                                                       │
│  CLI                                                  │
│  ├── Go + Cobra (CLI framework)                      │
│  ├── Single binary distribution                      │
│  └── Embedded preview server                         │
│                                                       │
│  DATA                                                 │
│  ├── Git (content source of truth)                   │
│  ├── PostgreSQL via Neon (metadata + vector store)   │
│  ├── pgvector (semantic search)                      │
│  ├── Cloudflare R2 (object storage)                  │
│  └── Upstash Redis (cache + queue)                   │
│                                                       │
│  INFRASTRUCTURE                                       │
│  ├── Vercel (frontend hosting)                       │
│  ├── Fly.io (API server)                             │
│  ├── Cloudflare (CDN + DNS + R2)                     │
│  ├── GitHub Actions (CI/CD)                          │
│  └── Sentry + Axiom (observability)                  │
│                                                       │
└─────────────────────────────────────────────────────┘
```

---

## 12. Complexity Assessment and MVP Scope

### 12.1 MVP Definition: "Documentation as Code with Agent Access"

The MVP must prove two hypotheses:
1. **Teams will adopt a Diataxis-enforced, Git-native documentation workflow**
2. **AI agents accessing documentation via MCP/API produces measurably better results than web-scraping or copy-paste**

### 12.2 MVP Features (Must-Have)

| Feature | Complexity | Time Estimate |
|---------|-----------|---------------|
| Git-based content ingestion (GitHub webhook + sync) | Medium | 2-3 weeks |
| MDX parsing and rendering pipeline | Medium | 2-3 weeks |
| Diataxis directory structure validation | Low | 1 week |
| Documentation web UI (read-only, using Fumadocs) | Medium | 2-3 weeks |
| Full-text search (PostgreSQL) | Low | 1 week |
| Semantic search (pgvector) | Medium | 1-2 weeks |
| MCP server (basic tools: search, get_document, list) | Medium | 2 weeks |
| llms.txt auto-generation | Low | 2-3 days |
| REST API for agents (search, retrieve, list) | Medium | 2 weeks |
| CLI: init, dev (local preview), validate, login | Medium | 3-4 weeks |
| Auth (GitHub OAuth login) | Low | 1 week |
| Project dashboard (list projects, sync status) | Low | 1 week |
| Basic onboarding flow | Low | 1 week |

**MVP Total: 16-22 weeks (4-5.5 months) with a 2-3 person team**

### 12.3 Post-MVP Features (Defer)

| Feature | Phase | Rationale for Deferral |
|---------|-------|----------------------|
| WYSIWYG web editor | Phase 2 | Complex, users can edit in IDE + CLI |
| Real-time collaboration | Phase 3 | Very complex (CRDTs), not core to value prop |
| Custom domains | Phase 2 | Nice-to-have, not core |
| Versioned documentation | Phase 2 | Important but adds significant complexity |
| Billing/subscription | Phase 2 | Free until product-market fit |
| GitLab integration | Phase 2 | Start with GitHub only |
| Analytics dashboard | Phase 2 | Can use external analytics initially |
| Team/org management | Phase 2 | Single-user or simple team model for MVP |
| PR preview deployments | Phase 2 | Valuable but requires substantial infra |
| AI writing assistant | Phase 3 | Focus on consumption (agents reading), not creation |
| Slack/Discord integrations | Phase 2 | Webhook notifications are sufficient |
| Self-hosted/on-prem | Phase 3 | Enterprise requirement, not MVP |

### 12.4 Team Size

**Minimum Viable Team: 2-3 engineers**

| Role | Focus |
|------|-------|
| Full-stack engineer #1 (lead) | Backend API, Git integration, search, MCP server |
| Full-stack engineer #2 | Frontend (Next.js), documentation rendering, CLI |
| (Optional) Engineer #3 | AI/search pipeline, CLI (Go), DevOps |

A founding team of 2 strong full-stack engineers can ship MVP in 5-6 months. Adding a third engineer compresses this to 4-5 months and reduces bus factor.

### 12.5 Estimated Timeline

```
Month 1:  Foundation
          ├── Project setup (monorepo, CI/CD, database schema)
          ├── Auth (GitHub OAuth)
          ├── Git sync engine (GitHub webhook → clone → parse)
          └── Basic API scaffolding (Elysia + Drizzle)

Month 2:  Content Pipeline
          ├── MDX parsing + rendering pipeline
          ├── Diataxis validation engine
          ├── Document web UI (Fumadocs-based)
          └── Full-text search (PostgreSQL)

Month 3:  Agent Infrastructure
          ├── Embedding pipeline + pgvector
          ├── Hybrid search (full-text + semantic)
          ├── MCP server (search, get_document, list)
          ├── REST API for agents
          └── llms.txt generation

Month 4:  CLI + Polish
          ├── Go CLI (init, dev, validate, login, push, pull)
          ├── Local preview server
          ├── Project dashboard
          └── Onboarding flow

Month 5:  Testing + Launch Prep
          ├── End-to-end testing
          ├── Performance optimization
          ├── Documentation (eating your own dogfood)
          ├── Beta user onboarding
          └── Launch
```

---

## 13. Phased Implementation Plan

### Phase 1: MVP (Months 1-5)
**Goal:** Prove that agent-optimized, Diataxis-structured docs deliver value.

- Git-native content from GitHub repositories
- Read-only web documentation UI
- Full-text + semantic search
- MCP server for AI agent access
- llms.txt for LLM discoverability
- Go CLI for local authoring and preview
- GitHub OAuth authentication
- Single-project support (one repo = one project)

### Phase 2: Growth (Months 6-9)
**Goal:** Multi-project support, team workflows, and monetization.

- Multi-project dashboard
- Team management and permissions
- Custom domains with SSL
- Versioned documentation (Git branches = versions)
- PR preview deployments
- GitLab integration
- Billing (Stripe integration)
- Analytics (page views, search queries, agent usage)
- Slack/Discord notifications
- WYSIWYG web editor (Tiptap-based)
- API key management for agent access
- Rate limiting and usage quotas

### Phase 3: Scale (Months 10-15)
**Goal:** Enterprise features, advanced AI, ecosystem.

- Real-time collaborative editing (Yjs)
- AI writing assistant (suggest improvements, generate drafts)
- Self-hosted deployment option
- SSO/SAML authentication
- Audit logs
- Advanced analytics (search quality, agent satisfaction)
- Plugin/extension system
- Dedicated search infrastructure (Typesense)
- Custom AI model support (fine-tuned embeddings)
- API marketplace (community MCP servers)
- SOC 2 compliance

---

## 14. Key Technical Risks and Mitigations

### Risk 1: Git Sync Reliability
**Risk:** Large repositories, force pushes, branch conflicts, and webhook failures could cause sync issues.
**Mitigation:**
- Implement idempotent sync operations (re-sync from scratch is always safe)
- Use shallow clones (`--depth 1`) for large repos
- Queue-based sync with retries and dead-letter handling
- Periodic full re-sync as safety net
- Store sync state (last commit SHA) for incremental updates

### Risk 2: Search Quality
**Risk:** Hybrid search results may not be relevant enough for agent consumption.
**Mitigation:**
- Start with a curated evaluation set (50+ query-answer pairs)
- Tune RRF parameters and chunk sizes based on evaluation
- Implement Diataxis-aware boosting (if user asks "how do I...", boost guides)
- Add re-ranking with a cross-encoder model if initial results are poor
- Log all search queries and results for continuous improvement

### Risk 3: MCP Protocol Evolution
**Risk:** MCP is a young protocol (2024) and may change significantly.
**Mitigation:**
- Use the official SDK which tracks protocol changes
- Abstract MCP-specific code behind an interface
- Also provide a REST API (stable, well-understood) as fallback
- Monitor MCP specification updates closely

### Risk 4: Rendering Fidelity (CLI vs. Web)
**Risk:** MDX rendered in CLI preview may differ from web rendering.
**Mitigation:**
- Use the same remark/rehype pipeline for both
- CLI preview calls a bundled Node.js renderer (same code as web)
- Visual regression testing between CLI preview and web output
- Accept minor differences in interactive components (CLI shows placeholder)

### Risk 5: Embedding Cost at Scale
**Risk:** Re-embedding all documents on every change could become expensive.
**Mitigation:**
- Content-hash-based change detection (only re-embed changed chunks)
- Batch embedding API calls
- Consider smaller models (text-embedding-3-small at 1536 dims vs. 3072)
- Budget cap with alerts

### Risk 6: Vendor Lock-in
**Risk:** Deep integration with Vercel, Neon, and Cloudflare creates switching costs.
**Mitigation:**
- Use standard interfaces (S3 API for storage, PostgreSQL for database)
- Elysia runs on Bun which is portable across platforms
- Drizzle ORM supports multiple Postgres providers
- Docker-based deployment as escape hatch from Fly.io/Railway
- Document all infrastructure decisions for future migration

---

## 15. Cost Projections

### MVP Phase (First 6 Months)

| Service | Monthly Cost | Notes |
|---------|-------------|-------|
| Vercel (Pro) | $20 | Next.js hosting, preview deployments |
| Fly.io | $10-30 | 1-2 shared CPU machines |
| Neon (Free → Launch) | $0-19 | Scale as needed |
| Upstash Redis | $0-10 | Free tier covers MVP |
| Cloudflare R2 | $0-5 | Free tier: 10 GB storage |
| Cloudflare (DNS/CDN) | $0 | Free tier |
| GitHub (Team) | $0-4/user | Free for public repos |
| Sentry (Developer) | $0 | Free tier: 5K errors/month |
| OpenAI (Embeddings) | $5-20 | ~1M tokens/month for embeddings |
| Domain | ~$1 | ~$15/year |
| **Total** | **$36-110/month** | |

### Annual Cost (Year 1)

| Phase | Duration | Monthly | Total |
|-------|----------|---------|-------|
| Development (no users) | 4 months | ~$50 | ~$200 |
| Beta (10-50 users) | 3 months | ~$80 | ~$240 |
| Launch (50-500 users) | 5 months | ~$200 | ~$1,000 |
| **Year 1 Total** | | | **~$1,440** |

This excludes personnel costs but demonstrates that infrastructure can be kept under $2,000/year for the first year with proper use of free tiers and serverless pricing.

---

## Appendix A: Directory Structure (Monorepo)

```
ohara/
├── apps/
│   ├── web/                    # Next.js frontend (Vercel)
│   │   ├── app/                # App Router pages
│   │   ├── components/         # React components
│   │   └── lib/                # Frontend utilities
│   └── api/                    # Elysia API server (Fly.io)
│       ├── src/
│       │   ├── modules/        # Domain modules
│       │   │   ├── content/    # Content parsing, rendering
│       │   │   ├── git/        # Git sync, webhooks
│       │   │   ├── search/     # Full-text + semantic search
│       │   │   ├── agent/      # MCP server, RAG pipeline
│       │   │   ├── auth/       # Authentication
│       │   │   └── billing/    # Subscriptions (Phase 2)
│       │   ├── db/             # Drizzle schema + migrations
│       │   ├── middleware/     # Auth, rate limiting, logging
│       │   └── index.ts        # Elysia app entry point
│       └── Dockerfile
├── packages/
│   ├── shared/                 # Shared TypeScript types + utilities
│   │   ├── types/              # API types, Diataxis types
│   │   ├── validation/         # Zod schemas (shared between API + CLI)
│   │   └── markdown/           # remark/rehype pipeline (shared)
│   └── config/                 # Shared ESLint, TypeScript configs
├── cli/                        # Go CLI tool
│   ├── cmd/                    # Cobra commands
│   ├── internal/               # Internal packages
│   │   ├── api/                # API client
│   │   ├── git/                # Git operations
│   │   ├── preview/            # Local preview server
│   │   └── validate/           # Diataxis validation
│   ├── go.mod
│   └── main.go
├── docs/                       # Ohara's own documentation (dogfooding)
├── turbo.json                  # Turborepo configuration
├── package.json                # Root package.json
└── pnpm-workspace.yaml         # pnpm workspace config
```

## Appendix B: Database Schema (Core Tables)

```sql
-- Projects (one per documentation repository)
CREATE TABLE projects (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  slug TEXT NOT NULL UNIQUE,
  github_repo_url TEXT NOT NULL,
  github_installation_id INTEGER,
  default_branch TEXT DEFAULT 'main',
  last_synced_at TIMESTAMPTZ,
  last_sync_commit_sha TEXT,
  settings JSONB DEFAULT '{}',
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

-- Documents (parsed from Git)
CREATE TABLE documents (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id),
  path TEXT NOT NULL,                          -- e.g., "tutorials/getting-started.mdx"
  title TEXT NOT NULL,
  diataxis_type TEXT NOT NULL CHECK (diataxis_type IN ('tutorial', 'guide', 'reference', 'explanation')),
  content_markdown TEXT NOT NULL,              -- Raw markdown
  content_html TEXT NOT NULL,                  -- Rendered HTML
  frontmatter JSONB DEFAULT '{}',
  content_hash TEXT NOT NULL,                  -- For change detection
  word_count INTEGER,
  search_vector TSVECTOR,                      -- Full-text search
  version TEXT DEFAULT 'main',
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now(),
  UNIQUE(project_id, path, version)
);

-- Document chunks (for semantic search / RAG)
CREATE TABLE document_chunks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id),
  chunk_index INTEGER NOT NULL,
  content TEXT NOT NULL,
  heading_hierarchy TEXT[] DEFAULT '{}',
  content_hash TEXT NOT NULL,
  embedding vector(1536),                      -- pgvector
  metadata JSONB DEFAULT '{}',
  created_at TIMESTAMPTZ DEFAULT now(),
  UNIQUE(document_id, chunk_index)
);

-- Indexes
CREATE INDEX idx_documents_project ON documents(project_id);
CREATE INDEX idx_documents_diataxis ON documents(project_id, diataxis_type);
CREATE INDEX idx_documents_fts ON documents USING GIN(search_vector);
CREATE INDEX idx_chunks_project ON document_chunks(project_id);
CREATE INDEX idx_chunks_embedding ON document_chunks
  USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);

-- Users
CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email TEXT NOT NULL UNIQUE,
  name TEXT,
  avatar_url TEXT,
  github_id TEXT UNIQUE,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

-- API Keys (for agent access)
CREATE TABLE api_keys (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id),
  project_id UUID REFERENCES projects(id),      -- NULL = all projects
  name TEXT NOT NULL,
  key_hash TEXT NOT NULL UNIQUE,                 -- Store hash, not plaintext
  prefix TEXT NOT NULL,                          -- First 8 chars for identification
  scopes TEXT[] DEFAULT '{read}',
  last_used_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT now()
);

-- Sync log
CREATE TABLE sync_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id),
  status TEXT NOT NULL CHECK (status IN ('pending', 'running', 'success', 'failed')),
  commit_sha TEXT,
  documents_added INTEGER DEFAULT 0,
  documents_updated INTEGER DEFAULT 0,
  documents_deleted INTEGER DEFAULT 0,
  chunks_embedded INTEGER DEFAULT 0,
  error_message TEXT,
  started_at TIMESTAMPTZ DEFAULT now(),
  completed_at TIMESTAMPTZ
);
```

## Appendix C: ohara.yaml Configuration Schema

```yaml
# ohara.yaml — placed in the root of a documentation repository
version: "1"

project:
  name: "Payment Service"
  slug: "payment-service"
  description: "Documentation for the Payment Service API and SDKs"

content:
  # Directory mapping to Diataxis types
  tutorials: "./tutorials"
  guides: "./guides"
  reference: "./reference"
  explanation: "./explanation"
  assets: "./assets"

  # File patterns to include
  include:
    - "**/*.md"
    - "**/*.mdx"

  # File patterns to exclude
  exclude:
    - "**/node_modules/**"
    - "**/_drafts/**"

rendering:
  # Custom MDX components
  components: "./components"

  # Syntax highlighting theme
  codeTheme: "github-dark"

  # Enable math rendering
  math: true

  # Enable Mermaid diagrams
  mermaid: true

navigation:
  # Auto-generate from directory structure (default)
  auto: true

  # Or provide explicit ordering
  # order:
  #   tutorials:
  #     - getting-started
  #     - advanced-setup
  #   reference:
  #     - api
  #     - config

search:
  # Boost certain Diataxis types in search results
  boost:
    reference: 1.2
    guides: 1.1

ai:
  # Generate llms.txt
  llmsTxt: true

  # Enable MCP server access
  mcp: true

  # Custom instructions for AI agents
  agentInstructions: |
    This service handles payment processing.
    Always check the API reference for exact endpoint signatures.
    For integration questions, refer to the guides section.
```
