// GET /health 探测:把 openapi-fetch 的结果收敛为 HealthProbe(网络/HTTP/成功三类),
// 判定交给纯逻辑 resolveConnectivity。
import type { ApiClient } from './client';
import { resolveConnectivity, type Connectivity, type HealthProbe } from './connectivity';

// 超时上限:黑洞地址(如同一网段外的主机)不设上限会让页面永远停在“正在检测”。
const HEALTH_TIMEOUT_MS = 5000;

export async function probeHealth(client: ApiClient): Promise<HealthProbe> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), HEALTH_TIMEOUT_MS);
  try {
    const { data, response } = await client.GET('/health', { signal: controller.signal });
    if (!data) {
      return { kind: 'http', status: response.status };
    }
    return { kind: 'ok', version: data.version };
  } catch (error) {
    if (controller.signal.aborted) {
      return { kind: 'network', detail: '连接超时' };
    }
    return { kind: 'network', detail: error instanceof Error ? error.message : String(error) };
  } finally {
    clearTimeout(timer);
  }
}

export async function checkHealth(client: ApiClient): Promise<Connectivity> {
  return resolveConnectivity(await probeHealth(client));
}
