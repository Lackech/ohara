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
	Short: "Clean up scratch space or prune agent memory",
	Long: `Removes temporary files from .scratch/tasks/ after a playbook completes.
Without arguments, lists active tasks. With a task ID, removes that task's scratch space.

Use --memory to manage agent memory:
  ohara clean --memory           List agent memory with sizes and ages
  ohara clean --memory --force   Delete memory entries older than 30 days`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		memoryFlag, _ := cmd.Flags().GetBool("memory")
		forceFlag, _ := cmd.Flags().GetBool("force")

		if memoryFlag {
			workDir, _ := os.Getwd()
			return pruneAgentMemory(workDir, forceFlag)
		}

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

// pruneAgentMemory scans .claude/agent-memory/ and reports/prunes stale entries
func pruneAgentMemory(workDir string, force bool) error {
	memoryDir := filepath.Join(workDir, ".claude", "agent-memory")

	if _, err := os.Stat(memoryDir); os.IsNotExist(err) {
		fmt.Println("No agent memory found (.claude/agent-memory/ does not exist).")
		return nil
	}

	staleThreshold := 30 * 24 * time.Hour // 30 days
	now := time.Now()
	totalSize := int64(0)
	staleCount := 0
	totalCount := 0

	fmt.Println("Agent memory:")
	fmt.Println()

	// Walk each agent's memory directory
	agentDirs, err := os.ReadDir(memoryDir)
	if err != nil {
		return fmt.Errorf("failed to read agent-memory: %w", err)
	}

	for _, agentDir := range agentDirs {
		if !agentDir.IsDir() {
			continue
		}

		agentPath := filepath.Join(memoryDir, agentDir.Name())
		agentSize := int64(0)
		agentFiles := 0
		agentStale := 0

		entries, _ := os.ReadDir(agentPath)
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			agentFiles++
			totalCount++
			agentSize += info.Size()
			totalSize += info.Size()

			age := now.Sub(info.ModTime())
			if age > staleThreshold {
				agentStale++
				staleCount++

				if force {
					os.Remove(filepath.Join(agentPath, entry.Name()))
					fmt.Printf("  deleted: %s/%s (age: %dd)\n", agentDir.Name(), entry.Name(), int(age.Hours()/24))
				}
			}
		}

		status := ""
		if agentStale > 0 && !force {
			status = fmt.Sprintf(" ⚠ %d stale (>30 days)", agentStale)
		}

		fmt.Printf("  %s: %d files, %s%s\n", agentDir.Name(), agentFiles, formatBytes(agentSize), status)
	}

	fmt.Println()
	if force && staleCount > 0 {
		fmt.Printf("✓ Pruned %d stale entries. Total remaining: %d files (%s)\n", staleCount, totalCount-staleCount, formatBytes(totalSize))
	} else if staleCount > 0 {
		fmt.Printf("Found %d stale entries (>30 days). Run with --force to prune.\n", staleCount)
		fmt.Printf("Total: %d files (%s)\n", totalCount, formatBytes(totalSize))
	} else {
		fmt.Printf("All %d entries are fresh. Total: %s\n", totalCount, formatBytes(totalSize))
	}

	if totalSize > 5*1024 {
		fmt.Printf("\n⚠ Agent memory is large (%s). Consider pruning to keep agents fast.\n", formatBytes(totalSize))
	}

	return nil
}

func formatBytes(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%dB", b)
	}
	return fmt.Sprintf("%.1fKB", float64(b)/1024)
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
	cleanCmd.Flags().Bool("memory", false, "List and prune stale agent memory entries")
	cleanCmd.Flags().Bool("force", false, "Actually delete stale entries (default: dry-run)")
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(cleanCmd)
}
