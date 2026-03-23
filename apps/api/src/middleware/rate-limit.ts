import { Elysia } from 'elysia';

/**
 * Simple in-memory rate limiter.
 * Sliding window counter per IP for auth endpoints.
 */

const windows = new Map<string, { count: number; resetAt: number }>();

// Clean up expired windows periodically
setInterval(() => {
  const now = Date.now();
  for (const [key, window] of windows) {
    if (window.resetAt < now) windows.delete(key);
  }
}, 60_000);

function isRateLimited(key: string, maxRequests: number, windowMs: number): boolean {
  const now = Date.now();
  const window = windows.get(key);

  if (!window || window.resetAt < now) {
    windows.set(key, { count: 1, resetAt: now + windowMs });
    return false;
  }

  window.count++;
  return window.count > maxRequests;
}

/**
 * Rate limiting middleware for auth endpoints.
 * 10 requests per minute per IP on auth routes.
 */
export const authRateLimit = new Elysia({ name: 'auth-rate-limit' }).onBeforeHandle(
  ({ request, set }) => {
    const url = new URL(request.url);

    // Only apply to auth endpoints
    if (!url.pathname.startsWith('/api/auth')) return;

    const ip =
      request.headers.get('x-forwarded-for')?.split(',')[0]?.trim() ??
      request.headers.get('x-real-ip') ??
      'unknown';

    if (isRateLimited(`auth:${ip}`, 10, 60_000)) {
      set.status = 429;
      set.headers['retry-after'] = '60';
      return { error: 'Too many requests. Try again later.' };
    }
  },
);

/**
 * Rate limiting middleware for API endpoints.
 * Uses API key rate limit from the database when available.
 */
export const apiRateLimit = new Elysia({ name: 'api-rate-limit' }).onBeforeHandle(
  ({ request, set }) => {
    const url = new URL(request.url);

    // Only apply to API endpoints
    if (!url.pathname.startsWith('/api/v1')) return;

    const ip =
      request.headers.get('x-forwarded-for')?.split(',')[0]?.trim() ??
      request.headers.get('x-real-ip') ??
      'unknown';

    // Default: 100 requests per minute per IP
    if (isRateLimited(`api:${ip}`, 100, 60_000)) {
      set.status = 429;
      set.headers['retry-after'] = '60';
      return { error: 'Rate limit exceeded.' };
    }
  },
);
