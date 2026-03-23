import { readFile, readdir, stat } from 'fs/promises';
import { join, relative, extname, basename } from 'path';

/**
 * Signals extracted from code analysis.
 * Each signal maps to a potential document to generate.
 */
export type Signal = {
  type: 'tutorial' | 'guide' | 'reference' | 'explanation';
  docName: string;
  title: string;
  confidence: number; // 0-1
  reason: string;
  context: string[]; // Relevant file paths
};

export type AnalysisResult = {
  projectName: string;
  language: string;
  framework: string | null;
  signals: Signal[];
  fileTree: string[];
  keyFiles: Record<string, string>; // path → content (Tier 1 files)
  codeSnippets: Record<string, string>; // path → content (Tier 2 files, trimmed)
  summary: string;
};

const TIER1_FILES = [
  'README.md', 'readme.md', 'README.mdx',
  'package.json', 'Cargo.toml', 'go.mod', 'pyproject.toml', 'pom.xml',
  'tsconfig.json', 'tsconfig.base.json',
  '.env.example', '.env.sample', 'env.example',
  'Dockerfile', 'docker-compose.yml', 'docker-compose.yaml',
  'Makefile',
  'ohara.yaml', 'ohara.yml',
];

const CI_PATTERNS = [
  '.github/workflows',
  '.gitlab-ci.yml',
  '.circleci/config.yml',
  'Jenkinsfile',
];

