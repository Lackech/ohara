package cmd

import (
	"bufio"
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
		gitignore := ".ohara-prompts/\n.scratch/\n.DS_Store\n"
		os.WriteFile(filepath.Join(hubDir, ".gitignore"), []byte(gitignore), 0644)

		// Create scratch space for agent coordination
		os.MkdirAll(filepath.Join(hubDir, ".scratch", "tasks"), 0755)
		os.MkdirAll(filepath.Join(hubDir, ".scratch", "handoffs"), 0755)
		os.WriteFile(filepath.Join(hubDir, ".scratch", "README.md"), []byte(
			"# Agent Scratch Space\n\n"+
				"Temporary workspace for agent coordination during tasks.\n"+
				"Gitignored — never committed. Cleaned up after tasks complete.\n\n"+
				"- `tasks/<task-name>/` — Working space per active task (plan, status, findings)\n"+
				"- `handoffs/` — Cross-agent context passing between phases\n",
		), 0644)

		// Create playbooks directory with starters
		createStarterPlaybooks(hubDir)

		// Create README
		readme := fmt.Sprintf("# %s — Documentation Hub\n\nManaged by [Ohara](https://github.com/Lackech/ohara). Diataxis-structured documentation for all services.\n\n## Structure\n\nEach directory corresponds to a tracked repository:\n\n```\n%s/\n├── .ohara.yaml          ← hub configuration\n├── <repo-name>/\n│   ├── tutorials/       ← learning-oriented\n│   ├── guides/          ← task-oriented\n│   ├── reference/       ← information-oriented\n│   └── explanation/     ← understanding-oriented\n└── ...\n```\n\n## Usage\n\n```bash\nohara add ../my-repo     # track a repo\nohara generate my-repo   # generate docs from code\nohara validate           # check structure\n```\n", name, hubName)
		os.WriteFile(filepath.Join(hubDir, "README.md"), []byte(readme), 0644)
		fmt.Printf("✓ Created %s/README.md\n", hubName)

		// Create CLAUDE.md in the WORKSPACE root using shared builder
		os.WriteFile(filepath.Join(workDir, "CLAUDE.md"), []byte(buildClaudeMd(config, hubName)), 0644)
		fmt.Printf("✓ Created CLAUDE.md (workspace root)\n")

		// Create subagents + skills
		createOharaAgentConfig(workDir, hubName)

		// Create MCP server configuration
		mcpConfigDir := filepath.Join(workDir, ".claude")
		os.MkdirAll(mcpConfigDir, 0755)

		writeSettingsJson(mcpConfigDir, hubName)
		fmt.Printf("✓ Created .claude/settings.json (MCP + hooks)\n")

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
