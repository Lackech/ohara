package cmd

import (
	"fmt"
	"os"
	"path/filepath"
)

func createStarterPlaybooks(hubDir string) {
	playbooksDir := filepath.Join(hubDir, ".ohara-playbooks")
	os.MkdirAll(playbooksDir, 0755)

	// fix-bug playbook
	os.WriteFile(filepath.Join(playbooksDir, "fix-bug.md"), []byte(`---
name: fix-bug
description: >-
  Investigate and fix a bug using a coordinated agent team.
  Investigates root cause, implements fix, tests, and updates docs.
pattern: sequential
phases:
  - name: investigate
    execution: subagent
    agent: Explore
    description: Find the root cause
  - name: implement
    execution: subagent
    agent: general-purpose
    isolation: worktree
    description: Implement the fix
  - name: test
    execution: subagent
    agent: general-purpose
    isolation: worktree
    description: Write and run tests
  - name: document
    execution: subagent
    agent: ohara-writer
    description: Update docs if behavior changed
review: true
---

# Bug Fix Playbook

## Context
Read the scratch space for this task: .scratch/tasks/<task-id>/

## Phase 1: Investigate
Agent: Explore (read-only, fast)

1. Read the bug description from the scratch task
2. Search the docs hub for related documentation
3. Read CHANGELOG.md for recent changes that might have caused it
4. Search the codebase for the affected area
5. Form a hypothesis
6. Write findings to .scratch/tasks/<task-id>/investigation.md:
   - Root cause hypothesis
   - Affected files
   - Related commits
   - Confidence level

## Phase 2: Implement
Agent: general-purpose (in worktree)

1. Read .scratch/tasks/<task-id>/investigation.md
2. Create a worktree branch: fix/<description>
3. Implement the minimal fix
4. Write to .scratch/tasks/<task-id>/implementation.md:
   - What was changed and why
   - Files modified
   - Any concerns or tradeoffs

## Phase 3: Test
Agent: general-purpose (in worktree)

1. Read investigation.md and implementation.md from scratch
2. Write a test that reproduces the bug (should fail without fix)
3. Verify the fix makes the test pass
4. Run the existing test suite for regressions
5. Write to .scratch/tasks/<task-id>/testing.md:
   - Test results
   - Regression check results
   - Any flaky or concerning tests

## Phase 4: Document
Agent: ohara-writer

1. Read all scratch files for this task
2. Check if the fix changes any documented behavior
3. If yes: update the relevant docs in the hub
4. Update the service CHANGELOG

## Phase 5: Review + PR
Pause for human review of:
- The fix (in worktree branch)
- Test results
- Doc changes

Then create PR with all context from the investigation.
`), 0644)

	// new-feature playbook
	os.WriteFile(filepath.Join(playbooksDir, "new-feature.md"), []byte(`---
name: new-feature
description: >-
  Implement a new feature across one or more repos. Plans the work,
  divides into parallel tasks, implements, tests, and documents.
pattern: phased
phases:
  - name: plan
    execution: subagent
    agent: general-purpose
    description: Break down the feature into tasks with file ownership
  - name: foundations
    execution: subagent
    agent: general-purpose
    isolation: worktree
    description: Shared types, interfaces, migrations (if needed)
  - name: implement
    execution: team
    agent: general-purpose
    isolation: worktree
    parallel: true
    description: Parallel implementation — one teammate per component/repo
  - name: integrate
    execution: subagent
    agent: general-purpose
    isolation: worktree
    description: Integration testing across components
  - name: document
    execution: subagent
    agent: ohara-writer
    isolation: none
    description: Write docs for the new feature
review: true
cross_repo: true
---

# New Feature Playbook

## Context
Read the scratch space: .scratch/tasks/<task-id>/

## Phase 1: Plan (single agent, no worktree)

1. Read the feature description from scratch
2. Read docs hub for all affected services
3. Read the codebase of affected repos
4. Produce a plan in .scratch/tasks/<task-id>/plan.md:
   - Feature breakdown into sub-tasks
   - Which repos are affected
   - File ownership per agent (CRITICAL: no two agents edit the same file)
   - Dependency order (what must be done first)
   - Risk assessment
5. PAUSE for human approval of the plan

## Phase 2: Foundations (sequential, worktree)

Only if the plan identifies shared dependencies:
1. Create shared types/interfaces
2. Run database migrations if needed
3. PR and merge before Phase 3
4. Write to .scratch/handoffs/foundations-ready.md

## Phase 3: Implement (parallel, worktrees)

One agent per component/repo, all in parallel:
- Each agent reads plan.md for their scope
- Each agent reads foundations handoff if applicable
- Each works in their own worktree
- Each writes status to .scratch/tasks/<task-id>/agent-<name>.md
- Each produces a branch ready for PR

File ownership from the plan is STRICT — no agent touches files
outside their assignment.

## Phase 4: Integrate (after all Phase 3 PRs)

1. Merge Phase 3 PRs in dependency order
2. Run full integration tests
3. Fix any cross-component issues

## Phase 5: Document

1. ohara-writer reads all scratch files
2. Creates or updates docs for the new feature
3. Updates CHANGELOGs
4. Rebuilds llms.txt
`), 0644)

	// investigate playbook
	os.WriteFile(filepath.Join(playbooksDir, "investigate.md"), []byte(`---
name: investigate
description: >-
  Research a problem using competing hypotheses. Multiple agents
  explore different theories in parallel, challenge each other,
  and converge on the most likely answer.
pattern: parallel-converge
phases:
  - name: hypothesize
    execution: subagent
    agent: general-purpose
    description: Form initial hypotheses
  - name: explore
    execution: team
    agent: Explore
    parallel: true
    team_size: 3
    description: Each teammate investigates one hypothesis, challenges others
  - name: converge
    execution: subagent
    agent: general-purpose
    description: Synthesize findings and determine root cause
review: false
---

# Investigation Playbook

## Context
Read the scratch space: .scratch/tasks/<task-id>/

## Phase 1: Hypothesize (single agent)

1. Read the problem description
2. Search docs and code for context
3. Form 3-5 competing hypotheses
4. Write to .scratch/tasks/<task-id>/hypotheses.md:
   - Hypothesis A: ...
   - Hypothesis B: ...
   - Hypothesis C: ...
   - Evidence needed to prove/disprove each

## Phase 2: Explore (parallel agents, one per hypothesis)

Each agent receives ONE hypothesis and must:
1. Search for evidence that SUPPORTS the hypothesis
2. Search for evidence that DISPROVES the hypothesis
3. Be rigorous — don't confirm bias
4. Write findings to .scratch/tasks/<task-id>/hypothesis-<letter>.md:
   - Supporting evidence
   - Contradicting evidence
   - Confidence level (low/medium/high)
   - What would change your mind

## Phase 3: Converge (single agent)

1. Read ALL hypothesis findings
2. Compare evidence across hypotheses
3. Determine the most likely explanation
4. Write conclusion to .scratch/tasks/<task-id>/conclusion.md:
   - Winner and why
   - Remaining uncertainty
   - Recommended next action
`), 0644)

	// review-pr playbook
	os.WriteFile(filepath.Join(playbooksDir, "review-pr.md"), []byte(`---
name: review-pr
description: >-
  Multi-perspective PR review. Parallel agents review from different
  angles: correctness, security, performance, test coverage.
pattern: parallel-converge
phases:
  - name: review
    execution: team
    agent: Explore
    parallel: true
    team_size: 3
    description: Three teammates review in parallel — correctness, security, quality
  - name: synthesize
    execution: subagent
    agent: general-purpose
    description: Consolidate all reviews into one summary with priorities
review: false
---

# PR Review Playbook

## Context
PR number or branch name in scratch: .scratch/tasks/<task-id>/

## Phase 1: Parallel Review

Three agents review simultaneously, each with a different lens:

### Agent 1: Correctness + Logic
- Does the code do what it claims?
- Edge cases handled?
- Error handling appropriate?
- Matches the requirements/issue?

### Agent 2: Security + Safety
- Input validation?
- Auth/authz correct?
- Secrets handling?
- SQL injection, XSS, etc.?
- Breaking changes to public APIs?

### Agent 3: Quality + Tests
- Test coverage for new code?
- Tests actually test the right thing?
- Code readability and maintainability?
- Performance implications?
- Documentation updated?

Each writes to .scratch/tasks/<task-id>/review-<perspective>.md

## Phase 2: Synthesize

1. Read all three reviews
2. Consolidate into a single review:
   - MUST FIX (blocking issues)
   - SHOULD FIX (important but not blocking)
   - SUGGESTIONS (nice to have)
3. Check for conflicts between reviewers
4. Final recommendation: approve, request changes, or needs discussion
`), 0644)

	fmt.Printf("✓ Created %s/.ohara-playbooks/ (4 playbooks: fix-bug, new-feature, investigate, review-pr)\n", filepath.Base(hubDir))
}
