import { Elysia } from 'elysia';
import { InMemoryTransport } from '@modelcontextprotocol/sdk/inMemory.js';
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { createMcpServer } from './server.js';
import { validateApiKey } from '../../../lib/api-keys.js';
import { logger } from '../../../lib/logger.js';

/**
 * MCP endpoint for Elysia.
 *
 * Uses in-memory transport to bridge Elysia HTTP to the MCP server.
 * Each request creates a fresh server+client pair, executes the call, and returns.
 *
 * Authentication: Bearer token (API key) in Authorization header.
 */
export const mcpModule = new Elysia({ name: 'mcp' }).post(
  '/mcp',
  async ({ request, body, set }) => {
    // Authenticate via API key
    const authHeader = request.headers.get('authorization');
    if (!authHeader?.startsWith('Bearer ')) {
      set.status = 401;
      return { error: 'API key required. Use Authorization: Bearer ohara_...' };
    }

    const apiKey = await validateApiKey(authHeader.slice(7));
    if (!apiKey) {
      set.status = 401;
      return { error: 'Invalid or expired API key' };
    }

    const jsonRpc = body as { method?: string; params?: Record<string, unknown>; id?: unknown };

    if (!jsonRpc.method) {
      set.status = 400;
      return { jsonrpc: '2.0', error: { code: -32600, message: 'Invalid request' } };
    }

    try {
      // Create server + client connected via in-memory transport
      const server = createMcpServer();
      const [serverTransport, clientTransport] = InMemoryTransport.createLinkedPair();

      await server.connect(serverTransport);

      const client = new Client({ name: 'ohara-http-bridge', version: '0.1.0' });
      await client.connect(clientTransport);

      let result: unknown;

      switch (jsonRpc.method) {
        case 'initialize':
          result = {
            protocolVersion: '2025-03-26',
            capabilities: {
              tools: { listChanged: false },
              resources: { subscribe: false, listChanged: false },
              prompts: { listChanged: false },
            },
            serverInfo: { name: 'ohara', version: '0.1.0' },
          };
          break;

        case 'tools/list':
          result = await client.listTools();
          break;

        case 'tools/call':
          result = await client.callTool({
            name: jsonRpc.params?.name as string,
            arguments: (jsonRpc.params?.arguments as Record<string, unknown>) ?? {},
          });
          break;

        case 'resources/list':
          result = await client.listResources();
          break;

        case 'resources/read':
          result = await client.readResource({ uri: jsonRpc.params?.uri as string });
          break;

        case 'prompts/list':
          result = await client.listPrompts();
          break;

        case 'prompts/get':
          result = await client.getPrompt({
            name: jsonRpc.params?.name as string,
            arguments: (jsonRpc.params?.arguments as Record<string, string>) ?? {},
          });
          break;

        default:
          await client.close();
          await server.close();
          return {
            jsonrpc: '2.0',
            id: jsonRpc.id,
            error: { code: -32601, message: `Unknown method: ${jsonRpc.method}` },
          };
      }

      await client.close();
      await server.close();

      return { jsonrpc: '2.0', id: jsonRpc.id, result };
    } catch (err) {
      logger.error({ err }, 'MCP request failed');
      set.status = 500;
      return {
        jsonrpc: '2.0',
        id: jsonRpc.id,
        error: { code: -32603, message: err instanceof Error ? err.message : 'Internal error' },
      };
    }
  },
);
