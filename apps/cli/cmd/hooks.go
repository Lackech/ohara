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

		// Boost keywords based on Diataxis intent matching
		keywords = boostByIntent(prompt, keywords)

		// Load already-injected doc paths for dedup
		injected := loadInjectedPaths()

		// Search hub docs for matching content, excluding already-injected
		var contextParts []string
		matches := searchHub(hubRoot, keywords, injected)

		if len(matches) > 0 {
			contextParts = append(contextParts,
				fmt.Sprintf("Relevant documentation from %s/ hub (%d matches):\n", hubDirName, len(matches)))

			for _, match := range matches {
				contextParts = append(contextParts, match)
			}
		}

		// Check changelogs for recent related changes
		changelogHits := searchChangelogs(hubRoot, config, keywords)
		if len(changelogHits) > 0 {
			contextParts = append(contextParts, "\nRecent related changes:\n")
			contextParts = append(contextParts, changelogHits...)
		}

		if len(contextParts) == 0 {
			return nil // Nothing found, don't inject
		}

		// Output Claude Code UserPromptSubmit response (no footer — moved to CLAUDE.md)
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

// boostByIntent adds Diataxis-type-specific terms based on query intent
func boostByIntent(prompt string, keywords []string) []string {
	lower := strings.ToLower(prompt)

	// Detect intent and add boosting terms
	if strings.Contains(lower, "how do i") || strings.Contains(lower, "how to") || strings.Contains(lower, "steps to") {
		keywords = append(keywords, "guides")
	} else if strings.Contains(lower, "what is") || strings.Contains(lower, "what does") || strings.Contains(lower, "api") || strings.Contains(lower, "config") {
		keywords = append(keywords, "reference")
	} else if strings.Contains(lower, "why") || strings.Contains(lower, "architecture") || strings.Contains(lower, "design") {
		keywords = append(keywords, "explanation")
	} else if strings.Contains(lower, "getting started") || strings.Contains(lower, "tutorial") || strings.Contains(lower, "walkthrough") {
		keywords = append(keywords, "tutorials")
	}

	return keywords
}

// loadInjectedPaths reads the session-level dedup tracker
func loadInjectedPaths() map[string]bool {
	injected := map[string]bool{}
	trackerPath := filepath.Join(os.TempDir(), fmt.Sprintf(".ohara-injected-%d.json", os.Getppid()))
	data, err := os.ReadFile(trackerPath)
	if err != nil {
		return injected
	}
	json.Unmarshal(data, &injected)
	return injected
}

// saveInjectedPaths persists injected doc paths for dedup across prompts
func saveInjectedPaths(injected map[string]bool) {
	trackerPath := filepath.Join(os.TempDir(), fmt.Sprintf(".ohara-injected-%d.json", os.Getppid()))
	data, _ := json.Marshal(injected)
	os.WriteFile(trackerPath, data, 0644)
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

// searchHub greps markdown files in the hub for keyword matches, excluding already-injected paths
func searchHub(hubRoot string, keywords []string, exclude map[string]bool) []string {
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

		rel := f
		// Skip already-injected docs this session
		if exclude[rel] {
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

	// Track newly injected paths for dedup
	newInjected := loadInjectedPaths()

	for _, sf := range scored[:limit] {
		rel, _ := filepath.Rel(hubRoot, sf.path)
		if seen[rel] {
			continue
		}
		seen[rel] = true
		newInjected[rel] = true

		title := extractTitle(sf.path)
		content, _ := os.ReadFile(sf.path)

		// Extract the section: frontmatter-stripped, first 300 chars
		text := string(content)
		if strings.HasPrefix(text, "---") {
			if idx := strings.Index(text[3:], "---"); idx >= 0 {
				text = strings.TrimSpace(text[idx+6:])
			}
		}

		// Find the Diataxis type from path
		dType := inferTypeFromPath(rel)

		// Trim to ~300 chars at a paragraph boundary
		if len(text) > 300 {
			cutoff := strings.Index(text[200:], "\n\n")
			if cutoff > 0 {
				text = text[:200+cutoff]
			} else {
				text = text[:300]
			}
			text += "\n..."
		}

		results = append(results, fmt.Sprintf("### %s (%s)\n_Source: %s_\n\n%s\n", title, dType, rel, text))
	}

	// Save injected paths for future dedup
	saveInjectedPaths(newInjected)

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
// ohara session-start — SessionStart hook (one-time bootstrap + watchPaths)
// --------------------------------------------------------------------------

var sessionStartCmd = &cobra.Command{
	Use:    "session-start",
	Short:  "Hook: initialize session with hub context and watchPaths",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		hubRoot, err := FindHubRoot(".")
		if err != nil {
			return nil // Not in an ohara workspace
		}

		config, err := LoadHubConfig(hubRoot)
		if err != nil {
			return nil
		}

		hubDirName := filepath.Base(hubRoot)

		// Collect all .md file paths in hub for watchPaths
		watchPaths := collectAllMdPaths(hubRoot)

		// Count docs
		totalDocs := 0
		for _, p := range watchPaths {
			if !strings.Contains(p, ".scratch") && !strings.Contains(p, ".ohara-prompts") {
				totalDocs++
			}
		}

		// Build brief context (injected once at session start)
		var context strings.Builder
		context.WriteString(fmt.Sprintf("Ohara hub: %s/ | %d services | %d docs tracked\n",
			hubDirName, len(config.Repos), totalDocs))

		// Read llms.txt if it exists for a quick index
		llmsPath := filepath.Join(hubRoot, "llms.txt")
		if data, err := os.ReadFile(llmsPath); err == nil {
			// Include just the first 1500 chars of llms.txt as overview
			text := string(data)
			if len(text) > 1500 {
				text = text[:1500] + "\n..."
			}
			context.WriteString("\nService index (from llms.txt):\n")
			context.WriteString(text)
		}

		// Clean stale session files from previous sessions
		cleanStaleSessionFiles()

		// Output SessionStart response with watchPaths
		output := map[string]interface{}{
			"hookSpecificOutput": map[string]interface{}{
				"hookEventName":     "SessionStart",
				"additionalContext": context.String(),
				"watchPaths":        watchPaths,
			},
		}

		data, _ := json.Marshal(output)
		fmt.Print(string(data))
		return nil
	},
}

// collectAllMdPaths walks the hub and returns absolute paths to all .md files
func collectAllMdPaths(hubRoot string) []string {
	var paths []string
	filepath.Walk(hubRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Skip hidden dirs except .ohara-playbooks
		if info.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") && base != ".ohara-playbooks" && path != hubRoot {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".md") {
			absPath, _ := filepath.Abs(path)
			paths = append(paths, absPath)
		}
		return nil
	})
	return paths
}

