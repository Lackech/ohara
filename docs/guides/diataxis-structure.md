---
title: Organizing Docs with Diataxis
description: Structure your documentation into four types for optimal human and agent consumption.
diataxis_type: guide
order: 1
---

# Organizing Docs with Diataxis

Ohara uses the Diataxis framework to organize documentation into four types. This guide shows you how to structure your docs for maximum effectiveness.

## The Four Types

### Tutorials (Learning-oriented)

**Purpose:** Walk newcomers through a complete experience.
**Directory:** `tutorials/`

Good tutorials:
- Have a clear beginning and end
- Produce a working result
- Minimize explanation (save that for explanation docs)

```markdown
---
title: Deploy Your First Service
diataxis_type: tutorial
difficulty: beginner
duration: 15
---
```

### How-to Guides (Task-oriented)

**Purpose:** Help users accomplish a specific task.
**Directory:** `guides/`

Good guides:
- Start with the goal, not background
- Are concise and actionable
- Assume the reader knows the basics

```markdown
---
title: Configure CI/CD Pipeline
diataxis_type: guide
---
```

### Reference (Information-oriented)

**Purpose:** Describe the system accurately and completely.
**Directory:** `reference/`

Good reference docs:
- Are structured consistently
- Cover every option and parameter
- Don't include tutorials or opinions

```markdown
---
title: REST API Reference
diataxis_type: reference
---
```

### Explanation (Understanding-oriented)

**Purpose:** Help readers understand why things work the way they do.
**Directory:** `explanation/`

Good explanation docs:
- Discuss design decisions and tradeoffs
- Provide context and background
- Connect concepts to each other

```markdown
---
title: Architecture Overview
diataxis_type: explanation
---
```

## Why This Matters for Agents

Diataxis isn't just for humans. When an AI agent needs to:

- **Execute a task** → Ohara serves how-to guides
- **Learn a new system** → Ohara serves tutorials
- **Look up a parameter** → Ohara serves reference docs
- **Make a design decision** → Ohara serves explanation docs

The `search_documentation` MCP tool automatically boosts the right type based on query patterns. "How do I deploy?" boosts guides. "What is the auth architecture?" boosts explanations.

## Frontmatter

Every document should have a `title` in its frontmatter. The `diataxis_type` can be set explicitly or inferred from the directory:

```yaml
---
title: Your Document Title
description: Optional description for search results
diataxis_type: guide  # Optional if in the right directory
tags: [deployment, ci-cd]
order: 1  # Controls sidebar ordering
draft: false  # Set to true to hide from public
---
```
