---
title: REST API Reference
description: Complete reference for the Ohara REST API.
diataxis_type: reference
order: 1
---

# REST API Reference

Base URL: `https://api.ohara.dev`

All endpoints require authentication via session cookie or API key (`Authorization: Bearer ohara_...`).

## Projects

### List Projects

```
GET /api/v1/projects
```

Returns all projects owned by the authenticated user.

### Create Project

```
POST /api/v1/projects
Content-Type: application/json

{
  "name": "My Project",
  "slug": "my-project",
  "description": "Optional description",
  "repoUrl": "https://github.com/user/repo",
  "repoBranch": "main",
  "docsDir": "docs"
}
```

### Get Project

```
GET /api/v1/projects/:slug
```

## Documents

### List Documents

```
GET /api/v1/projects/:projectId/documents
```

Returns all documents for a project, ordered by Diataxis type and title.

### Get Document

```
GET /api/v1/projects/:projectId/documents/:path
```

Supports content negotiation:
- `Accept: application/json` — Structured JSON response with metadata
- `Accept: text/markdown` — Raw markdown content

## Search

### Full-Text Search

```
GET /api/v1/projects/:projectId/search?q=query&type=guide&limit=20
```

Parameters:
- `q` (required) — Search query
- `type` — Filter by Diataxis type
- `limit` — Max results (default 20, max 100)
- `offset` — Pagination offset

Returns results with `ts_rank` scoring and `ts_headline` snippets.

### Hybrid Search

```
GET /api/v1/projects/:projectId/search/hybrid?q=query
```

Combines full-text and semantic search using Reciprocal Rank Fusion.

### Agent Search

```
GET /api/v1/search?query=...&project=my-project
```

Agent-optimized endpoint returning structured envelope with metadata and suggested queries.

## API Keys

### List Keys

```
GET /api/v1/api-keys
```

### Create Key

```
POST /api/v1/api-keys
Content-Type: application/json

{
  "name": "My CLI Key",
  "projectId": "optional-project-id",
  "scopes": ["read"]
}
```

The raw key is returned **only once** in the response. Store it securely.

### Revoke Key

```
DELETE /api/v1/api-keys/:id
```

## llms.txt

### Standard

```
GET /:projectSlug/llms.txt
```

Diataxis-typed sections with document links and descriptions.

### Full Content

```
GET /:projectSlug/llms-full.txt
```

Full inline document content, organized by Diataxis type.

## MCP

```
POST /mcp
Authorization: Bearer ohara_...
Content-Type: application/json
```

MCP endpoint for AI agents. Configure in your MCP client.

### Available Tools

- `search_documentation(query, project, type?, limit?)` — Hybrid search
- `get_document(project, path)` — Full document content with related docs
- `list_documents(project, type?)` — List all documents

### Available Prompts

- `troubleshoot(project, service, error)` — Guided troubleshooting
- `explain_concept(project, topic, level)` — Concept explanation

### Available Resources

- `docs://{project}/{path}` — Raw document content

## Rate Limits

- Auth endpoints: 10 requests/minute per IP
- API endpoints: 100 requests/minute per IP
- API key default: 1,000 requests/hour
