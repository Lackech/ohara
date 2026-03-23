package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build llms.txt and agent artifacts from the hub",
	Long: `Generates llms.txt (and llms-full.txt) from the documentation hub.
These files allow any LLM to discover and read your documentation.

Also generates AGENTS.md with instructions for AI agents.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		hubRoot, err := FindHubRoot(".")
		if err != nil {
			return err
		}

		config, err := LoadHubConfig(hubRoot)
		if err != nil {
			return err
		}

		// Build llms.txt
		if err := buildLlmsTxt(hubRoot, config); err != nil {
			return fmt.Errorf("failed to build llms.txt: %w", err)
		}

		// Build llms-full.txt
		if err := buildLlmsFullTxt(hubRoot, config); err != nil {
			return fmt.Errorf("failed to build llms-full.txt: %w", err)
		}

		// Build AGENTS.md
		if err := buildAgentsMd(hubRoot, config); err != nil {
			return fmt.Errorf("failed to build AGENTS.md: %w", err)
		}

		return nil
	},
}

func buildLlmsTxt(hubRoot string, config *HubConfig) error {
	var lines []string

	lines = append(lines, fmt.Sprintf("# %s", config.Name))
	lines = append(lines, "")
	lines = append(lines, "> Documentation hub managed by Ohara. Diataxis-structured docs for all services.")
	lines = append(lines, "")

	diataxisLabels := map[string]string{
		"tutorials": "Tutorials", "guides": "How-to Guides",
		"reference": "Reference", "explanation": "Explanation",
	}

	for _, repo := range config.Repos {
		repoDir := filepath.Join(hubRoot, repo.Name)
		if !dirExists(repoDir) {
			continue
		}

		lines = append(lines, fmt.Sprintf("## %s", repo.Name))
		lines = append(lines, "")

		for _, dDir := range []string{"tutorials", "guides", "reference", "explanation"} {
			typeDir := filepath.Join(repoDir, dDir)
			docs := collectMarkdownFiles(typeDir)
			if len(docs) == 0 {
				continue
			}

			lines = append(lines, fmt.Sprintf("### %s", diataxisLabels[dDir]))
			lines = append(lines, "")
			for _, doc := range docs {
				title := extractTitle(doc.path)
				relPath, _ := filepath.Rel(hubRoot, doc.path)
				desc := extractDescription(doc.path)
				stub := ""
				if isStubDoc(doc.path) {
					stub = " *(incomplete — needs content)*"
				}
				if desc != "" {
					lines = append(lines, fmt.Sprintf("- [%s](%s): %s%s", title, relPath, desc, stub))
				} else {
					lines = append(lines, fmt.Sprintf("- [%s](%s)%s", title, relPath, stub))
				}
			}
			lines = append(lines, "")
		}
	}

	content := strings.Join(lines, "\n")
	outPath := filepath.Join(hubRoot, "llms.txt")
	if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
		return err
	}
	fmt.Printf("✓ Generated llms.txt (%d lines)\n", len(lines))
	return nil
}

func buildLlmsFullTxt(hubRoot string, config *HubConfig) error {
	var lines []string

	lines = append(lines, fmt.Sprintf("# %s", config.Name))
	lines = append(lines, "")
	lines = append(lines, "> Full documentation content. Managed by Ohara.")
	lines = append(lines, "")

	diataxisLabels := map[string]string{
		"tutorials": "Tutorials", "guides": "How-to Guides",
		"reference": "Reference", "explanation": "Explanation",
	}

	for _, repo := range config.Repos {
		repoDir := filepath.Join(hubRoot, repo.Name)
		if !dirExists(repoDir) {
			continue
		}

		lines = append(lines, fmt.Sprintf("## %s", repo.Name))
		lines = append(lines, "")

		skipped := 0
		for _, dDir := range []string{"tutorials", "guides", "reference", "explanation"} {
			typeDir := filepath.Join(repoDir, dDir)
			docs := collectMarkdownFiles(typeDir)
			if len(docs) == 0 {
				continue
			}

			var completeDocs []mdFile
			for _, doc := range docs {
				if isStubDoc(doc.path) {
					skipped++
					continue
				}
				completeDocs = append(completeDocs, doc)
			}

			if len(completeDocs) == 0 {
				continue
			}

			lines = append(lines, fmt.Sprintf("### %s", diataxisLabels[dDir]))
			lines = append(lines, "")

			for _, doc := range completeDocs {
				title := extractTitle(doc.path)
				lines = append(lines, fmt.Sprintf("#### %s", title))
				lines = append(lines, "")

				content, _ := os.ReadFile(doc.path)
				// Strip frontmatter
				text := string(content)
				if strings.HasPrefix(text, "---") {
					if idx := strings.Index(text[3:], "---"); idx >= 0 {
						text = strings.TrimSpace(text[idx+6:])
					}
				}
				lines = append(lines, text)
				lines = append(lines, "")
				lines = append(lines, "---")
				lines = append(lines, "")
			}
		}
		if skipped > 0 {
			fmt.Printf("  ⊘ Skipped %d incomplete docs (still have TODO placeholders)\n", skipped)
		}
	}

	content := strings.Join(lines, "\n")
	outPath := filepath.Join(hubRoot, "llms-full.txt")
	if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
		return err
	}
	fmt.Printf("✓ Generated llms-full.txt (%d bytes)\n", len(content))
	return nil
}

func buildAgentsMd(hubRoot string, config *HubConfig) error {
	var lines []string

	lines = append(lines, fmt.Sprintf("# %s — Agent Instructions", config.Name))
	lines = append(lines, "")
	lines = append(lines, "This documentation hub is managed by Ohara. It contains Diataxis-structured")
	lines = append(lines, "documentation for the following services:")
	lines = append(lines, "")
	for _, repo := range config.Repos {
		remote := ""
		if repo.Remote != "" {
			remote = fmt.Sprintf(" (%s)", repo.Remote)
		}
		lines = append(lines, fmt.Sprintf("- **%s**%s", repo.Name, remote))
	}
	lines = append(lines, "")
	lines = append(lines, "## Documentation Types (Diataxis)")
	lines = append(lines, "")
	lines = append(lines, "Each service has four types of documentation:")
	lines = append(lines, "- **tutorials/** — Learning-oriented walkthroughs. Use when onboarding to a service.")
	lines = append(lines, "- **guides/** — Task-oriented how-to guides. Use when executing a specific task.")
	lines = append(lines, "- **reference/** — Precise technical specs. Use when looking up APIs, configs, types.")
	lines = append(lines, "- **explanation/** — Conceptual explanations. Use when making design decisions.")
	lines = append(lines, "")
	lines = append(lines, "## How to Use These Docs")
	lines = append(lines, "")
	lines = append(lines, "1. **Search by task type**: Need to deploy? Check `guides/`. Need API params? Check `reference/`.")
	lines = append(lines, "2. **Read llms.txt**: `llms.txt` has an index of all docs. `llms-full.txt` has full content.")
	lines = append(lines, "3. **Cross-reference**: Docs reference each other. Follow the links.")
	lines = append(lines, "")
	lines = append(lines, "## Contributing Documentation")
	lines = append(lines, "")
	lines = append(lines, "When you learn something new about a service, add it to the docs:")
	lines = append(lines, "")
	lines = append(lines, "1. Create a new `.md` file in the appropriate Diataxis directory")
	lines = append(lines, "2. Add frontmatter with `title`, `description`, and `diataxis_type`")
	lines = append(lines, "3. Write specific, code-referenced content (not generic placeholders)")
	lines = append(lines, "4. Create a branch, commit, and open a PR:")
	lines = append(lines, "   ```bash")
	lines = append(lines, "   git checkout -b docs/update-auth-reference")
	lines = append(lines, "   git add -A")
	lines = append(lines, "   git commit -m \"docs: update auth service reference\"")
	lines = append(lines, "   git push origin docs/update-auth-reference")
	lines = append(lines, "   gh pr create --title \"docs: update auth reference\" --body \"Updated API endpoints and added new config options\"")
	lines = append(lines, "   ```")
	lines = append(lines, "")
	lines = append(lines, "## Changelog")
	lines = append(lines, "")
	lines = append(lines, "To see recent documentation changes:")
	lines = append(lines, "```bash")
	lines = append(lines, "git log --oneline -20")
	lines = append(lines, "```")
	lines = append(lines, "")
	lines = append(lines, "To see what changed in a specific service:")
	lines = append(lines, "```bash")
	lines = append(lines, "git log --oneline -- <service-name>/")
	lines = append(lines, "```")

	content := strings.Join(lines, "\n")
	outPath := filepath.Join(hubRoot, "AGENTS.md")
	if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
		return err
	}
	fmt.Printf("✓ Generated AGENTS.md\n")
	return nil
}

