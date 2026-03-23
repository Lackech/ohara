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

		// Create CLAUDE.md in the WORKSPACE root (parent), not inside hub
		// This is where the developer opens Claude Code
		claudeMd := "# " + name + " — Workspace\n\n" +
			"Documentation hub is in `" + hubName + "/`. Managed by Ohara.\n\n" +
			"## Agent Commands\n\n" +
			"Use these slash commands:\n\n" +
			"- `/search-docs <query>` — Search across all documentation\n" +
			"- `/generate-docs <service>` — Generate docs from a service's source code\n" +
			"- `/validate-docs` — Check documentation structure and coverage\n" +
			"- `/create-docs-pr <description>` — Create a PR with doc changes\n" +
			"- `/docs-changelog [service]` — Show recent documentation changes\n\n" +
			"## Quick Reference\n\n" +
			"- `" + hubName + "/llms.txt` — Index of all docs (read this first)\n" +
			"- `" + hubName + "/llms-full.txt` — Full content of all docs\n" +
			"- `" + hubName + "/AGENTS.md` — Detailed agent instructions\n\n" +
			"## Diataxis Types\n\n" +
			"| Need | Look in | Type |\n" +
			"|------|---------|------|\n" +
			"| Execute a task | `<service>/guides/` | How-to Guide |\n" +
			"| Learn a system | `<service>/tutorials/` | Tutorial |\n" +
			"| Look up a param | `<service>/reference/` | Reference |\n" +
			"| Understand why | `<service>/explanation/` | Explanation |\n"
		os.WriteFile(filepath.Join(workDir, "CLAUDE.md"), []byte(claudeMd), 0644)
		fmt.Printf("✓ Created CLAUDE.md (workspace root)\n")

		// Create Claude Code commands in WORKSPACE root
		claudeCmdsDir := filepath.Join(workDir, ".claude", "commands")
		os.MkdirAll(claudeCmdsDir, 0755)

		// Search command
		os.WriteFile(filepath.Join(claudeCmdsDir, "search-docs.md"), []byte(
			"Search the documentation hub for information.\n\n"+
				"Arguments: $ARGUMENTS (the search query)\n\n"+
				"Steps:\n"+
				"1. Read `"+hubName+"/llms.txt` to understand the doc structure\n"+
				"2. Grep across all docs: `grep -ri \"$ARGUMENTS\" "+hubName+"/ --include=\"*.md\" -l`\n"+
				"3. Read the most relevant files\n"+
				"4. Summarize findings with links to the source files\n",
		), 0644)

		// Generate docs command
		os.WriteFile(filepath.Join(claudeCmdsDir, "generate-docs.md"), []byte(
			"Generate documentation for a tracked repository.\n\n"+
				"Arguments: $ARGUMENTS (the repo name, e.g., 'my-api')\n\n"+
				"Steps:\n"+
				"1. Run `cd "+hubName+" && ohara generate $ARGUMENTS`\n"+
				"2. Read the prompts in `"+hubName+"/$ARGUMENTS/.ohara-prompts/`\n"+
				"3. For EACH prompt file:\n"+
				"   a. Read the prompt to understand what doc to write\n"+
				"   b. Read the relevant source code from `$ARGUMENTS/` (the actual code repo)\n"+
				"   c. Write real, specific documentation based on the actual code\n"+
				"   d. Save to the corresponding path in `"+hubName+"/$ARGUMENTS/`\n"+
				"4. Run `cd "+hubName+" && ohara build` to regenerate llms.txt\n"+
				"5. Run `cd "+hubName+" && ohara validate` to check the result\n",
		), 0644)

		// Validate command
		os.WriteFile(filepath.Join(claudeCmdsDir, "validate-docs.md"), []byte(
			"Validate the documentation hub structure and coverage.\n\n"+
				"Steps:\n"+
				"1. Run `cd "+hubName+" && ohara validate`\n"+
				"2. Review the output for errors and warnings\n"+
				"3. For each TODO placeholder, read the prompt in `.ohara-prompts/`\n"+
				"   and the source code, then generate real content\n"+
				"4. For missing Diataxis types, suggest what docs to create\n",
		), 0644)

		// PR command
		os.WriteFile(filepath.Join(claudeCmdsDir, "create-docs-pr.md"), []byte(
			"Create a PR with documentation changes.\n\n"+
				"Arguments: $ARGUMENTS (description of the changes)\n\n"+
				"Steps:\n"+
				"1. `cd "+hubName+"`\n"+
				"2. Run `ohara build` to regenerate llms.txt and AGENTS.md\n"+
				"3. Run `ohara validate` to check for issues\n"+
				"4. `git checkout -b docs/$ARGUMENTS`\n"+
				"5. `git add -A`\n"+
				"6. `git commit -m \"docs: $ARGUMENTS\"`\n"+
				"7. `git push origin docs/$ARGUMENTS`\n"+
				"8. `gh pr create --title \"docs: $ARGUMENTS\" --body \"Documentation update\"`\n"+
				"9. Report the PR URL\n",
		), 0644)

		// Changelog command
		os.WriteFile(filepath.Join(claudeCmdsDir, "docs-changelog.md"), []byte(
			"Show recent documentation changes.\n\n"+
				"Arguments: $ARGUMENTS (optional: service name to filter)\n\n"+
				"Steps:\n"+
				"1. `cd "+hubName+"`\n"+
				"2. If a service name is provided:\n"+
				"   `git log --oneline -20 -- $ARGUMENTS/`\n"+
				"3. Otherwise show all recent changes:\n"+
				"   `git log --oneline -20`\n"+
				"4. For important changes, show the diff:\n"+
				"   `git show <commit-hash> --stat`\n"+
				"5. Summarize what changed, when, and why\n",
		), 0644)

		fmt.Printf("✓ Created .claude/commands/ (5 agent skills)\n")

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
