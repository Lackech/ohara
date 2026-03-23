import { Elysia, t } from 'elysia';
import { eq } from 'drizzle-orm';
import { db } from '../../db/index.js';
import { apiKeys } from '../../db/schema.js';
import { generateApiKey, revokeApiKey } from '../../lib/api-keys.js';
import { withAuth } from '../../middleware/auth.js';

export const apiKeysModule = new Elysia({ prefix: '/api/v1/api-keys', name: 'api-keys' })
  .use(withAuth())

  // List user's API keys (never returns the raw key)
  .get('/', async ({ user }) => {
    const keys = await db.select().from(apiKeys).where(eq(apiKeys.userId, user.id));

    return {
      data: keys.map((k) => ({
        id: k.id,
        name: k.name,
        keyPrefix: k.keyPrefix,
        projectId: k.projectId,
        scopes: k.scopes,
        rateLimit: k.rateLimit,
        lastUsedAt: k.lastUsedAt,
        expiresAt: k.expiresAt,
        createdAt: k.createdAt,
      })),
    };
  })

  // Generate a new API key (raw key returned only once)
  .post(
    '/',
    async ({ user, body, set }) => {
      const result = await generateApiKey({
        userId: user.id,
        name: body.name,
        projectId: body.projectId,
        scopes: body.scopes,
      });

      set.status = 201;
      return {
        data: {
          id: result.apiKey.id,
          name: result.apiKey.name,
          key: result.key,
          keyPrefix: result.apiKey.keyPrefix,
          scopes: result.apiKey.scopes,
          createdAt: result.apiKey.createdAt,
        },
      };
    },
    {
      body: t.Object({
        name: t.String({ minLength: 1 }),
        projectId: t.Optional(t.String()),
        scopes: t.Optional(t.Array(t.String())),
      }),
    },
  )

  // Revoke an API key
  .delete(
    '/:id',
    async ({ params, user, set }) => {
      const revoked = await revokeApiKey(params.id, user.id);
      if (!revoked) {
        set.status = 404;
        return { error: 'API key not found' };
      }
      return { data: { id: revoked.id, revokedAt: revoked.revokedAt } };
    },
    {
      params: t.Object({ id: t.String() }),
    },
  );
