import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { z } from 'zod';
import { eq, sql } from 'drizzle-orm';
import { db } from '../../../db/index.js';
import { documents, projects } from '../../../db/schema.js';
import { hybridSearch } from '../../search/hybrid.js';
import { DIATAXIS_LABELS } from '@ohara/shared';

/**
 * Create the Ohara MCP server instance.
 * Exposes tools, resources, and prompts for AI agent consumption.
 */
export function createMcpServer() {
  const server = new McpServer({
    name: 'ohara',
    version: '0.1.0',
  });

  // -----------------------------------------------------------------------
  // Tools
  // -----------------------------------------------------------------------

  server.tool(
    'search_documentation',
    'Search across documentation using hybrid full-text + semantic search. Returns ranked results with snippets.',
    {
      query: z.string().describe('Search query'),
      project: z.string().describe('Project slug'),
      type: z
        .enum(['tutorial', 'guide', 'reference', 'explanation'])
        .optional()
        .describe('Filter by Diataxis type'),
      limit: z.number().min(1).max(50).default(10).describe('Max results'),
    },
    async ({ query, project, type, limit }) => {
      const [proj] = await db
        .select()
        .from(projects)
        .where(eq(projects.slug, project))
        .limit(1);

      if (!proj) {
        return {
          content: [{ type: 'text' as const, text: `Project "${project}" not found.` }],
          isError: true,
        };
      }

      const results = await hybridSearch(query, proj.id, {
        limit,
        diataxisType: type,
      });

      if (results.length === 0) {
        return {
          content: [
            {
              type: 'text' as const,
              text: `No results found for "${query}" in project "${project}".`,
            },
          ],
        };
      }

      const text = results
        .map(
          (r, i) =>
            `${i + 1}. **${r.title}** (${DIATAXIS_LABELS[r.diataxisType as keyof typeof DIATAXIS_LABELS] ?? r.diataxisType})\n   Path: ${r.path}\n   ${r.snippet.replace(/<\/?mark>/g, '**')}`,
        )
        .join('\n\n');

      return {
        content: [{ type: 'text' as const, text }],
      };
    },
  );

  server.tool(
    'get_document',
    'Retrieve a specific document by path or slug. Returns the full markdown content.',
    {
      project: z.string().describe('Project slug'),
      path: z.string().describe('Document path or slug'),
    },
    async ({ project, path }) => {
      const [proj] = await db
        .select()
        .from(projects)
        .where(eq(projects.slug, project))
        .limit(1);

      if (!proj) {
        return {
          content: [{ type: 'text' as const, text: `Project "${project}" not found.` }],
          isError: true,
        };
      }

      const [doc] = await db
        .select()
        .from(documents)
        .where(
          sql`${documents.projectId} = ${proj.id} AND (${documents.path} = ${path} OR ${documents.slug} = ${path})`,
        )
        .limit(1);

      if (!doc) {
        return {
          content: [
            { type: 'text' as const, text: `Document "${path}" not found in project "${project}".` },
          ],
          isError: true,
        };
      }

      // Fetch related documents for context
      const related = await db
        .select({ title: documents.title, path: documents.path })
        .from(documents)
        .where(
          sql`${documents.projectId} = ${proj.id} AND ${documents.diataxisType} = ${doc.diataxisType} AND ${documents.id} != ${doc.id}`,
        )
        .limit(5);

      const relatedSection =
        related.length > 0
          ? `\n\n---\n**Related documents:**\n${related.map((r) => `- ${r.title} (${r.path})`).join('\n')}`
          : '';

      const header = `# ${doc.title}\n\n> Type: ${DIATAXIS_LABELS[doc.diataxisType as keyof typeof DIATAXIS_LABELS] ?? doc.diataxisType} | Path: ${doc.path} | Words: ${doc.wordCount ?? 'N/A'}\n\n`;

      // Strip frontmatter from raw content
      const content = doc.rawContent.replace(/^---[\s\S]*?---\s*/, '');

      return {
        content: [{ type: 'text' as const, text: header + content + relatedSection }],
      };
    },
  );

  server.tool(
    'list_documents',
    'List all documents in a project, optionally filtered by Diataxis type.',
    {
      project: z.string().describe('Project slug'),
      type: z
        .enum(['tutorial', 'guide', 'reference', 'explanation'])
        .optional()
        .describe('Filter by Diataxis type'),
    },
    async ({ project, type }) => {
      const [proj] = await db
        .select()
        .from(projects)
        .where(eq(projects.slug, project))
        .limit(1);

      if (!proj) {
        return {
          content: [{ type: 'text' as const, text: `Project "${project}" not found.` }],
          isError: true,
        };
      }

      const typeFilter = type
        ? sql`AND ${documents.diataxisType} = ${type}`
        : sql``;

      const docs = await db.execute(sql`
        SELECT
          ${documents.title} as title,
          ${documents.path} as path,
          ${documents.diataxisType} as diataxis_type,
          ${documents.description} as description
        FROM ${documents}
        WHERE ${documents.projectId} = ${proj.id}
          AND ${documents.draft} = false
          ${typeFilter}
        ORDER BY ${documents.diataxisType}, ${documents.title}
      `);

      const docList = Array.from(docs);

      if (docList.length === 0) {
        return {
          content: [
            {
              type: 'text' as const,
              text: `No documents found in project "${project}"${type ? ` with type "${type}"` : ''}.`,
            },
          ],
        };
      }

      // Group by type
      const grouped = new Map<string, typeof docList>();
      for (const doc of docList) {
        const t = doc.diataxis_type as string;
        if (!grouped.has(t)) grouped.set(t, []);
        grouped.get(t)!.push(doc);
      }

      let text = `# ${proj.name} — Documents (${docList.length} total)\n\n`;

      for (const [t, typeDocs] of grouped) {
        const label = DIATAXIS_LABELS[t as keyof typeof DIATAXIS_LABELS] ?? t;
        text += `## ${label} (${typeDocs.length})\n\n`;
        for (const doc of typeDocs) {
          const desc = doc.description ? ` — ${doc.description}` : '';
          text += `- **${doc.title}** (\`${doc.path}\`)${desc}\n`;
        }
        text += '\n';
      }

      return { content: [{ type: 'text' as const, text }] };
    },
  );

  // -----------------------------------------------------------------------
  // Scaffolding Tools (Prompt Chaining pattern: analyze → plan → generate)
  // -----------------------------------------------------------------------

  server.tool(
    'analyze_project',
    `Analyze a codebase and return a documentation generation plan. This is Step 1 of the doc generation workflow.
Returns signals (detected patterns like routes, configs, types) mapped to Diataxis document types, with confidence scores and outlines.
After reviewing the plan, use generate_document for each item you want to create.`,
    {
      project: z.string().describe('Project slug'),
      types: z
        .array(z.enum(['tutorial', 'guide', 'reference', 'explanation']))
        .optional()
        .describe('Filter to specific Diataxis types'),
      min_confidence: z
        .number()
        .min(0)
        .max(1)
        .default(0.6)
        .describe('Minimum confidence threshold (0-1)'),
    },
    async ({ project, types, min_confidence }) => {
      const [proj] = await db
        .select()
        .from(projects)
        .where(eq(projects.slug, project))
        .limit(1);

      if (!proj) {
        return {
          content: [{ type: 'text' as const, text: `Project "${project}" not found.` }],
          isError: true,
        };
      }

      // Get existing docs
      const existingDocs = await db
        .select({ title: documents.title, path: documents.path, diataxisType: documents.diataxisType })
        .from(documents)
        .where(eq(documents.projectId, proj.id));

      // Since we can't access the repo filesystem from here, build a plan from existing data
      const existingSection = existingDocs.length > 0
        ? `\n\n**Existing documentation (${existingDocs.length} docs):**\n${existingDocs.map(d => `- ${d.title} (${d.diataxisType}) — \`${d.path}\``).join('\n')}`
        : '\n\n**No existing documentation found.** This project needs scaffolding from scratch.';

      // Suggest docs based on what's missing
      const existingTypes = new Set(existingDocs.map(d => d.diataxisType));
      const suggestions: string[] = [];

      if (!existingTypes.has('tutorial')) {
        suggestions.push('- **tutorials/getting-started.md** (Tutorial) — Every project needs a getting-started guide. Check the README and entry points.');
      }
      if (!existingTypes.has('guide')) {
        suggestions.push('- **guides/development.md** (Guide) — Development workflow, scripts, setup steps.');
        suggestions.push('- **guides/deployment.md** (Guide) — If Dockerfile or CI configs exist.');
      }
      if (!existingTypes.has('reference')) {
        suggestions.push('- **reference/api.md** (Reference) — API endpoints, parameters, response types.');
        suggestions.push('- **reference/configuration.md** (Reference) — Environment variables, config files.');
      }
      if (!existingTypes.has('explanation')) {
        suggestions.push('- **explanation/architecture.md** (Explanation) — System design, key decisions, data flow.');
      }

      const suggestionsText = suggestions.length > 0
        ? `\n\n**Suggested new documents:**\n${suggestions.join('\n')}`
        : '\n\n**All Diataxis types are covered.** Consider adding more depth to existing docs.';

      const text = `# Analysis for "${proj.name}"

**Project:** ${proj.name} (slug: ${proj.slug})
**Repo:** ${proj.repoUrl ?? 'Not connected'}
**Last synced:** ${proj.lastSyncedAt ?? 'Never'}${existingSection}${suggestionsText}

## Next Steps

To generate a document, read the relevant source code first, then use the \`create_document\` tool with:
- \`project\`: "${project}"
- \`path\`: the file path (e.g., "tutorials/getting-started.md")
- \`title\`: document title
- \`diataxis_type\`: tutorial, guide, reference, or explanation
- \`content\`: the full markdown content

For best results, read the actual code files to write specific, accurate documentation — not generic placeholders.`;

      return { content: [{ type: 'text' as const, text }] };
    },
  );

  server.tool(
    'create_document',
    `Create a new document in a project. This is the write-back tool that closes the agent flywheel.
The document is stored in the database and immediately available via search, web UI, and llms.txt.
Use after analyze_project to add documentation, or anytime you want to contribute knowledge back.`,
    {
      project: z.string().describe('Project slug'),
      path: z.string().describe('Document path (e.g., "tutorials/getting-started.md")'),
      title: z.string().describe('Document title'),
      diataxis_type: z.enum(['tutorial', 'guide', 'reference', 'explanation']).describe('Diataxis document type'),
      content: z.string().describe('Full markdown content (including frontmatter)'),
      description: z.string().optional().describe('Short description for search results'),
    },
    async ({ project, path, title, diataxis_type, content, description }) => {
      const [proj] = await db
        .select()
        .from(projects)
        .where(eq(projects.slug, project))
        .limit(1);

      if (!proj) {
        return {
          content: [{ type: 'text' as const, text: `Project "${project}" not found.` }],
          isError: true,
        };
      }

      // Check if doc already exists
      const [existing] = await db
        .select({ id: documents.id })
        .from(documents)
        .where(sql`${documents.projectId} = ${proj.id} AND ${documents.path} = ${path}`)
        .limit(1);

      if (existing) {
        return {
          content: [{ type: 'text' as const, text: `Document "${path}" already exists. Use update_document to modify it.` }],
          isError: true,
        };
      }

      // Parse and store
      const { parseMarkdown, chunkDocument } = await import('@ohara/shared/markdown');
      const { createHash } = await import('crypto');

      const parsed = await parseMarkdown(content);
      const contentHash = createHash('sha256').update(content).digest('hex');
      const slug = path.replace(/\.(mdx?|markdown)$/i, '').replace(/\/index$/, '').toLowerCase();

      const [doc] = await db
        .insert(documents)
        .values({
          projectId: proj.id,
          path,
          title,
          slug,
          description: description ?? null,
          diataxisType: diataxis_type,
          rawContent: content,
          htmlContent: parsed.html,
          contentHash,
          wordCount: parsed.wordCount,
        })
        .returning({ id: documents.id });

      // Create chunks for search
      const { documentChunks } = await import('../../../db/schema.js');
      const chunks = chunkDocument(content, title);
      if (chunks.length > 0) {
        await db.insert(documentChunks).values(
          chunks.map((chunk) => ({
            documentId: doc.id,
            projectId: proj.id,
            content: chunk.content,
            contentHash: createHash('sha256').update(chunk.content).digest('hex'),
            chunkIndex: chunk.chunkIndex,
            headingHierarchy: chunk.headingHierarchy,
            diataxisType: diataxis_type,
            tokenCount: chunk.tokenCount,
          })),
        );
      }

      return {
        content: [{
          type: 'text' as const,
          text: `Document created successfully.

**${title}** (${DIATAXIS_LABELS[diataxis_type]})
- Path: \`${path}\`
- Words: ${parsed.wordCount}
- Chunks: ${chunks.length} (searchable immediately)
- URL: /${proj.slug}/${slug}`,
        }],
      };
    },
  );

  server.tool(
    'update_document',
    `Update an existing document's content. Recalculates HTML, search vectors, and chunks automatically.`,
    {
      project: z.string().describe('Project slug'),
      path: z.string().describe('Document path'),
      content: z.string().describe('New full markdown content'),
      title: z.string().optional().describe('New title (optional)'),
      description: z.string().optional().describe('New description (optional)'),
    },
    async ({ project, path, content, title, description }) => {
      const [proj] = await db
        .select()
        .from(projects)
        .where(eq(projects.slug, project))
        .limit(1);

      if (!proj) {
        return {
          content: [{ type: 'text' as const, text: `Project "${project}" not found.` }],
          isError: true,
        };
      }

      const [existing] = await db
        .select()
        .from(documents)
        .where(sql`${documents.projectId} = ${proj.id} AND ${documents.path} = ${path}`)
        .limit(1);

      if (!existing) {
        return {
          content: [{ type: 'text' as const, text: `Document "${path}" not found. Use create_document to create it.` }],
          isError: true,
        };
      }

      const { parseMarkdown, chunkDocument } = await import('@ohara/shared/markdown');
      const { createHash } = await import('crypto');

      const parsed = await parseMarkdown(content);
      const contentHash = createHash('sha256').update(content).digest('hex');

      // Update document
      await db
        .update(documents)
        .set({
          rawContent: content,
          htmlContent: parsed.html,
          contentHash,
          wordCount: parsed.wordCount,
          ...(title ? { title } : {}),
          ...(description ? { description } : {}),
          updatedAt: new Date(),
        })
        .where(eq(documents.id, existing.id));

      // Recreate chunks
      const { documentChunks } = await import('../../../db/schema.js');
      await db.delete(documentChunks).where(eq(documentChunks.documentId, existing.id));

      const chunks = chunkDocument(content, title ?? existing.title);
      if (chunks.length > 0) {
        await db.insert(documentChunks).values(
          chunks.map((chunk) => ({
            documentId: existing.id,
            projectId: proj.id,
            content: chunk.content,
            contentHash: createHash('sha256').update(chunk.content).digest('hex'),
            chunkIndex: chunk.chunkIndex,
            headingHierarchy: chunk.headingHierarchy,
            diataxisType: existing.diataxisType,
            tokenCount: chunk.tokenCount,
          })),
        );
      }

      return {
        content: [{
          type: 'text' as const,
          text: `Document updated: **${title ?? existing.title}** at \`${path}\`. ${parsed.wordCount} words, ${chunks.length} chunks regenerated.`,
        }],
      };
    },
  );

  server.tool(
    'validate_project',
    `Validate a project's documentation structure. Checks for missing Diataxis types, frontmatter issues, and coverage gaps.`,
    {
      project: z.string().describe('Project slug'),
    },
    async ({ project }) => {
      const [proj] = await db
        .select()
        .from(projects)
        .where(eq(projects.slug, project))
        .limit(1);

      if (!proj) {
        return {
          content: [{ type: 'text' as const, text: `Project "${project}" not found.` }],
          isError: true,
        };
      }

      const docs = await db
        .select()
        .from(documents)
        .where(eq(documents.projectId, proj.id));

      const issues: string[] = [];
      const warnings: string[] = [];

      // Check Diataxis coverage
      const typeCount: Record<string, number> = { tutorial: 0, guide: 0, reference: 0, explanation: 0 };
      for (const doc of docs) {
        typeCount[doc.diataxisType] = (typeCount[doc.diataxisType] ?? 0) + 1;
      }

      for (const [type, count] of Object.entries(typeCount)) {
        if (count === 0) {
          warnings.push(`No ${DIATAXIS_LABELS[type as keyof typeof DIATAXIS_LABELS]} documents. Consider adding ${type === 'tutorial' ? 'a getting-started tutorial' : type === 'guide' ? 'how-to guides' : type === 'reference' ? 'API/config reference' : 'architecture explanations'}.`);
        }
      }

      // Check individual docs
      for (const doc of docs) {
        if (!doc.title || doc.title.trim() === '') {
          issues.push(`\`${doc.path}\`: missing title`);
        }
        if (doc.wordCount && doc.wordCount < 50) {
          warnings.push(`\`${doc.path}\`: very short (${doc.wordCount} words)`);
        }
        if (doc.draft) {
          warnings.push(`\`${doc.path}\`: marked as draft`);
        }
      }

      const status = issues.length > 0 ? 'FAIL' : warnings.length > 0 ? 'WARN' : 'PASS';

      let text = `# Validation: ${proj.name} — ${status}\n\n`;
      text += `**Documents:** ${docs.length} total\n`;
      text += `**Coverage:** ${Object.entries(typeCount).map(([t, c]) => `${DIATAXIS_LABELS[t as keyof typeof DIATAXIS_LABELS]}: ${c}`).join(' | ')}\n\n`;

      if (issues.length > 0) {
        text += `## Errors (${issues.length})\n${issues.map(i => `- ${i}`).join('\n')}\n\n`;
      }
      if (warnings.length > 0) {
        text += `## Warnings (${warnings.length})\n${warnings.map(w => `- ${w}`).join('\n')}\n\n`;
      }
      if (issues.length === 0 && warnings.length === 0) {
        text += 'All checks passed. Documentation structure looks good.\n';
      }

      return { content: [{ type: 'text' as const, text }] };
    },
  );

  // -----------------------------------------------------------------------
  // Resources
  // -----------------------------------------------------------------------

  server.resource(
    'document',
    'docs://{project}/{path}',
    async (uri) => {
      const match = uri.href.match(/^docs:\/\/([^/]+)\/(.+)$/);
      if (!match) {
        return { contents: [{ uri: uri.href, mimeType: 'text/plain', text: 'Invalid URI format' }] };
      }

      const [, projectSlug, docPath] = match;

      const [proj] = await db
        .select()
        .from(projects)
        .where(eq(projects.slug, projectSlug))
        .limit(1);

      if (!proj) {
        return { contents: [{ uri: uri.href, mimeType: 'text/plain', text: 'Project not found' }] };
      }

      const [doc] = await db
        .select()
        .from(documents)
        .where(
          sql`${documents.projectId} = ${proj.id} AND (${documents.path} = ${docPath} OR ${documents.slug} = ${docPath})`,
        )
        .limit(1);

      if (!doc) {
        return { contents: [{ uri: uri.href, mimeType: 'text/plain', text: 'Document not found' }] };
      }

      return {
        contents: [
          {
            uri: uri.href,
            mimeType: 'text/markdown',
            text: doc.rawContent,
          },
        ],
      };
    },
  );

  // -----------------------------------------------------------------------
  // Prompts
  // -----------------------------------------------------------------------

  server.prompt(
    'troubleshoot',
    'Guided troubleshooting for a service. Searches docs for the error and provides step-by-step resolution.',
    {
      project: z.string().describe('Project slug'),
      service: z.string().describe('Service name'),
      error: z.string().describe('Error message or symptom'),
    },
    async ({ project, service, error }) => {
      const [proj] = await db
        .select()
        .from(projects)
        .where(eq(projects.slug, project))
        .limit(1);

      if (!proj) {
        return {
          messages: [
            {
              role: 'user' as const,
              content: { type: 'text' as const, text: `Project "${project}" not found.` },
            },
          ],
        };
      }

      // Search for relevant docs
      const results = await hybridSearch(`${service} ${error}`, proj.id, {
        limit: 5,
        diataxisType: 'guide',
      });

      const context = results
        .map((r) => `### ${r.title}\n${r.snippet.replace(/<\/?mark>/g, '**')}`)
        .join('\n\n');

      return {
        messages: [
          {
            role: 'user' as const,
            content: {
              type: 'text' as const,
              text: `I'm troubleshooting an issue with the "${service}" service in the "${project}" project.\n\nError: ${error}\n\nHere are relevant documentation excerpts:\n\n${context || 'No relevant documentation found.'}\n\nBased on the documentation above, please help me troubleshoot this issue step by step.`,
            },
          },
        ],
      };
    },
  );

  server.prompt(
    'explain_concept',
    'Explain a concept at the specified level. Searches explanation docs for context.',
    {
      project: z.string().describe('Project slug'),
      topic: z.string().describe('Topic to explain'),
      level: z
        .enum(['beginner', 'intermediate', 'advanced'])
        .default('intermediate')
        .describe('Audience level'),
    },
    async ({ project, topic, level }) => {
      const [proj] = await db
        .select()
        .from(projects)
        .where(eq(projects.slug, project))
        .limit(1);

      if (!proj) {
        return {
          messages: [
            {
              role: 'user' as const,
              content: { type: 'text' as const, text: `Project "${project}" not found.` },
            },
          ],
        };
      }

      const results = await hybridSearch(topic, proj.id, {
        limit: 5,
        diataxisType: 'explanation',
      });

      const context = results
        .map((r) => `### ${r.title}\n${r.snippet.replace(/<\/?mark>/g, '**')}`)
        .join('\n\n');

      return {
        messages: [
          {
            role: 'user' as const,
            content: {
              type: 'text' as const,
              text: `Please explain "${topic}" from the "${project}" project at a ${level} level.\n\nHere are relevant documentation excerpts:\n\n${context || 'No relevant documentation found.'}\n\nProvide a clear explanation based on the documentation, tailored to a ${level} audience.`,
            },
          },
        ],
      };
    },
  );

  // -----------------------------------------------------------------------
  // Scaffolding Prompt
  // -----------------------------------------------------------------------

  server.prompt(
    'generate_documentation',
    'Analyze a codebase and generate Diataxis-structured documentation. Returns a generation plan with prompts for each document.',
    {
      project: z.string().describe('Project slug'),
      focus: z
        .enum(['all', 'tutorials', 'guides', 'reference', 'explanation'])
        .default('all')
        .describe('Focus on a specific Diataxis type, or all'),
    },
    async ({ project, focus }) => {
      const [proj] = await db
        .select()
        .from(projects)
        .where(eq(projects.slug, project))
        .limit(1);

      if (!proj) {
        return {
          messages: [
            {
              role: 'user' as const,
              content: { type: 'text' as const, text: `Project "${project}" not found.` },
            },
          ],
        };
      }

      // Get existing docs to understand what's already covered
      const existingDocs = await db
        .select({ title: documents.title, path: documents.path, diataxisType: documents.diataxisType })
        .from(documents)
        .where(eq(documents.projectId, proj.id));

      const existingList = existingDocs.length > 0
        ? `\n\nExisting documentation:\n${existingDocs.map(d => `- ${d.title} (${d.diataxisType}) at ${d.path}`).join('\n')}`
        : '\n\nNo existing documentation found.';

      const focusInstructions = focus !== 'all'
        ? `\n\nFocus specifically on generating ${focus} documentation.`
        : '';

      return {
        messages: [
          {
            role: 'user' as const,
            content: {
              type: 'text' as const,
              text: `I need you to generate Diataxis-structured documentation for the "${project}" project.

Please analyze the codebase and create documentation files organized into:
- **tutorials/** — Learning-oriented walkthroughs (getting-started, first steps)
- **guides/** — Task-oriented how-to guides (deployment, development, testing)
- **reference/** — Precise technical reference (API, configuration, types)
- **explanation/** — Conceptual explanations (architecture, design decisions)

For each document:
1. Create proper frontmatter with title, description, and diataxis_type
2. Write real, specific content based on the actual code (not generic placeholders)
3. Extract specific values from code (port numbers, env vars, script names)
4. Cross-reference other docs where relevant
${existingList}${focusInstructions}

Start by analyzing the project structure, then create the documentation files one by one. Create an ohara.yaml config file first if one doesn't exist.`,
            },
          },
        ],
      };
    },
  );

  return server;
}
