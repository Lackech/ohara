package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
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

var generateCmd = &cobra.Command{
	Use:   "generate [directory]",
	Short: "Analyze a codebase and generate documentation scaffolding",
	Long: `Analyzes the current directory (or specified path) and generates a Diataxis-structured
documentation plan. Creates ohara.yaml and starter documents based on detected code patterns.

The command:
1. Scans the codebase for signals (routes, configs, types, CI, etc.)
2. Creates a generation plan with document outlines
3. Writes ohara.yaml and directory structure
4. Optionally generates doc content via LLM (with --execute)`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()
		if len(args) > 0 {
			dir = args[0]
		}

		absDir, err := filepath.Abs(dir)
		if err != nil {
			return fmt.Errorf("invalid directory: %w", err)
		}

		typesFlag, _ := cmd.Flags().GetStringSlice("types")
		minConf, _ := cmd.Flags().GetFloat64("min-confidence")
		execute, _ := cmd.Flags().GetBool("execute")
		outputDir, _ := cmd.Flags().GetString("output")

		if outputDir == "" {
			outputDir = filepath.Join(absDir, "docs")
		}

		fmt.Printf("Analyzing %s...\n\n", absDir)

		// Try API first, fall back to local analysis
		token, _ := loadToken()
		var result *AnalyzeResponse

		if token != "" {
			apiURL, _ := cmd.Root().Flags().GetString("api-url")
			result, err = analyzeViaAPI(apiURL, token, absDir, typesFlag, minConf)
			if err != nil {
				fmt.Fprintf(os.Stderr, "API analysis failed, falling back to local: %v\n", err)
			}
		}

		if result == nil {
			// Local analysis (no API needed)
			result, err = analyzeLocally(absDir, typesFlag, minConf)
			if err != nil {
				return fmt.Errorf("analysis failed: %w", err)
			}
		}

		// Display results
		analysis := result.Data.Analysis
		plan := result.Data.Plan

		fmt.Printf("Project: %s (%s", analysis.ProjectName, analysis.Language)
		if analysis.Framework != nil {
			fmt.Printf("/%s", *analysis.Framework)
		}
		fmt.Printf(")\n")
		fmt.Printf("Files:   %d\n", analysis.FileCount)
		fmt.Printf("\n")

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

		fmt.Printf("Plan: %d documents to generate\n", plan.TotalDocs)
		fmt.Printf("Estimated LLM tokens: ~%d\n\n", plan.EstimatedTokens)

		if !execute {
			// Just create the structure and save prompts
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return fmt.Errorf("failed to create output dir: %w", err)
			}

			// Write ohara.yaml
			oharaPath := filepath.Join(outputDir, "ohara.yaml")
			if err := os.WriteFile(oharaPath, []byte(plan.OharaYaml), 0644); err != nil {
				return fmt.Errorf("failed to write ohara.yaml: %w", err)
			}
			fmt.Printf("✓ Created %s\n", oharaPath)

			// Create directories and write plan files
			for _, doc := range plan.Docs {
				docDir := filepath.Join(outputDir, filepath.Dir(doc.Path))
				if err := os.MkdirAll(docDir, 0755); err != nil {
					continue
				}

				// Write a placeholder with the prompt as a comment
				content := fmt.Sprintf("---\ntitle: %s\ndescription: \"\"\ndiataxis_type: %s\n---\n\n# %s\n\n",
					doc.Title, doc.DiataxisType, doc.Title)
				for _, heading := range doc.Outline {
					content += fmt.Sprintf("## %s\n\nTODO\n\n", heading)
				}

				docPath := filepath.Join(outputDir, doc.Path)
				if err := os.WriteFile(docPath, []byte(content), 0644); err != nil {
					continue
				}
				fmt.Printf("✓ Created %s\n", docPath)
			}

			// Save prompts for later LLM execution
			promptsDir := filepath.Join(outputDir, ".ohara-prompts")
			os.MkdirAll(promptsDir, 0755)
			for _, doc := range plan.Docs {
				promptPath := filepath.Join(promptsDir, strings.ReplaceAll(doc.Path, "/", "_")+".prompt.md")
				os.WriteFile(promptPath, []byte(doc.Prompt), 0644)
			}
			fmt.Printf("✓ Saved LLM prompts to %s/\n", promptsDir)

			fmt.Printf("\nNext steps:\n")
			fmt.Printf("  1. Review and edit the generated files\n")
			fmt.Printf("  2. Run 'ohara generate --execute' to fill content via LLM\n")
			fmt.Printf("  3. Or use the prompts in .ohara-prompts/ with your preferred AI tool\n")
			fmt.Printf("  4. Run 'ohara validate' to check the structure\n")
		} else {
			fmt.Println("--execute flag: LLM generation not yet implemented in CLI.")
			fmt.Println("Use the prompts in .ohara-prompts/ with Claude Code or another AI tool.")
		}

		return nil
	},
}

func analyzeViaAPI(apiURL, token, dir string, types []string, minConf float64) (*AnalyzeResponse, error) {
	body := map[string]interface{}{
		"directory":     dir,
		"types":         types,
		"minConfidence": minConf,
	}
	data, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", apiURL+"/api/v1/scaffold/analyze-local", strings.NewReader(string(data)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned %d", resp.StatusCode)
	}

	var result AnalyzeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func analyzeLocally(dir string, types []string, minConf float64) (*AnalyzeResponse, error) {
	// For local analysis without the API, we do a simplified version
	// that walks the directory and detects basic signals
	result := &AnalyzeResponse{}
	result.Data.Analysis.ProjectName = filepath.Base(dir)
	result.Data.Analysis.FileCount = 0

	// Count files
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
		// Read package.json for framework detection
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

	// Basic signals
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

	signals = append(signals, Signal{Type: "explanation", DocName: "architecture", Title: "Architecture Overview", Confidence: 0.7, Reason: "Complex project structure"})

	// Filter by types and confidence
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
	result.Data.Analysis.Summary = fmt.Sprintf("Project \"%s\" (%s) with %d files", result.Data.Analysis.ProjectName, result.Data.Analysis.Language, result.Data.Analysis.FileCount)

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
	result.Data.Plan.OharaYaml = fmt.Sprintf("name: %s\ndocs_dir: \".\"\n\ndirectories:\n  tutorials: tutorials\n  guides: guides\n  reference: reference\n  explanation: explanation\n\ninclude:\n  - \"**/*.md\"\n  - \"**/*.mdx\"\n", result.Data.Analysis.ProjectName)

	return result, nil
}

func getDocOutline(name string) []string {
	switch name {
	case "getting-started":
		return []string{"Prerequisites", "Installation", "Quick Start", "Next Steps"}
	case "development":
		return []string{"Prerequisites", "Setup", "Available Scripts", "Development Workflow"}
	case "deployment":
		return []string{"Prerequisites", "Build", "Deploy", "Environment Variables"}
	case "configuration":
		return []string{"Overview", "Environment Variables", "Configuration Files"}
	case "architecture":
		return []string{"Overview", "Key Components", "Data Flow", "Design Decisions"}
	default:
		return []string{"Overview", "Details", "Examples"}
	}
}

func init() {
	generateCmd.Flags().StringSlice("types", nil, "Filter by Diataxis types (tutorial,guide,reference,explanation)")
	generateCmd.Flags().Float64("min-confidence", 0.6, "Minimum confidence threshold")
	generateCmd.Flags().Bool("execute", false, "Execute LLM generation (requires API key)")
	generateCmd.Flags().StringP("output", "o", "", "Output directory (default: ./docs)")
	rootCmd.AddCommand(generateCmd)
}
