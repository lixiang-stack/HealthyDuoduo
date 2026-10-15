// Base URL 的持久化(AsyncStorage);推导/规整的纯逻辑见 baseUrl.ts。
import AsyncStorage from '@react-native-async-storage/async-storage';

export const BASE_URL_STORAGE_KEY = 'healthyduoduo.baseUrl';

export async function loadBaseUrl(): Promise<string | undefined> {
  const stored = await AsyncStorage.getItem(BASE_URL_STORAGE_KEY);
  return stored ?? undefined;
}

export async function saveBaseUrl(baseUrl: string): Promise<void> {
  await AsyncStorage.setItem(BASE_URL_STORAGE_KEY, baseUrl);
}
