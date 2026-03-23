import { Elysia, t } from 'elysia';
import { eq, sql } from 'drizzle-orm';
import { db } from '../../db/index.js';
import { documents, projects } from '../../db/schema.js';
import { withAuth } from '../../middleware/auth.js';
import { hybridSearch } from './hybrid.js';

/** Resolve a project by slug or UUID */
async function resolveProject(slugOrId: string) {
  const [proj] = await db
    .select()
    .from(projects)
    .where(sql`${projects.slug} = ${slugOrId} OR ${projects.id}::text = ${slugOrId}`)
    .limit(1);
  return proj ?? null;
}

export const searchModule = new Elysia({ prefix: '/api/v1/projects', name: 'search' })
  .use(withAuth())

  // Full-text search (simple — for the search UI)
  .get(
    '/:slug/search',
    async ({ params, query, set }) => {
      const proj = await resolveProject(params.slug);
      if (!proj) {
        set.status = 404;
        return { error: 'Project not found' };
      }

      const { q, type, limit = '20', offset = '0' } = query;

      if (!q || q.trim().length === 0) {
        set.status = 400;
        return { error: 'Query parameter "q" is required' };
      }

      const limitNum = Math.min(parseInt(limit, 10) || 20, 100);
      const offsetNum = parseInt(offset, 10) || 0;

      const results = await db.execute(sql`
        SELECT
          ${documents.id} as id,
          ${documents.title} as title,
          ${documents.slug} as slug,
          ${documents.path} as path,
          ${documents.description} as description,
          ${documents.diataxisType} as diataxis_type,
          ${documents.wordCount} as word_count,
          ${documents.updatedAt} as updated_at,
          ts_rank(
            ${documents.searchVector},
            plainto_tsquery('english', ${q})
          ) as rank,
          ts_headline(
            'english',
            ${documents.rawContent},
            plainto_tsquery('english', ${q}),
            'StartSel=<mark>, StopSel=</mark>, MaxWords=50, MinWords=20, MaxFragments=3'
          ) as snippet
        FROM ${documents}
        WHERE ${documents.projectId} = ${proj.id}
          AND ${documents.searchVector} @@ plainto_tsquery('english', ${q})
          ${type ? sql`AND ${documents.diataxisType} = ${type}` : sql``}
        ORDER BY rank DESC
        LIMIT ${limitNum}
        OFFSET ${offsetNum}
      `);

      const facets = await db.execute(sql`
        SELECT
          ${documents.diataxisType} as diataxis_type,
          count(*)::int as count
        FROM ${documents}
        WHERE ${documents.projectId} = ${proj.id}
          AND ${documents.searchVector} @@ plainto_tsquery('english', ${q})
        GROUP BY ${documents.diataxisType}
        ORDER BY count DESC
      `);

      return {
        data: Array.from(results),
        facets: Array.from(facets),
        metadata: {
          query: q,
          type: type ?? null,
          limit: limitNum,
          offset: offsetNum,
          totalResults: results.length,
        },
      };
    },
    {
      params: t.Object({ slug: t.String() }),
      query: t.Object({
        q: t.String(),
        type: t.Optional(t.String()),
        limit: t.Optional(t.String()),
        offset: t.Optional(t.String()),
      }),
    },
  )

  // Hybrid search (full-text + semantic — for agents)
  .get(
    '/:slug/search/hybrid',
    async ({ params, query, set }) => {
      const proj = await resolveProject(params.slug);
      if (!proj) {
        set.status = 404;
        return { error: 'Project not found' };
      }

      const { q, type, limit = '10' } = query;

      if (!q || q.trim().length === 0) {
        set.status = 400;
        return { error: 'Query parameter "q" is required' };
      }

      const results = await hybridSearch(q, proj.id, {
        limit: Math.min(parseInt(limit, 10) || 10, 50),
        diataxisType: type,
      });

      return {
        data: results,
        metadata: {
          query: q,
          type: type ?? null,
          searchMethod: 'hybrid_rrf',
        },
      };
    },
    {
      params: t.Object({ slug: t.String() }),
      query: t.Object({
        q: t.String(),
        type: t.Optional(t.String()),
        limit: t.Optional(t.String()),
      }),
    },
  );
