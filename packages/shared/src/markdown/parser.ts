import { unified } from 'unified';
import remarkParse from 'remark-parse';
import remarkGfm from 'remark-gfm';
import remarkFrontmatter from 'remark-frontmatter';
import remarkRehype from 'remark-rehype';
import rehypeSlug from 'rehype-slug';
import rehypeStringify from 'rehype-stringify';
import type { Root, Heading, Text, PhrasingContent } from 'mdast';

export type ParsedDocument = {
  html: string;
  headings: ExtractedHeading[];
  wordCount: number;
};

export type ExtractedHeading = {
  depth: number;
  text: string;
  id: string;
};

/**
 * Extract plain text from mdast phrasing content nodes.
 */
function phrasingToText(nodes: PhrasingContent[]): string {
  return nodes
    .map((node) => {
      if (node.type === 'text') return (node as Text).value;
      if ('children' in node) return phrasingToText(node.children as PhrasingContent[]);
      return '';
    })
    .join('');
}

/**
 * Generate a slug from heading text.
 */
function slugify(text: string): string {
  return text
    .toLowerCase()
    .replace(/[^\w\s-]/g, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .trim();
}

/**
 * Extract headings from a markdown AST.
 */
function extractHeadings(tree: Root): ExtractedHeading[] {
  const headings: ExtractedHeading[] = [];

  function walk(node: Root | Root['children'][number]) {
    if (node.type === 'heading') {
      const heading = node as Heading;
      const text = phrasingToText(heading.children);
      headings.push({
        depth: heading.depth,
        text,
        id: slugify(text),
      });
    }
    if ('children' in node) {
      for (const child of (node as Root).children) {
        walk(child);
      }
    }
  }

  walk(tree);
  return headings;
}

/**
 * Count words in markdown content (strips frontmatter).
 */
function countWords(content: string): number {
  // Remove frontmatter
  const stripped = content.replace(/^---[\s\S]*?---\s*/, '');
  return stripped.split(/\s+/).filter(Boolean).length;
}

/**
 * Parse markdown/MDX content into HTML with extracted metadata.
 * This pipeline is shared between the API (sync) and frontend (rendering).
 */
export async function parseMarkdown(content: string): Promise<ParsedDocument> {
  const remarkProcessor = unified()
    .use(remarkParse)
    .use(remarkFrontmatter, ['yaml'])
    .use(remarkGfm);

  const mdast = remarkProcessor.parse(content);
  const headings = extractHeadings(mdast as Root);

  const htmlProcessor = remarkProcessor()
    .use(remarkRehype, { allowDangerousHtml: true })
    .use(rehypeSlug)
    .use(rehypeStringify, { allowDangerousHtml: true });

  const result = await htmlProcessor.process(content);

  return {
    html: String(result),
    headings,
    wordCount: countWords(content),
  };
}
