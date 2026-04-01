package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin [output-dir]",
	Short: "Generate a distributable Claude Code plugin",
	Long: `Generates a standalone Claude Code plugin directory from your Ohara hub.

Share this plugin with teammates who want to use your docs hub without
running 'ohara init' in their workspace. They install the plugin and
set their hub path when prompted.

The plugin includes agents, skills, hooks, and MCP server configuration.

Usage:
  ohara plugin                    # Creates ./ohara-plugin/
  ohara plugin ./my-plugin        # Custom output directory

Installation (for recipients):
  claude --plugin-dir ./ohara-plugin
  # Or copy to ~/.claude/plugins/ohara-docs/`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		outputDir := "ohara-plugin"
		if len(args) > 0 {
			outputDir = args[0]
		}

		// Find hub for metadata
		hubRoot, err := FindHubRoot(".")
		if err != nil {
			return fmt.Errorf("no Ohara hub found. Run 'ohara init' first")
		}

		config, err := LoadHubConfig(hubRoot)
		if err != nil {
			return fmt.Errorf("failed to read hub config: %w", err)
		}

		return generatePlugin(outputDir, config)
	},
}

func generatePlugin(outputDir string, config *HubConfig) error {
	// Create output directories
	os.MkdirAll(filepath.Join(outputDir, "agents"), 0755)
	os.MkdirAll(filepath.Join(outputDir, "skills"), 0755)

	fmt.Printf("Generating Claude Code plugin at %s/\n\n", outputDir)

	// 1. Generate plugin manifest
	if err := generatePluginManifest(outputDir); err != nil {
		return err
	}
	fmt.Println("✓ plugin.json")

	// 2. Generate agents (generic hub references)
	generatePluginAgents(outputDir)
	fmt.Println("✓ agents/ (5 subagents)")

	// 3. Generate skills (using $OHARA_HUB_PATH)
	generatePluginSkills(outputDir)
	fmt.Println("✓ skills/ (7 skills)")

	// 4. Generate README
	generatePluginReadme(outputDir, config)
	fmt.Println("✓ README.md")

	fmt.Printf("\n✓ Plugin generated at %s/\n", outputDir)
	fmt.Printf("\nTo use:\n")
	fmt.Printf("  claude --plugin-dir %s\n", outputDir)
	fmt.Printf("\nOr copy to ~/.claude/plugins/ohara-docs/ for persistent use.\n")

	return nil
}

func generatePluginManifest(outputDir string) error {
	// Hub path placeholder — Claude Code prompts user at install
	hubPathRef := "${user_config.OHARA_HUB_PATH}"

	manifest := map[string]interface{}{
		"name":        "ohara-docs",
		"version":     Version,
		"description": "Agent-optimized documentation hub for Claude Code. Diataxis-structured docs with hooks, agents, and MCP integration.",
		"author": map[string]interface{}{
			"name": "Lackech",
			"url":  "https://github.com/Lackech",
		},
		"repository": "https://github.com/Lackech/ohara",
		"license":    "MIT",
		"keywords":   []string{"documentation", "diataxis", "docs-hub", "agents", "mcp"},
		"userConfig": map[string]interface{}{
			"OHARA_HUB_PATH": map[string]interface{}{
				"type":        "directory",
				"title":       "Ohara hub directory",
				"description": "Path to your ohara docs hub (the directory containing .ohara.yaml)",
				"required":    true,
			},
		},
		"agents": []string{"./agents/"},
		"skills": []string{"./skills/"},
		"mcpServers": map[string]interface{}{
			"ohara": map[string]interface{}{
				"command": "ohara",
				"args":    []string{"serve"},
				"cwd":     hubPathRef,
			},
		},
		"hooks": pluginHooksConfig(hubPathRef),
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputDir, "plugin.json"), data, 0644)
}

