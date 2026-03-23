import { readFile, readdir, stat } from 'fs/promises';
import { join, relative, extname, basename, dirname } from 'path';
import matter from 'gray-matter';
import {
  DIATAXIS_DIRECTORIES,
  SUPPORTED_EXTENSIONS,
  MAX_DOCUMENT_SIZE,
  CONFIG_FILE_NAMES,
  type DiataxisType,
  projectConfigSchema,
  type ProjectConfig,
} from '@ohara/shared';

export type DiscoveredDocument = {
  path: string;
  title: string;
  slug: string;
  diataxisType: DiataxisType;
  rawContent: string;
  frontmatter: Record<string, unknown>;
  description?: string;
  draft: boolean;
  order?: number;
};

/**
 * Try to load ohara.yaml / ohara.yml from the repo root.
 */
export async function loadProjectConfig(repoDir: string): Promise<ProjectConfig | null> {
  for (const name of CONFIG_FILE_NAMES) {
    try {
      const content = await readFile(join(repoDir, name), 'utf-8');
      // Dynamic import yaml parser
      const { parse } = await import('yaml');
      const raw = parse(content);
      const result = projectConfigSchema.safeParse(raw);
      if (result.success) return result.data;
    } catch {
      // File doesn't exist, try next
    }
  }
  return null;
}

/**
 * Infer Diataxis type from a file's directory path.
 * Checks if the file is inside a known Diataxis directory.
 */
function inferDiataxisTypeFromPath(
  filePath: string,
  config: ProjectConfig | null,
): DiataxisType | null {
  const dirs = config?.directories;
  const dirMap: Record<string, DiataxisType> = {
    [dirs?.tutorials ?? 'tutorials']: 'tutorial',
    [dirs?.guides ?? 'guides']: 'guide',
    [dirs?.reference ?? 'reference']: 'reference',
    [dirs?.explanation ?? 'explanation']: 'explanation',
  };

  // Also check the default names from DIATAXIS_DIRECTORIES
  for (const [type, dirName] of Object.entries(DIATAXIS_DIRECTORIES)) {
    dirMap[dirName] = type as DiataxisType;
  }

  const parts = filePath.split('/');
  for (const part of parts) {
    if (part in dirMap) {
      return dirMap[part];
    }
  }

  return null;
}

/**
 * Generate a URL-safe slug from a file path.
 */
function pathToSlug(filePath: string): string {
  return filePath
    .replace(/\.(mdx?|markdown)$/i, '')
    .replace(/\/index$/, '')
    .replace(/[^a-z0-9/.-]/gi, '-')
    .replace(/-+/g, '-')
    .toLowerCase();
}

/**
 * Extract title from frontmatter or first heading.
 */
function extractTitle(frontmatter: Record<string, unknown>, content: string, filePath: string): string {
  if (typeof frontmatter.title === 'string' && frontmatter.title) {
    return frontmatter.title;
  }

  // Try first H1 heading
  const h1Match = content.match(/^#\s+(.+)$/m);
  if (h1Match) return h1Match[1].trim();

  // Fall back to filename
  return basename(filePath, extname(filePath))
    .replace(/[-_]/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase());
}

/**
 * Walk a directory tree and discover all documentation files.
 */
export async function walkFileTree(
  repoDir: string,
  docsDir: string,
  config: ProjectConfig | null,
): Promise<DiscoveredDocument[]> {
  const rootDir = join(repoDir, docsDir);
  const documents: DiscoveredDocument[] = [];
  const includeExts = new Set(SUPPORTED_EXTENSIONS);
  const defaultType = config?.default_type ?? 'explanation';

  const excludeDirs = new Set([
    'node_modules',
    '.git',
    'dist',
    '.next',
    '.turbo',
    'assets',
    'images',
    'static',
  ]);

  async function walk(dir: string) {
    let entries;
    try {
      entries = await readdir(dir, { withFileTypes: true });
    } catch {
      return;
    }

    for (const entry of entries) {
      const fullPath = join(dir, entry.name);

      if (entry.isDirectory()) {
        if (excludeDirs.has(entry.name) || entry.name.startsWith('.')) continue;
        await walk(fullPath);
        continue;
      }

      if (!entry.isFile()) continue;

      const ext = extname(entry.name).toLowerCase();
      if (!includeExts.has(ext as (typeof SUPPORTED_EXTENSIONS)[number])) continue;

      // Check file size
      try {
        const stats = await stat(fullPath);
        if (stats.size > MAX_DOCUMENT_SIZE) continue;
      } catch {
        continue;
      }

      const rawContent = await readFile(fullPath, 'utf-8');
      const relPath = relative(rootDir, fullPath);

      // Parse frontmatter
      let frontmatter: Record<string, unknown> = {};
      let content = rawContent;
      try {
        const parsed = matter(rawContent);
        frontmatter = parsed.data as Record<string, unknown>;
        content = parsed.content;
      } catch {
        // If frontmatter parsing fails, use raw content
      }

      // Determine Diataxis type
      const fmType = frontmatter.diataxis_type as DiataxisType | undefined;
      const pathType = inferDiataxisTypeFromPath(relPath, config);
      const diataxisType = fmType ?? pathType ?? defaultType;

      const title = extractTitle(frontmatter, content, relPath);
      const slug = pathToSlug(relPath);

      documents.push({
        path: relPath,
        title,
        slug,
        diataxisType,
        rawContent,
        frontmatter,
        description: typeof frontmatter.description === 'string' ? frontmatter.description : undefined,
        draft: frontmatter.draft === true,
        order: typeof frontmatter.order === 'number' ? frontmatter.order : undefined,
      });
    }
  }

  await walk(rootDir);
  return documents;
}
