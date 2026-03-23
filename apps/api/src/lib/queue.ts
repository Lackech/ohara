import { Queue } from 'bullmq';
import { getRedisConfig } from './redis.js';
import { logger } from './logger.js';

export type SyncJobData = {
  projectId: string;
  repoUrl: string;
  branch: string;
  docsDir: string;
  commitSha?: string;
  commitMessage?: string;
  triggerType: 'webhook' | 'manual' | 'scheduled';
};

const redisConfig = getRedisConfig();

export const syncQueue = redisConfig
  ? new Queue<SyncJobData>('sync', {
      connection: { url: redisConfig.url },
      defaultJobOptions: {
        attempts: 3,
        backoff: { type: 'exponential', delay: 5000 },
        removeOnComplete: { count: 100 },
        removeOnFail: { count: 500 },
      },
    })
  : null;

/**
 * Enqueue a sync job for a project.
 */
export async function enqueueSyncJob(data: SyncJobData) {
  if (!syncQueue) {
    logger.warn('Sync queue not available (no Redis)');
    return null;
  }

  const job = await syncQueue.add('sync', data, {
    jobId: `sync-${data.projectId}-${Date.now()}`,
  });

  logger.info({ jobId: job.id, projectId: data.projectId }, 'Sync job enqueued');
  return job;
}
