package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type Signal struct {
	Type       string   `json:"type"`
	DocName    string   `json:"docName"`
	Title      string   `json:"title"`
	Confidence float64  `json:"confidence"`
	Reason     string   `json:"reason"`
	Context    []string `json:"context"`
}

type DocPlan struct {
	Path         string   `json:"path"`
	Title        string   `json:"title"`
	DiataxisType string   `json:"diataxisType"`
	Confidence   float64  `json:"confidence"`
	Outline      []string `json:"outline"`
	Prompt       string   `json:"prompt"`
}

var generateCmd = &cobra.Command{
	Use:   "generate <repo-name>",
	Short: "Generate documentation for a tracked repository",
	Long: `Analyzes the code in a tracked repository and generates Diataxis-structured
documentation in the hub.

The command:
1. Reads code from the tracked repo's local path
2. Detects signals (routes, configs, types, CI, etc.)
3. Creates document outlines in the hub
4. Saves LLM prompts for content generation

Examples:
  ohara generate hzn-prices-service
  ohara generate hzn-auth-service --types reference,guide
  ohara generate hzn-prices-service --execute`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoName := args[0]
		typesFlag, _ := cmd.Flags().GetStringSlice("types")
		minConf, _ := cmd.Flags().GetFloat64("min-confidence")

		// Find hub
		hubRoot, err := FindHubRoot(".")
		if err != nil {
			return err
		}

		config, err := LoadHubConfig(hubRoot)
		if err != nil {
			return err
		}

		// Find the repo in config
		var repo *RepoEntry
		for i := range config.Repos {
			if config.Repos[i].Name == repoName {
				repo = &config.Repos[i]
				break
			}
		}

		if repo == nil {
			return fmt.Errorf("repo '%s' not found in hub. Run 'ohara add' first.\nTracked repos: %s",
				repoName, listRepoNames(config))
		}

		// Resolve code path
		codePath := ResolveRepoPath(hubRoot, *repo)
		if codePath == "" || !dirExists(codePath) {
			return fmt.Errorf("code directory not found: %s\nUpdate the path in .ohara.yaml or re-add with 'ohara add'", codePath)
		}

		fmt.Printf("Analyzing %s (%s)...\n\n", repoName, codePath)

		// Run analysis
		result, err := analyzeLocally(codePath, typesFlag, minConf)
		if err != nil {
			return fmt.Errorf("analysis failed: %w", err)
		}

		analysis := result.Data.Analysis
		plan := result.Data.Plan

		fmt.Printf("Project: %s (%s", analysis.ProjectName, analysis.Language)
		if analysis.Framework != nil {
			fmt.Printf("/%s", *analysis.Framework)
		}
		fmt.Printf(")\n")
		fmt.Printf("Files:   %d\n\n", analysis.FileCount)

		if len(analysis.Signals) == 0 {
			fmt.Println("No documentation opportunities detected.")
			return nil
		}

		fmt.Printf("Detected documentation opportunities:\n\n")
		for _, signal := range analysis.Signals {
			conf := "●●●"
			if signal.Confidence < 0.8 {
				conf = "●●○"
			}
			if signal.Confidence < 0.6 {
				conf = "●○○"
			}

			typeLabel := map[string]string{
				"tutorial":    "Tutorial",
				"guide":       "Guide",
				"reference":   "Reference",
				"explanation": "Explanation",
			}[signal.Type]

			fmt.Printf("  %s [%-11s] %s\n", conf, typeLabel, signal.Title)
			fmt.Printf("    %s\n\n", signal.Reason)
		}

		fmt.Printf("Plan: %d documents to generate\n\n", plan.TotalDocs)

		// Write docs into the hub (under repo name directory)
		outputDir := filepath.Join(hubRoot, repoName)

		for _, doc := range plan.Docs {
			docDir := filepath.Join(outputDir, filepath.Dir(doc.Path))
			os.MkdirAll(docDir, 0755)

			docPath := filepath.Join(outputDir, doc.Path)

			// Don't overwrite existing docs
			if _, err := os.Stat(docPath); err == nil {
				fmt.Printf("  ⊘ %s/%s (already exists, skipping)\n", repoName, doc.Path)
				continue
			}

			// Write scaffold with outline
			content := fmt.Sprintf("---\ntitle: %s\ndescription: \"\"\ndiataxis_type: %s\n---\n\n# %s\n\n",
				doc.Title, doc.DiataxisType, doc.Title)
			for _, heading := range doc.Outline {
				content += fmt.Sprintf("## %s\n\nTODO\n\n", heading)
			}

			if err := os.WriteFile(docPath, []byte(content), 0644); err != nil {
				fmt.Printf("  ✗ %s/%s: %v\n", repoName, doc.Path, err)
				continue
			}
			fmt.Printf("  ✓ %s/%s\n", repoName, doc.Path)
		}

		// Save LLM prompts
		promptsDir := filepath.Join(outputDir, ".ohara-prompts")
		os.MkdirAll(promptsDir, 0755)
		for _, doc := range plan.Docs {
			if doc.Prompt == "" {
				continue
			}
			promptPath := filepath.Join(promptsDir, strings.ReplaceAll(doc.Path, "/", "_")+".prompt.md")
			os.WriteFile(promptPath, []byte(doc.Prompt), 0644)
		}
		fmt.Printf("  ✓ Saved LLM prompts to %s/.ohara-prompts/\n", repoName)

		fmt.Printf("\nNext steps:\n")
		fmt.Printf("  1. Ask your AI agent to fill the docs:\n")
		fmt.Printf("     \"Read %s/.ohara-prompts/ and the code in %s,\n", repoName, codePath)
		fmt.Printf("      then write real documentation for each file in %s/\"\n", repoName)
		fmt.Printf("  2. Review and commit:\n")
		fmt.Printf("     git add -A && git commit -m \"docs: add %s documentation\"\n", repoName)
		fmt.Printf("  3. Validate: ohara validate\n")

		return nil
	},
}

