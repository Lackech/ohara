'use client';

import { Suspense, useState, useEffect } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:3001';
const GITHUB_APP_URL = process.env.NEXT_PUBLIC_GITHUB_APP_URL ?? '#';

type Repo = {
  id: number;
  fullName: string;
  url: string;
  description: string | null;
  defaultBranch: string;
};

type Detection = {
  hasOharaYaml: boolean;
  hasDocsDir: boolean;
  hasReadme: boolean;
  hasDiataxis: boolean;
  suggestedDocsDir: string;
  detectedFiles: string[];
};

type Step = 'connect' | 'select' | 'configure' | 'syncing' | 'done';

export default function NewProjectPage() {
  return (
    <Suspense fallback={<div className="container max-w-2xl py-12"><p>Loading...</p></div>}>
      <NewProjectWizard />
    </Suspense>
  );
}

function NewProjectWizard() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const installationId = searchParams.get('installation_id');

  const [step, setStep] = useState<Step>(installationId ? 'select' : 'connect');
  const [repos, setRepos] = useState<Repo[]>([]);
  const [selectedRepo, setSelectedRepo] = useState<Repo | null>(null);
  const [detection, setDetection] = useState<Detection | null>(null);
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [docsDir, setDocsDir] = useState('.');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  // Fetch repos when we have an installation ID
  useEffect(() => {
    if (!installationId) return;

    setLoading(true);
    fetch(`${API_URL}/api/v1/github/repos?installation_id=${installationId}`, {
      credentials: 'include',
    })
      .then((r) => r.json())
      .then((data) => setRepos(data.data ?? []))
      .catch(() => setError('Failed to load repositories'))
      .finally(() => setLoading(false));
  }, [installationId]);

  async function handleSelectRepo(repo: Repo) {
    setSelectedRepo(repo);
    setName(repo.fullName.split('/')[1]);
    setSlug(repo.fullName.split('/')[1].toLowerCase().replace(/[^a-z0-9-]/g, '-'));
    setLoading(true);

    // Detect structure
    try {
      const res = await fetch(`${API_URL}/api/v1/github/detect`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          installation_id: installationId,
          repo_url: repo.url,
          branch: repo.defaultBranch,
        }),
      });
      const data = await res.json();
      setDetection(data.data);
      setDocsDir(data.data?.suggestedDocsDir ?? '.');
    } catch {
      setError('Failed to detect structure');
    }

    setLoading(false);
    setStep('configure');
  }

  async function handleSetup() {
    if (!selectedRepo || !installationId) return;

    setLoading(true);
    setStep('syncing');

    try {
      const res = await fetch(`${API_URL}/api/v1/github/setup`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          name,
          slug,
          repo_url: selectedRepo.url,
          branch: selectedRepo.defaultBranch,
          docs_dir: docsDir,
          installation_id: installationId,
        }),
      });

      if (!res.ok) {
        const err = await res.json();
        throw new Error(err.error ?? 'Setup failed');
      }

      setStep('done');
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Setup failed');
      setStep('configure');
    }

    setLoading(false);
  }

  return (
    <main className="container max-w-2xl py-12">
      <h1 className="text-2xl font-bold">Add Project</h1>

      {error && (
        <div className="mt-4 rounded-lg bg-red-50 p-4 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-400">
          {error}
        </div>
      )}

      {/* Step 1: Connect GitHub */}
      {step === 'connect' && (
        <div className="mt-8">
          <p className="text-gray-600 dark:text-gray-400">
            Install the Ohara GitHub App to connect your repositories.
          </p>
          <a
            href={GITHUB_APP_URL}
            className="mt-4 inline-block rounded-lg bg-gray-900 px-6 py-3 text-sm font-medium text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-100"
          >
            Install GitHub App
          </a>
        </div>
      )}

      {/* Step 2: Select Repository */}
      {step === 'select' && (
        <div className="mt-8">
          <p className="mb-4 text-gray-600 dark:text-gray-400">
            Select a repository to set up documentation for:
          </p>
          {loading ? (
            <p className="text-gray-500">Loading repositories...</p>
          ) : (
            <div className="space-y-2">
              {repos.map((repo) => (
                <button
                  key={repo.id}
                  onClick={() => handleSelectRepo(repo)}
                  className="w-full rounded-lg border p-4 text-left transition-colors hover:bg-gray-50 dark:border-gray-700 dark:hover:bg-gray-800"
                >
                  <div className="font-medium">{repo.fullName}</div>
                  {repo.description && (
                    <div className="mt-1 text-sm text-gray-500">{repo.description}</div>
                  )}
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Step 3: Configure */}
      {step === 'configure' && selectedRepo && (
        <div className="mt-8 space-y-6">
          <div className="rounded-lg border p-4 dark:border-gray-700">
            <div className="font-medium">{selectedRepo.fullName}</div>
            {detection && (
              <div className="mt-2 flex flex-wrap gap-2">
                {detection.hasOharaYaml && (
                  <span className="rounded bg-green-100 px-2 py-0.5 text-xs text-green-700">ohara.yaml found</span>
                )}
                {detection.hasDiataxis && (
                  <span className="rounded bg-blue-100 px-2 py-0.5 text-xs text-blue-700">Diataxis structure</span>
                )}
                {detection.hasDocsDir && (
                  <span className="rounded bg-purple-100 px-2 py-0.5 text-xs text-purple-700">docs/ directory</span>
                )}
              </div>
            )}
          </div>

          <div>
            <label className="block text-sm font-medium">Project Name</label>
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="mt-1 w-full rounded-lg border px-3 py-2 dark:border-gray-700 dark:bg-gray-800"
            />
          </div>

          <div>
            <label className="block text-sm font-medium">Slug</label>
            <input
              value={slug}
              onChange={(e) => setSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '-'))}
              className="mt-1 w-full rounded-lg border px-3 py-2 dark:border-gray-700 dark:bg-gray-800"
            />
            <p className="mt-1 text-xs text-gray-500">URL: /{slug}</p>
          </div>

          <div>
            <label className="block text-sm font-medium">Docs Directory</label>
            <input
              value={docsDir}
              onChange={(e) => setDocsDir(e.target.value)}
              className="mt-1 w-full rounded-lg border px-3 py-2 dark:border-gray-700 dark:bg-gray-800"
            />
            <p className="mt-1 text-xs text-gray-500">Relative to repo root. Use &quot;.&quot; for root.</p>
          </div>

          <button
            onClick={handleSetup}
            disabled={loading || !name || !slug}
            className="rounded-lg bg-blue-600 px-6 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
          >
            {loading ? 'Setting up...' : 'Create Project & Sync'}
          </button>
        </div>
      )}

      {/* Step 4: Syncing */}
      {step === 'syncing' && (
        <div className="mt-8 text-center">
          <div className="text-lg font-medium">Syncing documentation...</div>
          <p className="mt-2 text-gray-500">
            This usually takes less than a minute.
          </p>
        </div>
      )}

      {/* Step 5: Done */}
      {step === 'done' && (
        <div className="mt-8 text-center">
          <div className="text-lg font-medium text-green-600">Project created!</div>
          <p className="mt-2 text-gray-500">
            Your documentation is being synced. It will be available shortly.
          </p>
          <button
            onClick={() => router.push(`/${slug}`)}
            className="mt-4 rounded-lg bg-blue-600 px-6 py-2 text-sm font-medium text-white hover:bg-blue-700"
          >
            View Documentation
          </button>
        </div>
      )}
    </main>
  );
}
