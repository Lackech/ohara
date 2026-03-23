import Link from 'next/link';

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:3001';

type Project = {
  id: string;
  name: string;
  slug: string;
  description: string | null;
  lastSyncedAt: string | null;
  last_synced_at: string | null;
};

async function getProjects(cookie: string): Promise<Project[]> {
  try {
    const res = await fetch(`${API_URL}/api/v1/projects`, {
      headers: { cookie },
      cache: 'no-store',
    });
    if (!res.ok) return [];
    const json = await res.json();
    return json.data ?? [];
  } catch {
    return [];
  }
}

export default async function DashboardPage() {
  // In production, forward the session cookie from the request
  const projects = await getProjects('');

  return (
    <main className="container max-w-4xl py-12">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">Projects</h1>
        <Link
          href="/dashboard/new"
          className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
        >
          Add Project
        </Link>
      </div>

      {projects.length === 0 ? (
        <div className="mt-12 text-center">
          <p className="text-lg text-gray-500">No projects yet.</p>
          <p className="mt-2 text-gray-400">
            Connect a GitHub repository to get started.
          </p>
        </div>
      ) : (
        <div className="mt-8 space-y-4">
          {projects.map((project) => {
            const synced = project.lastSyncedAt ?? project.last_synced_at;
            return (
              <Link
                key={project.id}
                href={`/${project.slug}`}
                className="block rounded-lg border p-6 transition-colors hover:bg-gray-50 dark:border-gray-700 dark:hover:bg-gray-800"
              >
                <h2 className="text-lg font-semibold">{project.name}</h2>
                {project.description && (
                  <p className="mt-1 text-sm text-gray-500">
                    {project.description}
                  </p>
                )}
                <div className="mt-3 flex items-center gap-4 text-xs text-gray-400">
                  <span>/{project.slug}</span>
                  {synced && (
                    <span>
                      Last synced:{' '}
                      {new Date(synced).toLocaleDateString()}
                    </span>
                  )}
                </div>
              </Link>
            );
          })}
        </div>
      )}
    </main>
  );
}