func listRepoNames(config *HubConfig) string {
	names := make([]string, len(config.Repos))
	for i, r := range config.Repos {
		names[i] = r.Name
	}
	if len(names) == 0 {
		return "(none)"
	}
	return strings.Join(names, ", ")
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

type AnalyzeResponse struct {
	Data struct {
		Analysis struct {
			ProjectName string   `json:"projectName"`
			Language    string   `json:"language"`
			Framework   *string  `json:"framework"`
			FileCount   int      `json:"fileCount"`
			Summary     string   `json:"summary"`
			Signals     []Signal `json:"signals"`
		} `json:"analysis"`
		Plan struct {
			OharaYaml       string    `json:"oharaYaml"`
			Docs            []DocPlan `json:"docs"`
			Summary         string    `json:"summary"`
			TotalDocs       int       `json:"totalDocs"`
			EstimatedTokens int       `json:"estimatedTokens"`
		} `json:"plan"`
	} `json:"data"`
}

func analyzeLocally(dir string, types []string, minConf float64) (*AnalyzeResponse, error) {
	result := &AnalyzeResponse{}
	result.Data.Analysis.ProjectName = filepath.Base(dir)
	result.Data.Analysis.FileCount = 0

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			base := filepath.Base(path)
			if base == "node_modules" || base == ".git" || base == "dist" || strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		result.Data.Analysis.FileCount++
		return nil
	})

	// Detect language
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		result.Data.Analysis.Language = "TypeScript"
		data, _ := os.ReadFile(filepath.Join(dir, "package.json"))
		var pkg map[string]interface{}
		json.Unmarshal(data, &pkg)
		if deps, ok := pkg["dependencies"].(map[string]interface{}); ok {
			for name := range deps {
				switch name {
				case "next":
					fw := "Next.js"
					result.Data.Analysis.Framework = &fw
				case "elysia":
					fw := "Elysia"
					result.Data.Analysis.Framework = &fw
				case "express":
					fw := "Express"
					result.Data.Analysis.Framework = &fw
				case "hono":
					fw := "Hono"
					result.Data.Analysis.Framework = &fw
				case "nestjs", "@nestjs/core":
					fw := "NestJS"
					result.Data.Analysis.Framework = &fw
				}
			}
		}
	} else if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		result.Data.Analysis.Language = "Go"
	} else if _, err := os.Stat(filepath.Join(dir, "Cargo.toml")); err == nil {
		result.Data.Analysis.Language = "Rust"
	} else if _, err := os.Stat(filepath.Join(dir, "pyproject.toml")); err == nil {
		result.Data.Analysis.Language = "Python"
	} else {
		result.Data.Analysis.Language = "Unknown"
	}

	// Detect signals
	signals := []Signal{}

	if _, err := os.Stat(filepath.Join(dir, "README.md")); err == nil {
		signals = append(signals, Signal{Type: "tutorial", DocName: "getting-started", Title: "Getting Started", Confidence: 0.9, Reason: "README.md found"})
	} else {
		signals = append(signals, Signal{Type: "tutorial", DocName: "getting-started", Title: "Getting Started", Confidence: 0.7, Reason: "No README — tutorial should be generated"})
	}

	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		signals = append(signals, Signal{Type: "guide", DocName: "development", Title: "Development Guide", Confidence: 0.85, Reason: "package.json with scripts found"})
	}

	if _, err := os.Stat(filepath.Join(dir, "Dockerfile")); err == nil {
		signals = append(signals, Signal{Type: "guide", DocName: "deployment", Title: "Deployment Guide", Confidence: 0.8, Reason: "Dockerfile found"})
	}

	if _, err := os.Stat(filepath.Join(dir, ".env.example")); err == nil {
		signals = append(signals, Signal{Type: "reference", DocName: "configuration", Title: "Configuration Reference", Confidence: 0.9, Reason: ".env.example found"})
	}

	// Check for route files
	hasRoutes := false
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if strings.Contains(base, "route") || strings.Contains(base, "controller") || strings.Contains(base, "handler") {
			hasRoutes = true
			return filepath.SkipAll
		}
		return nil
	})
	if hasRoutes {
		signals = append(signals, Signal{Type: "reference", DocName: "api-reference", Title: "API Reference", Confidence: 0.85, Reason: "Route/controller files found"})
	}

	// Check for CI
	if _, err := os.Stat(filepath.Join(dir, ".github", "workflows")); err == nil {
		signals = append(signals, Signal{Type: "guide", DocName: "ci-cd", Title: "CI/CD Pipeline", Confidence: 0.75, Reason: "GitHub Actions workflows found"})
	}

	// Complex project → architecture
	if result.Data.Analysis.FileCount > 20 {
		signals = append(signals, Signal{Type: "explanation", DocName: "architecture", Title: "Architecture Overview", Confidence: 0.7, Reason: fmt.Sprintf("Complex project (%d files)", result.Data.Analysis.FileCount)})
	}

	// Check for test files
	hasTests := false
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.Contains(filepath.Base(path), ".test.") || strings.Contains(filepath.Base(path), ".spec.") || strings.Contains(filepath.Base(path), "_test.") {
			hasTests = true
			return filepath.SkipAll
		}
		return nil
	})
	if hasTests {
		signals = append(signals, Signal{Type: "guide", DocName: "testing", Title: "Testing Guide", Confidence: 0.65, Reason: "Test files found"})
	}

	// Filter
	filtered := []Signal{}
	for _, s := range signals {
		if minConf > 0 && s.Confidence < minConf {
			continue
		}
		if len(types) > 0 {
			found := false
			for _, t := range types {
				if t == s.Type {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		filtered = append(filtered, s)
	}

	result.Data.Analysis.Signals = filtered
	result.Data.Analysis.Summary = fmt.Sprintf("Project \"%s\" (%s) with %d files",
		result.Data.Analysis.ProjectName, result.Data.Analysis.Language, result.Data.Analysis.FileCount)

	// Build plan
	docs := []DocPlan{}
	dirMap := map[string]string{"tutorial": "tutorials", "guide": "guides", "reference": "reference", "explanation": "explanation"}
	for _, s := range filtered {
		outline := getDocOutline(s.DocName)
		docs = append(docs, DocPlan{
			Path:         dirMap[s.Type] + "/" + s.DocName + ".md",
			Title:        s.Title,
			DiataxisType: s.Type,
			Confidence:   s.Confidence,
			Outline:      outline,
		})
	}

	result.Data.Plan.Docs = docs
	result.Data.Plan.TotalDocs = len(docs)

	return result, nil
}

func getDocOutline(name string) []string {
	switch name {
	case "getting-started":
		return []string{"Prerequisites", "Installation", "Quick Start", "Project Structure", "Next Steps"}
	case "development":
		return []string{"Prerequisites", "Setup", "Available Scripts", "Development Workflow", "Common Tasks"}
	case "deployment":
		return []string{"Prerequisites", "Build", "Deploy", "Environment Variables", "Health Checks"}
	case "configuration":
		return []string{"Overview", "Environment Variables", "Configuration Files", "Defaults"}
	case "api-reference":
		return []string{"Base URL", "Authentication", "Endpoints", "Error Handling", "Examples"}
	case "ci-cd":
		return []string{"Pipeline Overview", "Stages", "Configuration", "Secrets"}
	case "architecture":
		return []string{"Overview", "Key Components", "Data Flow", "Design Decisions"}
	case "testing":
		return []string{"Test Structure", "Running Tests", "Writing Tests", "CI Integration"}
	default:
		return []string{"Overview", "Details", "Examples"}
	}
}

func init() {
	generateCmd.Flags().StringSlice("types", nil, "Filter by Diataxis types (tutorial,guide,reference,explanation)")
	generateCmd.Flags().Float64("min-confidence", 0.6, "Minimum confidence threshold")
	generateCmd.Flags().Bool("execute", false, "Generate content via LLM (not yet implemented)")
	rootCmd.AddCommand(generateCmd)
}
