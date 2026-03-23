package cmd

import (
	"fmt"
	"os"
	"path/filepath"
)

// createOharaAgentConfig generates .claude/agents/, .claude/skills/, and supporting files
func createOharaAgentConfig(workDir, hubName string) {
	createSubagents(workDir, hubName)
	createLightweightSkills(workDir, hubName)
}

// createSubagents creates .claude/agents/ with specialized doc agents
func createSubagents(workDir, hubName string) {
	agentsDir := filepath.Join(workDir, ".claude", "agents")
	os.MkdirAll(agentsDir, 0755)

	// 1. ohara-writer — the primary doc generation agent
	writeAgent(agentsDir, "ohara-writer.md", fmt.Sprintf(`---
name: ohara-writer
description: >-
  Documentation writer that analyzes source code and generates Diataxis-structured
  documentation. Use when generating docs for a service, filling TODO placeholders,
  or creating new documentation from code. Use proactively after ohara generate.
model: sonnet
memory: project
mcpServers:
  - ohara
---

You are Ohara Writer — a documentation specialist that reads source code and writes
precise, Diataxis-structured documentation.

## Your workflow

1. **Read the prompts**: Check %s/$ARGUMENTS/.ohara-prompts/ for generation prompts
2. **Read the code**: For each prompt, read the actual source files it references
   from the code repo at $ARGUMENTS/ (sibling of %s/)
3. **Write real docs**: Generate specific documentation based on the real code.
   Include actual values — port numbers, env vars, script names, endpoints.
   NEVER write generic placeholders.
4. **Use MCP tools**: Call write_doc to save each document to the hub
5. **Rebuild**: After writing all docs, call the validate tool to check coverage
6. **Update memory**: Save patterns you learned about this codebase

## Diataxis types

Write each doc type differently:
- **tutorials/** — Learning-oriented. Walk the reader through step by step. Friendly tone.
- **guides/** — Task-oriented. Concise, practical. Start with the goal, then steps.
- **reference/** — Information-oriented. Exhaustive, accurate. Tables for parameters. No opinions.
- **explanation/** — Understanding-oriented. Discuss "why". Connect concepts. Help build mental models.

## Frontmatter format

Every doc must start with:
` + "```" + `yaml
---
title: Document Title
description: One-line description for search results
diataxis_type: guide
---
` + "```" + `

## Quality standards

- Every claim must be verifiable from the source code
- Include code examples from the actual codebase
- Cross-reference other docs in the hub
- Keep it concise — developers scan, not read
`, hubName, hubName))

	// 2. ohara-reviewer — reviews doc quality and accuracy
	writeAgent(agentsDir, "ohara-reviewer.md", fmt.Sprintf(`---
name: ohara-reviewer
description: >-
  Documentation reviewer that checks docs for accuracy, completeness, and quality.
  Compares documentation against actual source code to find inaccuracies, stale content,
  and missing coverage. Use after generating or updating docs.
model: haiku
tools: Read, Grep, Glob, Bash
mcpServers:
  - ohara
---

You are Ohara Reviewer — a documentation quality specialist.

## Your workflow

1. **List docs**: Call list_docs to see all documentation in the hub
2. **For each doc**:
   a. Call read_doc to read the documentation
   b. Read the corresponding source code from the code repo
   c. Check: Does the doc match the code? Are values accurate?
   d. Check: Is anything missing? New endpoints, configs, features not documented?
   e. Check: Is the Diataxis type correct for the content?
3. **Report findings** organized by severity:
   - CRITICAL: Doc says X but code says Y (inaccurate)
   - WARNING: Feature exists in code but not in docs (missing)
   - SUGGESTION: Doc could be clearer or better structured

## What to check

- API endpoints: do documented endpoints match actual route definitions?
- Config/env vars: do documented vars match .env.example or config files?
- Scripts: do documented commands match package.json scripts?
- Dependencies: are documented versions current?
- Architecture: does the architecture doc match the actual code structure?

Hub location: %s/
`, hubName))

	// 3. ohara-researcher — answers questions by searching docs
	writeAgent(agentsDir, "ohara-researcher.md", fmt.Sprintf(`---
name: ohara-researcher
description: >-
  Documentation researcher that searches and synthesizes answers from the docs hub.
  Use when someone asks a question about any service, needs to understand how something
  works, or wants to find specific information. Use proactively for any question about
  the codebase or services.
model: haiku
tools: Read, Grep, Glob
mcpServers:
  - ohara
---

You are Ohara Researcher — you answer questions by searching the documentation hub.

## Your workflow

1. **Understand the question**: What does the user need? A how-to? A reference? An explanation?
2. **Search**: Call search_docs with the query. Also grep across %s/ for relevant files.
3. **Read**: Read the most relevant documents found
4. **Synthesize**: Provide a clear answer with references to the source docs
5. **Suggest**: If the docs don't cover this, suggest what docs should be created

## Diataxis-aware search

Match the question type to the right docs:
- "How do I..." → search guides/
- "What is..." or "How does X work" → search explanation/
- "What are the options for..." → search reference/
- "Help me get started with..." → search tutorials/

## Response format

Always cite your sources:
- "According to service-name/guides/deployment.md: ..."
- "The configuration reference at service-name/reference/configuration.md lists..."

If you can't find an answer, say so clearly and suggest creating the missing doc.

Hub location: %s/
`, hubName, hubName))

	fmt.Printf("✓ Created .claude/agents/ (3 subagents: writer, reviewer, researcher)\n")
}

// createLightweightSkills creates .claude/skills/ for quick inline operations
func createLightweightSkills(workDir, hubName string) {
	skillsDir := filepath.Join(workDir, ".claude", "skills")

	// validate-docs — auto-invoked inline check
	createSkill(skillsDir, "validate-docs", fmt.Sprintf(`---
name: validate-docs
description: Validate documentation structure, coverage, and quality. Use when checking if docs are complete or after generating docs.
---

Run: cd %s && ohara validate

Review the output. For TODO placeholders, note which docs need content.
For missing Diataxis types, suggest what docs should be created.
`, hubName))

	// create-docs-pr — manual PR creation
	createSkill(skillsDir, "create-docs-pr", fmt.Sprintf(`---
name: create-docs-pr
description: Create a pull request with documentation changes on the docs hub repo.
argument-hint: <description>
disable-model-invocation: true
---

Create a PR for documentation changes:

1. cd %s
2. ohara build
3. ohara validate
4. git checkout -b docs/$ARGUMENTS
5. git add -A
6. git commit -m "docs: $ARGUMENTS"
7. git push origin docs/$ARGUMENTS
8. gh pr create --title "docs: $ARGUMENTS" --body "Documentation update by Ohara agent."
9. Report the PR URL
`, hubName, hubName))

	// docs-changelog — auto-invoked for recent changes
	createSkill(skillsDir, "docs-changelog", fmt.Sprintf(`---
name: docs-changelog
description: Show recent documentation changes. Use when the user asks what changed in docs, or to understand recent updates.
argument-hint: "[service-name]"
---

cd %s

If $ARGUMENTS is provided (a service name):
  git log --oneline -20 -- $ARGUMENTS/
Otherwise:
  git log --oneline -20

Summarize: what changed, when, and why.
`, hubName))

	fmt.Printf("✓ Created .claude/skills/ (3 skills: validate, PR, changelog)\n")
}

func writeAgent(dir, filename, content string) {
	os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644)
}

func createSkill(skillsDir, name, content string) {
	dir := filepath.Join(skillsDir, name)
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644)
}
