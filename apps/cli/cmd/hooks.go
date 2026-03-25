package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// ohara gate <file> — called by PreToolUse hook on Edit/Write
// Returns doc context for the file being edited, or empty if already checked
var gateCmd = &cobra.Command{
	Use:    "gate <file-path>",
	Short:  "Hook: return doc context before editing a file",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		// Find which service this file belongs to
		hubRoot, err := FindHubRoot(".")
		if err != nil {
			return nil // Not in ohara workspace, pass through silently
		}

		config, err := LoadHubConfig(hubRoot)
		if err != nil {
			return nil
		}

		// Match file path to a tracked service
		absPath, _ := filepath.Abs(filePath)
		var matchedRepo *RepoEntry
		for i, repo := range config.Repos {
			codePath := ResolveRepoPath(hubRoot, repo)
			if codePath != "" && strings.HasPrefix(absPath, codePath) {
				matchedRepo = &config.Repos[i]
				break
			}
		}

		if matchedRepo == nil {
			return nil // File not in a tracked service
		}

		// Check if we already gated this service this session
		sessionFile := filepath.Join(os.TempDir(), fmt.Sprintf(".ohara-gate-%d-%s", os.Getppid(), matchedRepo.Name))
		if _, err := os.Stat(sessionFile); err == nil {
			return nil // Already checked this session
		}
		os.WriteFile(sessionFile, []byte("1"), 0644)

		// Output context for this service
		hubDirName := filepath.Base(hubRoot)
		fmt.Fprintf(os.Stderr, "📚 Editing %s — docs at %s/%s/\n", matchedRepo.Name, hubDirName, matchedRepo.Name)

		// Check for key docs
		docsDir := filepath.Join(hubRoot, matchedRepo.Name)
		keyDocs := []string{}
		for _, ddir := range []string{"reference", "guides", "tutorials", "explanation"} {
			files := collectMarkdownFiles(filepath.Join(docsDir, ddir))
			for _, f := range files {
				rel, _ := filepath.Rel(hubRoot, f.path)
				title := extractTitle(f.path)
				if !isStubDoc(f.path) {
					keyDocs = append(keyDocs, fmt.Sprintf("  - %s (%s)", title, rel))
				}
			}
		}

		if len(keyDocs) > 0 {
			fmt.Fprintf(os.Stderr, "Key docs:\n%s\n", strings.Join(keyDocs, "\n"))
		} else {
			fmt.Fprintf(os.Stderr, "⚠ No completed docs for this service. Consider running: ohara generate %s\n", matchedRepo.Name)
		}

		return nil
	},
}

// ohara watch-hook <command> — called by PostToolUse hook on Bash
// Detects git pull/merge/checkout and checks for staleness
var watchHookCmd = &cobra.Command{
	Use:    "watch-hook",
	Short:  "Hook: detect git events and check staleness",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Read stdin for the tool input JSON
		var input struct {
			ToolInput struct {
				Command string `json:"command"`
			} `json:"tool_input"`
		}

		if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
			return nil // Can't parse, pass through
		}

		bashCmd := input.ToolInput.Command

		// Check if this is a PR creation — remind about docs
		isPR := strings.Contains(bashCmd, "gh pr create") || strings.Contains(bashCmd, "git push")
		if isPR {
			hubRoot, err := FindHubRoot(".")
			if err == nil {
				config, _ := LoadHubConfig(hubRoot)
				if config != nil {
					editedServices := []string{}
					for _, repo := range config.Repos {
						sessionFile := filepath.Join(os.TempDir(), fmt.Sprintf(".ohara-gate-%d-%s", os.Getppid(), repo.Name))
						if _, err := os.Stat(sessionFile); err == nil {
							editedServices = append(editedServices, repo.Name)
						}
					}
					if len(editedServices) > 0 {
						hubDirName := filepath.Base(hubRoot)
						fmt.Fprintf(os.Stderr, "📚 Creating PR — you edited: %s\n", strings.Join(editedServices, ", "))
						fmt.Fprintf(os.Stderr, "Consider: ohara build && ohara validate && /create-docs-pr\n")
						fmt.Fprintf(os.Stderr, "Hub: %s/\n", hubDirName)
					}
				}
			}
			return nil
		}

		// Check if this is a git event we care about
		isGitPull := strings.Contains(bashCmd, "git pull") || strings.Contains(bashCmd, "git fetch")
		isGitMerge := strings.Contains(bashCmd, "git merge")
		isGitCheckout := strings.Contains(bashCmd, "git checkout") && !strings.Contains(bashCmd, "-b")

		if !isGitPull && !isGitMerge && !isGitCheckout {
			return nil
		}

		hubRoot, err := FindHubRoot(".")
		if err != nil {
			return nil
		}

		config, err := LoadHubConfig(hubRoot)
		if err != nil {
			return nil
		}

		// Quick staleness check
		hubDirName := filepath.Base(hubRoot)
		staleServices := []string{}

		for _, repo := range config.Repos {
			codePath := ResolveRepoPath(hubRoot, repo)
			if codePath == "" || !dirExists(codePath) {
				continue
			}

			// Check if recent commits exist that docs might not cover
			gitLog := exec.Command("git", "log", "--oneline", "-3")
			gitLog.Dir = codePath
			output, err := gitLog.Output()
			if err != nil || len(output) == 0 {
				continue
			}

			// Check if docs were updated recently
			docsDir := filepath.Join(hubRoot, repo.Name)
			if !dirExists(docsDir) {
				staleServices = append(staleServices, repo.Name+" (no docs)")
				continue
			}
		}

		if len(staleServices) > 0 {
			fmt.Fprintf(os.Stderr, "📚 Git event detected. Services that may need doc updates:\n")
			for _, s := range staleServices {
				fmt.Fprintf(os.Stderr, "  - %s\n", s)
			}
			fmt.Fprintf(os.Stderr, "Run: cd %s && ohara validate\n", hubDirName)
		}

		return nil
	},
}

// ohara session-summary — called by Stop hook
// Checks if code was edited during the session and reminds about docs
var sessionSummaryCmd = &cobra.Command{
	Use:    "session-summary",
	Short:  "Hook: check if session needs doc follow-up",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		hubRoot, err := FindHubRoot(".")
		if err != nil {
			return nil
		}

		config, err := LoadHubConfig(hubRoot)
		if err != nil {
			return nil
		}

		// Check which services were gated (edited) this session
		editedServices := []string{}
		for _, repo := range config.Repos {
			sessionFile := filepath.Join(os.TempDir(), fmt.Sprintf(".ohara-gate-%d-%s", os.Getppid(), repo.Name))
			if _, err := os.Stat(sessionFile); err == nil {
				editedServices = append(editedServices, repo.Name)
				os.Remove(sessionFile) // Clean up
			}
		}

		if len(editedServices) > 0 {
			hubDirName := filepath.Base(hubRoot)
			fmt.Fprintf(os.Stderr, "📚 You edited code in: %s\n", strings.Join(editedServices, ", "))
			fmt.Fprintf(os.Stderr, "Consider updating docs: cd %s && ohara validate\n", hubDirName)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(gateCmd)
	rootCmd.AddCommand(watchHookCmd)
	rootCmd.AddCommand(sessionSummaryCmd)
}