// cleanStaleSessionFiles removes temp files from previous sessions
func cleanStaleSessionFiles() {
	tmpDir := os.TempDir()
	entries, _ := os.ReadDir(tmpDir)
	myPpid := fmt.Sprintf("-%d-", os.Getppid())
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".ohara-gate-") && !strings.Contains(name, myPpid) {
			os.Remove(filepath.Join(tmpDir, name))
		}
		if strings.HasPrefix(name, ".ohara-injected-") && !strings.Contains(name, myPpid) {
			os.Remove(filepath.Join(tmpDir, name))
		}
	}
}

// --------------------------------------------------------------------------
// ohara file-changed — FileChanged hook (auto-rebuild llms.txt)
// --------------------------------------------------------------------------

var fileChangedCmd = &cobra.Command{
	Use:    "file-changed",
	Short:  "Hook: auto-rebuild llms.txt when hub docs change",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		var input struct {
			FilePath string `json:"file_path"`
		}
		if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
			return nil
		}

		if !strings.HasSuffix(input.FilePath, ".md") {
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

		// Check if the changed file is inside the hub
		absChanged, _ := filepath.Abs(input.FilePath)
		absHub, _ := filepath.Abs(hubRoot)
		if !strings.HasPrefix(absChanged, absHub) {
			return nil
		}

		// Rebuild llms.txt silently
		buildLlmsTxt(hubRoot, config)
		fmt.Fprintf(os.Stderr, "llms.txt rebuilt (file changed: %s)\n", filepath.Base(input.FilePath))

		return nil
	},
}

// --------------------------------------------------------------------------
// ohara subagent-start — SubagentStart hook (role-specific context injection)
// --------------------------------------------------------------------------

var subagentStartCmd = &cobra.Command{
	Use:    "subagent-start",
	Short:  "Hook: inject role-specific context when subagent spawns",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		var input struct {
			AgentType string `json:"agent_type"`
		}
		if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
			return nil
		}

		// Only inject for ohara agents
		if !strings.HasPrefix(input.AgentType, "ohara-") {
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

		context := getAgentContext(hubRoot, config, input.AgentType)
		if context == "" {
			return nil
		}

		output := map[string]interface{}{
			"hookSpecificOutput": map[string]interface{}{
				"hookEventName":     "SubagentStart",
				"additionalContext": context,
			},
		}

		data, _ := json.Marshal(output)
		fmt.Print(string(data))
		return nil
	},
}

