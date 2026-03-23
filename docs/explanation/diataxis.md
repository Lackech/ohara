---
title: Why Diataxis?
description: The reasoning behind choosing Diataxis as the documentation framework.
diataxis_type: explanation
order: 2
---

# Why Diataxis?

Diataxis isn't just an organizational scheme — it's the foundation of Ohara's agent optimization strategy.

## The Problem with Flat Documentation

Most documentation is a grab bag. Tutorials mix with reference material, explanations sit next to how-to guides. Humans can usually figure out what they need by scanning headings and context clues.

AI agents can't.

When an agent loads documentation into its context window, it gets everything at once. A flat `CLAUDE.md` or `AGENTS.md` file dumps deployment guides next to API specs next to architectural rationale. The agent wastes context window space on irrelevant content and may retrieve the wrong type of information for its task.

## Diataxis as Structured Retrieval

By typing every document, Ohara can serve the right content for the right task:

| Agent Task | What It Needs | Diataxis Type |
|-----------|---------------|---------------|
| Execute a deployment | Step-by-step instructions | **Guide** |
| Learn a new codebase | Guided walkthrough | **Tutorial** |
| Look up an API parameter | Precise specification | **Reference** |
| Decide on an architecture | Design context and tradeoffs | **Explanation** |

This isn't just categorization — it's **selective retrieval**. Instead of dumping everything into the context window, Ohara gives agents exactly what they need.

## The Agent Flywheel

Diataxis enables a compound effect:

1. **Agents read** typed documentation → they become domain experts
2. **Agents work better** because they have the right context → they produce better output
3. **Better output** means agents can **contribute back** to documentation
4. **Better documentation** → agents become even more effective

This flywheel is Ohara's core thesis. Documentation isn't a static artifact — it's a living knowledge layer that compounds with agent usage.

## Why Not Just Tags?

Tags are arbitrary. "deployment" could be a tutorial about learning deployment, a step-by-step deployment guide, or reference documentation for deployment config.

Diataxis types are orthogonal to topics. You can have a deployment tutorial AND a deployment guide AND a deployment reference — each serving a different purpose. The type tells the agent (and the human) **how** the content is meant to be used, not just what it's about.
