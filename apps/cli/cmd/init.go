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
		gitignore := ".ohara-prompts/\n.DS_Store\n"
		os.WriteFile(filepath.Join(hubDir, ".gitignore"), []byte(gitignore), 0644)

		// Create README
		readme := fmt.Sprintf("# %s — Documentation Hub\n\nManaged by [Ohara](https://github.com/Lackech/ohara). Diataxis-structured documentation for all services.\n\n## Structure\n\nEach directory corresponds to a tracked repository:\n\n```\n%s/\n├── .ohara.yaml          ← hub configuration\n├── <repo-name>/\n│   ├── tutorials/       ← learning-oriented\n│   ├── guides/          ← task-oriented\n│   ├── reference/       ← information-oriented\n│   └── explanation/     ← understanding-oriented\n└── ...\n```\n\n## Usage\n\n```bash\nohara add ../my-repo     # track a repo\nohara generate my-repo   # generate docs from code\nohara validate           # check structure\n```\n", name, hubName)
		os.WriteFile(filepath.Join(hubDir, "README.md"), []byte(readme), 0644)
		fmt.Printf("✓ Created %s/README.md\n", hubName)

		// Create CLAUDE.md for agent discovery
		claudeMd := "# " + name + " — Documentation Hub\n\n" +
			"This is a Diataxis-structured documentation hub managed by Ohara.\n\n" +
			"## Agent Commands\n\n" +
			"Use these slash commands (defined in `.claude/commands/`):\n\n" +
			"- `/search-docs <query>` — Search across all documentation\n" +
			"- `/generate-docs <service>` — Generate docs from a service's source code\n" +
			"- `/validate-docs` — Check documentation structure and coverage\n" +
			"- `/create-docs-pr <description>` — Create a PR with doc changes\n" +
			"- `/docs-changelog [service]` — Show recent documentation changes\n\n" +
			"## Quick Reference\n\n" +
			"- `llms.txt` — Index of all docs (read this first)\n" +
			"- `llms-full.txt` — Full content of all docs\n" +
			"- `AGENTS.md` — Detailed agent instructions + PR workflow\n" +
			"- Each subdirectory = a tracked service with `tutorials/`, `guides/`, `reference/`, `explanation/`\n\n" +
			"## Diataxis Types\n\n" +
			"| Need | Look in | Type |\n" +
			"|------|---------|------|\n" +
			"| Execute a task | `guides/` | How-to Guide |\n" +
			"| Learn a system | `tutorials/` | Tutorial |\n" +
			"| Look up a param | `reference/` | Reference |\n" +
			"| Understand why | `explanation/` | Explanation |\n"
		os.WriteFile(filepath.Join(hubDir, "CLAUDE.md"), []byte(claudeMd), 0644)
		fmt.Printf("✓ Created %s/CLAUDE.md\n", hubName)

		// Create Claude Code commands (.claude/commands/)
		claudeCmdsDir := filepath.Join(hubDir, ".claude", "commands")
		os.MkdirAll(claudeCmdsDir, 0755)

		// Search command
		os.WriteFile(filepath.Join(claudeCmdsDir, "search-docs.md"), []byte(
			"Search the documentation hub for information.\n\n"+
				"Read `llms.txt` for a quick index, then search through the relevant\n"+
				"service directories. Use grep across the markdown files to find specific topics.\n\n"+
				"Arguments: $ARGUMENTS (the search query)\n\n"+
				"Steps:\n"+
				"1. Read llms.txt to understand the doc structure\n"+
				"2. Grep across all .md files for the query: `grep -ri \"$ARGUMENTS\" --include=\"*.md\" -l`\n"+
				"3. Read the most relevant files\n"+
				"4. Summarize findings with links to the source files\n",
		), 0644)

		// Generate docs command
		os.WriteFile(filepath.Join(claudeCmdsDir, "generate-docs.md"), []byte(
			"Generate documentation for a tracked repository.\n\n"+
				"Arguments: $ARGUMENTS (the repo name, e.g., 'my-api')\n\n"+
				"Steps:\n"+
				"1. Run `ohara generate $ARGUMENTS` to scaffold docs and create prompts\n"+
				"2. Read the prompts in `$ARGUMENTS/.ohara-prompts/`\n"+
				"3. For each prompt, read the referenced source code files\n"+
				"4. Write real, specific documentation based on the actual code\n"+
				"5. Write each doc to the corresponding path in `$ARGUMENTS/`\n"+
				"6. Run `ohara validate` to check the result\n"+
				"7. Run `ohara build` to regenerate llms.txt\n",
		), 0644)

		// Validate command
		os.WriteFile(filepath.Join(claudeCmdsDir, "validate-docs.md"), []byte(
			"Validate the documentation hub structure and coverage.\n\n"+
				"Steps:\n"+
				"1. Run `ohara validate`\n"+
				"2. Review the output for errors and warnings\n"+
				"3. For each TODO placeholder warning, read the corresponding prompt\n"+
				"   in `.ohara-prompts/` and generate the content\n"+
				"4. For missing Diataxis types, suggest what docs should be created\n",
		), 0644)

		// PR command
		os.WriteFile(filepath.Join(claudeCmdsDir, "create-docs-pr.md"), []byte(
			"Create a PR with documentation changes.\n\n"+
				"Arguments: $ARGUMENTS (description of the changes)\n\n"+
				"Steps:\n"+
				"1. Run `ohara build` to regenerate llms.txt and AGENTS.md\n"+
				"2. Run `ohara validate` to check for issues\n"+
				"3. Create a branch: `git checkout -b docs/$ARGUMENTS`\n"+
				"4. Stage changes: `git add -A`\n"+
				"5. Commit: `git commit -m \"docs: $ARGUMENTS\"`\n"+
				"6. Push: `git push origin docs/$ARGUMENTS`\n"+
				"7. Create PR: `gh pr create --title \"docs: $ARGUMENTS\" --body \"Auto-generated documentation update\"`\n"+
				"8. Report the PR URL\n",
		), 0644)

		// Changelog command
		os.WriteFile(filepath.Join(claudeCmdsDir, "docs-changelog.md"), []byte(
			"Show recent documentation changes.\n\n"+
				"Arguments: $ARGUMENTS (optional: service name to filter)\n\n"+
				"Steps:\n"+
				"1. If a service name is provided:\n"+
				"   `git log --oneline -20 -- $ARGUMENTS/`\n"+
				"2. Otherwise show all recent changes:\n"+
				"   `git log --oneline -20`\n"+
				"3. For important changes, show the full diff:\n"+
				"   `git show <commit-hash> --stat`\n"+
				"4. Summarize what changed, when, and why\n",
		), 0644)

		fmt.Printf("✓ Created %s/.claude/commands/ (5 agent skills)\n", hubName)

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
