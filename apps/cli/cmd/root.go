package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is set by goreleaser via ldflags, or "dev" for local builds
var Version = "dev"

var rootCmd = &cobra.Command{
	Version: Version,
	Use:   "ohara",
	Short: "Ohara — Agent-optimized documentation for your codebase",
	Long: `Ohara creates a documentation hub for your organization's repositories.
It uses the Diataxis framework (tutorials, guides, reference, explanation) to
structure docs so both humans and AI agents can find the right content.

Getting started:

  ohara init                    Create a docs hub in your workspace
  cd ohara-docs
  ohara add ../my-service       Track a code repository
  ohara generate my-service     Analyze code and scaffold docs
  ohara build                   Generate llms.txt and agent artifacts
  ohara validate                Check structure and coverage

How it works:

  1. 'ohara init' creates a docs hub (a git repo) in your workspace
  2. 'ohara add' registers code repos you want to document
  3. 'ohara generate' analyzes each repo and creates Diataxis doc scaffolds
  4. Your AI agent fills the docs using the prompts in .ohara-prompts/
  5. 'ohara build' generates llms.txt, AGENTS.md for agent consumption
  6. Push the hub to GitHub for team collaboration

Agent integration:

  ohara init also creates:
  - .claude/skills/       Claude Code skills (search, generate, validate, PR)
  - .claude/settings.json MCP server config (ohara serve)
  - CLAUDE.md             Agent discovery file

  Run 'ohara serve' to start a local MCP server with tools:
  search_docs, list_docs, read_doc, write_doc, validate, create_pr, changelog

Learn more: https://github.com/Lackech/ohara`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringP("api-url", "", "https://api.ohara.dev", "Ohara API URL")
	rootCmd.PersistentFlags().StringP("token", "t", "", "API token for authentication")
}
