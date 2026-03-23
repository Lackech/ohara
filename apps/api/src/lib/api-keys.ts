import { randomBytes, createHash } from 'crypto';
import { eq, and, isNull } from 'drizzle-orm';
import { db } from '../db/index.js';
import { apiKeys } from '../db/schema.js';
import { API_KEY_PREFIX } from '@ohara/shared';

/**
 * Generate a new API key.
 * Returns the raw key (shown once to user) and stores the SHA-256 hash.
 */
export async function generateApiKey(params: {
  userId: string;
  name: string;
  projectId?: string;
  rateLimit?: number;
  scopes?: string[];
}) {
  const rawKey = `${API_KEY_PREFIX}${randomBytes(32).toString('hex')}`;
  const keyHash = hashApiKey(rawKey);
  const keyPrefix = rawKey.slice(0, 12);

  const [inserted] = await db
    .insert(apiKeys)
    .values({
      userId: params.userId,
      name: params.name,
      projectId: params.projectId,
      keyHash,
      keyPrefix,
      rateLimit: params.rateLimit ?? 1000,
      scopes: params.scopes ?? ['read'],
    })
    .returning();

  return { key: rawKey, apiKey: inserted };
}

/**
 * Validate an API key and return the associated record.
 * Returns null if the key is invalid, expired, or revoked.
 */
export async function validateApiKey(rawKey: string) {
  if (!rawKey.startsWith(API_KEY_PREFIX)) {
    return null;
  }

  const keyHash = hashApiKey(rawKey);

  const [record] = await db
    .select()
    .from(apiKeys)
    .where(and(eq(apiKeys.keyHash, keyHash), isNull(apiKeys.revokedAt)))
    .limit(1);

  if (!record) {
    return null;
  }

  // Check expiration
  if (record.expiresAt && record.expiresAt < new Date()) {
    return null;
  }

  // Update last used timestamp (fire and forget)
  db.update(apiKeys)
    .set({ lastUsedAt: new Date() })
    .where(eq(apiKeys.id, record.id))
    .then(() => {});

  return record;
}

/**
 * Revoke an API key by setting revokedAt timestamp.
 */
export async function revokeApiKey(keyId: string, userId: string) {
  const [revoked] = await db
    .update(apiKeys)
    .set({ revokedAt: new Date() })
    .where(and(eq(apiKeys.id, keyId), eq(apiKeys.userId, userId)))
    .returning();

  return revoked ?? null;
}

function hashApiKey(rawKey: string): string {
  return createHash('sha256').update(rawKey).digest('hex');
}
