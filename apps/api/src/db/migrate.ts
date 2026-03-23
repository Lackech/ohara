/**
 * Custom SQL migrations that Drizzle can't express declaratively.
 * Run after `drizzle-kit push` to set up pgvector, indexes, etc.
 */
import postgres from 'postgres';

const connectionString = process.env.DATABASE_URL;

if (!connectionString) {
  throw new Error('DATABASE_URL environment variable is required');
}

const sql = postgres(connectionString);

async function migrate() {
  console.log('Running custom migrations...');

  // Enable pgvector extension
  await sql`CREATE EXTENSION IF NOT EXISTS vector`;

  // GIN index for full-text search on documents
  await sql`
    CREATE INDEX IF NOT EXISTS documents_search_vector_idx
    ON documents
    USING gin (search_vector)
  `;

  // HNSW index for semantic search on document_chunks
  await sql`
    CREATE INDEX IF NOT EXISTS chunks_embedding_hnsw_idx
    ON document_chunks
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64)
  `;

  // Trigger to auto-update search_vector on document insert/update
  await sql`
    CREATE OR REPLACE FUNCTION documents_search_vector_update() RETURNS trigger AS $$
    BEGIN
      NEW.search_vector :=
        setweight(to_tsvector('english', COALESCE(NEW.title, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(NEW.description, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(NEW.raw_content, '')), 'C');
      RETURN NEW;
    END;
    $$ LANGUAGE plpgsql
  `;

  await sql`
    DROP TRIGGER IF EXISTS documents_search_vector_trigger ON documents
  `;

  await sql`
    CREATE TRIGGER documents_search_vector_trigger
    BEFORE INSERT OR UPDATE OF title, description, raw_content ON documents
    FOR EACH ROW
    EXECUTE FUNCTION documents_search_vector_update()
  `;

  console.log('Custom migrations complete.');
  await sql.end();
}

migrate().catch((err) => {
  console.error('Migration failed:', err);
  process.exit(1);
});
