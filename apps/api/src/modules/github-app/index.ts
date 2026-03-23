import { Elysia, t } from 'elysia';
import { eq } from 'drizzle-orm';
import { db } from '../../db/index.js';
import { projects } from '../../db/schema.js';
import { enqueueSyncJob } from '../../lib/queue.js';
import { withAuth } from '../../middleware/auth.js';
import { logger } from '../../lib/logger.js';

/**
 * GitHub App installation flow.
 * Handles the callback after a user installs the GitHub App on their repos.
 */
export const githubAppModule = new Elysia({ prefix: '/api/v1/github', name: 'github-app' })
  .use(withAuth())

  // Handle GitHub App installation callback
  .get(
    '/callback',
    async ({ query: q, set }) => {
      const { installation_id, setup_action } = q;

      if (setup_action === 'install' && installation_id) {
        logger.info({ installationId: installation_id }, 'GitHub App installed');

        // Redirect to the onboarding wizard with the installation ID
        set.redirect = `${process.env.WEB_URL ?? 'http://localhost:3000'}/dashboard/new?installation_id=${installation_id}`;
        set.status = 302;
        return;
      }

      set.redirect = `${process.env.WEB_URL ?? 'http://localhost:3000'}/dashboard`;
      set.status = 302;
    },
    {
      query: t.Object({
        installation_id: t.Optional(t.String()),
        setup_action: t.Optional(t.String()),
      }),
    },
  )

  // List repos accessible by a GitHub App installation
  .get(
    '/repos',
    async ({ query: q, set }) => {
      const { installation_id } = q;
      if (!installation_id) {
        set.status = 400;
        return { error: 'installation_id required' };
      }

      // Get installation access token
      const token = await getInstallationToken(installation_id);
      if (!token) {
        set.status = 500;
        return { error: 'Failed to get installation token' };
      }

      // Fetch repos
      const res = await fetch('https://api.github.com/installation/repositories?per_page=100', {
        headers: {
          Authorization: `Bearer ${token}`,
          Accept: 'application/vnd.github+json',
        },
      });

      if (!res.ok) {
        set.status = 502;
        return { error: 'Failed to fetch repositories' };
      }

      const data = (await res.json()) as {
        repositories: {
          id: number;
          full_name: string;
          html_url: string;
          description: string | null;
          default_branch: string;
        }[];
      };

      return {
        data: data.repositories.map((r) => ({
          id: r.id,
          fullName: r.full_name,
          url: r.html_url,
          description: r.description,
          defaultBranch: r.default_branch,
        })),
      };
    },
    {
      query: t.Object({ installation_id: t.String() }),
    },
  )

  // Detect documentation structure in a repo
  .post(
    '/detect',
    async ({ body, set }) => {
      const { installation_id, repo_url, branch } = body;

      const token = await getInstallationToken(installation_id);
      if (!token) {
        set.status = 500;
        return { error: 'Failed to get installation token' };
      }

      // Parse owner/repo from URL
      const match = repo_url.match(/github\.com\/([^/]+\/[^/]+)/);
      if (!match) {
        set.status = 400;
        return { error: 'Invalid repo URL' };
      }

      const repoPath = match[1].replace(/\.git$/, '');

      // Check for common doc structures
      const detected = {
        hasOharaYaml: false,
        hasDocsDir: false,
        hasReadme: false,
        hasDiataxis: false,
        suggestedDocsDir: '.',
        detectedFiles: [] as string[],
      };

      // Check root files
      for (const file of ['ohara.yaml', 'ohara.yml', 'README.md', 'docs']) {
        const res = await fetch(
          `https://api.github.com/repos/${repoPath}/contents/${file}?ref=${branch}`,
          {
            headers: {
              Authorization: `Bearer ${token}`,
              Accept: 'application/vnd.github+json',
            },
          },
        );

        if (res.ok) {
          detected.detectedFiles.push(file);
          if (file.startsWith('ohara.y')) detected.hasOharaYaml = true;
          if (file === 'docs') {
            detected.hasDocsDir = true;
            detected.suggestedDocsDir = 'docs';
          }
          if (file === 'README.md') detected.hasReadme = true;
        }
      }

      // Check for Diataxis dirs in docs/ or root
      const prefix = detected.hasDocsDir ? 'docs/' : '';
      for (const dir of ['tutorials', 'guides', 'reference', 'explanation']) {
        const res = await fetch(
          `https://api.github.com/repos/${repoPath}/contents/${prefix}${dir}?ref=${branch}`,
          {
            headers: {
              Authorization: `Bearer ${token}`,
              Accept: 'application/vnd.github+json',
            },
          },
        );
        if (res.ok) {
          detected.hasDiataxis = true;
          detected.detectedFiles.push(`${prefix}${dir}/`);
        }
      }

      return { data: detected };
    },
    {
      body: t.Object({
        installation_id: t.String(),
        repo_url: t.String(),
        branch: t.String(),
      }),
    },
  )

  // Setup a project from GitHub App installation
  .post(
    '/setup',
    async ({ user, body, set }) => {
      const { name, slug, repo_url, branch, docs_dir, installation_id } = body;

      // Create project
      const [project] = await db
        .insert(projects)
        .values({
          name,
          slug,
          ownerId: user.id,
          repoUrl: repo_url,
          repoBranch: branch,
          docsDir: docs_dir,
          githubInstallationId: installation_id,
        })
        .returning();

      // Trigger initial sync
      await enqueueSyncJob({
        projectId: project.id,
        repoUrl: repo_url,
        branch,
        docsDir: docs_dir,
        triggerType: 'manual',
      });

      set.status = 201;
      return { data: project };
    },
    {
      body: t.Object({
        name: t.String({ minLength: 1 }),
        slug: t.String({ minLength: 1, pattern: '^[a-z0-9-]+$' }),
        repo_url: t.String(),
        branch: t.String(),
        docs_dir: t.String(),
        installation_id: t.String(),
      }),
    },
  );

