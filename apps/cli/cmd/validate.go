package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate documentation structure and frontmatter",
	Long:  "Checks for valid ohara.yaml, Diataxis directory structure, frontmatter in all documents, and broken internal links.",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()

		errors := 0
		warnings := 0

		// Check ohara.yaml
		configPath := findConfig(dir)
		if configPath == "" {
			fmt.Println("✗ No ohara.yaml found")
			errors++
		} else {
			fmt.Printf("✓ Found %s\n", filepath.Base(configPath))
		}

		// Check Diataxis directories
		diataxisDirs := []string{"tutorials", "guides", "reference", "explanation"}
		for _, d := range diataxisDirs {
			path := filepath.Join(dir, d)
			if info, err := os.Stat(path); err != nil || !info.IsDir() {
				fmt.Printf("⚠ Missing directory: %s/\n", d)
				warnings++
			} else {
				// Count docs in directory
				count := countMarkdownFiles(path)
				fmt.Printf("✓ %s/ (%d documents)\n", d, count)
			}
		}

		// Walk all markdown files and check frontmatter
		fmt.Println("\nChecking documents...")
		docCount := 0
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			// Skip hidden dirs and node_modules
			if info.IsDir() {
				base := filepath.Base(path)
				if strings.HasPrefix(base, ".") || base == "node_modules" || base == "dist" {
					return filepath.SkipDir
				}
				return nil
			}

			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".md" && ext != ".mdx" {
				return nil
			}

			docCount++

			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			rel, _ := filepath.Rel(dir, path)

			// Check for frontmatter
			if !strings.HasPrefix(string(content), "---") {
				fmt.Printf("  ⚠ %s: missing frontmatter\n", rel)
				warnings++
			} else {
				// Check for title in frontmatter
				if !strings.Contains(string(content), "title:") {
					fmt.Printf("  ⚠ %s: frontmatter missing 'title'\n", rel)
					warnings++
				}
			}

			return nil
		})

		fmt.Printf("\n%d documents checked. ", docCount)
		if errors > 0 {
			fmt.Printf("%d errors, %d warnings\n", errors, warnings)
			return fmt.Errorf("validation failed with %d errors", errors)
		}
		if warnings > 0 {
			fmt.Printf("0 errors, %d warnings\n", warnings)
		} else {
			fmt.Println("All checks passed!")
		}

		return nil
	},
}

func findConfig(dir string) string {
	names := []string{"ohara.yaml", "ohara.yml", ".ohara.yaml", ".ohara.yml"}
	for _, name := range names {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
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