func pluginHooksConfig(hubPathRef string) map[string]interface{} {
	return map[string]interface{}{
		"SessionStart": []map[string]interface{}{
			{
				"hooks": []map[string]interface{}{
					{"type": "command", "command": "ohara session-start", "cwd": hubPathRef, "statusMessage": "Loading ohara hub..."},
				},
			},
		},
		"UserPromptSubmit": []map[string]interface{}{
			{
				"hooks": []map[string]interface{}{
					{"type": "command", "command": "ohara hook-prompt", "cwd": hubPathRef, "statusMessage": "Searching docs..."},
				},
			},
		},
		"PreToolUse": []map[string]interface{}{
			{
				"matcher": "Edit|Write",
				"hooks": []map[string]interface{}{
					{"type": "command", "command": "ohara gate $TOOL_INPUT_FILE", "cwd": hubPathRef},
				},
			},
		},
		"PostToolUse": []map[string]interface{}{
			{
				"matcher": "Bash",
				"hooks": []map[string]interface{}{
					{"type": "command", "command": "ohara watch-hook", "cwd": hubPathRef, "if": "Bash(git *)|Bash(gh pr *)"},
				},
			},
		},
		"Stop": []map[string]interface{}{
			{
				"hooks": []map[string]interface{}{
					{"type": "command", "command": "ohara session-summary", "cwd": hubPathRef},
				},
			},
		},
		"FileChanged": []map[string]interface{}{
			{
				"hooks": []map[string]interface{}{
					{"type": "command", "command": "ohara file-changed", "cwd": hubPathRef, "statusMessage": "Rebuilding docs index..."},
				},
			},
		},
		"SubagentStart": []map[string]interface{}{
			{
				"hooks": []map[string]interface{}{
					{"type": "command", "command": "ohara subagent-start", "cwd": hubPathRef},
				},
			},
		},
		"PreCompact": []map[string]interface{}{
			{
				"hooks": []map[string]interface{}{
					{"type": "command", "command": "ohara pre-compact", "cwd": hubPathRef, "statusMessage": "Preserving hub context..."},
				},
			},
		},
	}
}

func generatePluginAgents(outputDir string) {
	agentsDir := filepath.Join(outputDir, "agents")

	// Plugin agents use generic hub reference — SessionStart hook injects actual path
	hubRef := "the ohara docs hub"

	writeAgent(agentsDir, "ohara-writer.md", `---
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

Hub: `+hubRef+`
`)

	writeAgent(agentsDir, "ohara-reviewer.md", `---
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

Hub: `+hubRef+`
`)

	writeAgent(agentsDir, "ohara-researcher.md", `---
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

Hub: `+hubRef+`
`)

	writeAgent(agentsDir, "ohara-watcher.md", `---
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

Hub: `+hubRef+`
`)

	writeAgent(agentsDir, "ohara-orchestrator.md", `---
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
  Follow playbook phases strictly. Give workers precise instructions with file paths and line numbers.
  Never fabricate results — wait for task notifications. File ownership is STRICT — no two agents edit the same file.
---

Execute playbooks using Claude Code's native tools.

## For subagent phases

Agent({ prompt: "task + scratch context", subagent_type: "Explore" or "general-purpose", isolation: "worktree" })

## For team phases

TeamCreate({ team_name: "<task-id>" })
TaskCreate({ subject: "per-agent task", description: "with file ownership" })
Agent({ team_name: "<task-id>", name: "agent-1", isolation: "worktree" })

## Rules

1. File ownership: no two agents edit the same file
2. Cross-repo: one agent per repo, shared types first
3. Review gates: STOP, present findings, wait for approval
4. After completion: ohara-writer for docs, ohara build, create PR
5. Write all coordination to .scratch/tasks/

Hub: `+hubRef+`
`)
}

