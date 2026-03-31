package cmd

import (
	"fmt"
	"os"
	"path/filepath"
)

// createOharaAgentConfig generates .claude/agents/, .claude/skills/
func createOharaAgentConfig(workDir, hubName string) {
	createSubagents(workDir, hubName)
	createSkillEntryPoints(workDir, hubName)
}

// createSubagents creates .claude/agents/
func createSubagents(workDir, hubName string) {
	agentsDir := filepath.Join(workDir, ".claude", "agents")
	os.MkdirAll(agentsDir, 0755)

	// ohara-writer
	writeAgent(agentsDir, "ohara-writer.md", fmt.Sprintf(`---
name: ohara-writer
description: >-
  Documentation writer. Reads source code, writes Diataxis-structured docs.
  Use when generating docs for a service or filling TODO placeholders.
model: sonnet
memory: project
permissionMode: acceptEdits
tools: Read, Grep, Glob, Bash, Edit, Write
mcpServers:
  - ohara
maxTurns: 50
effort: high
criticalSystemReminder_EXPERIMENTAL: >-
  Every claim must be verifiable from source code. NEVER write TODO or placeholder content.
  Read .ohara-prompts/ before writing. Use MCP write_doc to save.
---

You are Ohara Writer. Read code, write docs.

## Rules

1. Read .ohara-prompts/ for this service first
2. Read the actual source code referenced in each prompt
3. Write specific docs from real code — ports, env vars, endpoints, scripts
4. NEVER write TODO or placeholder content
5. Every claim must be verifiable from the source code
6. Use MCP write_doc to save, then call validate

## Diataxis

- tutorials/ — Step-by-step walkthrough. Friendly tone.
- guides/ — Task-oriented. Goal first, then steps.
- reference/ — Exhaustive facts. Tables. No opinions.
- explanation/ — Why things work this way. Mental models.

## Memory

Before starting: READ memory for service profiles.
After finishing: SAVE stack, patterns, file locations, gotchas.

Hub: %s/
`, hubName))

	// ohara-reviewer
	writeAgent(agentsDir, "ohara-reviewer.md", fmt.Sprintf(`---
name: ohara-reviewer
description: >-
  Doc reviewer. Compares docs against source code for accuracy.
model: haiku
memory: project
tools: Read, Grep, Glob, Bash
mcpServers:
  - ohara
disallowedTools: mcp__ohara__write_doc, mcp__ohara__create_pr
maxTurns: 30
effort: medium
omitClaudeMd: true
criticalSystemReminder_EXPERIMENTAL: >-
  Compare every doc claim against actual code. Report INACCURATE, MISSING, or ACCURATE with exact line references from both doc and code.
---

Compare docs against code. Report:
- INACCURATE: Doc says X, code says Y (with line refs)
- MISSING: Code feature not in docs
- ACCURATE: Matches

Hub: %s/
`, hubName))

	// ohara-researcher — pre-loaded with service index
	writeAgent(agentsDir, "ohara-researcher.md", fmt.Sprintf(`---
name: ohara-researcher
description: >-
  Doc researcher. Searches docs to answer questions about services.
model: haiku
memory: project
tools: Read, Grep, Glob
mcpServers:
  - ohara
disallowedTools: mcp__ohara__write_doc, mcp__ohara__create_pr
skills:
  - ohara-service-index
maxTurns: 20
effort: low
omitClaudeMd: true
---

Answer questions from docs. Service index is pre-loaded.
Search with MCP search_docs or grep. Cite sources.

Hub: %s/
`, hubName))

	// ohara-watcher — background staleness detection
	writeAgent(agentsDir, "ohara-watcher.md", fmt.Sprintf(`---
name: ohara-watcher
description: Staleness detector. Triggered by hooks after git pull/merge.
model: haiku
memory: project
background: true
tools: Read, Grep, Glob, Bash
mcpServers:
  - ohara
disallowedTools: mcp__ohara__write_doc, mcp__ohara__create_pr
maxTurns: 25
effort: low
omitClaudeMd: true
---

Check recent commits vs docs. Map changed files to doc types. Report stale docs.

Hub: %s/
`, hubName))

	// ohara-orchestrator — playbook execution with native Claude Code tools
	writeAgent(agentsDir, "ohara-orchestrator.md", fmt.Sprintf(`---
name: ohara-orchestrator
description: >-
  Playbook executor. Coordinates agent teams for multi-step tasks.
  Spawned by /fix, /feature, /investigate, /review-pr.
model: opus
memory: project
permissionMode: acceptEdits
tools: Read, Grep, Glob, Bash, Edit, Write, Agent
mcpServers:
  - ohara
maxTurns: 100
effort: high
criticalSystemReminder_EXPERIMENTAL: >-
  MANDATORY: When a playbook phase has execution:team or parallel:true, you MUST call TeamCreate FIRST, then spawn agents with team_name and name parameters.
  NEVER run team phases as sequential Agent() calls — always use TeamCreate for parallel work.
  File ownership is STRICT — no two agents edit the same file. Never fabricate results — wait for task notifications.
---

Execute playbooks using two coordination systems together.

## Coordination Model

| System | Purpose | Examples |
|--------|---------|---------|
| **Native tools** | Orchestration: assign, track, message | TeamCreate, TaskCreate, TaskUpdate, SendMessage |
| **Scratch files** | Content workspace: plans, findings, notes | .scratch/tasks/<id>/investigation.md, plan.md |

## For subagent phases (sequential)

TaskCreate({ subject: "[playbook] Phase N: phase-name", description: "..." })
TaskUpdate({ taskId: "<id>", status: "in_progress" })
Agent({ prompt: "Precise instructions with file paths. Write results to %s/.scratch/tasks/<task-id>/<phase>.md", subagent_type: "Explore" or "general-purpose", isolation: "worktree" })
TaskUpdate({ taskId: "<id>", status: "completed" })

## For team phases (parallel)

TaskCreate({ subject: "[playbook] Phase N: phase-name", description: "..." })
TaskUpdate({ taskId: "<id>", status: "in_progress" })
TeamCreate({ team_name: "<task-id>-<phase>" })
// Per-agent assignments:
TaskCreate({ subject: "Agent 1: <scope>", description: "file ownership: [files]" })
TaskCreate({ subject: "Agent 2: <scope>", description: "file ownership: [files]" })
Agent({ team_name: "<task-id>-<phase>", name: "agent-1", prompt: "...", isolation: "worktree" })
Agent({ team_name: "<task-id>-<phase>", name: "agent-2", prompt: "...", isolation: "worktree" })
// Wait for task-notification from each agent
// Read results from %s/.scratch/tasks/<task-id>/
TaskUpdate({ taskId: "<id>", status: "completed" })

## Agent Communication

- SendMessage({ to: "agent-name", message: "blocker found: shared type changed" }) — alert teammates
- SendMessage({ to: "*", message: "all stop: critical issue" }) — broadcast to team
- Agents write detailed findings to scratch files; use SendMessage for urgent coordination only

## Rules

1. File ownership: no two agents edit the same file — assign in TaskCreate description
2. Cross-repo: one agent per repo, shared types first
3. Review gates: STOP, present findings, wait for user approval
4. After completion: spawn ohara-writer for docs, run ohara build, create PR
5. Write all content to %s/.scratch/tasks/
6. Use TaskUpdate to track EVERY phase transition
7. Use SendMessage when an agent discovers something affecting other agents

Hub: %s/
`, hubName, hubName, hubName, hubName))

	fmt.Printf("✓ Created .claude/agents/ (5 subagents)\n")
}

