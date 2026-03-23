import { Elysia } from 'elysia';
import { cors } from '@elysiajs/cors';
import { logger } from './lib/logger.js';

// Middleware
import { betterAuthMount } from './middleware/auth.js';
import { requestLogger } from './middleware/request-logger.js';
import { authRateLimit, apiRateLimit } from './middleware/rate-limit.js';
import { securityHeaders } from './middleware/security.js';

// Modules
import { healthModule } from './modules/health/index.js';
import { projectsModule } from './modules/projects/index.js';
import { apiKeysModule } from './modules/api-keys/index.js';
import { githubWebhookModule } from './modules/webhooks/github.js';
import { documentsModule } from './modules/documents/index.js';
import { searchModule } from './modules/search/index.js';
import { agentModule } from './modules/agent/index.js';
import { llmsTxtModule } from './modules/llms-txt/index.js';
import { mcpModule } from './modules/agent/mcp/handler.js';
import { publicModule } from './modules/public/index.js';
import { scaffoldModule } from './modules/scaffold/index.js';
import { githubAppModule } from './modules/github-app/index.js';
import { startSyncWorker } from './modules/git/sync-worker.js';

const app = new Elysia()
  // Global middleware
  .use(requestLogger)
  .use(securityHeaders)
  .use(authRateLimit)
  .use(apiRateLimit)
  .use(
    cors({
      origin: process.env.WEB_URL ?? 'http://localhost:3000',
      methods: ['GET', 'POST', 'PUT', 'DELETE', 'OPTIONS'],
      credentials: true,
      allowedHeaders: ['Content-Type', 'Authorization'],
    }),
  )
  .onError(({ code, error, set }) => {
    logger.error({ err: error, code }, 'Unhandled error');

    if (code === 'NOT_FOUND') {
      set.status = 404;
      return { error: 'Not found' };
    }

    if (code === 'VALIDATION') {
      set.status = 400;
      return { error: 'Validation error', details: error.message };
    }

    set.status = 500;
    return { error: 'Internal server error' };
  })

  // Public endpoints (no auth)
  .use(publicModule)

  // Auth
  .use(betterAuthMount)

  // API modules
  .use(healthModule)
  .use(projectsModule)
  .use(apiKeysModule)
  .use(githubWebhookModule)
  .use(documentsModule)
  .use(searchModule)
  .use(agentModule)
  .use(llmsTxtModule)
  .use(mcpModule)
  .use(githubAppModule)
  .use(scaffoldModule)
  .listen(process.env.PORT ?? 3001);

// Start the background sync worker
startSyncWorker();

logger.info(`Ohara API running at ${app.server?.url}`);

export type App = typeof app;
