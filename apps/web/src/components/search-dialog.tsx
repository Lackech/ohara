'use client';

import { useState, useCallback, useEffect, useRef } from 'react';
import { Command } from 'cmdk';
import { useRouter } from 'next/navigation';
import { Search, FileText, X } from 'lucide-react';
import { DIATAXIS_LABELS } from '@ohara/shared';

type SearchResult = {
  id: string;
  title: string;
  slug: string;
  diataxis_type: string;
  snippet: string;
};

type Facet = {
  diataxis_type: string;
  count: number;
};

const TYPE_COLORS: Record<string, string> = {
  tutorial: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300',
  guide: 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300',
  reference: 'bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-300',
  explanation: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
};

export function SearchDialog({
  projectId,
  projectSlug,
}: {
  projectId: string;
  projectSlug: string;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [facets, setFacets] = useState<Facet[]>([]);
  const [typeFilter, setTypeFilter] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const router = useRouter();
  const debounceRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  // cmd+k shortcut
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        setOpen((o) => !o);
      }
    };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, []);

  const doSearch = useCallback(
    async (q: string, type: string | null) => {
      if (!q.trim()) {
        setResults([]);
        setFacets([]);
        return;
      }

      setLoading(true);
      try {
        const params = new URLSearchParams({ q });
        if (type) params.set('type', type);

        const apiUrl = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:3001';
        const res = await fetch(
          `${apiUrl}/api/v1/projects/${projectId}/search?${params}`,
        );
        const json = await res.json();
        setResults(json.data ?? []);
        setFacets(json.facets ?? []);
      } catch {
        setResults([]);
      } finally {
        setLoading(false);
      }
    },
    [projectId],
  );

  const handleQueryChange = useCallback(
    (value: string) => {
      setQuery(value);
      clearTimeout(debounceRef.current);
      debounceRef.current = setTimeout(() => doSearch(value, typeFilter), 200);
    },
    [doSearch, typeFilter],
  );

  const handleTypeFilter = useCallback(
    (type: string) => {
      const newType = typeFilter === type ? null : type;
      setTypeFilter(newType);
      doSearch(query, newType);
    },
    [typeFilter, query, doSearch],
  );

  if (!open) {
    return (
      <button
        onClick={() => setOpen(true)}
        className="flex items-center gap-2 rounded-lg border px-3 py-2 text-sm text-gray-500 transition-colors hover:bg-gray-50 dark:border-gray-700 dark:hover:bg-gray-800"
      >
        <Search className="h-4 w-4" />
        <span>Search docs...</span>
        <kbd className="ml-2 hidden rounded bg-gray-100 px-1.5 py-0.5 text-xs font-mono dark:bg-gray-800 sm:inline">
          ⌘K
        </kbd>
      </button>
    );
  }

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center pt-[15vh]">
      <div
        className="fixed inset-0 bg-black/50"
        onClick={() => setOpen(false)}
      />
      <div className="relative w-full max-w-xl rounded-xl border bg-white shadow-2xl dark:border-gray-700 dark:bg-gray-900">
        <Command shouldFilter={false}>
          <div className="flex items-center border-b px-4 dark:border-gray-700">
            <Search className="mr-2 h-4 w-4 shrink-0 text-gray-400" />
            <Command.Input
              value={query}
              onValueChange={handleQueryChange}
              placeholder="Search documentation..."
              className="flex-1 bg-transparent py-3 text-sm outline-none"
            />
            <button onClick={() => setOpen(false)}>
              <X className="h-4 w-4 text-gray-400" />
            </button>
          </div>

          {/* Type filter chips */}
          {facets.length > 0 && (
            <div className="flex gap-2 border-b px-4 py-2 dark:border-gray-700">
              {facets.map((f) => (
                <button
                  key={f.diataxis_type}
                  onClick={() => handleTypeFilter(f.diataxis_type)}
                  className={`rounded-full px-2.5 py-0.5 text-xs font-medium transition-colors ${
                    typeFilter === f.diataxis_type
                      ? TYPE_COLORS[f.diataxis_type] ?? 'bg-gray-200'
                      : 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400'
                  }`}
                >
                  {DIATAXIS_LABELS[f.diataxis_type as keyof typeof DIATAXIS_LABELS] ?? f.diataxis_type}{' '}
                  ({f.count})
                </button>
              ))}
            </div>
          )}

          <Command.List className="max-h-80 overflow-y-auto p-2">
            {loading && (
              <Command.Loading>
                <div className="p-4 text-center text-sm text-gray-500">
                  Searching...
                </div>
              </Command.Loading>
            )}

            <Command.Empty className="p-4 text-center text-sm text-gray-500">
              {query ? 'No results found.' : 'Start typing to search...'}
            </Command.Empty>

            {results.map((result) => (
              <Command.Item
                key={result.id}
                value={result.id}
                onSelect={() => {
                  router.push(`/${projectSlug}/${result.slug}`);
                  setOpen(false);
                }}
                className="flex cursor-pointer items-start gap-3 rounded-lg p-3 text-sm hover:bg-gray-100 dark:hover:bg-gray-800"
              >
                <FileText className="mt-0.5 h-4 w-4 shrink-0 text-gray-400" />
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="font-medium">{result.title}</span>
                    <span
                      className={`rounded px-1.5 py-0.5 text-[10px] font-medium ${TYPE_COLORS[result.diataxis_type] ?? ''}`}
                    >
                      {DIATAXIS_LABELS[result.diataxis_type as keyof typeof DIATAXIS_LABELS] ?? result.diataxis_type}
                    </span>
                  </div>
                  <div
                    className="mt-1 line-clamp-2 text-gray-500"
                    dangerouslySetInnerHTML={{ __html: result.snippet }}
                  />
                </div>
              </Command.Item>
            ))}
          </Command.List>
        </Command>
      </div>
    </div>
  );
}
