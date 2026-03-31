package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade Ohara agents, skills, playbooks, and config",
	Long: `Updates the Ohara agent infrastructure without touching your docs or hub content.

What gets updated:
  - .claude/agents/         (subagent definitions)
  - .claude/skills/         (skill definitions)
  - .claude/settings.json   (MCP server config)
  - CLAUDE.md               (workspace discovery file)
  - .ohara-playbooks/       (playbook definitions)

What is preserved:
  - All documentation files (your actual docs)
  - .ohara.yaml             (hub config + tracked repos)
  - .scratch/               (active task state)
  - Agent memory            (.claude/agent-memory/)
  - Custom playbooks you added
  - Git history`,
	RunE: func(cmd *cobra.Command, args []string) error {
		workDir, _ := os.Getwd()

		// Find the hub
		hubRoot, err := FindHubRoot(workDir)
		if err != nil {
			// Maybe we're in the workspace root, check for hub
			hubRoot, err = FindHubRoot(filepath.Join(workDir, "ohara-docs"))
			if err != nil {
				return fmt.Errorf("no Ohara hub found. Are you in the workspace root?")
			}
			// We're in the workspace root
		} else {
			// We're inside the hub, go to workspace root
			workDir = filepath.Dir(hubRoot)
		}

		hubName := filepath.Base(hubRoot)

		config, err := LoadHubConfig(hubRoot)
		if err != nil {
			return fmt.Errorf("failed to read hub config: %w", err)
		}

		fmt.Printf("Upgrading Ohara in workspace: %s\n", workDir)
		fmt.Printf("Hub: %s/ (%d repos tracked)\n\n", hubName, len(config.Repos))

		// 1. Update agents + skills
		fmt.Println("Updating agent infrastructure...")
		createOharaAgentConfig(workDir, hubName)

		// 2. Update CLAUDE.md
		claudeMd := buildClaudeMd(config, hubName)
		os.WriteFile(filepath.Join(workDir, "CLAUDE.md"), []byte(claudeMd), 0644)
		fmt.Printf("✓ Updated CLAUDE.md\n")

		// 3. Update settings (MCP + hooks)
		mcpConfigDir := filepath.Join(workDir, ".claude")
		os.MkdirAll(mcpConfigDir, 0755)
		writeSettingsJson(mcpConfigDir, hubName)
		fmt.Printf("✓ Updated .claude/settings.json\n")

		// 4. Update starter playbooks (don't overwrite custom ones)
		updatePlaybooks(hubRoot)

		// 5. Ensure scratch space exists
		os.MkdirAll(filepath.Join(hubRoot, ".scratch", "tasks"), 0755)
		os.MkdirAll(filepath.Join(hubRoot, ".scratch", "handoffs"), 0755)

		// 6. Ensure .gitignore has all entries
		updateGitignore(hubRoot)

		fmt.Printf("\n✓ Upgrade complete. Restart Claude Code to pick up changes.\n")

		return nil
	},
}

