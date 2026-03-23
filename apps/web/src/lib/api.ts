const API_URL = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:3001';

export type ApiDocument = {
  id: string;
  title: string;
  slug: string;
  path: string;
  description: string | null;
  diataxis_type: string;
  diataxisType: string;
  raw_content: string;
  rawContent: string;
  html_content: string | null;
  htmlContent: string | null;
  word_count: number | null;
  wordCount: number | null;
  updated_at: string;
  updatedAt: string;
  draft: boolean;
};

export type ApiProject = {
  id: string;
  name: string;
  slug: string;
  description: string | null;
  last_synced_at: string | null;
  lastSyncedAt: string | null;
};

export type SearchResult = {
  id: string;
  title: string;
  slug: string;
  path: string;
  description: string | null;
  diataxis_type: string;
  rank: number;
  snippet: string;
};

async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
    next: { revalidate: 60 },
  });

  if (!res.ok) {
    throw new Error(`API error: ${res.status} ${res.statusText}`);
  }

  return res.json();
}

export async function getProject(slug: string): Promise<ApiProject | null> {
  try {
    const res = await apiFetch<{ data: ApiProject }>(`/api/v1/public/projects/${slug}`);
    return res.data;
  } catch {
    return null;
  }
}

export async function getProjectDocuments(projectSlug: string): Promise<ApiDocument[]> {
  try {
    const res = await apiFetch<{ data: ApiDocument[] }>(
      `/api/v1/public/projects/${projectSlug}/documents`,
    );
    return res.data;
  } catch {
    return [];
  }
}

export async function getDocument(
  projectId: string,
  path: string,
): Promise<ApiDocument | null> {
  try {
    const res = await apiFetch<{ data: ApiDocument }>(
      `/api/v1/projects/${projectId}/documents/${encodeURIComponent(path)}`,
    );
    return res.data;
  } catch {
    return null;
  }
}

export async function searchDocuments(
  projectId: string,
  query: string,
  type?: string,
): Promise<{ data: SearchResult[]; facets: { diataxis_type: string; count: number }[] }> {
  const params = new URLSearchParams({ q: query });
  if (type) params.set('type', type);

  try {
    return await apiFetch(
      `/api/v1/projects/${projectId}/search?${params.toString()}`,
    );
  } catch {
    return { data: [], facets: [] };
  }
}
