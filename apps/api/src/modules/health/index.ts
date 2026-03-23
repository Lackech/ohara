import { Elysia } from 'elysia';

export const healthModule = new Elysia({ prefix: '/health', name: 'health' }).get('/', () => ({
  status: 'ok',
  timestamp: new Date().toISOString(),
  version: process.env.npm_package_version ?? '0.0.0',
}));
