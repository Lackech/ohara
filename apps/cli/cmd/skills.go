package cmd

import (
	"fmt"
	"os"
	"path/filepath"
)

// createOharaSkills generates .claude/skills/ in the workspace root
func createOharaSkills(workDir, hubName string) {
	skillsDir := filepath.Join(workDir, ".claude", "skills")

	// 1. search-docs — Claude auto-invokes when user asks about docs
	createSkill(skillsDir, "search-docs", fmt.Sprintf(`---
name: search-docs
description: Search documentation in the Ohara hub. Use when the user asks about how something works, looks for documentation, or needs to find information across services.
argument-hint: <query>
---

Search the documentation hub at %s/ for information matching the user's query.

## Steps

1. Read %s/llms.txt for the doc index
2. Use Grep to search: grep -ri "$ARGUMENTS" %s/ --include="*.md" -l
3. Read the top matching files
4. Summarize findings with specific references to the docs
`, hubName, hubName, hubName))

	// 2. generate-docs — user invokes to generate docs from code
	createSkill(skillsDir, "generate-docs", fmt.Sprintf(`---
name: generate-docs
description: Generate Diataxis documentation for a service by analyzing its source code. Creates tutorials, guides, references, and explanations.
argument-hint: <service-name>
disable-model-invocation: true
---

Generate documentation for the $ARGUMENTS service.

## Steps

1. Run: cd %s && ohara generate $ARGUMENTS
2. Read each prompt file in %s/$ARGUMENTS/.ohara-prompts/
3. For EACH prompt:
   a. Read the prompt to understand what document to write
   b. Read the actual source code from $ARGUMENTS/ (the code repo, sibling directory)
   c. Write specific, accurate documentation based on the real code
   d. Write it to the corresponding path in %s/$ARGUMENTS/
   e. Include proper frontmatter (title, description, diataxis_type)
4. Run: cd %s && ohara build
5. Run: cd %s && ohara validate

IMPORTANT: Write real documentation from the actual code. Do NOT write generic placeholders.
`, hubName, hubName, hubName, hubName, hubName))

	// 3. validate-docs — Claude auto-invokes to check quality
	createSkill(skillsDir, "validate-docs", fmt.Sprintf(`---
name: validate-docs
description: Validate documentation structure, coverage, and quality. Use when checking if docs are complete or after generating docs.
---

Validate the documentation hub.

## Steps

1. Run: cd %s && ohara validate
2. Review the output for errors and warnings
3. For TODO placeholders: read the prompt in .ohara-prompts/ and generate real content
4. For missing Diataxis types: suggest what docs should be created
5. Report the validation result
`, hubName))

	// 4. create-docs-pr — user invokes to create a PR
	createSkill(skillsDir, "create-docs-pr", fmt.Sprintf(`---
name: create-docs-pr
description: Create a pull request with documentation changes on the docs hub repo.
argument-hint: <description>
disable-model-invocation: true
---

Create a PR for documentation changes.

## Steps

1. cd %s
2. Run: ohara build  (regenerate llms.txt and AGENTS.md)
3. Run: ohara validate  (check for issues)
4. git checkout -b docs/$ARGUMENTS
5. git add -A
6. git commit -m "docs: $ARGUMENTS"
7. git push origin docs/$ARGUMENTS
8. gh pr create --title "docs: $ARGUMENTS" --body "Documentation update by Ohara agent."
9. Report the PR URL
`, hubName))

	// 5. docs-changelog — Claude auto-invokes to check recent changes
	createSkill(skillsDir, "docs-changelog", fmt.Sprintf(`---
name: docs-changelog
description: Show recent documentation changes. Use when the user asks what changed in docs, or to understand recent updates.
argument-hint: "[service-name]"
---

Show recent documentation changes.

## Steps

1. cd %s
2. If $ARGUMENTS is provided (a service name):
   git log --oneline -20 -- $ARGUMENTS/
3. Otherwise show all recent changes:
   git log --oneline -20
4. For important changes, show what changed:
   git show <commit-hash> --stat
5. Summarize: what changed, when, and why
`, hubName))
}

func createSkill(skillsDir, name, content string) {
	dir := filepath.Join(skillsDir, name)
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644)
}
