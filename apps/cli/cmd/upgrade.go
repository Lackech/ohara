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

		// 3. Update MCP config
		mcpConfigDir := filepath.Join(workDir, ".claude")
		os.MkdirAll(mcpConfigDir, 0755)
		mcpConfig := map[string]interface{}{
			"mcpServers": map[string]interface{}{
				"ohara": map[string]interface{}{
					"command": "ohara",
					"args":    []string{"serve"},
					"cwd":     hubName,
				},
			},
		}
		mcpData, _ := json.MarshalIndent(mcpConfig, "", "  ")
		os.WriteFile(filepath.Join(mcpConfigDir, "settings.json"), mcpData, 0644)
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

func buildClaudeMd(name, hubName string) string {
	return "# " + name + "\n\n" +
		"Documentation hub: `" + hubName + "/`. Managed by [Ohara](https://github.com/Lackech/ohara).\n\n" +
		"## Standard Operating Procedures\n\n" +
		"BEFORE starting any multi-step task:\n" +
		"1. Read `" + hubName + "/llms.txt` for service context\n" +
		"2. Check if a playbook in `" + hubName + "/.ohara-playbooks/` matches the task\n" +
		"3. If yes: use `/run-playbook <name> <description>`\n" +
		"4. If no: still read relevant docs from the hub before working\n\n" +
		"NEVER start multi-file or cross-repo work without checking the hub first.\n\n" +
		"### When to use playbooks\n\n" +
		"| User says | Playbook | What happens |\n" +
		"|-----------|----------|-------------|\n" +
		"| fix, bug, broken, error, failing | `fix-bug` | investigate → implement → test → document |\n" +
		"| add, build, create, feature | `new-feature` | plan → foundations → parallel implement → docs |\n" +
		"| why, investigate, debug, understand | `investigate` | competing hypotheses → converge |\n" +
		"| review, PR, check | `review-pr` | multi-perspective review → synthesize |\n\n" +
		"Custom playbooks: add `.md` files to `" + hubName + "/.ohara-playbooks/`\n\n" +
		"## Agents\n\n" +
		"| Agent | Auto-invoked | Purpose |\n" +
		"|-------|-------------|--------|\n" +
		"| **ohara-orchestrator** | When running playbooks | Coordinates agent teams, manages phases |\n" +
		"| **ohara-writer** | After ohara generate | Reads code, writes Diataxis docs |\n" +
		"| **ohara-reviewer** | When checking accuracy | Compares docs against code |\n" +
		"| **ohara-researcher** | When asking about services | Searches docs, cites sources |\n" +
		"| **ohara-watcher** | After git pull, PR merge | Detects stale docs (background) |\n\n" +
		"## Skills\n\n" +
		"| Skill | Trigger | Purpose |\n" +
		"|-------|---------|--------|\n" +
		"| `/run-playbook <name> <desc>` | Manual | Execute a playbook |\n" +
		"| `/validate-docs` | Auto | Check structure and coverage |\n" +
		"| `/check-staleness [service]` | Auto after git pull | Code changes vs docs |\n" +
		"| `/post-merge` | Auto after merge | Check if docs need updating |\n" +
		"| `/create-docs-pr <desc>` | Manual | Branch → commit → push → PR |\n" +
		"| `/docs-changelog [service]` | Auto | Recent changes from git log |\n\n" +
		"## Quick Reference\n\n" +
		"- `" + hubName + "/llms.txt` — Doc index (read this first)\n" +
		"- `" + hubName + "/llms-full.txt` — Full content (completed only)\n" +
		"- `" + hubName + "/<service>/CHANGELOG.md` — PR/commit history per service\n" +
		"- `" + hubName + "/AGENTS.md` — Detailed agent instructions\n\n" +
		"## Diataxis\n\n" +
		"| Need | Look in | Type |\n" +
		"|------|---------|------|\n" +
		"| Execute a task | `<service>/guides/` | How-to Guide |\n" +
		"| Learn a system | `<service>/tutorials/` | Tutorial |\n" +
		"| Look up a param | `<service>/reference/` | Reference |\n" +
		"| Understand why | `<service>/explanation/` | Explanation |\n"
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
