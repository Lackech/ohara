import { Elysia, t } from 'elysia';
import { eq, and, sql } from 'drizzle-orm';
import { db } from '../../db/index.js';
import { documents, projects } from '../../db/schema.js';
import { withAuth } from '../../middleware/auth.js';

/** Resolve a project by slug or UUID */
async function resolveProject(slugOrId: string) {
  const [proj] = await db
    .select()
    .from(projects)
    .where(sql`${projects.slug} = ${slugOrId} OR ${projects.id}::text = ${slugOrId}`)
    .limit(1);
  return proj ?? null;
}

export const documentsModule = new Elysia({ prefix: '/api/v1/projects', name: 'documents' })
  .use(withAuth())

  // List documents for a project
  .get(
    '/:slug/documents',
    async ({ params, set }) => {
      const proj = await resolveProject(params.slug);
      if (!proj) {
        set.status = 404;
        return { error: 'Project not found' };
      }

      const result = await db
        .select()
        .from(documents)
        .where(eq(documents.projectId, proj.id))
        .orderBy(documents.diataxisType, documents.order, documents.title);

      return { data: result };
    },
    {
      params: t.Object({ slug: t.String() }),
    },
  )

  // Get a single document by path
  .get(
    '/:slug/documents/:path',
    async ({ params, set, request }) => {
      const proj = await resolveProject(params.slug);
      if (!proj) {
        set.status = 404;
        return { error: 'Project not found' };
      }

      const docPath = decodeURIComponent(params.path);

      const [doc] = await db
        .select()
        .from(documents)
        .where(
          and(
            eq(documents.projectId, proj.id),
            eq(documents.path, docPath),
          ),
        )
        .limit(1);

      if (!doc) {
        set.status = 404;
        return { error: 'Document not found' };
      }

      // Content negotiation: text/markdown returns raw content
      const accept = request.headers.get('accept') ?? '';
      if (accept.includes('text/markdown')) {
        set.headers['content-type'] = 'text/markdown; charset=utf-8';
        return doc.rawContent;
      }

      return {
        data: doc,
        metadata: {
          projectId: proj.id,
          diataxisType: doc.diataxisType,
          path: doc.path,
          wordCount: doc.wordCount,
          updatedAt: doc.updatedAt,
        },
      };
    },
    {
      params: t.Object({
        slug: t.String(),
        path: t.String(),
      }),
    },
  );
