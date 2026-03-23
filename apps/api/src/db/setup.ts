/**
 * Enable required PostgreSQL extensions.
 * Run this BEFORE drizzle-kit push.
 */
import postgres from 'postgres';

const connectionString = process.env.DATABASE_URL;
if (!connectionString) {
  throw new Error('DATABASE_URL environment variable is required');
}

const sql = postgres(connectionString);

async function setup() {
  console.log('Enabling PostgreSQL extensions...');
  await sql`CREATE EXTENSION IF NOT EXISTS vector`;
  console.log('✓ pgvector extension enabled');
  await sql.end();
}

setup().catch((err) => {
  console.error('Setup failed:', err);
  process.exit(1);
});
