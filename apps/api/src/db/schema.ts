import {
  pgTable,
  text,
  timestamp,
  uuid,
  varchar,
  boolean,
  integer,
  jsonb,
  index,
  uniqueIndex,
  customType,
} from 'drizzle-orm/pg-core';
import { relations } from 'drizzle-orm';

// ---------------------------------------------------------------------------
// Custom types
// ---------------------------------------------------------------------------

/** pgvector vector type — stores embeddings as float arrays */
const vector = customType<{ data: number[]; driverData: string }>({
  dataType() {
    return 'vector(1536)';
  },
  toDriver(value: number[]): string {
    return `[${value.join(',')}]`;
  },
  fromDriver(value: string): number[] {
    return value
      .slice(1, -1)
      .split(',')
      .map((v) => parseFloat(v));
  },
});

/** PostgreSQL tsvector type for full-text search */
const tsvector = customType<{ data: string }>({
  dataType() {
    return 'tsvector';
  },
});

// ---------------------------------------------------------------------------
// Auth tables (managed by Better Auth)
// ---------------------------------------------------------------------------

/** Better Auth user table — the single source of truth for user identity */
export const user = pgTable('user', {
  id: text('id').primaryKey(),
  name: text('name').notNull(),
  email: text('email').notNull().unique(),
  emailVerified: boolean('email_verified').notNull().default(false),
  image: text('image'),
  createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
});

/** Better Auth session table */
export const session = pgTable('session', {
  id: text('id').primaryKey(),
  expiresAt: timestamp('expires_at', { withTimezone: true }).notNull(),
  token: text('token').notNull().unique(),
  createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
  ipAddress: text('ip_address'),
  userAgent: text('user_agent'),
  userId: text('user_id')
    .notNull()
    .references(() => user.id, { onDelete: 'cascade' }),
});

/** Better Auth account table (OAuth providers) */
export const account = pgTable('account', {
  id: text('id').primaryKey(),
  accountId: text('account_id').notNull(),
  providerId: text('provider_id').notNull(),
  userId: text('user_id')
    .notNull()
    .references(() => user.id, { onDelete: 'cascade' }),
  accessToken: text('access_token'),
  refreshToken: text('refresh_token'),
  idToken: text('id_token'),
  accessTokenExpiresAt: timestamp('access_token_expires_at', { withTimezone: true }),
  refreshTokenExpiresAt: timestamp('refresh_token_expires_at', { withTimezone: true }),
  scope: text('scope'),
  password: text('password'),
  createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
});

/** Better Auth verification table (email verification, password reset) */
export const verification = pgTable('verification', {
  id: text('id').primaryKey(),
  identifier: text('identifier').notNull(),
  value: text('value').notNull(),
  expiresAt: timestamp('expires_at', { withTimezone: true }).notNull(),
  createdAt: timestamp('created_at', { withTimezone: true }).defaultNow(),
  updatedAt: timestamp('updated_at', { withTimezone: true }).defaultNow(),
});

// ---------------------------------------------------------------------------
// App tables
// ---------------------------------------------------------------------------

// 1. Projects
export const projects = pgTable(
  'projects',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    name: varchar('name', { length: 255 }).notNull(),
    slug: varchar('slug', { length: 255 }).notNull(),
    description: text('description'),
    ownerId: text('owner_id')
      .notNull()
      .references(() => user.id, { onDelete: 'cascade' }),

    // Git integration
    repoUrl: text('repo_url'),
    repoBranch: varchar('repo_branch', { length: 255 }).default('main'),
    docsDir: varchar('docs_dir', { length: 500 }).default('.'),
    githubInstallationId: varchar('github_installation_id', { length: 255 }),
    lastSyncCommitSha: varchar('last_sync_commit_sha', { length: 40 }),
    lastSyncedAt: timestamp('last_synced_at', { withTimezone: true }),

    settings: jsonb('settings').$type<Record<string, unknown>>().default({}),

    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp('updated_at', { withTimezone: true })
      .notNull()
      .defaultNow()
      .$onUpdate(() => new Date()),
  },
  (table) => [uniqueIndex('projects_slug_idx').on(table.slug)],
);

// 2. Documents
export const documents = pgTable(
  'documents',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    projectId: uuid('project_id')
      .notNull()
      .references(() => projects.id, { onDelete: 'cascade' }),

    path: varchar('path', { length: 1000 }).notNull(),
    title: varchar('title', { length: 500 }).notNull(),
    slug: varchar('slug', { length: 500 }).notNull(),
    description: text('description'),

    diataxisType: varchar('diataxis_type', { length: 20 })
      .$type<'tutorial' | 'guide' | 'reference' | 'explanation'>()
      .notNull(),

    rawContent: text('raw_content').notNull(),
    htmlContent: text('html_content'),
    contentHash: varchar('content_hash', { length: 64 }).notNull(),

    frontmatter: jsonb('frontmatter').$type<Record<string, unknown>>().default({}),

    searchVector: tsvector('search_vector'),

    draft: boolean('draft').notNull().default(false),
    order: integer('order'),
    wordCount: integer('word_count'),

    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp('updated_at', { withTimezone: true })
      .notNull()
      .defaultNow()
      .$onUpdate(() => new Date()),
  },
  (table) => [
    uniqueIndex('documents_project_path_idx').on(table.projectId, table.path),
    index('documents_project_type_idx').on(table.projectId, table.diataxisType),
    // GIN index on search_vector created via custom migration (migrate.ts)
  ],
);

