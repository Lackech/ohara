package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <repo-path>",
	Short: "Add a repository to the documentation hub",
	Long: `Adds a code repository to be tracked by Ohara. Creates the Diataxis directory
structure for the repo in the hub.

Examples:
  ohara add ../hzn-prices-service
  ohara add ../hzn-auth-service --name auth
  ohara add https://github.com/org/repo --clone`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoArg := args[0]
		nameFlag, _ := cmd.Flags().GetString("name")
		cloneFlag, _ := cmd.Flags().GetBool("clone")

		// Find the hub root
		hubRoot, err := FindHubRoot(".")
		if err != nil {
			return fmt.Errorf("not inside an Ohara hub. Run 'ohara init' first")
		}

		config, err := LoadHubConfig(hubRoot)
		if err != nil {
			return err
		}

		var repoName string
		var repoPath string
		var repoRemote string

		if strings.HasPrefix(repoArg, "https://") || strings.HasPrefix(repoArg, "git@") {
			// Remote URL — clone it or just reference it
			repoRemote = repoArg

			// Extract name from URL
			parts := strings.Split(strings.TrimSuffix(repoArg, ".git"), "/")
			repoName = parts[len(parts)-1]

			if cloneFlag {
				// Clone next to the hub
				cloneDir := filepath.Join(filepath.Dir(hubRoot), repoName)
				fmt.Printf("Cloning %s to %s...\n", repoArg, cloneDir)

				gitClone := exec.Command("git", "clone", repoArg, cloneDir)
				gitClone.Stdout = os.Stdout
				gitClone.Stderr = os.Stderr
				if err := gitClone.Run(); err != nil {
					return fmt.Errorf("failed to clone: %w", err)
				}

				relPath, _ := filepath.Rel(hubRoot, cloneDir)
				repoPath = relPath
			} else {
				// Just reference the remote, no local path
				repoPath = ""
			}
		} else {
			// Local path
			absPath, err := filepath.Abs(repoArg)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}

			// Verify the path exists
			if info, err := os.Stat(absPath); err != nil || !info.IsDir() {
				return fmt.Errorf("directory not found: %s", absPath)
			}

			repoName = filepath.Base(absPath)
			relPath, _ := filepath.Rel(hubRoot, absPath)
			repoPath = relPath

			// Try to detect git remote
			gitRemote := exec.Command("git", "remote", "get-url", "origin")
			gitRemote.Dir = absPath
			if output, err := gitRemote.Output(); err == nil {
				repoRemote = strings.TrimSpace(string(output))
			}
		}

		// Override name if provided
		if nameFlag != "" {
			repoName = nameFlag
		}

		// Check if already tracked
		for _, r := range config.Repos {
			if r.Name == repoName {
				return fmt.Errorf("repo '%s' is already tracked. Use a different --name", repoName)
			}
		}

		// Add to config
		config.Repos = append(config.Repos, RepoEntry{
			Name:   repoName,
			Path:   repoPath,
			Remote: repoRemote,
		})

		if err := SaveHubConfig(hubRoot, config); err != nil {
			return err
		}

		// Create Diataxis directory structure in the hub
		docsDir := filepath.Join(hubRoot, repoName)
		dirs := []string{"tutorials", "guides", "reference", "explanation"}

		for _, dir := range dirs {
			os.MkdirAll(filepath.Join(docsDir, dir), 0755)
		}

		// Create a README for the repo section
		repoReadme := fmt.Sprintf("# %s\n\nDocumentation for [%s](%s).\n", repoName, repoName, repoRemote)
		os.WriteFile(filepath.Join(docsDir, "README.md"), []byte(repoReadme), 0644)

		fmt.Printf("✓ Added %s\n", repoName)
		if repoPath != "" {
			fmt.Printf("  Code: %s\n", repoPath)
		}
		if repoRemote != "" {
			fmt.Printf("  Remote: %s\n", repoRemote)
		}
		fmt.Printf("  Docs: %s/\n", repoName)
		fmt.Printf("\nDiataxis directories created:\n")
		for _, dir := range dirs {
			fmt.Printf("  %s/%s/\n", repoName, dir)
		}
		fmt.Printf("\nNext: ohara generate %s\n", repoName)

		return nil
	},
}

func init() {
	addCmd.Flags().StringP("name", "n", "", "Override the repo name")
	addCmd.Flags().BoolP("clone", "c", false, "Clone a remote repo next to the hub")
	rootCmd.AddCommand(addCmd)
}
