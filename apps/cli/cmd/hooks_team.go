package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// --------------------------------------------------------------------------
// ohara teammate-idle — TeammateIdle hook (detect parallel agent completion)
// --------------------------------------------------------------------------

var teammateIdleCmd = &cobra.Command{
	Use:    "teammate-idle",
	Short:  "Hook: detect when parallel agents finish their tasks",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		var input struct {
			TeammateName string `json:"teammate_name"`
			TeamName     string `json:"team_name"`
		}
		if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
			return nil
		}

		hubRoot, err := FindHubRoot(".")
		if err != nil {
			return nil
		}

		// Log to stderr for visibility
		fmt.Fprintf(os.Stderr, "Teammate idle: %s (team: %s)\n", input.TeammateName, input.TeamName)

		// Try to determine playbook task from team name (pattern: <task-id>-<phase>)
		taskID := input.TeamName
		if idx := strings.LastIndex(input.TeamName, "-"); idx > 0 {
			// Could be task-id-phase, try finding the task dir
			candidate := input.TeamName[:idx]
			if dirExists(filepath.Join(hubRoot, ".scratch", "tasks", candidate)) {
				taskID = candidate
			}
		}

		// Check scratch for completion status
		tasksDir := filepath.Join(hubRoot, ".scratch", "tasks", taskID)
		if !dirExists(tasksDir) {
			return nil
		}

		// Count result files (agents write <name>.md or similar)
		resultFiles := []string{}
		entries, _ := os.ReadDir(tasksDir)
		for _, e := range entries {
			name := e.Name()
			// Skip standard files, look for agent-produced results
			if name == "context.md" || name == "playbook.md" {
				continue
			}
			if strings.HasSuffix(name, ".md") {
				resultFiles = append(resultFiles, name)
			}
		}

		// Build status context
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Team %s: teammate %s is now idle.\n", input.TeamName, input.TeammateName))
		sb.WriteString(fmt.Sprintf("Task scratch: .scratch/tasks/%s/ (%d result files: %s)\n",
			taskID, len(resultFiles), strings.Join(resultFiles, ", ")))

		output := map[string]interface{}{
			"hookSpecificOutput": map[string]interface{}{
				"hookEventName":     "TeammateIdle",
				"additionalContext": sb.String(),
			},
		}

		data, _ := json.Marshal(output)
		fmt.Print(string(data))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(teammateIdleCmd)
}
