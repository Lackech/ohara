import type { PageTree } from 'fumadocs-core/server';
import { DIATAXIS_LABELS } from '@ohara/shared';
import type { ApiDocument } from './api';

type DiataxisType = 'tutorial' | 'guide' | 'reference' | 'explanation';

/**
 * Build a Fumadocs page tree from API documents, grouped by Diataxis type.
 */
export function buildPageTree(
  documents: ApiDocument[],
  projectSlug: string,
): PageTree.Root {
  const typeOrder: DiataxisType[] = ['tutorial', 'guide', 'reference', 'explanation'];

  const grouped = new Map<DiataxisType, ApiDocument[]>();
  for (const type of typeOrder) {
    grouped.set(type, []);
  }

  for (const doc of documents) {
    const type = (doc.diataxisType ?? doc.diataxis_type) as DiataxisType;
    const group = grouped.get(type);
    if (group) group.push(doc);
  }

  const children: PageTree.Node[] = [];

  for (const type of typeOrder) {
    const docs = grouped.get(type) ?? [];
    if (docs.length === 0) continue;

    const folderChildren: PageTree.Item[] = docs.map((doc) => ({
      type: 'page' as const,
      name: doc.title,
      url: `/${projectSlug}/${doc.slug}`,
    }));

    children.push({
      type: 'folder' as const,
      name: DIATAXIS_LABELS[type],
      children: folderChildren,
    });
  }

  return { name: 'Docs', children };
}
