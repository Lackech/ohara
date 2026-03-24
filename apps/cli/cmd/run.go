package cmd

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <playbook> <context>",
	Short: "Execute a playbook with an agent team",
	Long: `Runs an Ohara playbook — a reusable workflow that coordinates agents
to accomplish a task. Playbooks live in .ohara-playbooks/ in the hub.

Examples:
  ohara run fix-bug "Users can't login after password reset"
  ohara run new-feature "Add Stripe payment integration"
  ohara run investigate "Why is the API response time increasing?"
  ohara run review-pr "PR #42"

Available playbooks:
  fix-bug        Sequential: investigate → implement → test → document
  new-feature    Phased: plan → foundations → parallel implement → integrate → document
  investigate    Parallel: competing hypotheses → converge on answer
  review-pr      Parallel: multi-perspective review → synthesize`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		playbookName := args[0]
		context := strings.Join(args[1:], " ")

		// Find hub
		hubRoot, err := FindHubRoot(".")
		if err != nil {
			return fmt.Errorf("not inside an Ohara hub. Run 'ohara init' first")
		}

		config, err := LoadHubConfig(hubRoot)
		if err != nil {
			return err
		}

		// Find the playbook
		playbookPath := filepath.Join(hubRoot, ".ohara-playbooks", playbookName+".md")
		if _, err := os.Stat(playbookPath); os.IsNotExist(err) {
			// List available playbooks
			available := listPlaybooks(hubRoot)
			return fmt.Errorf("playbook '%s' not found.\nAvailable: %s", playbookName, strings.Join(available, ", "))
		}

		// Generate task ID
		taskID := generateTaskID(playbookName)

		// Create scratch space for this task
		taskDir := filepath.Join(hubRoot, ".scratch", "tasks", taskID)
		os.MkdirAll(taskDir, 0755)

		// Write task context
		taskContext := fmt.Sprintf("# Task: %s\n\n", taskID)
		taskContext += fmt.Sprintf("**Playbook:** %s\n", playbookName)
		taskContext += fmt.Sprintf("**Created:** %s\n", time.Now().Format(time.RFC3339))
		taskContext += fmt.Sprintf("**Hub:** %s\n", hubRoot)
		taskContext += fmt.Sprintf("**Services:** %s\n\n", listRepoNames(config))
		taskContext += fmt.Sprintf("## Description\n\n%s\n\n", context)
		taskContext += fmt.Sprintf("## Services Available\n\n")
		for _, repo := range config.Repos {
			codePath := ResolveRepoPath(hubRoot, repo)
			taskContext += fmt.Sprintf("- **%s**: code at `%s`, docs at `%s/%s/`\n",
				repo.Name, codePath, filepath.Base(hubRoot), repo.Name)
		}

		os.WriteFile(filepath.Join(taskDir, "context.md"), []byte(taskContext), 0644)

		// Read the playbook content
		playbookContent, _ := os.ReadFile(playbookPath)

		// Copy playbook to task dir for agent reference
		os.WriteFile(filepath.Join(taskDir, "playbook.md"), playbookContent, 0644)

		fmt.Printf("Playbook: %s\n", playbookName)
		fmt.Printf("Task ID:  %s\n", taskID)
		fmt.Printf("Scratch:  %s/.scratch/tasks/%s/\n", filepath.Base(hubRoot), taskID)
		fmt.Printf("Context:  %s\n", context)
		fmt.Printf("\n")

		// Generate the prompt for Claude Code
		hubDirName := filepath.Base(hubRoot)
		workDir := filepath.Dir(hubRoot)
		_ = workDir

		prompt := fmt.Sprintf(`Execute the "%s" playbook for this task.

Task: %s

Task ID: %s

Instructions:
1. Read the playbook at %s/.ohara-playbooks/%s.md
2. Read the task context at %s/.scratch/tasks/%s/context.md
3. Read relevant docs from the hub: %s/llms.txt and service docs
4. Follow the playbook phases in order
5. Write status/findings to %s/.scratch/tasks/%s/
6. For parallel phases: spawn an agent team with one agent per role
7. For worktree phases: use isolation: worktree for each agent
8. After completion: update docs, rebuild with ohara build, create PR`,
			playbookName, context, taskID,
			hubDirName, playbookName,
			hubDirName, taskID,
			hubDirName,
			hubDirName, taskID)

		fmt.Printf("Open Claude Code in your workspace and paste this prompt:\n")
		fmt.Printf("┌─────────────────────────────────────────────────────────────────────┐\n")
		for _, line := range strings.Split(prompt, "\n") {
			fmt.Printf("│  %s\n", line)
		}
		fmt.Printf("└─────────────────────────────────────────────────────────────────────┘\n")

		fmt.Printf("\nOr run manually:\n")
		fmt.Printf("  1. Read: %s/.ohara-playbooks/%s.md\n", hubDirName, playbookName)
		fmt.Printf("  2. Scratch: %s/.scratch/tasks/%s/\n", hubDirName, taskID)
		fmt.Printf("  3. Clean up after: ohara clean %s\n", taskID)

		return nil
	},
}

var cleanCmd = &cobra.Command{
	Use:   "clean [task-id]",
	Short: "Clean up scratch space after a task is complete",
	Long: `Removes temporary files from .scratch/tasks/ after a playbook completes.
Without arguments, lists active tasks. With a task ID, removes that task's scratch space.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		hubRoot, err := FindHubRoot(".")
		if err != nil {
			return err
		}

		scratchDir := filepath.Join(hubRoot, ".scratch", "tasks")

		if len(args) == 0 {
			// List active tasks
			entries, err := os.ReadDir(scratchDir)
			if err != nil || len(entries) == 0 {
				fmt.Println("No active tasks.")
				return nil
			}

			fmt.Println("Active tasks:")
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				// Read context.md for details
				contextPath := filepath.Join(scratchDir, entry.Name(), "context.md")
				if data, err := os.ReadFile(contextPath); err == nil {
					// Extract first description line
					lines := strings.Split(string(data), "\n")
					for _, line := range lines {
						if strings.HasPrefix(line, "**Playbook:**") {
							fmt.Printf("  %s  %s\n", entry.Name(), line)
							break
						}
					}
				} else {
					fmt.Printf("  %s\n", entry.Name())
				}
			}
			return nil
		}

		// Clean specific task
		taskID := args[0]
		taskDir := filepath.Join(scratchDir, taskID)

		if _, err := os.Stat(taskDir); os.IsNotExist(err) {
			return fmt.Errorf("task '%s' not found", taskID)
		}

		if err := os.RemoveAll(taskDir); err != nil {
			return fmt.Errorf("failed to clean task: %w", err)
		}

		// Also clean handoffs
		handoffsDir := filepath.Join(hubRoot, ".scratch", "handoffs")
		os.RemoveAll(handoffsDir)
		os.MkdirAll(handoffsDir, 0755)

		fmt.Printf("✓ Cleaned task: %s\n", taskID)
		return nil
	},
}

func generateTaskID(playbookName string) string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%s-%x", playbookName, b)
}

func listPlaybooks(hubRoot string) []string {
	playbooksDir := filepath.Join(hubRoot, ".ohara-playbooks")
	entries, err := os.ReadDir(playbooksDir)
	if err != nil {
		return nil
	}

	var names []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".md") {
			names = append(names, strings.TrimSuffix(entry.Name(), ".md"))
		}
	}
	return names
}

func init() {
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(cleanCmd)
}
