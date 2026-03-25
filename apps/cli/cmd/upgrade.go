package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
		claudeMd := buildClaudeMd(config.Name, hubName)
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
		"mcpServers": map[string]interface{}{
			"ohara": map[string]interface{}{
				"command": "ohara",
				"args":    []string{"serve"},
				"cwd":     hubName,
			},
		},
		"hooks": map[string]interface{}{
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
						{"type": "command", "command": "ohara watch-hook"},
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
		},
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(filepath.Join(configDir, "settings.json"), data, 0644)
}

func buildClaudeMd(name, hubName string) string {
	return "# " + name + "\n\n" +
		"Doc hub: `" + hubName + "/` — read `" + hubName + "/llms.txt` for index. Ohara v" + Version + ".\n\n" +
		"## Team\n\n" +
		"| Agent | Strength | When to use |\n" +
		"|-------|----------|-------------|\n" +
		"| ohara-researcher | Fast doc lookup | Before touching unfamiliar service |\n" +
		"| ohara-writer | Doc generation from code | After code changes |\n" +
		"| ohara-reviewer | Accuracy check | After doc changes |\n" +
		"| ohara-watcher | Staleness detection | After git pull (auto via hook) |\n" +
		"| ohara-orchestrator | Playbook execution | Spawned by /fix, /feature, etc. |\n\n" +
		"## Commands\n\n" +
		"| Command | What it does |\n" +
		"|---------|-------------|\n" +
		"| `/fix <desc>` | Bug fix: investigate → implement → test → document |\n" +
		"| `/feature <desc>` | New feature: plan → parallel implement → integrate → docs |\n" +
		"| `/investigate <desc>` | Research: competing hypotheses → converge |\n" +
		"| `/review-pr <#>` | PR review: correctness + security + quality → synthesis |\n" +
		"| `/validate-docs` | Check doc structure and coverage |\n" +
		"| `/create-docs-pr <desc>` | Create a PR with doc changes |\n\n" +
		"## Rules\n\n" +
		"1. Never edit a service you haven't read docs for\n" +
		"2. Multi-step work → use /fix, /feature, /investigate, or /review-pr\n" +
		"3. After code changes, ask user if docs need updating\n"
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
