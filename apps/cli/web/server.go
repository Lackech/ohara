package web

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"go.yaml.in/yaml/v3"
)

//go:embed templates/*.html
var templateFS embed.FS

var templates *template.Template
var md goldmark.Markdown

func init() {
	templates = template.Must(template.ParseFS(templateFS, "templates/*.html"))
	md = goldmark.New(goldmark.WithExtensions(extension.GFM, extension.Table))
}

// HubConfig mirrors the CLI config
type HubConfig struct {
	Name  string      `yaml:"name"`
	Repos []RepoEntry `yaml:"repos"`
}

type RepoEntry struct {
	Name   string `yaml:"name"`
	Path   string `yaml:"path"`
	Remote string `yaml:"remote"`
}

// Serve starts the local documentation viewer
func Serve(hubRoot string, port int) error {
	config, err := loadConfig(hubRoot)
	if err != nil {
		return fmt.Errorf("failed to load hub config: %w", err)
	}

	s := &server{hubRoot: hubRoot, config: config}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleDashboard)
	mux.HandleFunc("/nav", s.handleNav)
	mux.HandleFunc("/service/", s.handleService)
	mux.HandleFunc("/doc/", s.handleDoc)
	mux.HandleFunc("/search", s.handleSearch)

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("Ohara docs viewer: http://localhost:%d\n", port)
	return http.ListenAndServe(addr, mux)
}

type server struct {
	hubRoot string
	config  *HubConfig
}

// --- Handlers ---

func (s *server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	type serviceInfo struct {
		Name         string
		Tutorials    int
		Guides       int
		References   int
		Explanations int
		Stubs        int
		TotalDocs    int
		CompleteDocs int
	}

	type changeInfo struct {
		Hash, Message, Service string
	}

	services := []serviceInfo{}
	changes := []changeInfo{}

	for _, repo := range s.config.Repos {
		si := serviceInfo{Name: repo.Name}
		repoDir := filepath.Join(s.hubRoot, repo.Name)

		for _, ddir := range []string{"tutorials", "guides", "reference", "explanation"} {
			docs := collectMdFiles(filepath.Join(repoDir, ddir))
			count := len(docs)
			stubs := 0
			for _, d := range docs {
				if isStub(d) {
					stubs++
				}
			}
			switch ddir {
			case "tutorials":
				si.Tutorials = count
			case "guides":
				si.Guides = count
			case "reference":
				si.References = count
			case "explanation":
				si.Explanations = count
			}
			si.TotalDocs += count
			si.Stubs += stubs
		}
		si.CompleteDocs = si.TotalDocs - si.Stubs
		services = append(services, si)

		// Get recent commits for changelog
		codePath := repo.Path
		if !filepath.IsAbs(codePath) {
			codePath = filepath.Join(s.hubRoot, codePath)
		}
		gitLog := exec.Command("git", "log", "--oneline", "-3")
		gitLog.Dir = codePath
		if output, err := gitLog.Output(); err == nil {
			for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
				if line != "" {
					parts := strings.SplitN(line, " ", 2)
					msg := ""
					if len(parts) > 1 {
						msg = parts[1]
					}
					changes = append(changes, changeInfo{Hash: parts[0], Message: msg, Service: repo.Name})
				}
			}
		}
	}

	data := map[string]interface{}{
		"HubName":       s.config.Name,
		"ServiceCount":  len(services),
		"Services":      services,
		"HasChangelogs": len(changes) > 0,
		"RecentChanges": changes,
	}

	s.renderPage(w, "dashboard.html", s.config.Name, data)
}