// getAgentContext returns role-specific context for an ohara agent
func getAgentContext(hubRoot string, config *HubConfig, agentType string) string {
	hubDirName := filepath.Base(hubRoot)
	var sb strings.Builder

	switch agentType {
	case "ohara-writer":
		sb.WriteString(fmt.Sprintf("Hub: %s/ | Writing docs for %d services\n\n", hubDirName, len(config.Repos)))

		// List available .ohara-prompts
		promptsDir := filepath.Join(hubRoot, ".ohara-prompts")
		if entries, err := os.ReadDir(promptsDir); err == nil && len(entries) > 0 {
			sb.WriteString("Available prompts (.ohara-prompts/):\n")
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".prompt.md") {
					sb.WriteString(fmt.Sprintf("  - %s\n", e.Name()))
				}
			}
			sb.WriteString("\nRead these prompts FIRST before writing any doc.\n")
		}

		// List services with their doc status
		for _, repo := range config.Repos {
			docsDir := filepath.Join(hubRoot, repo.Name)
			docCount := 0
			filepath.Walk(docsDir, func(path string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() && strings.HasSuffix(path, ".md") {
					docCount++
				}
				return nil
			})
			sb.WriteString(fmt.Sprintf("\nService %s: %d docs, code at %s\n", repo.Name, docCount, repo.Path))
		}

	case "ohara-reviewer":
		sb.WriteString(fmt.Sprintf("Hub: %s/ | Reviewing docs for accuracy\n\n", hubDirName))

		// Show recently changed docs (git diff)
		gitDiff := exec.Command("git", "diff", "--name-only", "HEAD~5", "--", "*.md")
		gitDiff.Dir = hubRoot
		if diffOut, err := gitDiff.Output(); err == nil && len(diffOut) > 0 {
			sb.WriteString("Recently changed docs:\n")
			for _, line := range strings.Split(strings.TrimSpace(string(diffOut)), "\n") {
				if line != "" {
					sb.WriteString(fmt.Sprintf("  - %s\n", line))
				}
			}
			sb.WriteString("\nPrioritize reviewing these files.\n")
		}

	case "ohara-researcher":
		sb.WriteString(fmt.Sprintf("Hub: %s/ | Searching docs\n\n", hubDirName))

		// Inject full llms.txt for fast lookup
		llmsPath := filepath.Join(hubRoot, "llms.txt")
		if data, err := os.ReadFile(llmsPath); err == nil {
			text := string(data)
			if len(text) > 3000 {
				text = text[:3000] + "\n..."
			}
			sb.WriteString("Full doc index:\n")
			sb.WriteString(text)
		}

	case "ohara-watcher":
		sb.WriteString(fmt.Sprintf("Hub: %s/ | Checking staleness\n\n", hubDirName))

		// List services with last commit dates
		for _, repo := range config.Repos {
			codePath := ResolveRepoPath(hubRoot, repo)
			if codePath == "" {
				continue
			}
			// Get last commit date for the service code
			gitLog := exec.Command("git", "log", "-1", "--format=%cr", "--", ".")
			gitLog.Dir = codePath
			if out, err := gitLog.Output(); err == nil {
				sb.WriteString(fmt.Sprintf("  %s: last code change %s", repo.Name, strings.TrimSpace(string(out))))
			}

			// Get last doc change
			docsDir := filepath.Join(hubRoot, repo.Name)
			gitLogDocs := exec.Command("git", "log", "-1", "--format=%cr", "--", ".")
			gitLogDocs.Dir = docsDir
			if out, err := gitLogDocs.Output(); err == nil {
				sb.WriteString(fmt.Sprintf(", last doc change %s\n", strings.TrimSpace(string(out))))
			} else {
				sb.WriteString(", no docs yet\n")
			}
		}

	case "ohara-orchestrator":
		sb.WriteString(fmt.Sprintf("Hub: %s/ | Executing playbook\n\n", hubDirName))

		// Check for active tasks
		tasksDir := filepath.Join(hubRoot, ".scratch", "tasks")
		if entries, err := os.ReadDir(tasksDir); err == nil {
			activeTasks := []string{}
			for _, e := range entries {
				if e.IsDir() {
					activeTasks = append(activeTasks, e.Name())
				}
			}
			if len(activeTasks) > 0 {
				sb.WriteString("Active tasks:\n")
				for _, t := range activeTasks {
					sb.WriteString(fmt.Sprintf("  - %s\n", t))
					// Read context.md if it exists
					ctxPath := filepath.Join(tasksDir, t, "context.md")
					if data, err := os.ReadFile(ctxPath); err == nil {
						text := string(data)
						if len(text) > 500 {
							text = text[:500] + "\n..."
						}
						sb.WriteString(text + "\n")
					}
				}
			}
		}

		// List available services
		sb.WriteString("\nTracked services:\n")
		for _, repo := range config.Repos {
			sb.WriteString(fmt.Sprintf("  - %s (code: %s, docs: %s/%s/)\n", repo.Name, repo.Path, hubDirName, repo.Name))
		}
	}

	return sb.String()
}

