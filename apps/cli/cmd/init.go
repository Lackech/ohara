package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [directory]",
	Short: "Initialize an Ohara documentation hub",
	Long: `Creates a new documentation hub — a git repo that stores Diataxis-structured
documentation for all the repos you work on.

Run this in your workspace directory (the parent of your code repos):

  ~/work/
  ├── repo-1/          ← your code
  ├── repo-2/          ← your code
  └── ohara-docs/      ← ohara creates this

The hub is a regular git repo. Push it to GitHub for team collaboration,
or keep it local for your agents to read.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workDir, _ := os.Getwd()

		// Determine hub directory name
		hubName := "ohara-docs"
		if len(args) > 0 {
			hubName = args[0]
		}

		hubDir := filepath.Join(workDir, hubName)

		// Check if hub already exists
		if _, err := os.Stat(filepath.Join(hubDir, hubConfigFile)); err == nil {
			return fmt.Errorf("hub already exists at %s", hubDir)
		}

		// Ask for org/project name
		reader := bufio.NewReader(os.Stdin)
		nameFlag, _ := cmd.Flags().GetString("name")
		name := nameFlag
		if name == "" {
			fmt.Print("Hub name (e.g., your org or project): ")
			name, _ = reader.ReadString('\n')
			name = strings.TrimSpace(name)
			if name == "" {
				name = hubName
			}
		}

		// Create hub directory
		if err := os.MkdirAll(hubDir, 0755); err != nil {
			return fmt.Errorf("failed to create hub directory: %w", err)
		}

		// Create .ohara.yaml
		config := &HubConfig{
			Name:  name,
			Repos: []RepoEntry{},
		}

		if err := SaveHubConfig(hubDir, config); err != nil {
			return err
		}
		fmt.Printf("✓ Created %s/%s\n", hubName, hubConfigFile)

		// Create .gitignore
		gitignore := ".ohara-prompts/\n.DS_Store\n"
		os.WriteFile(filepath.Join(hubDir, ".gitignore"), []byte(gitignore), 0644)

		// Create README
		readme := fmt.Sprintf("# %s — Documentation Hub\n\nManaged by [Ohara](https://github.com/Lackech/ohara). Diataxis-structured documentation for all services.\n\n## Structure\n\nEach directory corresponds to a tracked repository:\n\n```\n%s/\n├── .ohara.yaml          ← hub configuration\n├── <repo-name>/\n│   ├── tutorials/       ← learning-oriented\n│   ├── guides/          ← task-oriented\n│   ├── reference/       ← information-oriented\n│   └── explanation/     ← understanding-oriented\n└── ...\n```\n\n## Usage\n\n```bash\nohara add ../my-repo     # track a repo\nohara generate my-repo   # generate docs from code\nohara validate           # check structure\n```\n", name, hubName)
		os.WriteFile(filepath.Join(hubDir, "README.md"), []byte(readme), 0644)
		fmt.Printf("✓ Created %s/README.md\n", hubName)

		// Create CLAUDE.md in the WORKSPACE root (parent), not inside hub
		// This is where the developer opens Claude Code
		claudeMd := "# " + name + "\n\n" +
			"Documentation hub: `" + hubName + "/`. Managed by [Ohara](https://github.com/Lackech/ohara).\n\n" +
			"## Agents\n\n" +
			"Four specialized subagents (in `.claude/agents/`):\n\n" +
			"- **ohara-writer** — Reads code, writes Diataxis docs. Persistent memory + MCP tools.\n" +
			"- **ohara-reviewer** — Reviews docs against code for accuracy.\n" +
			"- **ohara-researcher** — Searches docs to answer questions. Auto-invoked.\n" +
			"- **ohara-watcher** — Detects code changes that make docs stale. Auto-invoked after git pull.\n\n" +
			"## Skills\n\n" +
			"- `/validate-docs` — Check structure and coverage (auto-invoked)\n" +
			"- `/check-staleness [service]` — Compare code changes vs docs (auto-invoked after git pull)\n" +
			"- `/post-merge` — Check docs after PR merge (auto-invoked)\n" +
			"- `/create-docs-pr <desc>` — Branch, commit, push, open PR\n" +
			"- `/docs-changelog [service]` — Recent changes from git log\n\n" +
			"## MCP Tools\n\n" +
			"`ohara serve` provides: `search_docs`, `list_docs`, `read_doc`, `write_doc`, `validate`, `create_pr`, `changelog`\n\n" +
			"## Quick Reference\n\n" +
			"- `" + hubName + "/llms.txt` — Doc index\n" +
			"- `" + hubName + "/llms-full.txt` — Full content (completed only)\n" +
			"- `" + hubName + "/AGENTS.md` — Agent instructions\n\n" +
			"## Diataxis\n\n" +
			"| Need | Look in | Type |\n" +
			"|------|---------|------|\n" +
			"| Execute a task | `<service>/guides/` | How-to Guide |\n" +
			"| Learn a system | `<service>/tutorials/` | Tutorial |\n" +
			"| Look up a param | `<service>/reference/` | Reference |\n" +
			"| Understand why | `<service>/explanation/` | Explanation |\n"
		os.WriteFile(filepath.Join(workDir, "CLAUDE.md"), []byte(claudeMd), 0644)
		fmt.Printf("✓ Created CLAUDE.md (workspace root)\n")

		// Create subagents + skills
		createOharaAgentConfig(workDir, hubName)

		// Create MCP server configuration
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
		fmt.Printf("✓ Created .claude/settings.json (MCP server: ohara serve)\n")

		// Initialize git repo
		gitInit := exec.Command("git", "init")
		gitInit.Dir = hubDir
		gitInit.Stdout = os.Stdout
		gitInit.Stderr = os.Stderr
		if err := gitInit.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to init git repo: %v\n", err)
		} else {
			fmt.Printf("✓ Initialized git repo\n")
		}

		// Ask if user wants to create a remote repo
		remoteFlag, _ := cmd.Flags().GetString("remote")
		if remoteFlag != "" {
			config.DocsRepo = &DocsRepo{Remote: remoteFlag}
			SaveHubConfig(hubDir, config)

			addRemote := exec.Command("git", "remote", "add", "origin", remoteFlag)
			addRemote.Dir = hubDir
			addRemote.Run()
			fmt.Printf("✓ Added remote: %s\n", remoteFlag)
		}

		fmt.Printf("\n✓ Hub \"%s\" created at %s/\n", name, hubName)
		fmt.Printf("\nNext steps:\n")
		fmt.Printf("  cd %s\n", hubName)
		fmt.Printf("  ohara add ../your-repo-1\n")
		fmt.Printf("  ohara add ../your-repo-2\n")
		fmt.Printf("  ohara generate your-repo-1\n")

		return nil
	},
}

func init() {
	initCmd.Flags().StringP("name", "n", "", "Hub name")
	initCmd.Flags().StringP("remote", "r", "", "Git remote URL for the docs repo")
	rootCmd.AddCommand(initCmd)
}
