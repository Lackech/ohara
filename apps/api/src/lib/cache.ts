/**
 * Simple caching layer with TTL.
 * In-memory LRU cache. Redis integration added when REDIS_URL is configured
 * via the BullMQ connection (shared infrastructure).
 */

const memoryCache = new Map<string, { value: string; expiresAt: number }>();
const MAX_MEMORY_ENTRIES = 500;

/**
 * Get a cached value by key.
 */
export async function cacheGet<T>(key: string): Promise<T | null> {
  const entry = memoryCache.get(key);
  if (entry && entry.expiresAt > Date.now()) {
    return JSON.parse(entry.value) as T;
  }
  if (entry) memoryCache.delete(key);
  return null;
}

/**
 * Set a cached value with TTL in seconds.
 */
export async function cacheSet(key: string, value: unknown, ttlSeconds: number): Promise<void> {
  const serialized = JSON.stringify(value);

  // Evict oldest entries if at capacity
  if (memoryCache.size >= MAX_MEMORY_ENTRIES) {
    const firstKey = memoryCache.keys().next().value;
    if (firstKey) memoryCache.delete(firstKey);
  }

  memoryCache.set(key, {
    value: serialized,
    expiresAt: Date.now() + ttlSeconds * 1000,
  });
}

/**
 * Invalidate cache entries matching a pattern.
 * Used after sync to clear stale data for a project.
 */
export async function cacheInvalidate(pattern: string): Promise<void> {
  const prefix = pattern.replace('*', '');
  for (const key of memoryCache.keys()) {
    if (key.startsWith(prefix)) {
      memoryCache.delete(key);
    }
  }
}
