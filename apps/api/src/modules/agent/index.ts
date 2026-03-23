import { Elysia, t } from 'elysia';
import { eq, sql } from 'drizzle-orm';
import { db } from '../../db/index.js';
import { documents, projects, documentChunks } from '../../db/schema.js';
import { withAuth } from '../../middleware/auth.js';
import { hybridSearch } from '../search/hybrid.js';
import type { DiataxisType } from '@ohara/shared';

/**
 * Agent-optimized REST API.
 * Every response includes metadata and context for agent consumption.
 */
export const agentModule = new Elysia({ prefix: '/api/v1', name: 'agent' })
  .use(withAuth())

  // Search documentation (agent-optimized)
  .get(
    '/search',
    async ({ query: q, set }) => {
      const { query, project, type, limit = '10' } = q;

      if (!query?.trim()) {
        set.status = 400;
        return { error: 'Query parameter "query" is required' };
      }

      if (!project) {
        set.status = 400;
        return { error: 'Query parameter "project" is required' };
      }

      // Resolve project by slug or ID
      const [proj] = await db
        .select()
        .from(projects)
        .where(eq(projects.slug, project))
        .limit(1);

      if (!proj) {
        set.status = 404;
        return { error: 'Project not found' };
      }

      const results = await hybridSearch(query, proj.id, {
        limit: Math.min(parseInt(limit, 10) || 10, 50),
        diataxisType: type,
      });

      return {
        data: {
          results: results.map((r) => ({
            title: r.title,
            path: r.path,
            diataxisType: r.diataxisType,
            snippet: r.snippet,
            score: r.score,
            matchType: r.matchType,
          })),
        },
        metadata: {
          project: proj.slug,
          query,
          searchMethod: 'hybrid_rrf',
          resultCount: results.length,
        },
        context: {
          suggestedQueries: generateSuggestions(query),
        },
      };
    },
    {
      query: t.Object({
        query: t.String(),
        project: t.String(),
        type: t.Optional(t.String()),
        limit: t.Optional(t.String()),
      }),
    },
  )

  // Get document (agent-optimized)
  .get(
    '/documents/:projectSlug/:path',
    async ({ params, request, set }) => {
      const [proj] = await db
        .select()
        .from(projects)
        .where(eq(projects.slug, params.projectSlug))
        .limit(1);

      if (!proj) {
        set.status = 404;
        return { error: 'Project not found' };
      }

      const docPath = decodeURIComponent(params.path);
      const [doc] = await db
        .select()
        .from(documents)
        .where(
          sql`${documents.projectId} = ${proj.id} AND (${documents.path} = ${docPath} OR ${documents.slug} = ${docPath})`,
        )
        .limit(1);

      if (!doc) {
        set.status = 404;
        return { error: 'Document not found' };
      }

      // Content negotiation
      const accept = request.headers.get('accept') ?? '';
      if (accept.includes('text/markdown')) {
        set.headers['content-type'] = 'text/markdown; charset=utf-8';
        return doc.rawContent;
      }

      // Fetch related documents (same Diataxis type, same project)
      const related = await db
        .select({ title: documents.title, slug: documents.slug, path: documents.path })
        .from(documents)
        .where(
          sql`${documents.projectId} = ${proj.id} AND ${documents.diataxisType} = ${doc.diataxisType} AND ${documents.id} != ${doc.id}`,
        )
        .limit(5);

      return {
        data: {
          title: doc.title,
          path: doc.path,
          content: doc.rawContent,
          htmlContent: doc.htmlContent,
          description: doc.description,
          frontmatter: doc.frontmatter,
          wordCount: doc.wordCount,
        },
        metadata: {
          project: proj.slug,
          diataxisType: doc.diataxisType,
          lastSynced: proj.lastSyncedAt,
          updatedAt: doc.updatedAt,
        },
        context: {
          relatedDocuments: related.map((r) => ({
            title: r.title,
            path: r.path,
            slug: r.slug,
          })),
        },
      };
    },
    {
      params: t.Object({
        projectSlug: t.String(),
        path: t.String(),
      }),
    },
  )

  // List documents (agent-optimized)
  .get(
    '/documents/:projectSlug',
    async ({ params, query: q, set }) => {
      const [proj] = await db
        .select()
        .from(projects)
        .where(eq(projects.slug, params.projectSlug))
        .limit(1);

      if (!proj) {
        set.status = 404;
        return { error: 'Project not found' };
      }

      const typeFilter = q.type
        ? sql`AND ${documents.diataxisType} = ${q.type}`
        : sql``;

      const docs = await db.execute(sql`
        SELECT
          ${documents.title} as title,
          ${documents.slug} as slug,
          ${documents.path} as path,
          ${documents.description} as description,
          ${documents.diataxisType} as diataxis_type,
          ${documents.wordCount} as word_count,
          ${documents.updatedAt} as updated_at
        FROM ${documents}
        WHERE ${documents.projectId} = ${proj.id}
          AND ${documents.draft} = false
          ${typeFilter}
        ORDER BY ${documents.diataxisType}, ${documents.order} NULLS LAST, ${documents.title}
      `);

      return {
        data: { documents: Array.from(docs) },
        metadata: {
          project: proj.slug,
          diataxisType: q.type ?? null,
          totalCount: docs.length,
          lastSynced: proj.lastSyncedAt,
        },
      };
    },
    {
      params: t.Object({ projectSlug: t.String() }),
      query: t.Object({
        type: t.Optional(t.String()),
      }),
    },
  );

function generateSuggestions(query: string): string[] {
  const suggestions: string[] = [];
  if (!query.toLowerCase().includes('how')) {
    suggestions.push(`How to ${query}`);
  }
  if (!query.toLowerCase().includes('api')) {
    suggestions.push(`${query} API reference`);
  }
  suggestions.push(`${query} examples`);
  return suggestions.slice(0, 3);
}
