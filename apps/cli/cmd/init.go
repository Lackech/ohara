package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Initialize a new Ohara documentation project",
	Long:  "Creates ohara.yaml and Diataxis directory structure (tutorials/, guides/, reference/, explanation/).",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()

		name := filepath.Base(dir)
		if len(args) > 0 {
			name = args[0]
		}

		// Create ohara.yaml
		config := fmt.Sprintf(`name: %s
description: ""
docs_dir: "."

directories:
  tutorials: tutorials
  guides: guides
  reference: reference
  explanation: explanation

include:
  - "**/*.md"
  - "**/*.mdx"

exclude:
  - node_modules/**
  - .git/**
  - dist/**
`, name)

		if err := os.WriteFile(filepath.Join(dir, "ohara.yaml"), []byte(config), 0644); err != nil {
			return fmt.Errorf("failed to create ohara.yaml: %w", err)
		}
		fmt.Println("✓ Created ohara.yaml")

		// Create Diataxis directories
		dirs := []string{"tutorials", "guides", "reference", "explanation"}
		for _, d := range dirs {
			path := filepath.Join(dir, d)
			if err := os.MkdirAll(path, 0755); err != nil {
				return fmt.Errorf("failed to create %s/: %w", d, err)
			}

			// Create a starter file
			starter := fmt.Sprintf("---\ntitle: Getting Started\ndiataxis_type: %s\n---\n\n# Getting Started\n\nAdd your %s content here.\n", getDiataxisType(d), d)
			starterPath := filepath.Join(path, "getting-started.md")
			if _, err := os.Stat(starterPath); os.IsNotExist(err) {
				os.WriteFile(starterPath, []byte(starter), 0644)
			}
		}
		fmt.Println("✓ Created Diataxis directories (tutorials/, guides/, reference/, explanation/)")

		fmt.Printf("\nProject \"%s\" initialized. Next steps:\n", name)
		fmt.Println("  1. Add documentation to the Diataxis directories")
		fmt.Println("  2. Run 'ohara validate' to check structure")
		fmt.Println("  3. Run 'ohara dev' to preview locally")

		return nil
	},
}

func getDiataxisType(dir string) string {
	switch dir {
	case "tutorials":
		return "tutorial"
	case "guides":
		return "guide"
	default:
		return dir
	}
}

func init() {
	rootCmd.AddCommand(initCmd)
}
