import { z } from 'zod';
import { DIATAXIS_TYPES } from '../diataxis/types';

/**
 * Schema for ohara.yaml — per-project configuration file.
 * Lives at the root of a documentation project in Git.
 */
export const projectConfigSchema = z.object({
  /** Display name for the project */
  name: z.string().min(1, 'Project name is required'),

  /** Short description */
  description: z.string().optional(),

  /** Documentation root directory relative to repo root (default: '.') */
  docs_dir: z.string().optional().default('.'),

  /** Custom directory mapping (overrides default Diataxis directory names) */
  directories: z
    .object({
      tutorials: z.string().optional().default('tutorials'),
      guides: z.string().optional().default('guides'),
      reference: z.string().optional().default('reference'),
      explanation: z.string().optional().default('explanation'),
    })
    .optional()
    .default({}),

  /** File patterns to include (glob) */
  include: z.array(z.string()).optional().default(['**/*.md', '**/*.mdx']),

  /** File patterns to exclude (glob) */
  exclude: z.array(z.string()).optional().default(['node_modules/**', '.git/**', 'dist/**']),

  /** Default Diataxis type if not specified in frontmatter */
  default_type: z.enum(DIATAXIS_TYPES).optional(),

  /** Navigation configuration */
  navigation: z
    .object({
      /** Group documents by Diataxis type in sidebar */
      group_by_type: z.boolean().optional().default(true),
      /** Custom sidebar ordering */
      order: z.array(z.string()).optional(),
    })
    .optional()
    .default({}),

  /** Base URL for the project (used in llms.txt generation) */
  base_url: z.string().url().optional(),

  /** Version identifier */
  version: z.string().optional(),
});

export type ProjectConfig = z.infer<typeof projectConfigSchema>;
