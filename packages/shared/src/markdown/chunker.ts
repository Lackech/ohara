import { CHUNK_MIN_TOKENS, CHUNK_MAX_TOKENS } from '../constants';

export type DocumentChunkData = {
  content: string;
  chunkIndex: number;
  headingHierarchy: string[];
  tokenCount: number;
};

/**
 * Rough token count estimation (1 token ≈ 4 chars for English text).
 */
function estimateTokens(text: string): number {
  return Math.ceil(text.length / 4);
}

/**
 * Split markdown content into chunks at heading boundaries.
 *
 * Strategy:
 * 1. Split on h2/h3 boundaries (structural chunking)
 * 2. Within long sections, split on paragraph boundaries at ~500-800 tokens
 * 3. Each chunk carries its heading hierarchy for retrieval context
 */
export function chunkDocument(
  rawContent: string,
  documentTitle: string,
): DocumentChunkData[] {
  // Strip frontmatter
  const content = rawContent.replace(/^---[\s\S]*?---\s*/, '');

  const lines = content.split('\n');
  const sections: { headingHierarchy: string[]; lines: string[] }[] = [];

  // Track heading hierarchy
  const headingStack: { depth: number; text: string }[] = [
    { depth: 1, text: documentTitle },
  ];
  let currentLines: string[] = [];

  function flushSection() {
    const text = currentLines.join('\n').trim();
    if (text) {
      sections.push({
        headingHierarchy: headingStack.map((h) => h.text),
        lines: [...currentLines],
      });
    }
    currentLines = [];
  }

  for (const line of lines) {
    const headingMatch = line.match(/^(#{2,3})\s+(.+)$/);

    if (headingMatch) {
      // Flush current section before starting a new one
      flushSection();

      const depth = headingMatch[1].length;
      const text = headingMatch[2].trim();

      // Pop headings at same or deeper level
      while (headingStack.length > 1 && headingStack[headingStack.length - 1].depth >= depth) {
        headingStack.pop();
      }
      headingStack.push({ depth, text });
    }

    currentLines.push(line);
  }

  // Flush final section
  flushSection();

  // If no sections were created, treat the whole document as one chunk
  if (sections.length === 0 && content.trim()) {
    sections.push({
      headingHierarchy: [documentTitle],
      lines: content.split('\n'),
    });
  }

  // Split oversized sections at paragraph boundaries
  const chunks: DocumentChunkData[] = [];
  let chunkIndex = 0;

  for (const section of sections) {
    const sectionText = section.lines.join('\n').trim();
    const tokens = estimateTokens(sectionText);

    if (tokens <= CHUNK_MAX_TOKENS) {
      // Section fits in one chunk
      if (sectionText) {
        chunks.push({
          content: sectionText,
          chunkIndex: chunkIndex++,
          headingHierarchy: [...section.headingHierarchy],
          tokenCount: tokens,
        });
      }
    } else {
      // Split at paragraph boundaries (double newline)
      const paragraphs = sectionText.split(/\n\n+/);
      let currentChunk = '';
      let currentTokens = 0;

      for (const para of paragraphs) {
        const paraTokens = estimateTokens(para);

        if (currentTokens + paraTokens > CHUNK_MAX_TOKENS && currentChunk) {
          // Flush current chunk
          chunks.push({
            content: currentChunk.trim(),
            chunkIndex: chunkIndex++,
            headingHierarchy: [...section.headingHierarchy],
            tokenCount: currentTokens,
          });
          currentChunk = '';
          currentTokens = 0;
        }

        currentChunk += (currentChunk ? '\n\n' : '') + para;
        currentTokens += paraTokens;
      }

      // Flush remaining content
      if (currentChunk.trim()) {
        // If the remaining chunk is too small, merge with previous
        if (currentTokens < CHUNK_MIN_TOKENS && chunks.length > 0) {
          const prev = chunks[chunks.length - 1];
          prev.content += '\n\n' + currentChunk.trim();
          prev.tokenCount += currentTokens;
        } else {
          chunks.push({
            content: currentChunk.trim(),
            chunkIndex: chunkIndex++,
            headingHierarchy: [...section.headingHierarchy],
            tokenCount: currentTokens,
          });
        }
      }
    }
  }

  return chunks;
}
