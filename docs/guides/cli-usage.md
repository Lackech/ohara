---
title: Using the Ohara CLI
description: Initialize, validate, and manage documentation projects from the command line.
diataxis_type: guide
order: 2
---

# Using the Ohara CLI

The Ohara CLI lets you manage documentation projects locally.

## Installation

### Homebrew (macOS/Linux)

```bash
brew install ohara-project/tap/ohara
```

### Binary Download

Download the latest binary from [GitHub Releases](https://github.com/ohara-project/cli/releases) for your platform.

## Authentication

```bash
ohara login
# Enter your API key when prompted
```

Your token is stored in `~/.ohara/config.json`.

## Initialize a Project

```bash
ohara init my-project
```

This creates:
- `ohara.yaml` — Project configuration
- `tutorials/` — Tutorial documents
- `guides/` — How-to guides
- `reference/` — Reference documentation
- `explanation/` — Explanatory articles

Each directory includes a starter `getting-started.md`.

## Validate Structure

```bash
ohara validate
```

Checks:
- `ohara.yaml` exists and is valid
- Diataxis directories are present
- All documents have frontmatter with a `title`
- Reports warnings for missing fields

## Check Status

```bash
ohara status
```

Shows your projects and their last sync time.

## Search

```bash
ohara search "authentication" --project my-project
```

Searches your documentation using the hybrid search API.