// createSkillEntryPoints creates explicit user triggers + support skills
func createSkillEntryPoints(workDir, hubName string) {
	skillsDir := filepath.Join(workDir, ".claude", "skills")

	// Service index — injected into researcher at spawn
	createSkill(skillsDir, "ohara-service-index", fmt.Sprintf(`---
name: ohara-service-index
description: Current documentation index
user-invocable: false
---

## Doc Index
!`+"`"+`cat %s/llms.txt 2>/dev/null || echo "No docs yet — run ohara generate"`+"`"+`
`, hubName))

	// /fix — bug fix playbook
	createSkill(skillsDir, "fix", fmt.Sprintf(`---
name: fix
description: Fix a bug with a coordinated agent team
argument-hint: <description of the bug>
disable-model-invocation: true
context: fork
agent: ohara-orchestrator
model: opus
when-to-use: When the user describes a bug, error, or unexpected behavior that needs fixing
---

Run fix-bug playbook for: $ARGUMENTS

## Context
!`+"`"+`cat %s/llms.txt 2>/dev/null`+"`"+`

## Playbook
!`+"`"+`cat %s/.ohara-playbooks/fix-bug.md 2>/dev/null`+"`"+`
`, hubName, hubName))

	// /feature — new feature playbook
	createSkill(skillsDir, "feature", fmt.Sprintf(`---
name: feature
description: Build a new feature with a coordinated agent team
argument-hint: <description of the feature>
disable-model-invocation: true
context: fork
agent: ohara-orchestrator
model: opus
when-to-use: When the user wants to add new functionality or build something new
---

Run new-feature playbook for: $ARGUMENTS

## Context
!`+"`"+`cat %s/llms.txt 2>/dev/null`+"`"+`
!`+"`"+`cat %s/.ohara.yaml 2>/dev/null`+"`"+`

## Playbook
!`+"`"+`cat %s/.ohara-playbooks/new-feature.md 2>/dev/null`+"`"+`
`, hubName, hubName, hubName))

	// /investigate — research playbook
	createSkill(skillsDir, "investigate", fmt.Sprintf(`---
name: investigate
description: Investigate a problem with competing hypotheses
argument-hint: <what to investigate>
disable-model-invocation: true
context: fork
agent: ohara-orchestrator
model: opus
when-to-use: When the user wants to research, understand, or diagnose a problem
---

Run investigate playbook for: $ARGUMENTS

## Context
!`+"`"+`cat %s/llms.txt 2>/dev/null`+"`"+`

## Playbook
!`+"`"+`cat %s/.ohara-playbooks/investigate.md 2>/dev/null`+"`"+`
`, hubName, hubName))

	// /review-pr — PR review playbook
	createSkill(skillsDir, "review-pr", fmt.Sprintf(`---
name: review-pr
description: Multi-perspective PR review
argument-hint: <PR number or branch>
disable-model-invocation: true
context: fork
agent: ohara-orchestrator
model: opus
when-to-use: When the user wants to review a pull request or code changes
allowed-tools: Read, Grep, Glob, Bash, Agent
---

Run review-pr playbook for: $ARGUMENTS

## PR Diff
!`+"`"+`gh pr diff $ARGUMENTS 2>/dev/null || git diff main...HEAD --stat 2>/dev/null || echo "Could not get diff"`+"`"+`

## Context
!`+"`"+`cat %s/llms.txt 2>/dev/null`+"`"+`

## Playbook
!`+"`"+`cat %s/.ohara-playbooks/review-pr.md 2>/dev/null`+"`"+`
`, hubName, hubName))

	// /validate-docs — inline check
	createSkill(skillsDir, "validate-docs", fmt.Sprintf(`---
name: validate-docs
description: Check documentation structure and coverage
model: haiku
effort: low
when-to-use: When the user asks about documentation quality, coverage, or structure
allowed-tools: Read, Grep, Glob, Bash
---

!`+"`"+`cd %s && ohara validate 2>&1`+"`"+`
`, hubName))

	// /create-docs-pr — manual PR
	createSkill(skillsDir, "create-docs-pr", fmt.Sprintf(`---
name: create-docs-pr
description: Create a PR with documentation changes
argument-hint: <description>
disable-model-invocation: true
allowed-tools: Read, Grep, Glob, Bash
---

cd %s && ohara build && ohara validate
git checkout -b docs/$ARGUMENTS
git add -A && git commit -m "docs: $ARGUMENTS"
git push origin docs/$ARGUMENTS
gh pr create --title "docs: $ARGUMENTS" --body "Documentation update"
`, hubName))

	fmt.Printf("✓ Created .claude/skills/ (7 skills)\n")
}

func writeAgent(dir, filename, content string) {
	os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644)
}

func createSkill(skillsDir, name, content string) {
	dir := filepath.Join(skillsDir, name)
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644)
}