func generatePluginSkills(outputDir string) {
	skillsDir := filepath.Join(outputDir, "skills")
	hubVar := "$OHARA_HUB_PATH"

	createSkill(skillsDir, "ohara-service-index", fmt.Sprintf(`---
name: ohara-service-index
description: Current documentation index
user-invocable: false
---

## Doc Index
`+"`"+`cat %s/llms.txt 2>/dev/null || echo "No docs yet — run ohara generate"`+"`"+`
`, hubVar))

	createSkill(skillsDir, "fix", fmt.Sprintf(`---
name: fix
description: Fix a bug with a coordinated agent team
argument-hint: <description of the bug>
disable-model-invocation: true
agent: ohara-orchestrator
model: opus
when-to-use: When the user describes a bug, error, or unexpected behavior that needs fixing
---

Run fix-bug playbook for: $ARGUMENTS

## Context
`+"`"+`cat %s/llms.txt 2>/dev/null`+"`"+`

## Playbook
`+"`"+`cat %s/.ohara-playbooks/fix-bug.md 2>/dev/null`+"`"+`
`, hubVar, hubVar))

	createSkill(skillsDir, "feature", fmt.Sprintf(`---
name: feature
description: Build a new feature with a coordinated agent team
argument-hint: <description of the feature>
disable-model-invocation: true
agent: ohara-orchestrator
model: opus
when-to-use: When the user wants to add new functionality or build something new
---

Run new-feature playbook for: $ARGUMENTS

## Context
`+"`"+`cat %s/llms.txt 2>/dev/null`+"`"+`
`+"`"+`cat %s/.ohara.yaml 2>/dev/null`+"`"+`

## Playbook
`+"`"+`cat %s/.ohara-playbooks/new-feature.md 2>/dev/null`+"`"+`
`, hubVar, hubVar, hubVar))

	createSkill(skillsDir, "investigate", fmt.Sprintf(`---
name: investigate
description: Investigate a problem with competing hypotheses
argument-hint: <what to investigate>
disable-model-invocation: true
agent: ohara-orchestrator
model: opus
when-to-use: When the user wants to research, understand, or diagnose a problem
---

Run investigate playbook for: $ARGUMENTS

## Context
`+"`"+`cat %s/llms.txt 2>/dev/null`+"`"+`

## Playbook
`+"`"+`cat %s/.ohara-playbooks/investigate.md 2>/dev/null`+"`"+`
`, hubVar, hubVar))

	createSkill(skillsDir, "review-pr", fmt.Sprintf(`---
name: review-pr
description: Multi-perspective PR review
argument-hint: <PR number or branch>
disable-model-invocation: true
agent: ohara-orchestrator
model: opus
when-to-use: When the user wants to review a pull request or code changes
allowed-tools: Read, Grep, Glob, Bash, Agent
---

Run review-pr playbook for: $ARGUMENTS

## PR Diff
`+"`"+`gh pr diff $ARGUMENTS 2>/dev/null || git diff main...HEAD --stat 2>/dev/null || echo "Could not get diff"`+"`"+`

## Context
`+"`"+`cat %s/llms.txt 2>/dev/null`+"`"+`

## Playbook
`+"`"+`cat %s/.ohara-playbooks/review-pr.md 2>/dev/null`+"`"+`
`, hubVar, hubVar))

	createSkill(skillsDir, "validate-docs", fmt.Sprintf(`---
name: validate-docs
description: Check documentation structure and coverage
model: haiku
effort: low
when-to-use: When the user asks about documentation quality, coverage, or structure
allowed-tools: Read, Grep, Glob, Bash
---

`+"`"+`cd %s && ohara validate 2>&1`+"`"+`
`, hubVar))

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
`, hubVar))
}

func generatePluginReadme(outputDir string, config *HubConfig) {
	readme := "# Ohara Docs Plugin for Claude Code\n\n" +
		fmt.Sprintf("Generated by [Ohara](https://github.com/Lackech/ohara) v%s.\n\n", Version) +
		"## Prerequisites\n\n" +
		"Install the ohara CLI:\n\n" +
		"    brew install lackech/ohara/ohara\n\n" +
		"## Installation\n\n" +
		"**Option 1: Session-only**\n\n" +
		fmt.Sprintf("    claude --plugin-dir %s\n\n", outputDir) +
		"**Option 2: Persistent**\n\n" +
		fmt.Sprintf("    cp -r %s ~/.claude/plugins/ohara-docs\n\n", outputDir) +
		"## Setup\n\n" +
		"When you first use the plugin, Claude Code will ask for your hub path.\n" +
		"Point it to the directory containing your .ohara.yaml file.\n\n" +
		"## What's Included\n\n" +
		"- **5 agents**: writer, reviewer, researcher, watcher, orchestrator\n" +
		"- **7 skills**: /fix, /feature, /investigate, /review-pr, /validate-docs, /create-docs-pr\n" +
		"- **8 hooks**: SessionStart, UserPromptSubmit, PreToolUse, PostToolUse, Stop, FileChanged, SubagentStart, PreCompact\n" +
		"- **MCP server**: search_docs, read_doc, write_doc, validate, list_docs, create_pr, changelog\n" +
		"- **Status line**: live hub status in Claude Code UI\n\n" +
		fmt.Sprintf("## Hub: %s (%d services)\n", config.Name, len(config.Repos))

	os.WriteFile(filepath.Join(outputDir, "README.md"), []byte(readme), 0644)
}

func init() {
	rootCmd.AddCommand(pluginCmd)
}
