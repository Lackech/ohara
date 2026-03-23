import { Elysia, t } from 'elysia';
import { eq, sql } from 'drizzle-orm';
import { db } from '../../db/index.js';
import { projects, documents } from '../../db/schema.js';

/**
 * Public endpoints — no auth required.
 * Used by the frontend to render docs for visitors.
 */
export const publicModule = new Elysia({ prefix: '/api/v1/public', name: 'public' })

  // Get project by slug (public)
  .get(
    '/projects/:slug',
    async ({ params, set }) => {
      const [project] = await db
        .select({
          id: projects.id,
          name: projects.name,
          slug: projects.slug,
          description: projects.description,
          lastSyncedAt: projects.lastSyncedAt,
        })
        .from(projects)
        .where(eq(projects.slug, params.slug))
        .limit(1);

      if (!project) {
        set.status = 404;
        return { error: 'Project not found' };
      }

      return { data: project };
    },
    { params: t.Object({ slug: t.String() }) },
  )

  // List documents for a project (public)
  .get(
    '/projects/:slug/documents',
    async ({ params, set }) => {
      const [project] = await db
        .select({ id: projects.id })
        .from(projects)
        .where(eq(projects.slug, params.slug))
        .limit(1);

      if (!project) {
        set.status = 404;
        return { error: 'Project not found' };
      }

      const docs = await db
        .select({
          id: documents.id,
          title: documents.title,
          slug: documents.slug,
          path: documents.path,
          description: documents.description,
          diataxisType: documents.diataxisType,
          htmlContent: documents.htmlContent,
          rawContent: documents.rawContent,
          wordCount: documents.wordCount,
          draft: documents.draft,
          updatedAt: documents.updatedAt,
        })
        .from(documents)
        .where(eq(documents.projectId, project.id))
        .orderBy(documents.diataxisType, documents.order, documents.title);

      return { data: docs };
    },
    { params: t.Object({ slug: t.String() }) },
  );