const ROUTE_PATTERNS = [
  /\.(get|post|put|delete|patch)\s*\(/i,
  /router\.(get|post|put|delete|patch)/i,
  /app\.(get|post|put|delete|patch)/i,
  /@(Get|Post|Put|Delete|Patch)\(/,
  /\.handle\s*\(/,
];

const EXCLUDE_DIRS = new Set([
  'node_modules', '.git', 'dist', 'build', '.next', '.turbo',
  'vendor', '__pycache__', '.venv', 'venv', 'target',
  'coverage', '.nyc_output', '.cache',
]);

/**
 * Analyze a codebase and extract signals for doc generation.
 */
export async function analyzeCodebase(rootDir: string): Promise<AnalysisResult> {
  const fileTree: string[] = [];
  const keyFiles: Record<string, string> = {};
  const codeSnippets: Record<string, string> = {};
  const signals: Signal[] = [];

  // Walk file tree
  await walkDir(rootDir, rootDir, fileTree);

  // Read Tier 1 files
  for (const name of TIER1_FILES) {
    const path = join(rootDir, name);
    try {
      const content = await readFile(path, 'utf-8');
      keyFiles[name] = content;
    } catch {
      // File doesn't exist
    }
  }

  // Check CI configs
  for (const pattern of CI_PATTERNS) {
    const path = join(rootDir, pattern);
    try {
      const s = await stat(path);
      if (s.isDirectory()) {
        const files = await readdir(path);
        for (const f of files) {
          const content = await readFile(join(path, f), 'utf-8');
          keyFiles[`${pattern}/${f}`] = content;
        }
      } else {
        const content = await readFile(path, 'utf-8');
        keyFiles[pattern] = content;
      }
    } catch {
      // Not found
    }
  }

  // Detect language and framework
  const { language, framework } = detectStack(keyFiles, fileTree);

  // Detect project name
  const projectName = detectProjectName(keyFiles, rootDir);

  // Extract signals
  // 1. README exists → getting-started tutorial
  if (keyFiles['README.md'] || keyFiles['readme.md']) {
    signals.push({
      type: 'tutorial',
      docName: 'getting-started',
      title: 'Getting Started',
      confidence: 0.9,
      reason: 'README.md found — can be adapted into a structured tutorial',
      context: ['README.md'],
    });
  } else {
    signals.push({
      type: 'tutorial',
      docName: 'getting-started',
      title: 'Getting Started',
      confidence: 0.7,
      reason: 'No README found — a getting-started tutorial should be generated from entry points',
      context: [],
    });
  }

  // 2. Package scripts → development guide
  const pkgJson = keyFiles['package.json'];
  if (pkgJson) {
    try {
      const pkg = JSON.parse(pkgJson);
      if (pkg.scripts && Object.keys(pkg.scripts).length > 0) {
        signals.push({
          type: 'guide',
          docName: 'development',
          title: 'Development Guide',
          confidence: 0.85,
          reason: `${Object.keys(pkg.scripts).length} npm scripts found`,
          context: ['package.json'],
        });
      }
    } catch { /* invalid JSON */ }
  }

  // 3. Dockerfile → deployment guide
  if (keyFiles['Dockerfile'] || keyFiles['docker-compose.yml'] || keyFiles['docker-compose.yaml']) {
    signals.push({
      type: 'guide',
      docName: 'deployment',
      title: 'Deployment Guide',
      confidence: 0.8,
      reason: 'Docker configuration found',
      context: Object.keys(keyFiles).filter(k => k.toLowerCase().includes('docker')),
    });
  }

  // 4. .env.example → configuration reference
  const envFile = keyFiles['.env.example'] || keyFiles['.env.sample'] || keyFiles['env.example'];
  if (envFile) {
    const envVarCount = envFile.split('\n').filter(l => l.match(/^[A-Z_]+=/) || l.match(/^[A-Z_]+=/)).length;
    signals.push({
      type: 'reference',
      docName: 'configuration',
      title: 'Configuration Reference',
      confidence: 0.9,
      reason: `${envVarCount} environment variables found`,
      context: ['.env.example'],
    });
  }

  // 5. Route files → API reference
  const routeFiles = await findRouteFiles(rootDir, fileTree);
  if (routeFiles.length > 0) {
    for (const rf of routeFiles.slice(0, 5)) {
      try {
        const content = await readFile(join(rootDir, rf), 'utf-8');
        codeSnippets[rf] = content.slice(0, 3000);
      } catch { /* */ }
    }
    signals.push({
      type: 'reference',
      docName: 'api-reference',
      title: 'API Reference',
      confidence: 0.85,
      reason: `${routeFiles.length} files with route definitions found`,
      context: routeFiles.slice(0, 5),
    });
  }

  // 6. Type/interface files → types reference
  const typeFiles = fileTree.filter(f =>
    f.match(/types?\.(ts|d\.ts)$/) ||
    f.match(/interfaces?\.(ts|d\.ts)$/) ||
    f.match(/models?\.(ts|go|py)$/) ||
    f.match(/schema\.(ts|prisma|graphql)$/),
  );
  if (typeFiles.length > 0) {
    for (const tf of typeFiles.slice(0, 3)) {
      try {
        const content = await readFile(join(rootDir, tf), 'utf-8');
        codeSnippets[tf] = content.slice(0, 3000);
      } catch { /* */ }
    }
    signals.push({
      type: 'reference',
      docName: 'types',
      title: 'Types & Data Models',
      confidence: 0.75,
      reason: `${typeFiles.length} type/model/schema files found`,
      context: typeFiles.slice(0, 5),
    });
  }

  // 7. CI configs → CI/CD guide
  const ciFiles = Object.keys(keyFiles).filter(k =>
    k.includes('.github/') || k.includes('.gitlab') || k.includes('.circleci'),
  );
  if (ciFiles.length > 0) {
    signals.push({
      type: 'guide',
      docName: 'ci-cd',
      title: 'CI/CD Pipeline',
      confidence: 0.75,
      reason: `CI configuration found: ${ciFiles.join(', ')}`,
      context: ciFiles,
    });
  }

  // 8. Complex project → architecture explanation
  const srcDirs = fileTree.filter(f => f.includes('/') && !f.includes('node_modules'));
  const topDirs = new Set(srcDirs.map(f => f.split('/')[0]));
  if (topDirs.size > 4 || fileTree.length > 30) {
    signals.push({
      type: 'explanation',
      docName: 'architecture',
      title: 'Architecture Overview',
      confidence: 0.7,
      reason: `Complex project structure: ${topDirs.size} top-level directories, ${fileTree.length} files`,
      context: [],
    });
  }

  // 9. Test files → testing guide
  const testFiles = fileTree.filter(f =>
    f.match(/\.(test|spec)\.(ts|js|tsx|jsx|py|go)$/) ||
    f.match(/test_.*\.(py|go)$/) ||
    f.includes('__tests__/'),
  );
  if (testFiles.length > 0) {
    signals.push({
      type: 'guide',
      docName: 'testing',
      title: 'Testing Guide',
      confidence: 0.65,
      reason: `${testFiles.length} test files found`,
      context: testFiles.slice(0, 3),
    });
  }

  // Sort by confidence
  signals.sort((a, b) => b.confidence - a.confidence);

  const summary = buildSummary(projectName, language, framework, fileTree, signals);

  return {
    projectName,
    language,
    framework,
    signals,
    fileTree,
    keyFiles,
    codeSnippets,
    summary,
  };
}

async function walkDir(dir: string, rootDir: string, files: string[]) {
  let entries;
  try {
    entries = await readdir(dir, { withFileTypes: true });
  } catch {
    return;
  }

  for (const entry of entries) {
    if (EXCLUDE_DIRS.has(entry.name) || entry.name.startsWith('.')) continue;

    const fullPath = join(dir, entry.name);
    const relPath = relative(rootDir, fullPath);

    if (entry.isDirectory()) {
      await walkDir(fullPath, rootDir, files);
    } else {
      files.push(relPath);
    }
  }
}

function detectStack(
  keyFiles: Record<string, string>,
  fileTree: string[],
): { language: string; framework: string | null } {
  if (keyFiles['package.json']) {
    try {
      const pkg = JSON.parse(keyFiles['package.json']);
      const deps = { ...pkg.dependencies, ...pkg.devDependencies };
      if (deps['next']) return { language: 'TypeScript', framework: 'Next.js' };
      if (deps['elysia']) return { language: 'TypeScript', framework: 'Elysia' };
      if (deps['express']) return { language: 'TypeScript', framework: 'Express' };
      if (deps['react']) return { language: 'TypeScript', framework: 'React' };
      if (deps['vue']) return { language: 'TypeScript', framework: 'Vue' };
      return { language: fileTree.some(f => f.endsWith('.ts')) ? 'TypeScript' : 'JavaScript', framework: null };
    } catch { /* */ }
  }
  if (keyFiles['go.mod']) return { language: 'Go', framework: null };
  if (keyFiles['Cargo.toml']) return { language: 'Rust', framework: null };
  if (keyFiles['pyproject.toml']) return { language: 'Python', framework: null };
  if (keyFiles['pom.xml']) return { language: 'Java', framework: null };

  return { language: 'Unknown', framework: null };
}

function detectProjectName(keyFiles: Record<string, string>, rootDir: string): string {
  if (keyFiles['package.json']) {
    try {
      return JSON.parse(keyFiles['package.json']).name ?? basename(rootDir);
    } catch { /* */ }
  }
  return basename(rootDir);
}

async function findRouteFiles(rootDir: string, fileTree: string[]): Promise<string[]> {
  const candidates = fileTree.filter(f =>
    f.match(/\.(ts|js|go|py)$/) &&
    (f.includes('route') || f.includes('controller') || f.includes('handler') || f.includes('endpoint') || f.includes('api')),
  );

  const routeFiles: string[] = [];
  for (const f of candidates.slice(0, 10)) {
    try {
      const content = await readFile(join(rootDir, f), 'utf-8');
      if (ROUTE_PATTERNS.some(p => p.test(content))) {
        routeFiles.push(f);
      }
    } catch { /* */ }
  }
  return routeFiles;
}

function buildSummary(
  name: string,
  language: string,
  framework: string | null,
  files: string[],
  signals: Signal[],
): string {
  const stack = framework ? `${language}/${framework}` : language;
  const docCount = signals.length;
  const highConf = signals.filter(s => s.confidence >= 0.8).length;

  return `Project "${name}" is a ${stack} project with ${files.length} files. ` +
    `Analysis found ${docCount} documentation opportunities (${highConf} high-confidence). ` +
    `Recommended: generate ${signals.filter(s => s.confidence >= 0.7).map(s => s.docName).join(', ')}.`;
}