/**
 * Get an installation access token from a GitHub App installation ID.
 */
async function getInstallationToken(installationId: string): Promise<string | null> {
  const appId = process.env.GITHUB_APP_ID;
  const privateKey = process.env.GITHUB_APP_PRIVATE_KEY;

  if (!appId || !privateKey) {
    logger.warn('GitHub App credentials not configured');
    return null;
  }

  try {
    // Generate JWT for the GitHub App
    const jwt = await generateAppJwt(appId, privateKey);

    const res = await fetch(
      `https://api.github.com/app/installations/${installationId}/access_tokens`,
      {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${jwt}`,
          Accept: 'application/vnd.github+json',
        },
      },
    );

    if (!res.ok) return null;

    const data = (await res.json()) as { token: string };
    return data.token;
  } catch (err) {
    logger.error({ err }, 'Failed to get installation token');
    return null;
  }
}

/**
 * Generate a JWT for the GitHub App.
 * Uses Node.js crypto (handles PKCS#1 RSA keys from GitHub).
 */
async function generateAppJwt(appId: string, privateKeyPem: string): Promise<string> {
  const { createSign } = await import('crypto');
  const { readFileSync, existsSync } = await import('fs');

  // Load key: if it looks like a file path, read the file; otherwise use as-is
  let pem = privateKeyPem;
  if (!pem.includes('-----BEGIN') && existsSync(pem)) {
    pem = readFileSync(pem, 'utf-8');
  }
  // Handle literal \n from env vars
  pem = pem.replace(/\\n/g, '\n');

  const now = Math.floor(Date.now() / 1000);

  function base64url(input: string): string {
    return Buffer.from(input).toString('base64url');
  }

  const header = base64url(JSON.stringify({ alg: 'RS256', typ: 'JWT' }));
  const payload = base64url(JSON.stringify({ iat: now - 60, exp: now + 600, iss: appId }));

  const data = `${header}.${payload}`;

  const sign = createSign('RSA-SHA256');
  sign.update(data);
  const signature = sign.sign(pem, 'base64url');

  return `${data}.${signature}`;
}
