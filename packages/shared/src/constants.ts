/** API key prefix for Ohara-generated API keys */
export const API_KEY_PREFIX = 'ohara_';

/** Default rate limit: requests per hour */
export const DEFAULT_RATE_LIMIT = 1000;

/** Chunk size bounds (in tokens) for document chunking */
export const CHUNK_MIN_TOKENS = 500;
export const CHUNK_MAX_TOKENS = 800;

/** Embedding model configuration */
export const EMBEDDING_MODEL = 'text-embedding-3-small';
export const EMBEDDING_DIMENSIONS = 1536;

/** Config file names to detect in repos */
export const CONFIG_FILE_NAMES = ['ohara.yaml', 'ohara.yml', '.ohara.yaml', '.ohara.yml'] as const;

/** Supported document file extensions */
export const SUPPORTED_EXTENSIONS = ['.md', '.mdx'] as const;

/** Maximum document size in bytes (1MB) */
export const MAX_DOCUMENT_SIZE = 1024 * 1024;

/** Search result limits */
export const SEARCH_DEFAULT_LIMIT = 20;
export const SEARCH_MAX_LIMIT = 100;

/** RRF constant for hybrid search */
export const RRF_K = 60;
