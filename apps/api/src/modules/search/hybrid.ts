import { sql, eq } from 'drizzle-orm';
import { db } from '../../db/index.js';
import { documents, documentChunks } from '../../db/schema.js';
import { semanticSearch } from './embeddings.js';
import { RRF_K } from '@ohara/shared';
import { inferDiataxisType } from '@ohara/shared';

type HybridResult = {
  documentId: string;
  title: string;
  slug: string;
  path: string;
  description: string | null;
  diataxisType: string;
  score: number;
  snippet: string;
  matchType: 'full_text' | 'semantic' | 'both';
};

/**
 * Full-text search using PostgreSQL tsvector.
 */
async function fullTextSearch(
  query: string,
  projectId: string,
  limit: number,
  diataxisType?: string,
) {
  const typeFilter = diataxisType
    ? sql`AND ${documents.diataxisType} = ${diataxisType}`
    : sql``;

  const results = await db.execute(sql`
    SELECT
      ${documents.id} as id,
      ${documents.title} as title,
      ${documents.slug} as slug,
      ${documents.path} as path,
      ${documents.description} as description,
      ${documents.diataxisType} as diataxis_type,
      ts_rank(${documents.searchVector}, plainto_tsquery('english', ${query})) as rank,
      ts_headline(
        'english',
        ${documents.rawContent},
        plainto_tsquery('english', ${query}),
        'StartSel=<mark>, StopSel=</mark>, MaxWords=35, MinWords=15, MaxFragments=2'
      ) as snippet
    FROM ${documents}
    WHERE ${documents.projectId} = ${projectId}
      AND ${documents.searchVector} @@ plainto_tsquery('english', ${query})
      ${typeFilter}
    ORDER BY rank DESC
    LIMIT ${limit}
  `);

  return Array.from(results);
}

/**
 * Hybrid search combining full-text and semantic results with RRF.
 *
 * Reciprocal Rank Fusion formula: score = Σ 1/(k + rank)
 * where k is a constant (default 60) that controls the impact of high-ranked results.
 */
export async function hybridSearch(
  query: string,
  projectId: string,
  options: { limit?: number; diataxisType?: string } = {},
): Promise<HybridResult[]> {
  const limit = options.limit ?? 10;
  const fetchLimit = limit * 2; // Fetch more to allow for deduplication

  // Diataxis-aware boosting: detect query intent
  const inferredType = options.diataxisType ?? inferDiataxisType(query) ?? undefined;

  // Run both searches in parallel
  const hasOpenAIKey = !!process.env.OPENAI_API_KEY;

  const [ftResults, semanticResults] = await Promise.all([
    fullTextSearch(query, projectId, fetchLimit, inferredType),
    hasOpenAIKey
      ? semanticSearch(query, projectId, fetchLimit, inferredType).catch(() => [])
      : Promise.resolve([]),
  ]);

  // RRF scoring
  const scores = new Map<
    string,
    { score: number; matchType: 'full_text' | 'semantic' | 'both'; data: Record<string, unknown> }
  >();

  // Score full-text results
  ftResults.forEach((doc: Record<string, unknown>, rank: number) => {
    const id = doc.id as string;
    const existing = scores.get(id);
    const rrfScore = 1 / (RRF_K + rank + 1);

    if (existing) {
      existing.score += rrfScore;
      existing.matchType = 'both';
    } else {
      scores.set(id, { score: rrfScore, matchType: 'full_text', data: doc });
    }
  });

  // Score semantic results (map chunk documentId to document)
  const seenDocIds = new Set<string>();
  semanticResults.forEach((chunk, rank) => {
    const docId = chunk.documentId;
    if (seenDocIds.has(docId)) return; // One score per document
    seenDocIds.add(docId);

    const rrfScore = 1 / (RRF_K + rank + 1);
    const existing = scores.get(docId);

    if (existing) {
      existing.score += rrfScore;
      existing.matchType = 'both';
    } else {
      // Need to fetch document metadata for semantic-only results
      scores.set(docId, {
        score: rrfScore,
        matchType: 'semantic',
        data: {
          id: docId,
          snippet: chunk.content.slice(0, 200) + '...',
          diataxis_type: chunk.diataxisType,
        },
      });
    }
  });

  // Sort by RRF score and take top results
  const sorted = Array.from(scores.entries())
    .sort(([, a], [, b]) => b.score - a.score)
    .slice(0, limit);

  // For semantic-only results, fetch document metadata
  const results: HybridResult[] = [];
  for (const [docId, entry] of sorted) {
    let title = entry.data.title as string;
    let slug = entry.data.slug as string;
    let path = entry.data.path as string;
    let description = entry.data.description as string | null;

    if (!title && entry.matchType === 'semantic') {
      // Fetch from DB
      const [doc] = await db
        .select()
        .from(documents)
        .where(eq(documents.id, docId))
        .limit(1);
      if (doc) {
        title = doc.title;
        slug = doc.slug;
        path = doc.path;
        description = doc.description;
      }
    }

    results.push({
      documentId: docId,
      title: title ?? '',
      slug: slug ?? '',
      path: path ?? '',
      description: description ?? null,
      diataxisType: (entry.data.diataxis_type as string) ?? '',
      score: entry.score,
      snippet: (entry.data.snippet as string) ?? '',
      matchType: entry.matchType,
    });
  }

  return results;
}
