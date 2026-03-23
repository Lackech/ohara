import { Elysia } from 'elysia';

/**
 * Security headers middleware.
 */
export const securityHeaders = new Elysia({ name: 'security-headers' }).onAfterHandle(
  ({ set }) => {
    set.headers['x-content-type-options'] = 'nosniff';
    set.headers['x-frame-options'] = 'DENY';
    set.headers['x-xss-protection'] = '0';
    set.headers['referrer-policy'] = 'strict-origin-when-cross-origin';
    set.headers['permissions-policy'] = 'camera=(), microphone=(), geolocation=()';
  },
);
