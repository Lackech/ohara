package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// MCP JSON-RPC types
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start a local MCP server (stdio) for AI agents",
	Long: `Starts an MCP server over stdio that gives agents tools to work with
the documentation hub. Configure in your MCP client settings.

Tools provided:
  - search_docs: Search across all documentation
  - list_docs: List documents by service and type
  - read_doc: Read a specific document
  - write_doc: Create or update a document
  - validate: Check documentation structure
  - create_pr: Create a PR with doc changes
  - changelog: Show recent documentation changes`,
	RunE: func(cmd *cobra.Command, args []string) error {
		hubRoot, err := FindHubRoot(".")
		if err != nil {
			// Try parent directory (workspace root)
			hubRoot, err = FindHubRoot("..")
			if err != nil {
				return fmt.Errorf("no ohara hub found. Run 'ohara init' first")
			}
		}

		scanner := bufio.NewScanner(os.Stdin)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			var req jsonRPCRequest
			if err := json.Unmarshal([]byte(line), &req); err != nil {
				continue
			}

			resp := handleMCPRequest(req, hubRoot)
			data, _ := json.Marshal(resp)
			fmt.Println(string(data))
		}

		return nil
	},
}

func handleMCPRequest(req jsonRPCRequest, hubRoot string) jsonRPCResponse {
	switch req.Method {
	case "initialize":
		return jsonRPCResponse{
			JSONRPC: "2.0", ID: req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				"serverInfo": map[string]interface{}{
					"name":    "ohara",
					"version": "0.5.0",
				},
			},
		}

	case "notifications/initialized":
		// No response needed for notifications
		return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: nil}

	case "tools/list":
		return jsonRPCResponse{
			JSONRPC: "2.0", ID: req.ID,
			Result: map[string]interface{}{
				"tools": getTools(),
			},
		}

	case "tools/call":
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		json.Unmarshal(req.Params, &params)
		return handleToolCall(req.ID, params.Name, params.Arguments, hubRoot)

	default:
		return jsonRPCResponse{
			JSONRPC: "2.0", ID: req.ID,
			Error: &rpcError{Code: -32601, Message: "Method not found: " + req.Method},
		}
	}
}

func getTools() []mcpTool {
	return []mcpTool{
		{
			Name:        "search_docs",
			Description: "Search across all documentation in the hub. Returns matching files and relevant excerpts.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query":   map[string]interface{}{"type": "string", "description": "Search query"},
					"service": map[string]interface{}{"type": "string", "description": "Filter to a specific service (optional)"},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "list_docs",
			Description: "List all documents in the hub, grouped by service and Diataxis type.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"service": map[string]interface{}{"type": "string", "description": "Filter to a specific service (optional)"},
					"type":    map[string]interface{}{"type": "string", "description": "Filter by Diataxis type: tutorial, guide, reference, explanation (optional)"},
				},
			},
		},
		{
			Name:        "read_doc",
			Description: "Read the full content of a specific document.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string", "description": "Document path relative to hub (e.g., 'my-service/guides/deployment.md')"},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "write_doc",
			Description: "Create or update a document in the hub. Provide the full markdown content including frontmatter.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":    map[string]interface{}{"type": "string", "description": "Document path relative to hub (e.g., 'my-service/guides/deployment.md')"},
					"content": map[string]interface{}{"type": "string", "description": "Full markdown content including frontmatter"},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			Name:        "validate",
			Description: "Validate the documentation hub structure, coverage, and quality. Returns errors and warnings.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "create_pr",
			Description: "Create a git branch, commit all changes, push, and open a PR on the docs hub repo.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"description": map[string]interface{}{"type": "string", "description": "PR description (used for branch name and commit message)"},
				},
				"required": []string{"description"},
			},
		},
		{
			Name:        "changelog",
			Description: "Show recent documentation changes from git history.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"service": map[string]interface{}{"type": "string", "description": "Filter to a specific service (optional)"},
					"count":   map[string]interface{}{"type": "number", "description": "Number of entries to show (default: 20)"},
				},
			},
		},
	}
}

func handleToolCall(id interface{}, name string, args map[string]interface{}, hubRoot string) jsonRPCResponse {
	var text string
	var isError bool

	switch name {
	case "search_docs":
		query, _ := args["query"].(string)
		service, _ := args["service"].(string)
		text = toolSearchDocs(hubRoot, query, service)

	case "list_docs":
		service, _ := args["service"].(string)
		docType, _ := args["type"].(string)
		text = toolListDocs(hubRoot, service, docType)

	case "read_doc":
		path, _ := args["path"].(string)
		text = toolReadDoc(hubRoot, path)

	case "write_doc":
		path, _ := args["path"].(string)
		content, _ := args["content"].(string)
		text = toolWriteDoc(hubRoot, path, content)

	case "validate":
		text = toolValidate(hubRoot)

	case "create_pr":
		desc, _ := args["description"].(string)
		text = toolCreatePR(hubRoot, desc)

	case "changelog":
		service, _ := args["service"].(string)
		count := 20
		if c, ok := args["count"].(float64); ok {
			count = int(c)
		}
		text = toolChangelog(hubRoot, service, count)

	default:
		text = "Unknown tool: " + name
		isError = true
	}

	result := map[string]interface{}{
		"content": []mcpContent{{Type: "text", Text: text}},
	}
	if isError {
		result["isError"] = true
	}

	return jsonRPCResponse{JSONRPC: "2.0", ID: id, Result: result}
}

