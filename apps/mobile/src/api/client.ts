// API 封装:类型来自 OpenAPI codegen(src/api/schema.d.ts,npm run codegen),
// 运行时用 openapi-fetch;业务代码只经 createApiClient 拿类型安全的 client。
import createClient from 'openapi-fetch';

import type { paths } from './schema';

export function createApiClient(baseUrl: string) {
  return createClient<paths>({ baseUrl });
}

export type ApiClient = ReturnType<typeof createApiClient>;