func (s *server) handleNav(w http.ResponseWriter, r *http.Request) {
	type navDoc struct {
		Title string
		Path  string
	}
	type navSection struct {
		Label string
		Type  string
		Docs  []navDoc
	}
	type navService struct {
		Name     string
		Sections []navSection
	}

	labels := map[string]string{
		"tutorials": "Tutorials", "guides": "Guides",
		"reference": "Reference", "explanation": "Explanation",
	}
	types := map[string]string{
		"tutorials": "tutorial", "guides": "guide",
		"reference": "reference", "explanation": "explanation",
	}

	services := []navService{}
	for _, repo := range s.config.Repos {
		ns := navService{Name: repo.Name}
		repoDir := filepath.Join(s.hubRoot, repo.Name)

		for _, ddir := range []string{"tutorials", "guides", "reference", "explanation"} {
			docs := collectMdFiles(filepath.Join(repoDir, ddir))
			if len(docs) == 0 {
				continue
			}
			section := navSection{Label: labels[ddir], Type: types[ddir]}
			for _, d := range docs {
				rel, _ := filepath.Rel(s.hubRoot, d)
				section.Docs = append(section.Docs, navDoc{
					Title: extractDocTitle(d),
					Path:  rel,
				})
			}
			ns.Sections = append(ns.Sections, section)
		}
		services = append(services, ns)
	}

	templates.ExecuteTemplate(w, "nav.html", map[string]interface{}{"Services": services})
}

func (s *server) handleService(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/service/")

	type docInfo struct {
		Title       string
		Description string
		Path        string
		IsStub      bool
	}
	type sectionInfo struct {
		Label string
		Type  string
		Docs  []docInfo
	}

	labels := map[string]string{
		"tutorials": "Tutorials", "guides": "Guides",
		"reference": "Reference", "explanation": "Explanation",
	}
	types := map[string]string{
		"tutorials": "tutorial", "guides": "guide",
		"reference": "reference", "explanation": "explanation",
	}

	repoDir := filepath.Join(s.hubRoot, name)
	sections := []sectionInfo{}
	totalDocs := 0

	for _, ddir := range []string{"tutorials", "guides", "reference", "explanation"} {
		docs := collectMdFiles(filepath.Join(repoDir, ddir))
		if len(docs) == 0 {
			continue
		}
		section := sectionInfo{Label: labels[ddir], Type: types[ddir]}
		for _, d := range docs {
			rel, _ := filepath.Rel(s.hubRoot, d)
			section.Docs = append(section.Docs, docInfo{
				Title:       extractDocTitle(d),
				Description: extractDocDescription(d),
				Path:        rel,
				IsStub:      isStub(d),
			})
			totalDocs++
		}
		sections = append(sections, section)
	}

	// Read changelog
	var changelog []string
	changelogPath := filepath.Join(repoDir, "CHANGELOG.md")
	if data, err := os.ReadFile(changelogPath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "- ") {
				changelog = append(changelog, line)
				if len(changelog) >= 10 {
					break
				}
			}
		}
	}

	data := map[string]interface{}{
		"Name":      name,
		"TotalDocs": totalDocs,
		"Sections":  sections,
		"Changelog": changelog,
	}

	if isHTMX(r) {
		templates.ExecuteTemplate(w, "service.html", data)
	} else {
		s.renderPage(w, "service.html", name, data)
	}
}

func (s *server) handleDoc(w http.ResponseWriter, r *http.Request) {
	docPath := strings.TrimPrefix(r.URL.Path, "/doc/")
	fullPath := filepath.Join(s.hubRoot, docPath)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Parse frontmatter
	text := string(content)
	diataxisType := "explanation"
	if strings.HasPrefix(text, "---") {
		if idx := strings.Index(text[3:], "---"); idx >= 0 {
			fm := text[3 : idx+3]
			for _, line := range strings.Split(fm, "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "diataxis_type:") {
					diataxisType = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "diataxis_type:"))
				}
			}
			text = text[idx+6:]
		}
	}

	// Render markdown
	var buf bytes.Buffer
	md.Convert([]byte(text), &buf)

	labels := map[string]string{
		"tutorial": "Tutorial", "guide": "Guide",
		"reference": "Reference", "explanation": "Explanation",
	}

	// Extract service name from path
	service := strings.Split(docPath, "/")[0]

	data := map[string]interface{}{
		"Service":      service,
		"Path":         docPath,
		"DiataxisType": diataxisType,
		"DiataxisLabel": labels[diataxisType],
		"HTML":         template.HTML(buf.String()),
	}

	if isHTMX(r) {
		templates.ExecuteTemplate(w, "doc.html", data)
	} else {
		s.renderPage(w, "doc.html", extractDocTitle(fullPath), data)
	}
}

