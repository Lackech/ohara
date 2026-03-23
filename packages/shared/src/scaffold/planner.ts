import type { AnalysisResult, Signal } from './analyzer';

/**
 * A generation plan for a single document.
 */
export type DocPlan = {
  path: string;           // e.g., "guides/deployment.md"
  title: string;
  diataxisType: 'tutorial' | 'guide' | 'reference' | 'explanation';
  confidence: number;
  outline: string[];      // Section headings
  contextFiles: string[]; // Files to include as context for LLM
  prompt: string;         // The prompt to send to the LLM
};

/**
 * A complete generation plan for a project.
 */
export type GenerationPlan = {
  projectName: string;
  oharaYaml: string;
  docs: DocPlan[];
  summary: string;
  totalDocs: number;
  estimatedTokens: number;
};

/**
 * Convert analysis results into a structured generation plan.
 * This is deterministic — no LLM needed.
 */
export function createGenerationPlan(
  analysis: AnalysisResult,
  options: {
    minConfidence?: number;
    types?: string[];
    maxDocs?: number;
  } = {},
): GenerationPlan {
  const minConfidence = options.minConfidence ?? 0.6;
  const maxDocs = options.maxDocs ?? 10;

  let signals = analysis.signals.filter((s) => s.confidence >= minConfidence);

  if (options.types?.length) {
    signals = signals.filter((s) => options.types!.includes(s.type));
  }

  signals = signals.slice(0, maxDocs);

  const docs: DocPlan[] = signals.map((signal) => buildDocPlan(signal, analysis));

  // Generate ohara.yaml
  const oharaYaml = buildOharaYaml(analysis);

  // Estimate total tokens for LLM calls
  const estimatedTokens = docs.reduce((sum, d) => {
    const contextSize = d.contextFiles.reduce((s, f) => {
      const content = analysis.keyFiles[f] ?? analysis.codeSnippets[f] ?? '';
      return s + Math.ceil(content.length / 4);
    }, 0);
    return sum + contextSize + Math.ceil(d.prompt.length / 4) + 800; // ~800 output tokens per doc
  }, 0);

  return {
    projectName: analysis.projectName,
    oharaYaml,
    docs,
    summary: `Plan: generate ${docs.length} documents for "${analysis.projectName}" (${analysis.language}). ` +
      `Types: ${[...new Set(docs.map((d) => d.diataxisType))].join(', ')}. ` +
      `Estimated LLM tokens: ~${estimatedTokens.toLocaleString()}.`,
    totalDocs: docs.length,
    estimatedTokens,
  };
}

function buildDocPlan(signal: Signal, analysis: AnalysisResult): DocPlan {
  const dirMap = { tutorial: 'tutorials', guide: 'guides', reference: 'reference', explanation: 'explanation' };
  const dir = dirMap[signal.type];
  const path = `${dir}/${signal.docName}.md`;

  const outline = getOutline(signal);
  const contextFiles = signal.context;

  // Build a context-rich prompt for the LLM
  const prompt = buildPrompt(signal, analysis, outline);

  return {
    path,
    title: signal.title,
    diataxisType: signal.type,
    confidence: signal.confidence,
    outline,
    contextFiles,
    prompt,
  };
}

function getOutline(signal: Signal): string[] {
  switch (signal.docName) {
    case 'getting-started':
      return ['Prerequisites', 'Installation', 'Quick Start', 'Project Structure', 'Next Steps'];
    case 'development':
      return ['Prerequisites', 'Setup', 'Available Scripts', 'Development Workflow', 'Common Tasks'];
    case 'deployment':
      return ['Prerequisites', 'Build', 'Deploy', 'Environment Variables', 'Health Checks'];
    case 'configuration':
      return ['Overview', 'Environment Variables', 'Configuration Files', 'Defaults'];
    case 'api-reference':
      return ['Base URL', 'Authentication', 'Endpoints', 'Error Handling'];
    case 'types':
      return ['Core Types', 'Request/Response Types', 'Database Models'];
    case 'ci-cd':
      return ['Pipeline Overview', 'Stages', 'Configuration', 'Secrets'];
    case 'architecture':
      return ['Overview', 'Key Components', 'Data Flow', 'Design Decisions'];
    case 'testing':
      return ['Test Structure', 'Running Tests', 'Writing Tests', 'CI Integration'];
    default:
      return ['Overview', 'Details', 'Examples'];
  }
}

function buildPrompt(signal: Signal, analysis: AnalysisResult, outline: string[]): string {
  const typeGuidance = {
    tutorial: 'Write a learning-oriented tutorial. Walk the reader through a complete experience step by step. Use a friendly, encouraging tone. Every step should produce a visible result.',
    guide: 'Write a task-oriented how-to guide. Be concise and practical. Start with the goal, list prerequisites, then provide step-by-step instructions. No background explanations.',
    reference: 'Write precise, information-oriented reference documentation. Be exhaustive and accurate. Use tables for parameters/options. No tutorials or opinions — just facts.',
    explanation: 'Write an understanding-oriented explanation. Discuss the "why" behind design decisions. Connect concepts to each other. Help the reader build a mental model.',
  };

  const sections = outline.map((h) => `## ${h}`).join('\n\n');

  let contextBlock = '';
  for (const file of signal.context) {
    const content = analysis.keyFiles[file] ?? analysis.codeSnippets[file];
    if (content) {
      contextBlock += `\n### File: ${file}\n\`\`\`\n${content.slice(0, 2000)}\n\`\`\`\n`;
    }
  }

  return `You are generating documentation for the project "${analysis.projectName}" (${analysis.language}${analysis.framework ? '/' + analysis.framework : ''}).

${typeGuidance[signal.type]}

Generate a complete markdown document with the following structure:

---
title: ${signal.title}
description: [write a one-line description]
diataxis_type: ${signal.type}
---

# ${signal.title}

${sections}

Use the following project context to write accurate, specific documentation:

**Project summary:** ${analysis.summary}

**Relevant files:**
${contextBlock || 'No specific files extracted. Use the project summary and file tree to infer content.'}

**File tree (top-level):**
${analysis.fileTree.slice(0, 30).join('\n')}${analysis.fileTree.length > 30 ? `\n... and ${analysis.fileTree.length - 30} more files` : ''}

Important:
- Write REAL documentation based on the actual code context, not generic placeholders
- If you can extract specific values (port numbers, script names, env vars), use them
- Cross-reference other docs in the project where relevant
- Keep it concise — developers scan, they don't read novels`;
}

function buildOharaYaml(analysis: AnalysisResult): string {
  return `name: ${analysis.projectName}
description: ""
docs_dir: "."

directories:
  tutorials: tutorials
  guides: guides
  reference: reference
  explanation: explanation

include:
  - "**/*.md"
  - "**/*.mdx"

exclude:
  - node_modules/**
  - .git/**
  - dist/**
`;
}
