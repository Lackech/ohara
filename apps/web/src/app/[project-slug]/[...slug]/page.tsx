import { getProject, getProjectDocuments } from '@/lib/api';
import { DocsPage, DocsBody, DocsTitle, DocsDescription } from 'fumadocs-ui/page';
import { notFound } from 'next/navigation';
import { DIATAXIS_LABELS } from '@ohara/shared';

export default async function DocumentPage({
  params,
}: {
  params: Promise<{ 'project-slug': string; slug: string[] }>;
}) {
  const { 'project-slug': projectSlug, slug } = await params;
  const project = await getProject(projectSlug);
  if (!project) notFound();

  const docSlug = slug.join('/');

  // Find document by matching slug
  const documents = await getProjectDocuments(projectSlug);
  const doc = documents.find((d) => d.slug === docSlug);
  if (!doc) notFound();

  const diataxisType = (doc.diataxisType ?? doc.diataxis_type) as keyof typeof DIATAXIS_LABELS;
  const htmlContent = doc.htmlContent ?? doc.html_content;

  return (
    <DocsPage>
      <DocsTitle>{doc.title}</DocsTitle>
      {doc.description && <DocsDescription>{doc.description}</DocsDescription>}
      <div className="mb-4 flex gap-2">
        <span className="inline-flex items-center rounded-md bg-blue-50 px-2 py-1 text-xs font-medium text-blue-700 ring-1 ring-blue-700/10 ring-inset dark:bg-blue-400/10 dark:text-blue-400 dark:ring-blue-400/30">
          {DIATAXIS_LABELS[diataxisType] ?? diataxisType}
        </span>
      </div>
      <DocsBody>
        {htmlContent ? (
          <div dangerouslySetInnerHTML={{ __html: htmlContent }} />
        ) : (
          <pre className="whitespace-pre-wrap">{doc.rawContent ?? doc.raw_content}</pre>
        )}
      </DocsBody>
    </DocsPage>
  );
}
