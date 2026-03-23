package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate [repo-name]",
	Short: "Validate documentation structure and coverage",
	Long: `Validates the documentation hub or a specific repo's docs.

Without arguments, validates the entire hub.
With a repo name, validates only that repo's documentation.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		hubRoot, err := FindHubRoot(".")
		if err != nil {
			return err
		}

		config, err := LoadHubConfig(hubRoot)
		if err != nil {
			return err
		}

		fmt.Printf("Hub: %s (%d repos tracked)\n\n", config.Name, len(config.Repos))

		if len(config.Repos) == 0 {
			fmt.Println("No repos tracked yet. Run 'ohara add ../your-repo' to get started.")
			return nil
		}

		totalErrors := 0
		totalWarnings := 0

		// Validate specific repo or all
		repos := config.Repos
		if len(args) > 0 {
			found := false
			for _, r := range config.Repos {
				if r.Name == args[0] {
					repos = []RepoEntry{r}
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("repo '%s' not found. Tracked: %s", args[0], listRepoNames(config))
			}
		}

		for _, repo := range repos {
			errors, warnings := validateRepo(hubRoot, repo)
			totalErrors += errors
			totalWarnings += warnings
		}

		fmt.Printf("\n")
		if totalErrors > 0 {
			fmt.Printf("Result: %d errors, %d warnings\n", totalErrors, totalWarnings)
			return fmt.Errorf("validation failed")
		}
		if totalWarnings > 0 {
			fmt.Printf("Result: 0 errors, %d warnings\n", totalWarnings)
		} else {
			fmt.Println("Result: All checks passed!")
		}

		return nil
	},
}

func validateRepo(hubRoot string, repo RepoEntry) (int, int) {
	errors := 0
	warnings := 0
	repoDocsDir := filepath.Join(hubRoot, repo.Name)

	fmt.Printf("── %s ", repo.Name)

	// Check docs directory exists
	if _, err := os.Stat(repoDocsDir); os.IsNotExist(err) {
		fmt.Printf("✗ no docs directory\n")
		return 1, 0
	}

	// Check Diataxis directories and count docs
	diataxisDirs := map[string]string{
		"tutorials":   "Tutorial",
		"guides":      "How-to Guide",
		"reference":   "Reference",
		"explanation": "Explanation",
	}

	typeCounts := map[string]int{}
	totalDocs := 0

	for dir, label := range diataxisDirs {
		dirPath := filepath.Join(repoDocsDir, dir)
		count := countMarkdownFiles(dirPath)
		typeCounts[label] = count
		totalDocs += count
	}

	if totalDocs == 0 {
		fmt.Printf("⚠ no documents (run 'ohara generate %s')\n", repo.Name)
		return 0, 1
	}

	fmt.Printf("(%d docs) ", totalDocs)

	// Check coverage
	missingTypes := []string{}
	for _, label := range []string{"Tutorial", "How-to Guide", "Reference", "Explanation"} {
		if typeCounts[label] == 0 {
			missingTypes = append(missingTypes, label)
		}
	}

	if len(missingTypes) > 0 {
		warnings += len(missingTypes)
		fmt.Printf("⚠ missing: %s\n", strings.Join(missingTypes, ", "))
	} else {
		fmt.Printf("✓ all types covered\n")
	}

	// Check individual docs for frontmatter
	filepath.Walk(repoDocsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".mdx" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		rel, _ := filepath.Rel(hubRoot, path)
		text := string(content)

		if !strings.HasPrefix(text, "---") {
			fmt.Printf("   ⚠ %s: missing frontmatter\n", rel)
			warnings++
		} else if !strings.Contains(text, "title:") {
			fmt.Printf("   ⚠ %s: frontmatter missing 'title'\n", rel)
			warnings++
		}

		// Check for TODO placeholders
		if strings.Count(text, "TODO") > 2 {
			fmt.Printf("   ⚠ %s: contains TODO placeholders (needs content generation)\n", rel)
			warnings++
		}

		return nil
	})

	// Check if code repo is accessible
	if repo.Path != "" {
		codePath := ResolveRepoPath(hubRoot, repo)
		if !dirExists(codePath) {
			fmt.Printf("   ⚠ code path not found: %s\n", codePath)
			warnings++
		}
	}

	return errors, warnings
}

func countMarkdownFiles(dir string) int {
	count := 0
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".md" || ext == ".mdx" {
			count++
		}
		return nil
	})
	return count
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
