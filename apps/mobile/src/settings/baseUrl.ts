// 后端 Base URL 的纯逻辑:默认推导与手输规整(无 IO/框架,便于单测)。
// 后端 = 既有 hdd 二进制的 serve 子命令,API 前缀 /api/v1(schemas/openapi.yaml 的 servers.url)。

/** compose 栈里 hdd serve 映射到宿主机的端口(cmd/hdd/serve.go 的 HDD_SERVE_ADDR 缺省 8080)。 */
export const DEFAULT_BACKEND_PORT = 8080;

/** 契约前缀;Base URL 一律含此前缀。 */
export const API_PREFIX = '/api/v1';

const HTTP_URL = /^https?:\/\/[^\s]+$/i;

/**
 * 从 Expo 开发服务器的 host(如 "192.168.1.5:8081")推导局域网后端地址。
 * 真机由此拿到开发机的 LAN IP,避免误用真机不可达的 localhost。
 */
export function deriveDefaultBaseUrl(devServerHost: string | undefined): string | undefined {
  const host = devServerHost?.split(':')[0];
  if (!host) {
    return undefined;
  }
  return `http://${host}:${DEFAULT_BACKEND_PORT}${API_PREFIX}`;
}

/**
 * 校验并规整手输 Base URL:仅接受 http(s),去首尾空白与末尾斜杠;
 * 未带 API 前缀时补上(家长多会直接输 IP,避免存下无 /api/v1 的地址与 health 404)。
 * 非法返回 undefined。
 */
export function normalizeBaseUrl(input: string): string | undefined {
  const trimmed = input.trim();
  if (!HTTP_URL.test(trimmed)) {
    return undefined;
  }
  const withoutTrailingSlash = trimmed.replace(/\/+$/, '');
  return withoutTrailingSlash.endsWith(API_PREFIX)
    ? withoutTrailingSlash
    : `${withoutTrailingSlash}${API_PREFIX}`;
}
