import { DocsLayout } from 'fumadocs-ui/layouts/docs';
import type { ReactNode } from 'react';
import { getProject, getProjectDocuments } from '@/lib/api';
import { buildPageTree } from '@/lib/source';
import { notFound } from 'next/navigation';

export default async function ProjectLayout({
  children,
  params,
}: {
  children: ReactNode;
  params: Promise<{ 'project-slug': string }>;
}) {
  const { 'project-slug': projectSlug } = await params;
  const project = await getProject(projectSlug);
  if (!project) notFound();

  const documents = await getProjectDocuments(projectSlug);
  const tree = buildPageTree(documents, projectSlug);

  return (
    <DocsLayout
      tree={tree}
      nav={{ title: project.name }}
    >
      {children}
    </DocsLayout>
  );
}
