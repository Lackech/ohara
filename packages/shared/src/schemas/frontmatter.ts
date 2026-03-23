import { z } from 'zod';
import { DIATAXIS_TYPES } from '../diataxis/types';

/**
 * Zod schema for document frontmatter.
 * Validates YAML frontmatter in markdown/MDX files.
 */
export const frontmatterSchema = z.object({
  title: z.string().min(1, 'Title is required'),
  description: z.string().optional(),
  diataxis_type: z.enum(DIATAXIS_TYPES).optional(),
  tags: z.array(z.string()).optional().default([]),
  author: z.string().optional(),
  created_at: z.coerce.date().optional(),
  updated_at: z.coerce.date().optional(),
  draft: z.boolean().optional().default(false),
  order: z.number().int().optional(),
  slug: z.string().optional(),
  // For reference docs: link to source code or API
  source: z.string().url().optional(),
  // For tutorials/guides: estimated completion time in minutes
  duration: z.number().positive().optional(),
  // For tutorials: difficulty level
  difficulty: z.enum(['beginner', 'intermediate', 'advanced']).optional(),
  // Prerequisites (references to other docs by path)
  prerequisites: z.array(z.string()).optional().default([]),
});

export type Frontmatter = z.infer<typeof frontmatterSchema>;

/**
 * Lenient version that doesn't fail on extra fields.
 * Used during sync to preserve unknown frontmatter keys.
 */
export const frontmatterSchemaLenient = frontmatterSchema.passthrough();