// 3. Document Chunks (for vector search / RAG)
export const documentChunks = pgTable(
  'document_chunks',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    documentId: uuid('document_id')
      .notNull()
      .references(() => documents.id, { onDelete: 'cascade' }),
    projectId: uuid('project_id')
      .notNull()
      .references(() => projects.id, { onDelete: 'cascade' }),

    content: text('content').notNull(),
    contentHash: varchar('content_hash', { length: 64 }).notNull(),

    chunkIndex: integer('chunk_index').notNull(),
    headingHierarchy: jsonb('heading_hierarchy').$type<string[]>().default([]),

    embedding: vector('embedding'),

    diataxisType: varchar('diataxis_type', { length: 20 })
      .$type<'tutorial' | 'guide' | 'reference' | 'explanation'>()
      .notNull(),
    tokenCount: integer('token_count'),

    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => [
    index('chunks_document_idx').on(table.documentId),
    index('chunks_project_type_idx').on(table.projectId, table.diataxisType),
    // HNSW index created via custom migration (migrate.ts)
  ],
);

// 4. API Keys
export const apiKeys = pgTable(
  'api_keys',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    userId: text('user_id')
      .notNull()
      .references(() => user.id, { onDelete: 'cascade' }),
    projectId: uuid('project_id').references(() => projects.id, { onDelete: 'cascade' }),

    name: varchar('name', { length: 255 }).notNull(),
    keyHash: varchar('key_hash', { length: 64 }).notNull().unique(),
    keyPrefix: varchar('key_prefix', { length: 12 }).notNull(),

    rateLimit: integer('rate_limit').default(1000),
    scopes: jsonb('scopes').$type<string[]>().default(['read']),

    lastUsedAt: timestamp('last_used_at', { withTimezone: true }),
    expiresAt: timestamp('expires_at', { withTimezone: true }),
    revokedAt: timestamp('revoked_at', { withTimezone: true }),

    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => [
    index('api_keys_user_idx').on(table.userId),
    uniqueIndex('api_keys_hash_idx').on(table.keyHash),
  ],
);

// 5. Sync Logs
export const syncLogs = pgTable(
  'sync_logs',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    projectId: uuid('project_id')
      .notNull()
      .references(() => projects.id, { onDelete: 'cascade' }),

    status: varchar('status', { length: 20 })
      .$type<'pending' | 'running' | 'completed' | 'failed'>()
      .notNull()
      .default('pending'),
    triggerType: varchar('trigger_type', { length: 20 })
      .$type<'webhook' | 'manual' | 'scheduled'>()
      .notNull(),

    commitSha: varchar('commit_sha', { length: 40 }),
    commitMessage: text('commit_message'),

    documentsAdded: integer('documents_added').default(0),
    documentsUpdated: integer('documents_updated').default(0),
    documentsDeleted: integer('documents_deleted').default(0),
    chunksCreated: integer('chunks_created').default(0),

    startedAt: timestamp('started_at', { withTimezone: true }),
    completedAt: timestamp('completed_at', { withTimezone: true }),

    error: text('error'),

    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => [
    index('sync_logs_project_idx').on(table.projectId),
    index('sync_logs_status_idx').on(table.status),
  ],
);

// ---------------------------------------------------------------------------
// Relations
// ---------------------------------------------------------------------------

export const userRelations = relations(user, ({ many }) => ({
  sessions: many(session),
  accounts: many(account),
  projects: many(projects),
  apiKeys: many(apiKeys),
}));

export const sessionRelations = relations(session, ({ one }) => ({
  user: one(user, { fields: [session.userId], references: [user.id] }),
}));

export const accountRelations = relations(account, ({ one }) => ({
  user: one(user, { fields: [account.userId], references: [user.id] }),
}));

export const projectsRelations = relations(projects, ({ one, many }) => ({
  owner: one(user, { fields: [projects.ownerId], references: [user.id] }),
  documents: many(documents),
  documentChunks: many(documentChunks),
  syncLogs: many(syncLogs),
  apiKeys: many(apiKeys),
}));

export const documentsRelations = relations(documents, ({ one, many }) => ({
  project: one(projects, { fields: [documents.projectId], references: [projects.id] }),
  chunks: many(documentChunks),
}));

export const documentChunksRelations = relations(documentChunks, ({ one }) => ({
  document: one(documents, { fields: [documentChunks.documentId], references: [documents.id] }),
  project: one(projects, { fields: [documentChunks.projectId], references: [projects.id] }),
}));

export const apiKeysRelations = relations(apiKeys, ({ one }) => ({
  user: one(user, { fields: [apiKeys.userId], references: [user.id] }),
  project: one(projects, { fields: [apiKeys.projectId], references: [projects.id] }),
}));

export const syncLogsRelations = relations(syncLogs, ({ one }) => ({
  project: one(projects, { fields: [syncLogs.projectId], references: [projects.id] }),
}));