type mdFile struct {
	path string
}

func collectMarkdownFiles(dir string) []mdFile {
	var files []mdFile
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".md" || ext == ".mdx" {
			files = append(files, mdFile{path: path})
		}
		return nil
	})
	return files
}

func extractTitle(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return filepath.Base(path)
	}

	text := string(data)
	// Check frontmatter
	if strings.HasPrefix(text, "---") {
		if idx := strings.Index(text[3:], "---"); idx >= 0 {
			fm := text[3 : idx+3]
			for _, line := range strings.Split(fm, "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "title:") {
					title := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "title:"))
					title = strings.Trim(title, "\"'")
					if title != "" {
						return title
					}
				}
			}
		}
	}

	// Check first H1
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}

	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

func extractDescription(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	text := string(data)
	if strings.HasPrefix(text, "---") {
		if idx := strings.Index(text[3:], "---"); idx >= 0 {
			fm := text[3 : idx+3]
			for _, line := range strings.Split(fm, "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "description:") {
					desc := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "description:"))
					desc = strings.Trim(desc, "\"'")
					return desc
				}
			}
		}
	}
	return ""
}

// isStubDoc returns true if a doc is still a TODO scaffold (not yet filled by an agent/human)
func isStubDoc(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	text := string(data)
	// Strip frontmatter
	if strings.HasPrefix(text, "---") {
		if idx := strings.Index(text[3:], "---"); idx >= 0 {
			text = strings.TrimSpace(text[idx+6:])
		}
	}
	// A stub has many TODOs relative to content, or is very short
	todoCount := strings.Count(text, "TODO")
	wordCount := len(strings.Fields(text))
	return todoCount >= 2 || (wordCount < 30 && todoCount >= 1)
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
