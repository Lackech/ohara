import Link from 'next/link';

export default function Home() {
  return (
    <main className="flex min-h-screen flex-col">
      {/* Hero */}
      <section className="flex flex-col items-center justify-center px-6 py-24 text-center">
        <h1 className="max-w-3xl text-5xl font-bold tracking-tight sm:text-6xl">
          Documentation that
          <br />
          <span className="text-blue-600">agents actually use</span>
        </h1>
        <p className="mt-6 max-w-2xl text-lg text-gray-600 dark:text-gray-400">
          Ohara is an agent-optimized documentation platform built on Diataxis.
          Your agents read docs to become experts on your org — and write back to keep them current.
        </p>
        <div className="mt-8 flex gap-4">
          <Link
            href="/login"
            className="rounded-lg bg-blue-600 px-6 py-3 text-sm font-medium text-white hover:bg-blue-700"
          >
            Get Started
          </Link>
          <a
            href="https://github.com/ohara-project/ohara"
            className="rounded-lg border px-6 py-3 text-sm font-medium hover:bg-gray-50 dark:border-gray-700 dark:hover:bg-gray-800"
          >
            View on GitHub
          </a>
        </div>
      </section>

      {/* How it works */}
      <section className="border-t px-6 py-20 dark:border-gray-800">
        <div className="mx-auto max-w-5xl">
          <h2 className="text-center text-3xl font-bold">How it works</h2>
          <div className="mt-12 grid gap-8 sm:grid-cols-3">
            <div className="rounded-lg border p-6 dark:border-gray-800">
              <div className="text-2xl font-bold text-blue-600">1</div>
              <h3 className="mt-3 text-lg font-semibold">Connect your repo</h3>
              <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
                Install the GitHub App. Ohara syncs your markdown docs automatically on every push.
              </p>
            </div>
            <div className="rounded-lg border p-6 dark:border-gray-800">
              <div className="text-2xl font-bold text-blue-600">2</div>
              <h3 className="mt-3 text-lg font-semibold">Docs go live instantly</h3>
              <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
                Beautiful docs site with Diataxis sidebar, full-text search, and llms.txt — zero config.
              </p>
            </div>
            <div className="rounded-lg border p-6 dark:border-gray-800">
              <div className="text-2xl font-bold text-blue-600">3</div>
              <h3 className="mt-3 text-lg font-semibold">Agents become experts</h3>
              <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
                MCP server, REST API, and llms.txt let your AI agents search and retrieve docs with precision.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* Diataxis */}
      <section className="border-t px-6 py-20 dark:border-gray-800">
        <div className="mx-auto max-w-5xl">
          <h2 className="text-center text-3xl font-bold">Powered by Diataxis</h2>
          <p className="mx-auto mt-4 max-w-2xl text-center text-gray-600 dark:text-gray-400">
            Every document is typed. Agents get the right content for the right task.
          </p>
          <div className="mt-12 grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
            {[
              { type: 'Tutorials', desc: 'Learning-oriented walkthroughs', color: 'bg-green-50 dark:bg-green-900/20' },
              { type: 'How-to Guides', desc: 'Task-oriented step-by-step', color: 'bg-blue-50 dark:bg-blue-900/20' },
              { type: 'Reference', desc: 'Precise technical specs', color: 'bg-purple-50 dark:bg-purple-900/20' },
              { type: 'Explanation', desc: 'Conceptual understanding', color: 'bg-amber-50 dark:bg-amber-900/20' },
            ].map(({ type, desc, color }) => (
              <div key={type} className={`rounded-lg p-5 ${color}`}>
                <h3 className="font-semibold">{type}</h3>
                <p className="mt-1 text-sm text-gray-600 dark:text-gray-400">{desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Agent interfaces */}
      <section className="border-t px-6 py-20 dark:border-gray-800">
        <div className="mx-auto max-w-5xl">
          <h2 className="text-center text-3xl font-bold">Three ways agents consume</h2>
          <div className="mt-12 grid gap-8 sm:grid-cols-3">
            <div>
              <h3 className="text-lg font-semibold">MCP Server</h3>
              <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
                Tools, resources, and prompts. Claude Code searches and retrieves docs natively.
              </p>
            </div>
            <div>
              <h3 className="text-lg font-semibold">REST API</h3>
              <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
                Full API with content negotiation. Request markdown or structured JSON with metadata.
              </p>
            </div>
            <div>
              <h3 className="text-lg font-semibold">llms.txt</h3>
              <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
                Diataxis-typed text file. Zero setup — any LLM can fetch and understand your docs.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="border-t px-6 py-20 dark:border-gray-800">
        <div className="mx-auto max-w-2xl text-center">
          <h2 className="text-3xl font-bold">Ready to make your docs agent-ready?</h2>
          <p className="mt-4 text-gray-600 dark:text-gray-400">
            Free for open source. Start in under 5 minutes.
          </p>
          <Link
            href="/login"
            className="mt-6 inline-block rounded-lg bg-blue-600 px-8 py-3 text-sm font-medium text-white hover:bg-blue-700"
          >
            Get Started Free
          </Link>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t px-6 py-8 dark:border-gray-800">
        <div className="mx-auto flex max-w-5xl items-center justify-between text-sm text-gray-500">
          <span>Ohara — Agent-optimized documentation</span>
          <div className="flex gap-4">
            <a href="https://github.com/ohara-project/ohara" className="hover:text-gray-900 dark:hover:text-gray-300">GitHub</a>
            <Link href="/docs" className="hover:text-gray-900 dark:hover:text-gray-300">Docs</Link>
          </div>
        </div>
      </footer>
    </main>
  );
}
