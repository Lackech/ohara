import { Elysia, t } from 'elysia';
import { withAuth } from '../../middleware/auth.js';
import { analyzeCodebase, createGenerationPlan } from '@ohara/shared/scaffold';
import { mkdtemp, rm } from 'fs/promises';
import { tmpdir } from 'os';
import { join } from 'path';
import simpleGit from 'simple-git';
import { logger } from '../../lib/logger.js';

/**
 * Scaffolding API — analyze a repo and return a generation plan.
 * The plan contains prompts that any LLM can execute to produce docs.
 */
export const scaffoldModule = new Elysia({ prefix: '/api/v1/scaffold', name: 'scaffold' })
  .use(withAuth())

  // Analyze a Git repo and return a generation plan
  .post(
    '/analyze',
    async ({ body, set }) => {
      const { repoUrl, branch = 'main', types, minConfidence } = body;

      let tempDir: string | null = null;

      try {
        // Clone the repo
        tempDir = await mkdtemp(join(tmpdir(), 'ohara-scaffold-'));
        const git = simpleGit();

        logger.info({ repoUrl, branch, tempDir }, 'Cloning for analysis');

        await git.clone(repoUrl, tempDir, [
          '--depth', '1',
          '--branch', branch,
          '--single-branch',
        ]);

        // Analyze
        const analysis = await analyzeCodebase(tempDir);

        // Create plan
        const plan = createGenerationPlan(analysis, {
          minConfidence,
          types,
        });

        return {
          data: {
            analysis: {
              projectName: analysis.projectName,
              language: analysis.language,
              framework: analysis.framework,
              fileCount: analysis.fileTree.length,
              summary: analysis.summary,
              signals: analysis.signals,
            },
            plan: {
              oharaYaml: plan.oharaYaml,
              docs: plan.docs.map((d) => ({
                path: d.path,
                title: d.title,
                diataxisType: d.diataxisType,
                confidence: d.confidence,
                outline: d.outline,
                prompt: d.prompt,
              })),
              summary: plan.summary,
              totalDocs: plan.totalDocs,
              estimatedTokens: plan.estimatedTokens,
            },
          },
        };
      } catch (err) {
        logger.error({ err, repoUrl }, 'Scaffold analysis failed');
        set.status = 500;
        return { error: 'Analysis failed', details: err instanceof Error ? err.message : String(err) };
      } finally {
        if (tempDir) {
          await rm(tempDir, { recursive: true, force: true }).catch(() => {});
        }
      }
    },
    {
      body: t.Object({
        repoUrl: t.String(),
        branch: t.Optional(t.String()),
        types: t.Optional(t.Array(t.String())),
        minConfidence: t.Optional(t.Number()),
      }),
    },
  )

  // Analyze a local directory (for CLI/agent use via API)
  .post(
    '/analyze-local',
    async ({ body }) => {
      const { directory, types, minConfidence } = body;

      const analysis = await analyzeCodebase(directory);
      const plan = createGenerationPlan(analysis, { minConfidence, types });

      return {
        data: {
          analysis: {
            projectName: analysis.projectName,
            language: analysis.language,
            framework: analysis.framework,
            fileCount: analysis.fileTree.length,
            summary: analysis.summary,
            signals: analysis.signals,
          },
          plan: {
            oharaYaml: plan.oharaYaml,
            docs: plan.docs,
            summary: plan.summary,
            totalDocs: plan.totalDocs,
            estimatedTokens: plan.estimatedTokens,
          },
        },
      };
    },
    {
      body: t.Object({
        directory: t.String(),
        types: t.Optional(t.Array(t.String())),
        minConfidence: t.Optional(t.Number()),
      }),
    },
  );
