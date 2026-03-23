import { Elysia, t } from 'elysia';
import { eq } from 'drizzle-orm';
import { db } from '../../db/index.js';
import { projects } from '../../db/schema.js';
import { withAuth } from '../../middleware/auth.js';

export const projectsModule = new Elysia({ prefix: '/api/v1/projects', name: 'projects' })
  .use(withAuth())

  // List user's projects
  .get('/', async ({ user }) => {
    const result = await db.select().from(projects).where(eq(projects.ownerId, user.id));
    return { data: result };
  })

  // Create a project
  .post(
    '/',
    async ({ user, body, set }) => {
      const [project] = await db
        .insert(projects)
        .values({
          name: body.name,
          slug: body.slug,
          description: body.description,
          ownerId: user.id,
          repoUrl: body.repoUrl,
          repoBranch: body.repoBranch,
          docsDir: body.docsDir,
        })
        .returning();

      set.status = 201;
      return { data: project };
    },
    {
      body: t.Object({
        name: t.String({ minLength: 1 }),
        slug: t.String({ minLength: 1, pattern: '^[a-z0-9-]+$' }),
        description: t.Optional(t.String()),
        repoUrl: t.Optional(t.String()),
        repoBranch: t.Optional(t.String()),
        docsDir: t.Optional(t.String()),
      }),
    },
  )

  // Get a project by slug
  .get(
    '/:slug',
    async ({ params, set }) => {
      const [project] = await db
        .select()
        .from(projects)
        .where(eq(projects.slug, params.slug))
        .limit(1);

      if (!project) {
        set.status = 404;
        return { error: 'Project not found' };
      }

      return { data: project };
    },
    {
      params: t.Object({ slug: t.String() }),
    },
  );
