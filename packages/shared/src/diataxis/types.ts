/**
 * Diataxis documentation framework types.
 *
 * Diataxis organizes documentation into four types along two axes:
 * - Action (doing) vs Cognition (understanding)
 * - Study (learning) vs Work (applying)
 */

export const DIATAXIS_TYPES = ['tutorial', 'guide', 'reference', 'explanation'] as const;

export type DiataxisType = (typeof DIATAXIS_TYPES)[number];

/** Maps Diataxis types to their directory names in the repo */
export const DIATAXIS_DIRECTORIES: Record<DiataxisType, string> = {
  tutorial: 'tutorials',
  guide: 'guides',
  reference: 'reference',
  explanation: 'explanation',
} as const;

/** Human-readable labels for each Diataxis type */
export const DIATAXIS_LABELS: Record<DiataxisType, string> = {
  tutorial: 'Tutorial',
  guide: 'How-to Guide',
  reference: 'Reference',
  explanation: 'Explanation',
} as const;

/** Short descriptions of each Diataxis type for UI/agent context */
export const DIATAXIS_DESCRIPTIONS: Record<DiataxisType, string> = {
  tutorial: 'Learning-oriented guided experiences',
  guide: 'Task-oriented practical steps',
  reference: 'Information-oriented precise technical descriptions',
  explanation: 'Understanding-oriented conceptual discussions',
} as const;

/** Maps natural language query patterns to Diataxis types for boosting */
export const DIATAXIS_QUERY_PATTERNS: Record<DiataxisType, RegExp[]> = {
  guide: [/how (do|can|to|should)/i, /steps? (to|for)/i, /set ?up/i, /configure/i, /deploy/i],
  tutorial: [/getting started/i, /learn/i, /beginner/i, /introduction/i, /walkthrough/i],
  reference: [/api/i, /config(uration)?/i, /schema/i, /type(s)?/i, /parameter/i, /endpoint/i],
  explanation: [/why/i, /architecture/i, /design/i, /concept/i, /how .+ work/i, /overview/i],
} as const;

/**
 * Infer which Diataxis type a query is likely targeting based on language patterns.
 * Returns null if no strong signal is detected.
 */
export function inferDiataxisType(query: string): DiataxisType | null {
  for (const [type, patterns] of Object.entries(DIATAXIS_QUERY_PATTERNS)) {
    if (patterns.some((p) => p.test(query))) {
      return type as DiataxisType;
    }
  }
  return null;
}
