import { createHash } from 'crypto';
import { mkdtemp, rm } from 'fs/promises';
import { tmpdir } from 'os';
import { join } from 'path';
import simpleGit from 'simple-git';
import { eq, and, notInArray } from 'drizzle-orm';
import { db } from '../../db/index.js';
import { projects, documents, documentChunks, syncLogs } from '../../db/schema.js';
import { logger } from '../../lib/logger.js';
import { acquireSyncLock, releaseSyncLock } from '../../lib/sync-lock.js';
import { cacheInvalidate } from '../../lib/cache.js';
import { walkFileTree, loadProjectConfig } from './file-walker.js';
import { parseMarkdown, chunkDocument } from '@ohara/shared/markdown';
import type { SyncJobData } from '../../lib/queue.js';

function sha256(content: string): string {
  return createHash('sha256').update(content).digest('hex');
}

/**
 * Main sync function — called by the BullMQ worker.
 * Clones repo → walks files → parses markdown → chunks → upserts to DB.
 */
export async function syncProject(data: SyncJobData): Promise<{
  added: number;
  updated: number;
  deleted: number;
  chunksCreated: number;
}> {
  const { projectId, repoUrl, branch, docsDir, commitSha, commitMessage, triggerType } = data;

  // Acquire sync lock to prevent concurrent syncs for the same project
  const lockAcquired = await acquireSyncLock(projectId);
  if (!lockAcquired) {
    logger.warn({ projectId }, 'Sync already in progress, skipping');
    return { added: 0, updated: 0, deleted: 0, chunksCreated: 0 };
  }

  // Create sync log entry
  const [syncLog] = await db
    .insert(syncLogs)
    .values({
      projectId,
      status: 'running',
      triggerType,
      commitSha,
      commitMessage,
      startedAt: new Date(),
    })
    .returning();

  const syncLogId = syncLog.id;
  let tempDir: string | null = null;

  try {
    // 1. Shallow clone to temp directory
    tempDir = await mkdtemp(join(tmpdir(), 'ohara-sync-'));
    const git = simpleGit();

    logger.info({ projectId, repoUrl, branch, tempDir }, 'Cloning repository');

    await git.clone(repoUrl, tempDir, [
      '--depth', '1',
      '--branch', branch,
      '--single-branch',
    ]);

    // 2. Load project config (ohara.yaml)
    const config = await loadProjectConfig(tempDir);

    // 3. Walk the file tree and discover documents
    const discovered = await walkFileTree(tempDir, docsDir, config);

    logger.info({ projectId, documentCount: discovered.length }, 'Documents discovered');

    // 4. Get existing documents for this project (for diffing)
    const existingDocs = await db
      .select({ id: documents.id, path: documents.path, contentHash: documents.contentHash })
      .from(documents)
      .where(eq(documents.projectId, projectId));

    const existingByPath = new Map(existingDocs.map((d) => [d.path, d]));

    // 5. Upsert documents with parsing + chunking
    let added = 0;
    let updated = 0;
    let totalChunksCreated = 0;
    const syncedPaths: string[] = [];

    for (const doc of discovered) {
      const hash = sha256(doc.rawContent);
      syncedPaths.push(doc.path);

      const existing = existingByPath.get(doc.path);
      const isNew = !existing;
      const isChanged = existing && existing.contentHash !== hash;

      if (!isNew && !isChanged) continue; // Unchanged — skip

      // Parse markdown → HTML
      const parsed = await parseMarkdown(doc.rawContent);

      // Generate chunks
      const chunks = chunkDocument(doc.rawContent, doc.title);

      if (isNew) {
        // Insert new document
        const [inserted] = await db
          .insert(documents)
          .values({
            projectId,
            path: doc.path,
            title: doc.title,
            slug: doc.slug,
            description: doc.description,
            diataxisType: doc.diataxisType,
            rawContent: doc.rawContent,
            htmlContent: parsed.html,
            contentHash: hash,
            frontmatter: doc.frontmatter,
            draft: doc.draft,
            order: doc.order,
            wordCount: parsed.wordCount,
          })
          .returning({ id: documents.id });

        // Insert chunks
        if (chunks.length > 0) {
          await db.insert(documentChunks).values(
            chunks.map((chunk) => ({
              documentId: inserted.id,
              projectId,
              content: chunk.content,
              contentHash: sha256(chunk.content),
              chunkIndex: chunk.chunkIndex,
              headingHierarchy: chunk.headingHierarchy,
              diataxisType: doc.diataxisType,
              tokenCount: chunk.tokenCount,
            })),
          );
          totalChunksCreated += chunks.length;
        }

        added++;
      } else if (isChanged && existing) {
        // Update existing document
        await db
          .update(documents)
          .set({
            title: doc.title,
            slug: doc.slug,
            description: doc.description,
            diataxisType: doc.diataxisType,
            rawContent: doc.rawContent,
            htmlContent: parsed.html,
            contentHash: hash,
            frontmatter: doc.frontmatter,
            draft: doc.draft,
            order: doc.order,
            wordCount: parsed.wordCount,
            updatedAt: new Date(),
          })
          .where(eq(documents.id, existing.id));

        // Delete old chunks and insert new ones
        await db
          .delete(documentChunks)
          .where(eq(documentChunks.documentId, existing.id));

        if (chunks.length > 0) {
          await db.insert(documentChunks).values(
            chunks.map((chunk) => ({
              documentId: existing.id,
              projectId,
              content: chunk.content,
              contentHash: sha256(chunk.content),
              chunkIndex: chunk.chunkIndex,
              headingHierarchy: chunk.headingHierarchy,
              diataxisType: doc.diataxisType,
              tokenCount: chunk.tokenCount,
            })),
          );
          totalChunksCreated += chunks.length;
        }

        updated++;
      }
    }

    // 6. Delete documents that no longer exist in the repo
    let deleted = 0;
    if (syncedPaths.length > 0) {
      const deletedDocs = await db
        .delete(documents)
        .where(
          and(
            eq(documents.projectId, projectId),
            notInArray(documents.path, syncedPaths),
          ),
        )
        .returning({ id: documents.id });
      deleted = deletedDocs.length;
      // Chunks cascade-deleted via foreign key
    } else {
      logger.warn({ projectId }, 'No documents discovered, skipping deletion');
    }

    // 7. Update project's last sync info
    await db
      .update(projects)
      .set({
        lastSyncCommitSha: commitSha,
        lastSyncedAt: new Date(),
      })
      .where(eq(projects.id, projectId));

    // 8. Update sync log
    await db
      .update(syncLogs)
      .set({
        status: 'completed',
        documentsAdded: added,
        documentsUpdated: updated,
        documentsDeleted: deleted,
        chunksCreated: totalChunksCreated,
        completedAt: new Date(),
      })
      .where(eq(syncLogs.id, syncLogId));

    logger.info(
      { projectId, added, updated, deleted, chunksCreated: totalChunksCreated },
      'Sync completed',
    );

    // Invalidate caches for this project
    await cacheInvalidate(`project:${projectId}:*`);

    return { added, updated, deleted, chunksCreated: totalChunksCreated };
  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : String(error);

    await db
      .update(syncLogs)
      .set({
        status: 'failed',
        error: errorMessage,
        completedAt: new Date(),
      })
      .where(eq(syncLogs.id, syncLogId));

    logger.error({ err: error, projectId }, 'Sync failed');
    throw error;
  } finally {
    // Release sync lock
    await releaseSyncLock(projectId);

    // Clean up temp directory
    if (tempDir) {
      await rm(tempDir, { recursive: true, force: true }).catch(() => {});
    }
  }
}
