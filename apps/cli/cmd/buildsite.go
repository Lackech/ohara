package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var buildSiteCmd = &cobra.Command{
	Use:   "build:site",
	Short: "Generate the Starlight documentation site from the hub",
	Long: `Reads the hub docs and generates a Starlight (Astro) project.
Creates astro.config.mjs with sidebar, copies docs into content directory,
and generates the landing page.

After running, use 'ohara view' to preview or 'astro build' to generate static HTML.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		hubRoot, err := FindHubRoot(".")
		if err != nil {
			return fmt.Errorf("no ohara hub found")
		}

		config, err := LoadHubConfig(hubRoot)
		if err != nil {
			return err
		}

		// Find or create viewer directory
		workDir := filepath.Dir(hubRoot)
		viewerDir := filepath.Join(workDir, ".ohara-viewer")

		// Check if viewer is initialized
		if _, err := os.Stat(filepath.Join(viewerDir, "package.json")); os.IsNotExist(err) {
			return fmt.Errorf("viewer not initialized. Run 'ohara init:viewer' first")
		}

		contentDir := filepath.Join(viewerDir, "src", "content", "docs")

		// Clean existing content (but keep index.mdx template)
		entries, _ := os.ReadDir(contentDir)
		for _, entry := range entries {
			if entry.Name() != "index.mdx" {
				os.RemoveAll(filepath.Join(contentDir, entry.Name()))
			}
		}

		// Copy docs from hub into content directory
		totalDocs := 0
		for _, repo := range config.Repos {
			repoDocsDir := filepath.Join(hubRoot, repo.Name)
			if !dirExists(repoDocsDir) {
				continue
			}

			for _, ddir := range []string{"tutorials", "guides", "reference", "explanation"} {
				srcDir := filepath.Join(repoDocsDir, ddir)
				if !dirExists(srcDir) {
					continue
				}

				destDir := filepath.Join(contentDir, repo.Name, ddir)
				os.MkdirAll(destDir, 0755)

				filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
					if err != nil || info.IsDir() {
						return nil
					}
					ext := strings.ToLower(filepath.Ext(path))
					if ext != ".md" && ext != ".mdx" {
						return nil
					}
					// Skip stubs and prompt files
					if strings.Contains(path, ".ohara-prompts") {
						return nil
					}

					destPath := filepath.Join(destDir, info.Name())
					copyFileFixLangs(path, destPath)
					totalDocs++
					return nil
				})
			}

			// Copy CHANGELOG.md as a doc page
			changelogSrc := filepath.Join(repoDocsDir, "CHANGELOG.md")
			if _, err := os.Stat(changelogSrc); err == nil {
				changelogDest := filepath.Join(contentDir, repo.Name, "changelog.md")
				data, _ := os.ReadFile(changelogSrc)
				clContent := string(data)
				if !strings.HasPrefix(clContent, "---") {
					clContent = "---\ntitle: Changelog\ndescription: Recent changes and PR history\n---\n\n" + clContent
				}
				os.WriteFile(changelogDest, []byte(clContent), 0644)
			}

			// Generate service index page
			serviceIndexContent := fmt.Sprintf("---\ntitle: %s\ndescription: Documentation for %s\n---\n\n", repo.Name, repo.Name)
			serviceIndexContent += fmt.Sprintf("# %s\n\n", repo.Name)
			if repo.Remote != "" {
				serviceIndexContent += fmt.Sprintf("**Repository:** [%s](%s)\n\n", repo.Remote, repo.Remote)
			}
			for _, ddir := range []string{"tutorials", "guides", "reference", "explanation"} {
				srcDir := filepath.Join(repoDocsDir, ddir)
				docs := collectMarkdownFiles(srcDir)
				if len(docs) == 0 {
					continue
				}
				labels := map[string]string{"tutorials": "Tutorials", "guides": "Guides", "reference": "Reference", "explanation": "Explanation"}
				serviceIndexContent += fmt.Sprintf("## %s\n\n", labels[ddir])
				for _, doc := range docs {
					title := extractTitle(doc.path)
					name := strings.TrimSuffix(filepath.Base(doc.path), filepath.Ext(doc.path))
					serviceIndexContent += fmt.Sprintf("- [%s](/%s/%s/%s/)\n", title, repo.Name, ddir, name)
				}
				serviceIndexContent += "\n"
			}
			os.MkdirAll(filepath.Join(contentDir, repo.Name), 0755)
			os.WriteFile(filepath.Join(contentDir, repo.Name, "index.md"), []byte(serviceIndexContent), 0644)
		}

		fmt.Printf("✓ Copied %d docs into viewer\n", totalDocs)

		// Generate landing page
		generateLandingPage(contentDir, config, hubRoot)
		fmt.Printf("✓ Generated landing page\n")

		// Generate astro.config.mjs with sidebar
		generateAstroConfig(viewerDir, config)
		fmt.Printf("✓ Generated astro.config.mjs\n")

		return nil
	},
}

func generateLandingPage(contentDir string, config *HubConfig, hubRoot string) {
	var sections []string

	sections = append(sections, fmt.Sprintf(`---
title: %s
description: Diataxis-structured documentation for all services
template: splash
hero:
  title: %s
  tagline: Diataxis-structured documentation hub. %d services tracked.
---

import { LinkCard, CardGrid } from '@astrojs/starlight/components';

<CardGrid>
`, config.Name, config.Name, len(config.Repos)))

	diataxisLabels := map[string]string{
		"tutorials": "Tutorials", "guides": "Guides",
		"reference": "Reference", "explanation": "Explanation",
	}

	for _, repo := range config.Repos {
		repoDir := filepath.Join(hubRoot, repo.Name)
		counts := map[string]int{}
		total := 0

		for _, ddir := range []string{"tutorials", "guides", "reference", "explanation"} {
			n := len(collectMarkdownFiles(filepath.Join(repoDir, ddir)))
			counts[ddir] = n
			total += n
		}

		badges := []string{}
		for _, ddir := range []string{"tutorials", "guides", "reference", "explanation"} {
			if counts[ddir] > 0 {
				badges = append(badges, fmt.Sprintf("%d %s", counts[ddir], diataxisLabels[ddir]))
			}
		}

		desc := fmt.Sprintf("%d docs — %s", total, strings.Join(badges, " · "))

		sections = append(sections, fmt.Sprintf(
			"<LinkCard title=\"%s\" href=\"/%s/\" description=\"%s\" />\n\n",
			repo.Name, repo.Name, desc,
		))
	}

	sections = append(sections, "</CardGrid>\n")

	os.WriteFile(filepath.Join(contentDir, "index.mdx"), []byte(strings.Join(sections, "")), 0644)
}

func generateAstroConfig(viewerDir string, config *HubConfig) {
	type autoGenConf struct {
		Directory string `json:"directory"`
	}
	type sidebarItem struct {
		Label        string        `json:"label"`
		Autogenerate *autoGenConf  `json:"autogenerate,omitempty"`
		Items        []sidebarItem `json:"items,omitempty"`
		Link         string        `json:"link,omitempty"`
	}

	var sidebar []sidebarItem

	diataxisGroups := []struct {
		dir, label, badgeVariant string
	}{
		{"tutorials", "Tutorials", "success"},
		{"guides", "Guides", "note"},
		{"reference", "Reference", "caution"},
		{"explanation", "Explanation", "tip"},
	}

	for _, repo := range config.Repos {
		items := []sidebarItem{}

		for _, dg := range diataxisGroups {
			items = append(items, sidebarItem{
				Label:        dg.label,
				Autogenerate: &autoGenConf{Directory: repo.Name + "/" + dg.dir},
			})
		}

		// Add changelog link
		items = append(items, sidebarItem{
			Label: "Changelog",
			Link:  "/" + repo.Name + "/changelog",
		})

		sidebar = append(sidebar, sidebarItem{
			Label: repo.Name,
			Items: items,
		})
	}

	sidebarJSON, _ := json.MarshalIndent(sidebar, "      ", "  ")

	astroConfig := fmt.Sprintf(`// Auto-generated by ohara build:site — do not edit manually

import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  integrations: [
    starlight({
      title: '%s',
      customCss: ['./src/custom.css'],
      sidebar: %s,
    }),
  ],
});
`, config.Name, string(sidebarJSON))

	os.WriteFile(filepath.Join(viewerDir, "astro.config.mjs"), []byte(astroConfig), 0644)
}

// copyFileFixLangs copies a markdown file, replacing unsupported code block languages
func copyFileFixLangs(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	// Replace unsupported code block languages with supported ones
	content := string(data)
	unsupported := map[string]string{
		"```env": "```bash",
		"```dotenv": "```bash",
		"```conf": "```ini",
		"```config": "```ini",
	}
	for old, newLang := range unsupported {
		content = strings.ReplaceAll(content, old, newLang)
	}

	return os.WriteFile(dst, []byte(content), 0644)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func init() {
	rootCmd.AddCommand(buildSiteCmd)
}