// --------------------------------------------------------------------------
// ohara pre-compact — PreCompact hook (preserve essential context)
// --------------------------------------------------------------------------

var preCompactCmd = &cobra.Command{
	Use:    "pre-compact",
	Short:  "Hook: preserve critical context before conversation compression",
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

		summary := buildCompactionSummary(hubRoot, config)
		if summary == "" {
			return nil
		}

		output := map[string]interface{}{
			"hookSpecificOutput": map[string]interface{}{
				"hookEventName":     "PreCompact",
				"additionalContext": summary,
			},
		}

		data, _ := json.Marshal(output)
		fmt.Print(string(data))
		return nil
	},
}

// buildCompactionSummary creates essential context that should survive compression
func buildCompactionSummary(hubRoot string, config *HubConfig) string {
	hubDirName := filepath.Base(hubRoot)
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("=== Ohara Hub Context (preserved across compaction) ===\n"))
	sb.WriteString(fmt.Sprintf("Hub: %s/ | %d services tracked\n", hubDirName, len(config.Repos)))

	// Services edited this session
	editedServices := []string{}
	for _, repo := range config.Repos {
		sessionFile := filepath.Join(os.TempDir(), fmt.Sprintf(".ohara-gate-%d-%s", os.Getppid(), repo.Name))
		if _, err := os.Stat(sessionFile); err == nil {
			editedServices = append(editedServices, repo.Name)
		}
	}
	if len(editedServices) > 0 {
		sb.WriteString(fmt.Sprintf("Services edited this session: %s\n", strings.Join(editedServices, ", ")))
	}

	// Active task
	tasksDir := filepath.Join(hubRoot, ".scratch", "tasks")
	if entries, err := os.ReadDir(tasksDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				sb.WriteString(fmt.Sprintf("Active task: %s\n", e.Name()))
				// Include task context summary
				ctxPath := filepath.Join(tasksDir, e.Name(), "context.md")
				if data, err := os.ReadFile(ctxPath); err == nil {
					text := string(data)
					if len(text) > 300 {
						text = text[:300] + "..."
					}
					sb.WriteString(text + "\n")
				}
			}
		}
	}

	// Service list for reference
	sb.WriteString("\nTracked services:\n")
	for _, repo := range config.Repos {
		sb.WriteString(fmt.Sprintf("  - %s (code: %s)\n", repo.Name, repo.Path))
	}

	sb.WriteString("\nCommands: /fix, /feature, /investigate, /review-pr, /validate-docs, /create-docs-pr\n")
	sb.WriteString(fmt.Sprintf("MCP tools: search_docs, read_doc, write_doc, validate, list_docs\n"))

	return sb.String()
}

// --------------------------------------------------------------------------
// ohara status-line — StatusLine hook (Claude Code status bar)
// --------------------------------------------------------------------------

var statusLineCmd = &cobra.Command{
	Use:    "status-line",
	Short:  "Hook: provide status bar content for Claude Code",
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

		// Count total docs
		totalDocs := 0
		stubDocs := 0
		for _, repo := range config.Repos {
			docsDir := filepath.Join(hubRoot, repo.Name)
			filepath.Walk(docsDir, func(path string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() && strings.HasSuffix(path, ".md") && filepath.Base(path) != "CHANGELOG.md" && filepath.Base(path) != "README.md" {
					totalDocs++
					if isStubDoc(path) {
						stubDocs++
					}
				}
				return nil
			})
		}

		completeDocs := totalDocs - stubDocs

		// Count active tasks
		activeTasks := 0
		tasksDir := filepath.Join(hubRoot, ".scratch", "tasks")
		if entries, err := os.ReadDir(tasksDir); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					activeTasks++
				}
			}
		}

		// Build status line
		parts := []string{
			fmt.Sprintf("ohara: %d services", len(config.Repos)),
			fmt.Sprintf("%d/%d docs", completeDocs, totalDocs),
		}

		if activeTasks > 0 {
			parts = append(parts, fmt.Sprintf("%d active tasks", activeTasks))
		}

		fmt.Print(strings.Join(parts, " | "))
		return nil
	},
}

// --------------------------------------------------------------------------
// init — register all hook commands
// --------------------------------------------------------------------------

func init() {
	rootCmd.AddCommand(hookPromptCmd)
	rootCmd.AddCommand(gateCmd)
	rootCmd.AddCommand(watchHookCmd)
	rootCmd.AddCommand(sessionSummaryCmd)
	rootCmd.AddCommand(sessionStartCmd)
	rootCmd.AddCommand(fileChangedCmd)
	rootCmd.AddCommand(subagentStartCmd)
	rootCmd.AddCommand(preCompactCmd)
	rootCmd.AddCommand(statusLineCmd)
}
