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

// --------------------------------------------------------------------------
// ohara hook prompt — UserPromptSubmit hook
// Searches the hub for docs relevant to the user's prompt and injects context
// --------------------------------------------------------------------------

var hookPromptCmd = &cobra.Command{
	Use:    "hook-prompt",
	Short:  "Hook: inject relevant hub docs before agent starts",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Read prompt from stdin
		var input struct {
			UserPrompt string `json:"user_prompt"`
		}
		if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
			return nil // Can't parse, pass through silently
		}

		prompt := input.UserPrompt
		if prompt == "" {
			return nil
		}

		hubRoot, err := FindHubRoot(".")
		if err != nil {
			return nil // Not in an ohara workspace
		}

		config, err := LoadHubConfig(hubRoot)
		if err != nil {
			return nil
		}

		hubDirName := filepath.Base(hubRoot)

		// Extract search keywords (simple: split, remove stop words, take top terms)
		keywords := extractKeywords(prompt)
		if len(keywords) == 0 {
			return nil
		}

		// Search hub docs for matching content
		var contextParts []string

		// 1. Search across all markdown files
		matches := searchHub(hubRoot, keywords)

		if len(matches) > 0 {
			contextParts = append(contextParts,
				fmt.Sprintf("Relevant documentation from %s/ hub (%d matches):\n", hubDirName, len(matches)))

			for _, match := range matches {
				contextParts = append(contextParts, match)
			}
		}

		// 2. Check changelogs for recent related changes
		changelogHits := searchChangelogs(hubRoot, config, keywords)
		if len(changelogHits) > 0 {
			contextParts = append(contextParts, "\nRecent related changes:\n")
			contextParts = append(contextParts, changelogHits...)
		}

		if len(contextParts) == 0 {
			return nil // Nothing found, don't inject
		}

		// Add a footer with navigation hints
		contextParts = append(contextParts,
			fmt.Sprintf("\n---\nHub: %s/ | Commands: /fix, /feature, /investigate, /review-pr | Search: ohara serve (MCP)", hubDirName))

		// Output Claude Code UserPromptSubmit response
		output := map[string]interface{}{
			"hookSpecificOutput": map[string]interface{}{
				"hookEventName":     "UserPromptSubmit",
				"additionalContext": strings.Join(contextParts, "\n"),
			},
		}

		data, _ := json.Marshal(output)
		fmt.Print(string(data))
		return nil
	},
}

// extractKeywords pulls meaningful search terms from a prompt
func extractKeywords(prompt string) []string {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "shall": true, "can": true,
		"i": true, "me": true, "my": true, "we": true, "our": true, "you": true,
		"your": true, "it": true, "its": true, "they": true, "their": true,
		"this": true, "that": true, "these": true, "those": true,
		"in": true, "on": true, "at": true, "to": true, "for": true,
		"of": true, "with": true, "from": true, "by": true, "about": true,
		"into": true, "through": true, "during": true, "before": true, "after": true,
		"and": true, "or": true, "but": true, "not": true, "no": true,
		"if": true, "then": true, "so": true, "because": true, "when": true,
		"what": true, "how": true, "why": true, "where": true, "who": true,
		"fix": true, "add": true, "make": true, "get": true, "set": true,
		"use": true, "need": true, "want": true, "help": true, "please": true,
	}

	// Split on non-alphanumeric, lowercase, filter
	words := strings.FieldsFunc(strings.ToLower(prompt), func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_')
	})

	seen := map[string]bool{}
	var keywords []string
	for _, w := range words {
		if len(w) < 3 || stopWords[w] || seen[w] {
			continue
		}
		seen[w] = true
		keywords = append(keywords, w)
		if len(keywords) >= 8 { // Cap at 8 keywords
			break
		}
	}
	return keywords
}