func (s *server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		templates.ExecuteTemplate(w, "search.html", map[string]interface{}{
			"Query":   "",
			"Results": nil,
		})
		return
	}

	type result struct {
		Title        string
		Path         string
		Service      string
		DiataxisType string
		DiataxisLabel string
		Snippet      string
	}

	labels := map[string]string{
		"tutorial": "Tutorial", "guide": "Guide",
		"reference": "Reference", "explanation": "Explanation",
	}

	var results []result
	queryLower := strings.ToLower(query)

	for _, repo := range s.config.Repos {
		repoDir := filepath.Join(s.hubRoot, repo.Name)
		for _, ddir := range []string{"tutorials", "guides", "reference", "explanation"} {
			for _, f := range collectMdFiles(filepath.Join(repoDir, ddir)) {
				data, err := os.ReadFile(f)
				if err != nil {
					continue
				}
				content := string(data)
				contentLower := strings.ToLower(content)

				if !strings.Contains(contentLower, queryLower) {
					continue
				}

				rel, _ := filepath.Rel(s.hubRoot, f)
				dType := ddir
				if dType == "tutorials" {
					dType = "tutorial"
				} else if dType == "guides" {
					dType = "guide"
				}

				// Extract snippet around match
				snippet := ""
				idx := strings.Index(contentLower, queryLower)
				if idx >= 0 {
					start := idx - 80
					if start < 0 {
						start = 0
					}
					end := idx + len(query) + 80
					if end > len(content) {
						end = len(content)
					}
					snippet = "..." + strings.ReplaceAll(
						content[start:end],
						query,
						"<mark>"+query+"</mark>",
					) + "..."
				}

				results = append(results, result{
					Title:         extractDocTitle(f),
					Path:          rel,
					Service:       repo.Name,
					DiataxisType:  dType,
					DiataxisLabel: labels[dType],
					Snippet:       snippet,
				})
			}
		}
	}

	templates.ExecuteTemplate(w, "search.html", map[string]interface{}{
		"Query":   query,
		"Results": results,
	})
}

// --- Helpers ---

func (s *server) renderPage(w http.ResponseWriter, tmplName, title string, data interface{}) {
	var content bytes.Buffer
	templates.ExecuteTemplate(&content, tmplName, data)

	templates.ExecuteTemplate(w, "layout.html", map[string]interface{}{
		"Title":   title,
		"HubName": s.config.Name,
		"Content": template.HTML(content.String()),
	})
}

func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func loadConfig(hubRoot string) (*HubConfig, error) {
	data, err := os.ReadFile(filepath.Join(hubRoot, ".ohara.yaml"))
	if err != nil {
		return nil, err
	}
	var config HubConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func collectMdFiles(dir string) []string {
	var files []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".md" || ext == ".mdx" {
			files = append(files, path)
		}
		return nil
	})
	return files
}

func extractDocTitle(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return filepath.Base(path)
	}
	text := string(data)
	if strings.HasPrefix(text, "---") {
		if idx := strings.Index(text[3:], "---"); idx >= 0 {
			for _, line := range strings.Split(text[3:idx+3], "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "title:") {
					t := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "title:"))
					return strings.Trim(t, "\"'")
				}
			}
		}
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

func extractDocDescription(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	text := string(data)
	if strings.HasPrefix(text, "---") {
		if idx := strings.Index(text[3:], "---"); idx >= 0 {
			for _, line := range strings.Split(text[3:idx+3], "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "description:") {
					d := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "description:"))
					return strings.Trim(d, "\"'")
				}
			}
		}
	}
	return ""
}

func isStub(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	text := string(data)
	if strings.HasPrefix(text, "---") {
		if idx := strings.Index(text[3:], "---"); idx >= 0 {
			text = strings.TrimSpace(text[idx+6:])
		}
	}
	todoCount := strings.Count(text, "TODO")
	wordCount := len(strings.Fields(text))
	return todoCount >= 2 || (wordCount < 30 && todoCount >= 1)
}
