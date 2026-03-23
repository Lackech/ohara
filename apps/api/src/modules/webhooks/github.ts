import { Elysia, t } from 'elysia';
import { createHmac, timingSafeEqual } from 'crypto';
import { eq } from 'drizzle-orm';
import { db } from '../../db/index.js';
import { projects } from '../../db/schema.js';
import { enqueueSyncJob } from '../../lib/queue.js';
import { logger } from '../../lib/logger.js';

/**
 * Verify GitHub webhook signature (HMAC-SHA256).
 */
function verifySignature(payload: string, signature: string, secret: string): boolean {
  const expected = `sha256=${createHmac('sha256', secret).update(payload).digest('hex')}`;

  if (expected.length !== signature.length) return false;

  return timingSafeEqual(Buffer.from(expected), Buffer.from(signature));
}

export const githubWebhookModule = new Elysia({
  prefix: '/webhooks/github',
  name: 'github-webhook',
}).post(
  '/',
  async ({ body, request, set }) => {
    const secret = process.env.GITHUB_WEBHOOK_SECRET;
    if (!secret) {
      logger.error('GITHUB_WEBHOOK_SECRET not configured');
      set.status = 500;
      return { error: 'Webhook secret not configured' };
    }

    // Verify signature
    const signature = request.headers.get('x-hub-signature-256');
    if (!signature) {
      set.status = 401;
      return { error: 'Missing signature' };
    }

    const rawBody = JSON.stringify(body);
    if (!verifySignature(rawBody, signature, secret)) {
      set.status = 401;
      return { error: 'Invalid signature' };
    }

    const event = request.headers.get('x-github-event');

    // Handle ping event (GitHub sends this on app install)
    if (event === 'ping') {
      return { status: 'pong' };
    }

    // Only process push events
    if (event !== 'push') {
      return { status: 'ignored', event };
    }

    const payload = body as PushEvent;
    const repoUrl = payload.repository?.html_url;
    const branch = payload.ref?.replace('refs/heads/', '');

    if (!repoUrl || !branch) {
      set.status = 400;
      return { error: 'Invalid push event payload' };
    }

    // Find matching projects
    const matchingProjects = await db
      .select()
      .from(projects)
      .where(eq(projects.repoUrl, repoUrl));

    if (matchingProjects.length === 0) {
      logger.info({ repoUrl }, 'No projects found for repo');
      return { status: 'no_matching_projects' };
    }

    // Enqueue sync for each matching project on the tracked branch
    const enqueued: string[] = [];

    for (const project of matchingProjects) {
      if (project.repoBranch !== branch) {
        continue;
      }

      await enqueueSyncJob({
        projectId: project.id,
        repoUrl,
        branch,
        docsDir: project.docsDir ?? '.',
        commitSha: payload.after,
        commitMessage: payload.head_commit?.message,
        triggerType: 'webhook',
      });

      enqueued.push(project.id);
    }

    logger.info({ repoUrl, branch, projectCount: enqueued.length }, 'Sync jobs enqueued');

    return { status: 'enqueued', projects: enqueued };
  },
  {
    body: t.Any(),
  },
);

// Minimal type for the push event fields we use
type PushEvent = {
  ref?: string;
  after?: string;
  repository?: { html_url?: string };
  head_commit?: { message?: string };
};
