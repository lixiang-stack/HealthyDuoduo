// API 封装:类型来自 OpenAPI codegen(src/api/schema.d.ts,npm run codegen),
// 运行时用 openapi-fetch;业务代码只经 createApiClient 拿类型安全的 client。
import createClient from 'openapi-fetch';

import type { paths } from './schema';

export function createApiClient(baseUrl: string) {
  return createClient<paths>({ baseUrl });
}

export type ApiClient = ReturnType<typeof createApiClient>;

/** 契约错误体(Error 形状)的最小镜像:message 为中性说明。 */
type ContractError = { message?: string };

/** 解包带响应体的端点:命中返回 data;否则抛契约说明(message),未声明错误体的端点回落 HTTP 状态。 */
export function unwrap<T>(result: { data?: T; error?: ContractError; response: Response }): T {
  if (result.data !== undefined) {
    return result.data;
  }
  throw httpError(result.error, result.response);
}

/** 解包无响应体(204)端点:非 2xx 抛契约说明/HTTP 状态。 */
export function assertOk(result: { error?: ContractError; response: Response }): void {
  if (!result.response.ok) {
    throw httpError(result.error, result.response);
  }
}

/** 失败解包共用的错误构造:契约声明错误体用其中性说明,否则回落 HTTP 状态。 */
function httpError(error: ContractError | undefined, response: Response): Error {
  return new Error(error?.message ?? `HTTP ${response.status}`);
}
