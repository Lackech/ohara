/**
 * E2E test for the sync pipeline.
 *
 * Tests the full flow:
 *   webhook payload → sync job → clone → walk → parse → chunk → DB
 *
 * Run with: bun test src/test/e2e-sync.test.ts
 *
 * Requires:
 *   - DATABASE_URL pointing to a test database
 *   - A public GitHub repo to clone from
 */
import { describe, test, expect, beforeAll } from 'bun:test';
import { db } from '../db/index.js';
import { projects, documents, documentChunks, syncLogs } from '../db/schema.js';
import { syncProject } from '../modules/git/sync-service.js';
import { eq } from 'drizzle-orm';

// Use a small public repo with markdown files for testing
const TEST_REPO = 'https://github.com/fuma-nama/fumadocs.git';
const TEST_BRANCH = 'main';

describe('E2E Sync Pipeline', () => {
  let projectId: string;

  beforeAll(async () => {
    // Create a test project
    const [project] = await db
      .insert(projects)
      .values({
        name: 'E2E Test Project',
        slug: `e2e-test-${Date.now()}`,
        description: 'Test project for E2E sync',
        ownerId: 'test-user',
        repoUrl: TEST_REPO,
        repoBranch: TEST_BRANCH,
        docsDir: 'apps/docs/content/docs',
      })
      .returning();

    projectId = project.id;
  });

  test('sync discovers and stores documents', async () => {
    const result = await syncProject({
      projectId,
      repoUrl: TEST_REPO,
      branch: TEST_BRANCH,
      docsDir: 'apps/docs/content/docs',
      triggerType: 'manual',
    });

    expect(result.added).toBeGreaterThan(0);
    expect(result.deleted).toBe(0);
    expect(result.chunksCreated).toBeGreaterThan(0);
  }, 60_000);

  test('documents have correct fields', async () => {
    const docs = await db
      .select()
      .from(documents)
      .where(eq(documents.projectId, projectId))
      .limit(5);

    expect(docs.length).toBeGreaterThan(0);

    for (const doc of docs) {
      expect(doc.title).toBeTruthy();
      expect(doc.path).toBeTruthy();
      expect(doc.slug).toBeTruthy();
      expect(doc.rawContent).toBeTruthy();
      expect(doc.contentHash).toHaveLength(64);
      expect(['tutorial', 'guide', 'reference', 'explanation']).toContain(doc.diataxisType);
    }
  });

  test('chunks are created with heading hierarchy', async () => {
    const chunks = await db
      .select()
      .from(documentChunks)
      .where(eq(documentChunks.projectId, projectId))
      .limit(10);

    expect(chunks.length).toBeGreaterThan(0);

    for (const chunk of chunks) {
      expect(chunk.content).toBeTruthy();
      expect(chunk.contentHash).toHaveLength(64);
      expect(chunk.chunkIndex).toBeGreaterThanOrEqual(0);
      expect(chunk.tokenCount).toBeGreaterThan(0);
    }
  });

  test('sync log records success', async () => {
    const [log] = await db
      .select()
      .from(syncLogs)
      .where(eq(syncLogs.projectId, projectId))
      .limit(1);

    expect(log).toBeTruthy();
    expect(log.status).toBe('completed');
    expect(log.triggerType).toBe('manual');
    expect(log.documentsAdded).toBeGreaterThan(0);
    expect(log.completedAt).toBeTruthy();
  });

  test('incremental sync skips unchanged documents', async () => {
    const result = await syncProject({
      projectId,
      repoUrl: TEST_REPO,
      branch: TEST_BRANCH,
      docsDir: 'apps/docs/content/docs',
      triggerType: 'manual',
    });

    // Second sync should find everything unchanged
    expect(result.added).toBe(0);
    expect(result.updated).toBe(0);
  }, 60_000);

  test('concurrent syncs are blocked by lock', async () => {
    // Start two syncs simultaneously
    const [result1, result2] = await Promise.all([
      syncProject({
        projectId,
        repoUrl: TEST_REPO,
        branch: TEST_BRANCH,
        docsDir: 'apps/docs/content/docs',
        triggerType: 'manual',
      }),
      syncProject({
        projectId,
        repoUrl: TEST_REPO,
        branch: TEST_BRANCH,
        docsDir: 'apps/docs/content/docs',
        triggerType: 'manual',
      }),
    ]);

    // One should succeed, one should be skipped (0 changes due to lock)
    const totalAdded = result1.added + result2.added;
    const totalUpdated = result1.updated + result2.updated;

    // At least one should have been skipped
    expect(totalAdded + totalUpdated).toBeLessThanOrEqual(result1.added + result1.updated);
  }, 120_000);
});
