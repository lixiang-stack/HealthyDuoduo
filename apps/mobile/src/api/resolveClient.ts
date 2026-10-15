// 有效后端客户端的解析:已保存地址优先,否则从 Expo 开发服务器 host 推导
// (真机不误用不可达的 localhost)。独立于 client.ts,保持其纯 openapi-fetch 封装。
import Constants from 'expo-constants';

import { deriveDefaultBaseUrl } from '../settings/baseUrl';
import { loadBaseUrl } from '../settings/baseUrlStore';
import { createApiClient, type ApiClient } from './client';

export type ResolvedApiClient = { client: ApiClient; baseUrl: string };

export async function resolveApiClient(): Promise<ResolvedApiClient | undefined> {
  const baseUrl = (await loadBaseUrl()) ?? deriveDefaultBaseUrl(Constants.expoConfig?.hostUri);
  return baseUrl ? { client: createApiClient(baseUrl), baseUrl } : undefined;
}
