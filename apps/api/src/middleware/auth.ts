import { Elysia } from 'elysia';
import { auth } from '../lib/auth.js';
import { validateApiKey } from '../lib/api-keys.js';

export type AuthUser = {
  id: string;
  name: string;
  email: string;
  image: string | null;
};

/**
 * Mounts Better Auth HTTP handler.
 * Use once in the main app entry point.
 */
export const betterAuthMount = new Elysia({ name: 'better-auth-mount' }).mount(auth.handler);

/**
 * Resolve user from session or API key.
 */
async function resolveUser(headers: Headers): Promise<AuthUser | null> {
  // Try session auth first (Better Auth)
  const sessionResult = await auth.api.getSession({ headers });
  if (sessionResult) {
    return sessionResult.user as AuthUser;
  }

  // Fall back to API key auth
  const authHeader = headers.get('authorization');
  if (authHeader?.startsWith('Bearer ')) {
    const token = authHeader.slice(7);
    const apiKeyRecord = await validateApiKey(token);
    if (apiKeyRecord) {
      return { id: apiKeyRecord.userId, name: '', email: '', image: null };
    }
  }

  return null;
}

/**
 * Auth guard plugin factory.
 * Use `withAuth()` in modules that require authentication.
 * The `{ as: 'scoped' }` option ensures types propagate through `.use()`.
 */
export function withAuth() {
  return new Elysia({ name: 'auth-guard' }).derive(
    { as: 'scoped' },
    async ({ request: { headers }, status }) => {
      const user = await resolveUser(headers);
      if (!user) return status(401) as never;
      return { user };
    },
  );
}
