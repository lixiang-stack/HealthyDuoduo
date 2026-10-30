import type { ApiClient } from '../api/client';

export const TEST_BASE_URL = 'http://test';

type Method = 'GET' | 'POST' | 'PATCH' | 'DELETE';

export type FakeApiOptions = {
  params?: { path?: Record<string, unknown>; query?: Record<string, unknown> };
  body?: unknown;
};

type Handler = (options?: FakeApiOptions) => unknown | Promise<unknown>;

export type FakeApiRoutes = Partial<Record<Method, Record<string, Handler>>>;

/** Minimal typed ApiClient double: routes are keyed by the same path template
 * the real openapi-fetch client receives. Unmatched calls fail loudly so a
 * missing fixture cannot silently turn into an empty screen. */
export function createFakeApiClient(routes: FakeApiRoutes): ApiClient {
  const invoke =
    (method: Method) =>
    async (path: string, options?: Parameters<Handler>[0]) => {
      const handler = routes[method]?.[path];
      if (!handler) {
        throw new Error(`unexpected ${method} ${path}`);
      }
      return {
        data: await handler(options),
        error: undefined,
        response: { ok: true, status: 200 } as Response,
      };
    };

  return {
    GET: invoke('GET'),
    POST: invoke('POST'),
    PATCH: invoke('PATCH'),
    DELETE: invoke('DELETE'),
  } as unknown as ApiClient;
}
