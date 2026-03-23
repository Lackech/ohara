import { logger } from './logger.js';

const redisUrl = process.env.REDIS_URL;

/**
 * Get Redis connection options for BullMQ.
 * BullMQ uses its own bundled ioredis, so we pass the URL string.
 */
export function getRedisConfig(): { url: string } | null {
  if (!redisUrl) {
    logger.warn('REDIS_URL not set — queue and caching features disabled');
    return null;
  }
  return { url: redisUrl };
}
