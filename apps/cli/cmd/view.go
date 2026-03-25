package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

var viewCmd = &cobra.Command{
	Use:     "view",
	Aliases: []string{"dev:view"},
	Short:   "Start the Starlight documentation viewer",
	Long: `Opens a Mintlify-quality documentation viewer powered by Astro Starlight.
Runs ohara build:site first, then starts the Astro dev server.

First time? Run 'ohara init:viewer' to set up the Starlight project.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		hubRoot, err := FindHubRoot(".")
		if err != nil {
			return fmt.Errorf("no ohara hub found. Run 'ohara init' first")
		}

		workDir := filepath.Dir(hubRoot)
		viewerDir := filepath.Join(workDir, ".ohara-viewer")

		// Check if viewer is initialized
		if _, err := os.Stat(filepath.Join(viewerDir, "package.json")); os.IsNotExist(err) {
			fmt.Println("Viewer not set up. Initializing...")
			if err := initViewer(workDir); err != nil {
				return err
			}
		}

		// Build site content
		fmt.Println("Building site from hub...")
		buildSiteCmd.RunE(cmd, args)

		// Start Astro dev server
		port, _ := cmd.Flags().GetInt("port")
		noBrowser, _ := cmd.Flags().GetBool("no-browser")

		if !noBrowser {
			go openBrowser(fmt.Sprintf("http://localhost:%d", port))
		}

		fmt.Printf("\nStarting Starlight at http://localhost:%d\n", port)

		astro := exec.Command("npx", "astro", "dev", "--port", fmt.Sprintf("%d", port))
		astro.Dir = viewerDir
		astro.Stdout = os.Stdout
		astro.Stderr = os.Stderr
		astro.Stdin = os.Stdin

		return astro.Run()
	},
}

var initViewerCmd = &cobra.Command{
	Use:   "init:viewer",
	Short: "Initialize the Starlight documentation viewer",
	Long:  `Sets up the Astro Starlight project for the documentation viewer.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		hubRoot, err := FindHubRoot(".")
		if err != nil {
			return fmt.Errorf("no ohara hub found. Run 'ohara init' first")
		}
		workDir := filepath.Dir(hubRoot)
		return initViewer(workDir)
	},
}

func initViewer(workDir string) error {
	viewerDir := filepath.Join(workDir, ".ohara-viewer")

	if _, err := os.Stat(filepath.Join(viewerDir, "package.json")); err == nil {
		fmt.Println("Viewer already initialized.")
		return nil
	}

	os.MkdirAll(viewerDir, 0755)

	// Create package.json
	pkgJSON := `{
  "name": "ohara-viewer",
  "private": true,
  "scripts": {
    "dev": "astro dev",
    "build": "astro build"
  },
  "dependencies": {
    "astro": "5.18.1",
    "@astrojs/starlight": "0.33.2",
    "zod": "3.24.2"
  }
}
`
	os.WriteFile(filepath.Join(viewerDir, "package.json"), []byte(pkgJSON), 0644)

	// Create directory structure
	os.MkdirAll(filepath.Join(viewerDir, "src", "content", "docs"), 0755)
	os.MkdirAll(filepath.Join(viewerDir, "public"), 0755)

	// Create content config
	contentConfig := `import { defineCollection } from 'astro:content';
import { docsSchema } from '@astrojs/starlight/schema';

export const collections = {
  docs: defineCollection({ schema: docsSchema() }),
};
`
	os.WriteFile(filepath.Join(viewerDir, "src", "content.config.ts"), []byte(contentConfig), 0644)

	// Create custom CSS
	customCSS := `:root {
  --sl-color-accent-low: #1e3a5f;
  --sl-color-accent: #3b82f6;
  --sl-color-accent-high: #93c5fd;
}
`
	os.WriteFile(filepath.Join(viewerDir, "src", "custom.css"), []byte(customCSS), 0644)

	// Create placeholder astro config
	astroConfig := `import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  integrations: [
    starlight({
      title: 'Ohara Docs',
      sidebar: [],
    }),
  ],
});
`
	os.WriteFile(filepath.Join(viewerDir, "astro.config.mjs"), []byte(astroConfig), 0644)

	// Create placeholder index
	indexPage := `---
title: Documentation Hub
description: Ohara documentation hub
template: splash
hero:
  title: Documentation Hub
  tagline: Run 'ohara build:site' to populate with your docs.
---
`
	os.WriteFile(filepath.Join(viewerDir, "src", "content", "docs", "index.mdx"), []byte(indexPage), 0644)

	// Add to gitignore
	gitignorePath := filepath.Join(workDir, ".gitignore")
	if data, err := os.ReadFile(gitignorePath); err == nil {
		if !containsLine(string(data), ".ohara-viewer/") {
			f, _ := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0644)
			f.WriteString("\n.ohara-viewer/\n")
			f.Close()
		}
	}

	// Install dependencies
	fmt.Println("Installing Starlight dependencies...")
	npmInstall := exec.Command("npm", "install")
	npmInstall.Dir = viewerDir
	npmInstall.Stdout = os.Stdout
	npmInstall.Stderr = os.Stderr
	if err := npmInstall.Run(); err != nil {
		return fmt.Errorf("failed to install dependencies: %w", err)
	}

	fmt.Println("✓ Viewer initialized at .ohara-viewer/")
	return nil
}

func openBrowser(url string) {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", url).Start()
	case "linux":
		exec.Command("xdg-open", url).Start()
	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}
}

func init() {
	viewCmd.Flags().IntP("port", "p", 4321, "Port to serve on")
	viewCmd.Flags().Bool("no-browser", false, "Don't open browser automatically")
	rootCmd.AddCommand(viewCmd)
	rootCmd.AddCommand(initViewerCmd)
}
