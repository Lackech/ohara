import { Worker } from 'bullmq';
import { getRedisConfig } from '../../lib/redis.js';
import { logger } from '../../lib/logger.js';
import { syncProject } from './sync-service.js';
import type { SyncJobData } from '../../lib/queue.js';

/**
 * Start the BullMQ sync worker.
 * Processes sync jobs from the queue.
 */
export function startSyncWorker() {
  const redisConfig = getRedisConfig();
  if (!redisConfig) {
    logger.warn('Sync worker not started — no Redis connection');
    return null;
  }

  const worker = new Worker<SyncJobData>(
    'sync',
    async (job) => {
      logger.info({ jobId: job.id, projectId: job.data.projectId }, 'Processing sync job');

      const result = await syncProject(job.data);

      logger.info(
        { jobId: job.id, projectId: job.data.projectId, ...result },
        'Sync job completed',
      );

      return result;
    },
    {
      connection: { url: redisConfig.url },
      concurrency: 2,
      limiter: {
        max: 5,
        duration: 60_000,
      },
    },
  );

  worker.on('failed', (job, err) => {
    logger.error(
      { jobId: job?.id, projectId: job?.data.projectId, err },
      'Sync job failed',
    );
  });

  worker.on('error', (err) => {
    logger.error({ err }, 'Sync worker error');
  });

  logger.info('Sync worker started');
  return worker;
}