// searchHub greps markdown files in the hub for keyword matches
func searchHub(hubRoot string, keywords []string) []string {
	var results []string
	seen := map[string]bool{}

	// Build grep pattern: word1|word2|word3
	pattern := strings.Join(keywords, "|")

	// Use grep to find matching files
	grepCmd := exec.Command("grep", "-rli", "-E", pattern, "--include=*.md")
	grepCmd.Dir = hubRoot
	output, err := grepCmd.Output()
	if err != nil || len(output) == 0 {
		return nil
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")

	// Score files by number of keyword matches (more matches = more relevant)
	type scoredFile struct {
		path  string
		score int
	}
	var scored []scoredFile

	for _, f := range files {
		// Skip prompts, scratch, cache
		if strings.Contains(f, ".ohara-prompts") ||
			strings.Contains(f, ".scratch") ||
			strings.Contains(f, ".cache") ||
			strings.Contains(f, "CHANGELOG") { // Changelogs searched separately
			continue
		}

		fullPath := filepath.Join(hubRoot, f)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		contentLower := strings.ToLower(string(content))
		score := 0
		for _, kw := range keywords {
			score += strings.Count(contentLower, kw)
		}

		if score > 0 {
			scored = append(scored, scoredFile{path: fullPath, score: score})
		}
	}

	// Sort by score (simple bubble sort, few items)
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Take top 5 results
	limit := 5
	if len(scored) < limit {
		limit = len(scored)
	}

	for _, sf := range scored[:limit] {
		rel, _ := filepath.Rel(hubRoot, sf.path)
		if seen[rel] {
			continue
		}
		seen[rel] = true

		title := extractTitle(sf.path)
		content, _ := os.ReadFile(sf.path)

		// Extract the section: frontmatter-stripped, first 500 chars
		text := string(content)
		if strings.HasPrefix(text, "---") {
			if idx := strings.Index(text[3:], "---"); idx >= 0 {
				text = strings.TrimSpace(text[idx+6:])
			}
		}

		// Find the Diataxis type from path
		dType := inferTypeFromPath(rel)

		// Trim to ~500 chars at a paragraph boundary
		if len(text) > 500 {
			cutoff := strings.Index(text[300:], "\n\n")
			if cutoff > 0 {
				text = text[:300+cutoff]
			} else {
				text = text[:500]
			}
			text += "\n..."
		}

		results = append(results, fmt.Sprintf("### %s (%s)\n_Source: %s_\n\n%s\n", title, dType, rel, text))
	}

	return results
}

func inferTypeFromPath(path string) string {
	if strings.Contains(path, "/tutorials/") {
		return "Tutorial"
	}
	if strings.Contains(path, "/guides/") {
		return "Guide"
	}
	if strings.Contains(path, "/reference/") {
		return "Reference"
	}
	if strings.Contains(path, "/explanation/") {
		return "Explanation"
	}
	return "Doc"
}

// searchChangelogs looks for keyword matches in CHANGELOG.md files
func searchChangelogs(hubRoot string, config *HubConfig, keywords []string) []string {
	var results []string
	pattern := strings.Join(keywords, "|")

	for _, repo := range config.Repos {
		changelogPath := filepath.Join(hubRoot, repo.Name, "CHANGELOG.md")
		data, err := os.ReadFile(changelogPath)
		if err != nil {
			continue
		}

		lines := strings.Split(string(data), "\n")
		var matchedLines []string
		for _, line := range lines {
			if strings.HasPrefix(line, "- ") {
				lineLower := strings.ToLower(line)
				for _, kw := range keywords {
					if strings.Contains(lineLower, kw) {
						matchedLines = append(matchedLines, line)
						break
					}
				}
			}
		}

		if len(matchedLines) > 0 {
			// Cap at 5 changelog entries per service
			if len(matchedLines) > 5 {
				matchedLines = matchedLines[:5]
			}
			results = append(results, fmt.Sprintf("**%s:**\n%s\n",
				repo.Name, strings.Join(matchedLines, "\n")))
		}
	}

	// Also try grep if no line matches (search commit messages)
	if len(results) == 0 {
		grepCmd := exec.Command("grep", "-ri", "-E", pattern, "--include=CHANGELOG.md", "-l")
		grepCmd.Dir = hubRoot
		if output, err := grepCmd.Output(); err == nil && len(output) > 0 {
			_ = output // Files exist but no line-level matches
		}
	}

	return results
}

// --------------------------------------------------------------------------
// ohara gate <file> — PreToolUse hook on Edit/Write
// --------------------------------------------------------------------------

var gateCmd = &cobra.Command{
	Use:    "gate",
	Short:  "Hook: return doc context before editing a file",
	Hidden: true,
	Args:   cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil
		}
		filePath := args[0]

		hubRoot, err := FindHubRoot(".")
		if err != nil {
			return nil
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
			return nil
		}

		// Check if we already gated this service this session
		sessionFile := filepath.Join(os.TempDir(), fmt.Sprintf(".ohara-gate-%d-%s", os.Getppid(), matchedRepo.Name))
		if _, err := os.Stat(sessionFile); err == nil {
			return nil
		}
		os.WriteFile(sessionFile, []byte("1"), 0644)

		hubDirName := filepath.Base(hubRoot)
		fmt.Fprintf(os.Stderr, "📚 Editing %s — docs at %s/%s/\n", matchedRepo.Name, hubDirName, matchedRepo.Name)

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
			fmt.Fprintf(os.Stderr, "⚠ No completed docs for this service. Consider: ohara generate %s\n", matchedRepo.Name)
		}

		return nil
	},
}

