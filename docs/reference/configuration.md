---
title: Configuration Reference
description: Complete reference for ohara.yaml configuration options.
diataxis_type: reference
order: 2
---

# Configuration Reference

## ohara.yaml

The `ohara.yaml` file configures an Ohara documentation project. Place it at the root of your documentation directory.

```yaml
# Required
name: My Project

# Optional
description: "A short description of the project"
docs_dir: "."  # Docs root relative to repo root
version: "1.0.0"
base_url: "https://docs.example.com"

# Custom Diataxis directory names (defaults shown)
directories:
  tutorials: tutorials
  guides: guides
  reference: reference
  explanation: explanation

# File inclusion patterns (glob)
include:
  - "**/*.md"
  - "**/*.mdx"

# File exclusion patterns (glob)
exclude:
  - node_modules/**
  - .git/**
  - dist/**

# Default Diataxis type for docs without explicit type
default_type: explanation

# Navigation configuration
navigation:
  group_by_type: true  # Group sidebar by Diataxis type
  order: []  # Custom sidebar ordering
```

## Document Frontmatter

```yaml
---
# Required
title: Document Title

# Optional
description: Short description for search results and llms.txt
diataxis_type: guide  # tutorial | guide | reference | explanation
tags: [tag1, tag2]
author: Jane Doe
draft: false  # true to hide from public
order: 1  # Sidebar sort order within type group
slug: custom-slug  # Override auto-generated slug

# Tutorial/Guide specific
duration: 15  # Estimated completion time in minutes
difficulty: beginner  # beginner | intermediate | advanced
prerequisites:
  - tutorials/getting-started  # Paths to prerequisite docs

# Reference specific
source: https://github.com/org/repo/blob/main/src/api.ts
---
```

## Environment Variables

### Required

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL connection string (Neon) |

### Authentication

| Variable | Description |
|----------|-------------|
| `BETTER_AUTH_SECRET` | Secret for session encryption |
| `GITHUB_CLIENT_ID` | GitHub OAuth app client ID |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth app client secret |

### GitHub App

| Variable | Description |
|----------|-------------|
| `GITHUB_APP_ID` | GitHub App ID |
| `GITHUB_APP_PRIVATE_KEY` | GitHub App private key (PEM) |
| `GITHUB_WEBHOOK_SECRET` | Webhook signature secret |

### Optional Services

| Variable | Description | Default |
|----------|-------------|---------|
| `REDIS_URL` | Redis connection for queues/caching | In-memory fallback |
| `OPENAI_API_KEY` | OpenAI key for embeddings | Semantic search disabled |
| `WEB_URL` | Frontend URL for CORS | `http://localhost:3000` |
| `PORT` | API server port | `3001` |
| `LOG_LEVEL` | Logging level | `info` |
