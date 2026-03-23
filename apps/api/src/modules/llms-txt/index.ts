import { Elysia, t } from 'elysia';
import { eq, sql } from 'drizzle-orm';
import { db } from '../../db/index.js';
import { documents, projects } from '../../db/schema.js';
import { DIATAXIS_LABELS } from '@ohara/shared';
import { cacheGet, cacheSet } from '../../lib/cache.js';

/**
 * llms.txt endpoint — Diataxis-typed format for LLM consumption.
 * Sections per Diataxis type, with links and descriptions.
 */
export const llmsTxtModule = new Elysia({ name: 'llms-txt' })

  // Standard llms.txt (links + descriptions)
  .get(
    '/:projectSlug/llms.txt',
    async ({ params, set }) => {
      const [proj] = await db
        .select()
        .from(projects)
        .where(eq(projects.slug, params.projectSlug))
        .limit(1);

      if (!proj) {
        set.status = 404;
        return 'Project not found';
      }

      const docs = await db
        .select()
        .from(documents)
        .where(
          sql`${documents.projectId} = ${proj.id} AND ${documents.draft} = false`,
        )
        .orderBy(documents.diataxisType, documents.order, documents.title);

      const baseUrl = proj.settings && typeof proj.settings === 'object' && 'base_url' in proj.settings
        ? (proj.settings as Record<string, string>).base_url
        : `https://docs.ohara.dev/${proj.slug}`;

      set.headers['content-type'] = 'text/plain; charset=utf-8';
      set.headers['cache-control'] = 'public, max-age=3600';

      return generateLlmsTxt(proj, docs, baseUrl);
    },
    {
      params: t.Object({ projectSlug: t.String() }),
    },
  )

  // llms-full.txt (full inline content)
  .get(
    '/:projectSlug/llms-full.txt',
    async ({ params, set }) => {
      const [proj] = await db
        .select()
        .from(projects)
        .where(eq(projects.slug, params.projectSlug))
        .limit(1);

      if (!proj) {
        set.status = 404;
        return 'Project not found';
      }

      const docs = await db
        .select()
        .from(documents)
        .where(
          sql`${documents.projectId} = ${proj.id} AND ${documents.draft} = false`,
        )
        .orderBy(documents.diataxisType, documents.order, documents.title);

      set.headers['content-type'] = 'text/plain; charset=utf-8';
      set.headers['cache-control'] = 'public, max-age=3600';

      return generateLlmsFullTxt(proj, docs);
    },
    {
      params: t.Object({ projectSlug: t.String() }),
    },
  );

type Project = typeof projects.$inferSelect;
type Document = typeof documents.$inferSelect;

function generateLlmsTxt(proj: Project, docs: Document[], baseUrl: string): string {
  const lines: string[] = [];

  lines.push(`# ${proj.name}`);
  lines.push('');
  if (proj.description) {
    lines.push(`> ${proj.description}`);
    lines.push('');
  }

  const typeOrder = ['tutorial', 'guide', 'reference', 'explanation'] as const;
  const grouped = new Map<string, Document[]>();
  for (const type of typeOrder) grouped.set(type, []);
  for (const doc of docs) {
    grouped.get(doc.diataxisType)?.push(doc);
  }

  for (const type of typeOrder) {
    const typeDocs = grouped.get(type) ?? [];
    if (typeDocs.length === 0) continue;

    lines.push(`## ${DIATAXIS_LABELS[type]}`);
    lines.push('');
    for (const doc of typeDocs) {
      const desc = doc.description ? `: ${doc.description}` : '';
      lines.push(`- [${doc.title}](${baseUrl}/${doc.slug})${desc}`);
    }
    lines.push('');
  }

  return lines.join('\n');
}

function generateLlmsFullTxt(proj: Project, docs: Document[]): string {
  const lines: string[] = [];

  lines.push(`# ${proj.name}`);
  lines.push('');
  if (proj.description) {
    lines.push(`> ${proj.description}`);
    lines.push('');
  }

  const typeOrder = ['tutorial', 'guide', 'reference', 'explanation'] as const;
  const grouped = new Map<string, Document[]>();
  for (const type of typeOrder) grouped.set(type, []);
  for (const doc of docs) {
    grouped.get(doc.diataxisType)?.push(doc);
  }

  for (const type of typeOrder) {
    const typeDocs = grouped.get(type) ?? [];
    if (typeDocs.length === 0) continue;

    lines.push(`## ${DIATAXIS_LABELS[type]}`);
    lines.push('');

    for (const doc of typeDocs) {
      lines.push(`### ${doc.title}`);
      lines.push('');
      // Strip frontmatter from raw content
      const content = doc.rawContent.replace(/^---[\s\S]*?---\s*/, '');
      lines.push(content.trim());
      lines.push('');
      lines.push('---');
      lines.push('');
    }
  }

  return lines.join('\n');
}
