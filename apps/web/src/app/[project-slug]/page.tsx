import { getProject, getProjectDocuments } from '@/lib/api';
import { DIATAXIS_LABELS } from '@ohara/shared';
import { notFound } from 'next/navigation';
import Link from 'next/link';

export default async function ProjectPage({
  params,
}: {
  params: Promise<{ 'project-slug': string }>;
}) {
  const { 'project-slug': projectSlug } = await params;
  const project = await getProject(projectSlug);
  if (!project) notFound();

  const documents = await getProjectDocuments(projectSlug);

  const typeOrder = ['tutorial', 'guide', 'reference', 'explanation'] as const;
  const grouped = new Map<string, typeof documents>();
  for (const type of typeOrder) grouped.set(type, []);
  for (const doc of documents) {
    const type = doc.diataxisType ?? doc.diataxis_type;
    grouped.get(type)?.push(doc);
  }

  return (
    <main className="container max-w-4xl py-12">
      <h1 className="text-3xl font-bold">{project.name}</h1>
      {project.description && (
        <p className="mt-2 text-lg text-gray-600 dark:text-gray-400">
          {project.description}
        </p>
      )}

      <div className="mt-10 grid gap-8 sm:grid-cols-2">
        {typeOrder.map((type) => {
          const docs = grouped.get(type) ?? [];
          if (docs.length === 0) return null;
          return (
            <div key={type} className="rounded-lg border p-6">
              <h2 className="text-lg font-semibold">
                {DIATAXIS_LABELS[type]}
              </h2>
              <p className="mt-1 text-sm text-gray-500">
                {docs.length} {docs.length === 1 ? 'document' : 'documents'}
              </p>
              <ul className="mt-4 space-y-2">
                {docs.slice(0, 5).map((doc) => (
                  <li key={doc.id}>
                    <Link
                      href={`/${projectSlug}/${doc.slug}`}
                      className="text-blue-600 hover:underline dark:text-blue-400"
                    >
                      {doc.title}
                    </Link>
                  </li>
                ))}
                {docs.length > 5 && (
                  <li className="text-sm text-gray-500">
                    +{docs.length - 5} more
                  </li>
                )}
              </ul>
            </div>
          );
        })}
      </div>
    </main>
  );
}
