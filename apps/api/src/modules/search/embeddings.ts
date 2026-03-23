import { createHash } from 'crypto';
import { eq, and, isNull } from 'drizzle-orm';
import { db } from '../../db/index.js';
import { documentChunks } from '../../db/schema.js';
import { logger } from '../../lib/logger.js';
import { EMBEDDING_DIMENSIONS } from '@ohara/shared';

/**
 * Generate embeddings using OpenAI text-embedding-3-small.
 * Uses the Vercel AI SDK pattern for provider-agnostic embedding.
 */
async function generateEmbedding(text: string): Promise<number[]> {
  const apiKey = process.env.OPENAI_API_KEY;
  if (!apiKey) {
    throw new Error('OPENAI_API_KEY required for embeddings');
  }

  const res = await fetch('https://api.openai.com/v1/embeddings', {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${apiKey}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      model: 'text-embedding-3-small',
      input: text,
      dimensions: EMBEDDING_DIMENSIONS,
    }),
  });

  if (!res.ok) {
    const err = await res.text();
    throw new Error(`OpenAI embedding failed: ${res.status} ${err}`);
  }

  const json = (await res.json()) as { data: { embedding: number[] }[] };
  return json.data[0].embedding;
}

/**
 * Embed all un-embedded chunks for a project.
 * Checks content hash to skip chunks that haven't changed.
 */
export async function embedProjectChunks(projectId: string): Promise<number> {
  // Find chunks without embeddings
  const chunks = await db
    .select({ id: documentChunks.id, content: documentChunks.content })
    .from(documentChunks)
    .where(
      and(
        eq(documentChunks.projectId, projectId),
        isNull(documentChunks.embedding),
      ),
    );

  if (chunks.length === 0) return 0;

  logger.info({ projectId, chunkCount: chunks.length }, 'Embedding chunks');

  let embedded = 0;

  // Process in batches of 20
  const batchSize = 20;
  for (let i = 0; i < chunks.length; i += batchSize) {
    const batch = chunks.slice(i, i + batchSize);

    const embeddings = await Promise.all(
      batch.map((chunk) => generateEmbedding(chunk.content)),
    );

    for (let j = 0; j < batch.length; j++) {
      await db
        .update(documentChunks)
        .set({ embedding: embeddings[j] })
        .where(eq(documentChunks.id, batch[j].id));
      embedded++;
    }
  }

  logger.info({ projectId, embedded }, 'Embedding complete');
  return embedded;
}

/**
 * Semantic search: find the nearest chunks to a query embedding.
 */
export async function semanticSearch(
  queryText: string,
  projectId: string,
  limit: number = 20,
  diataxisType?: string,
): Promise<{ id: string; documentId: string; content: string; similarity: number; diataxisType: string; headingHierarchy: string[] }[]> {
  const queryEmbedding = await generateEmbedding(queryText);
  const embeddingStr = `[${queryEmbedding.join(',')}]`;

  const typeFilter = diataxisType
    ? `AND diataxis_type = '${diataxisType}'`
    : '';

  const results = await db.execute<{
    id: string;
    document_id: string;
    content: string;
    similarity: number;
    diataxis_type: string;
    heading_hierarchy: string[];
  }>(
    // Raw SQL for pgvector cosine distance
    {
      sql: `
        SELECT
          id,
          document_id,
          content,
          1 - (embedding <=> $1::vector) as similarity,
          diataxis_type,
          heading_hierarchy
        FROM document_chunks
        WHERE project_id = $2
          AND embedding IS NOT NULL
          ${typeFilter}
        ORDER BY embedding <=> $1::vector
        LIMIT $3
      `,
      params: [embeddingStr, projectId, limit],
    } as never,
  );

  return Array.from(results).map((r: Record<string, unknown>) => ({
    id: r.id as string,
    documentId: r.document_id as string,
    content: r.content as string,
    similarity: r.similarity as number,
    diataxisType: r.diataxis_type as string,
    headingHierarchy: r.heading_hierarchy as string[],
  }));
}
