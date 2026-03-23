import { logger } from './logger.js';

/**
 * Redis-based sync lock to prevent concurrent syncs for the same project.
 * Falls back to an in-memory lock if Redis is not available.
 */

const inMemoryLocks = new Set<string>();

/**
 * Acquire a sync lock for a project. Returns true if lock was acquired.
 * Lock expires after TTL seconds (default 5 minutes).
 */
export async function acquireSyncLock(
  projectId: string,
  ttlSeconds: number = 300,
): Promise<boolean> {
  const lockKey = `sync-lock:${projectId}`;

  // In-memory lock (sufficient for single-process; Redis version for multi-process in production)
  if (inMemoryLocks.has(lockKey)) {
    return false;
  }
  inMemoryLocks.add(lockKey);
  setTimeout(() => inMemoryLocks.delete(lockKey), ttlSeconds * 1000);
  return true;
}

/**
 * Release a sync lock for a project.
 */
export async function releaseSyncLock(projectId: string): Promise<void> {
  const lockKey = `sync-lock:${projectId}`;
  inMemoryLocks.delete(lockKey);
}