func writeSettingsJson(configDir, hubName string) {
	settings := map[string]interface{}{
		"env": map[string]interface{}{
			"CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1",
		},
		"mcpServers": map[string]interface{}{
			"ohara": map[string]interface{}{
				"command": "ohara",
				"args":    []string{"serve"},
				"cwd":     hubName,
			},
		},
		"statusLine": map[string]interface{}{
			"type":    "command",
			"command": "ohara status-line",
		},
		"hooks": map[string]interface{}{
			"SessionStart": []map[string]interface{}{
				{
					"hooks": []map[string]interface{}{
						{"type": "command", "command": "ohara session-start", "statusMessage": "Loading ohara hub..."},
					},
				},
			},
			"UserPromptSubmit": []map[string]interface{}{
				{
					"hooks": []map[string]interface{}{
						{"type": "command", "command": "ohara hook-prompt", "statusMessage": "Searching docs..."},
					},
				},
			},
			"PreToolUse": []map[string]interface{}{
				{
					"matcher": "Edit|Write",
					"hooks": []map[string]interface{}{
						{"type": "command", "command": "ohara gate $TOOL_INPUT_FILE"},
					},
				},
			},
			"PostToolUse": []map[string]interface{}{
				{
					"matcher": "Bash",
					"hooks": []map[string]interface{}{
						{"type": "command", "command": "ohara watch-hook", "if": "Bash(git *)|Bash(gh pr *)"},
					},
				},
			},
			"Stop": []map[string]interface{}{
				{
					"hooks": []map[string]interface{}{
						{"type": "command", "command": "ohara session-summary"},
					},
				},
			},
			"FileChanged": []map[string]interface{}{
				{
					"hooks": []map[string]interface{}{
						{"type": "command", "command": "ohara file-changed", "statusMessage": "Rebuilding docs index..."},
					},
				},
			},
			"SubagentStart": []map[string]interface{}{
				{
					"hooks": []map[string]interface{}{
						{"type": "command", "command": "ohara subagent-start"},
					},
				},
			},
			"PreCompact": []map[string]interface{}{
				{
					"hooks": []map[string]interface{}{
						{"type": "command", "command": "ohara pre-compact", "statusMessage": "Preserving hub context..."},
					},
				},
			},
			"TeammateIdle": []map[string]interface{}{
				{
					"hooks": []map[string]interface{}{
						{"type": "command", "command": "ohara teammate-idle", "statusMessage": "Checking team progress..."},
					},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(filepath.Join(configDir, "settings.json"), data, 0644)
}

func buildClaudeMd(config *HubConfig, hubName string) string {
	var sb strings.Builder

	sb.WriteString("# " + config.Name + "\n\n")
	sb.WriteString("Doc hub: `" + hubName + "/` — read `" + hubName + "/llms.txt` for index. Ohara v" + Version + ".\n\n")

	// Diataxis overview (Layer A: static, cached by prompt cache)
	sb.WriteString("## Diataxis Structure\n\n")
	sb.WriteString("All docs follow [Diataxis](https://diataxis.fr/) — four complementary types:\n\n")
	sb.WriteString("| Type | Purpose | Directory | Query patterns |\n")
	sb.WriteString("|------|---------|-----------|----------------|\n")
	sb.WriteString("| Tutorial | Learning-oriented walkthrough | `tutorials/` | \"getting started\", \"walkthrough\" |\n")
	sb.WriteString("| Guide | Task-oriented how-to | `guides/` | \"how do I\", \"how to\" |\n")
	sb.WriteString("| Reference | Exhaustive facts, API specs | `reference/` | \"what is\", \"API\", \"config\" |\n")
	sb.WriteString("| Explanation | Design rationale, architecture | `explanation/` | \"why\", \"architecture\" |\n\n")

	// Services list
	if len(config.Repos) > 0 {
		sb.WriteString("## Tracked Services\n\n")
		for _, repo := range config.Repos {
			sb.WriteString(fmt.Sprintf("- **%s** — docs: `%s/%s/`, code: `%s`\n", repo.Name, hubName, repo.Name, repo.Path))
		}
		sb.WriteString("\n")
	}

	// Team table
	sb.WriteString("## Team\n\n")
	sb.WriteString("| Agent | Strength | When to use |\n")
	sb.WriteString("|-------|----------|-------------|\n")
	sb.WriteString("| ohara-researcher | Fast doc lookup | Before touching unfamiliar service |\n")
	sb.WriteString("| ohara-writer | Doc generation from code | After code changes |\n")
	sb.WriteString("| ohara-reviewer | Accuracy check | After doc changes |\n")
	sb.WriteString("| ohara-watcher | Staleness detection | After git pull (auto via hook) |\n")
	sb.WriteString("| ohara-orchestrator | Playbook execution | Spawned by /fix, /feature, etc. |\n\n")

	// Commands table
	sb.WriteString("## Commands\n\n")
	sb.WriteString("| Command | What it does |\n")
	sb.WriteString("|---------|-------------|\n")
	sb.WriteString("| `/fix <desc>` | Bug fix: investigate → implement → test → document |\n")
	sb.WriteString("| `/feature <desc>` | New feature: plan → parallel implement → integrate → docs |\n")
	sb.WriteString("| `/investigate <desc>` | Research: competing hypotheses → converge |\n")
	sb.WriteString("| `/review-pr <#>` | PR review: correctness + security + quality → synthesis |\n")
	sb.WriteString("| `/validate-docs` | Check doc structure and coverage |\n")
	sb.WriteString("| `/create-docs-pr <desc>` | Create a PR with doc changes |\n\n")

	// Rules
	sb.WriteString("## Rules\n\n")
	sb.WriteString("1. Never edit a service you haven't read docs for — check `" + hubName + "/<service>/`\n")
	sb.WriteString("2. Multi-step work → use /fix, /feature, /investigate, or /review-pr\n")
	sb.WriteString("3. After code changes, ask user if docs need updating\n")
	sb.WriteString("4. MCP tools: search_docs, read_doc, write_doc, validate, list_docs, create_pr, changelog\n")

	return sb.String()
}

func updatePlaybooks(hubRoot string) {
	playbooksDir := filepath.Join(hubRoot, ".ohara-playbooks")
	os.MkdirAll(playbooksDir, 0755)

	// Only write starter playbooks if they don't exist (preserve customizations)
	starters := []string{"fix-bug.md", "new-feature.md", "investigate.md", "review-pr.md"}
	missing := false
	for _, name := range starters {
		if _, err := os.Stat(filepath.Join(playbooksDir, name)); os.IsNotExist(err) {
			missing = true
			break
		}
	}

	if missing {
		createStarterPlaybooks(hubRoot)
	} else {
		fmt.Printf("✓ Playbooks unchanged (customizations preserved)\n")
	}
}

func updateGitignore(hubRoot string) {
	gitignorePath := filepath.Join(hubRoot, ".gitignore")
	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		content = []byte{}
	}

	text := string(content)
	entries := []string{".ohara-prompts/", ".scratch/", ".DS_Store"}
	updated := false

	for _, entry := range entries {
		if !containsLine(text, entry) {
			text += entry + "\n"
			updated = true
		}
	}

	if updated {
		os.WriteFile(gitignorePath, []byte(text), 0644)
		fmt.Printf("✓ Updated .gitignore\n")
	}
}

func containsLine(text, line string) bool {
	for _, l := range splitLines(text) {
		if l == line {
			return true
		}
	}
	return false
}

func splitLines(text string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			lines = append(lines, text[start:i])
			start = i + 1
		}
	}
	if start < len(text) {
		lines = append(lines, text[start:])
	}
	return lines
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
}
