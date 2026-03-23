import { Elysia } from 'elysia';
import { logger } from '../lib/logger.js';

/**
 * Request logging middleware.
 * Logs method, path, status, and response time for every request.
 */
export const requestLogger = new Elysia({ name: 'request-logger' })
  .onRequest(({ request, store }) => {
    (store as Record<string, unknown>).startTime = performance.now();
  })
  .onAfterResponse(({ request, set, store }) => {
    const startTime = (store as Record<string, unknown>).startTime as number;
    const duration = Math.round(performance.now() - startTime);
    const url = new URL(request.url);

    logger.info(
      {
        method: request.method,
        path: url.pathname,
        status: typeof set.status === 'number' ? set.status : 200,
        duration,
      },
      'request',
    );
  });