func toolSearchDocs(hubRoot, query, service string) string {
	searchDir := hubRoot
	if service != "" {
		searchDir = filepath.Join(hubRoot, service)
	}

	cmd := exec.Command("grep", "-ri", query, "--include=*.md", "-l")
	cmd.Dir = searchDir
	output, err := cmd.Output()
	if err != nil || len(output) == 0 {
		return fmt.Sprintf("No results found for \"%s\".", query)
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d files matching \"%s\":\n\n", len(files), query))

	for _, f := range files {
		relPath, _ := filepath.Rel(hubRoot, filepath.Join(searchDir, f))
		title := extractTitle(filepath.Join(searchDir, f))
		result.WriteString(fmt.Sprintf("- **%s** (`%s`)\n", title, relPath))

		// Get matching lines
		grepLines := exec.Command("grep", "-i", query, "--include=*.md", "-n", "-m", "3", f)
		grepLines.Dir = searchDir
		if lineOutput, err := grepLines.Output(); err == nil {
			for _, line := range strings.Split(strings.TrimSpace(string(lineOutput)), "\n") {
				if line != "" {
					result.WriteString(fmt.Sprintf("  %s\n", strings.TrimSpace(line)))
				}
			}
		}
		result.WriteString("\n")
	}

	return result.String()
}

func toolListDocs(hubRoot, service, docType string) string {
	config, err := LoadHubConfig(hubRoot)
	if err != nil {
		return "Failed to load hub config: " + err.Error()
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("# %s — Documentation Index\n\n", config.Name))

	diataxisDirs := map[string]string{
		"tutorials": "Tutorial", "guides": "How-to Guide",
		"reference": "Reference", "explanation": "Explanation",
	}

	repos := config.Repos
	if service != "" {
		repos = nil
		for _, r := range config.Repos {
			if r.Name == service {
				repos = []RepoEntry{r}
				break
			}
		}
		if repos == nil {
			return fmt.Sprintf("Service \"%s\" not found.", service)
		}
	}

	for _, repo := range repos {
		repoDir := filepath.Join(hubRoot, repo.Name)
		result.WriteString(fmt.Sprintf("## %s\n\n", repo.Name))

		for _, dDir := range []string{"tutorials", "guides", "reference", "explanation"} {
			if docType != "" && !strings.HasPrefix(dDir, docType) && diataxisDirs[dDir] != docType {
				continue
			}

			typeDir := filepath.Join(repoDir, dDir)
			docs := collectMarkdownFiles(typeDir)
			if len(docs) == 0 {
				continue
			}

			result.WriteString(fmt.Sprintf("### %s\n", diataxisDirs[dDir]))
			for _, doc := range docs {
				relPath, _ := filepath.Rel(hubRoot, doc.path)
				title := extractTitle(doc.path)
				result.WriteString(fmt.Sprintf("- [%s](%s)\n", title, relPath))
			}
			result.WriteString("\n")
		}
	}

	return result.String()
}

func toolReadDoc(hubRoot, path string) string {
	fullPath := filepath.Join(hubRoot, path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Sprintf("Document not found: %s", path)
	}
	return string(data)
}

func toolWriteDoc(hubRoot, path, content string) string {
	fullPath := filepath.Join(hubRoot, path)

	// Create directory if needed
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Sprintf("Failed to create directory: %v", err)
	}

	isNew := true
	if _, err := os.Stat(fullPath); err == nil {
		isNew = false
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return fmt.Sprintf("Failed to write document: %v", err)
	}

	action := "Created"
	if !isNew {
		action = "Updated"
	}

	// Rebuild llms.txt
	config, _ := LoadHubConfig(hubRoot)
	if config != nil {
		buildLlmsTxt(hubRoot, config)
	}

	return fmt.Sprintf("%s document: %s\nllms.txt regenerated.", action, path)
}

func toolValidate(hubRoot string) string {
	cmd := exec.Command("ohara", "validate")
	cmd.Dir = hubRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output)
	}
	return string(output)
}

func toolCreatePR(hubRoot, description string) string {
	// Sanitize description for branch name
	branchName := "docs/" + strings.ReplaceAll(strings.ToLower(description), " ", "-")

	commands := []struct {
		name string
		args []string
	}{
		{"git", []string{"checkout", "-b", branchName}},
		{"git", []string{"add", "-A"}},
		{"git", []string{"commit", "-m", "docs: " + description}},
		{"git", []string{"push", "origin", branchName}},
		{"gh", []string{"pr", "create", "--title", "docs: " + description, "--body", "Documentation update by Ohara agent."}},
	}

	var result strings.Builder
	for _, c := range commands {
		cmd := exec.Command(c.name, c.args...)
		cmd.Dir = hubRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			result.WriteString(fmt.Sprintf("Command `%s %s` failed: %s\n%s",
				c.name, strings.Join(c.args, " "), err.Error(), string(output)))
			return result.String()
		}
		if len(output) > 0 {
			result.WriteString(string(output))
		}
	}

	return result.String()
}

func toolChangelog(hubRoot, service string, count int) string {
	args := []string{"log", fmt.Sprintf("--oneline=-%d", count), fmt.Sprintf("-n%d", count), "--oneline"}
	if service != "" {
		args = append(args, "--", service+"/")
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = hubRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "No git history found. Commit your docs first."
	}

	if len(output) == 0 {
		return "No changes recorded yet."
	}

	return fmt.Sprintf("Recent documentation changes:\n\n%s", string(output))
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
