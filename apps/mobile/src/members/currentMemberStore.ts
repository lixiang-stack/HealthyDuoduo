// 「当前成员」选择的持久化(AsyncStorage);选择逻辑见 CurrentMemberContext。
import AsyncStorage from '@react-native-async-storage/async-storage';

export const CURRENT_MEMBER_STORAGE_KEY = 'healthyduoduo.currentMemberId';

export async function loadCurrentMemberId(): Promise<number | undefined> {
  const stored = await AsyncStorage.getItem(CURRENT_MEMBER_STORAGE_KEY);
  if (!stored) {
    return undefined;
  }
  const id = Number(stored);
  return Number.isSafeInteger(id) && id > 0 ? id : undefined;
}

export async function saveCurrentMemberId(memberId: number): Promise<void> {
  await AsyncStorage.setItem(CURRENT_MEMBER_STORAGE_KEY, String(memberId));
}

export async function clearCurrentMemberId(): Promise<void> {
  await AsyncStorage.removeItem(CURRENT_MEMBER_STORAGE_KEY);
}