// --------------------------------------------------------------------------
// ohara watch-hook — PostToolUse hook on Bash
// --------------------------------------------------------------------------

var watchHookCmd = &cobra.Command{
	Use:    "watch-hook",
	Short:  "Hook: detect git events and check staleness",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		var input struct {
			ToolInput struct {
				Command string `json:"command"`
			} `json:"tool_input"`
		}

		if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
			return nil
		}

		bashCmd := input.ToolInput.Command

		// PR creation — remind about docs
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

		// Git events — staleness check
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

		config, _ := LoadHubConfig(hubRoot)
		if config == nil {
			return nil
		}

		hubDirName := filepath.Base(hubRoot)
		staleServices := []string{}

		for _, repo := range config.Repos {
			docsDir := filepath.Join(hubRoot, repo.Name)
			if !dirExists(docsDir) {
				staleServices = append(staleServices, repo.Name+" (no docs)")
			}
		}

		if len(staleServices) > 0 {
			fmt.Fprintf(os.Stderr, "📚 Git event detected. Services without docs:\n")
			for _, s := range staleServices {
				fmt.Fprintf(os.Stderr, "  - %s\n", s)
			}
			fmt.Fprintf(os.Stderr, "Run: cd %s && ohara validate\n", hubDirName)
		}

		return nil
	},
}

// --------------------------------------------------------------------------
// ohara session-summary — Stop hook (enforcement)
// --------------------------------------------------------------------------

var sessionSummaryCmd = &cobra.Command{
	Use:    "session-summary",
	Short:  "Hook: enforce doc updates before session ends",
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

		// Check which services were edited this session
		editedServices := []string{}
		for _, repo := range config.Repos {
			sessionFile := filepath.Join(os.TempDir(), fmt.Sprintf(".ohara-gate-%d-%s", os.Getppid(), repo.Name))
			if _, err := os.Stat(sessionFile); err == nil {
				editedServices = append(editedServices, repo.Name)
				os.Remove(sessionFile)
			}
		}

		if len(editedServices) == 0 {
			return nil // No code was edited, nothing to enforce
		}

		// Check if hub docs were also modified (git diff)
		hubDirName := filepath.Base(hubRoot)
		gitDiff := exec.Command("git", "diff", "HEAD", "--numstat")
		gitDiff.Dir = hubRoot
		diffOutput, _ := gitDiff.Output()

		docLinesChanged := 0
		if len(diffOutput) > 0 {
			for _, line := range strings.Split(string(diffOutput), "\n") {
				parts := strings.Split(line, "\t")
				if len(parts) >= 3 {
					added := 0
					fmt.Sscanf(parts[0], "%d", &added)
					removed := 0
					fmt.Sscanf(parts[1], "%d", &removed)
					docLinesChanged += added + removed
				}
			}
		}

		if docLinesChanged > 0 {
			// Docs were updated, just remind to commit
			fmt.Fprintf(os.Stderr, "📚 You edited: %s. Hub docs were updated (%d lines). Don't forget to commit %s/.\n",
				strings.Join(editedServices, ", "), docLinesChanged, hubDirName)
			return nil
		}

		// Code was edited but docs weren't — enforce
		fmt.Fprintf(os.Stderr, "📚 You edited code in: %s but hub docs were not updated.\n",
			strings.Join(editedServices, ", "))
		fmt.Fprintf(os.Stderr, "Update docs: use ohara-writer or manually edit %s/<service>/\n", hubDirName)
		fmt.Fprintf(os.Stderr, "Then: ohara build && ohara validate\n")

		return nil
	},
}

// --------------------------------------------------------------------------
// Parent command: ohara hook
// --------------------------------------------------------------------------

func init() {
	rootCmd.AddCommand(hookPromptCmd)
	rootCmd.AddCommand(gateCmd)
	rootCmd.AddCommand(watchHookCmd)
	rootCmd.AddCommand(sessionSummaryCmd)
}
