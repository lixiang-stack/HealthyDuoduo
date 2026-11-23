// 「当前成员 / 当前孩子」选择的持久化(AsyncStorage);选择逻辑见 CurrentMemberContext。
// 两者都是每台设备本地的选择,但用途不同:当前成员是健康档案的查看对象,
// 当前孩子是护理事件的记录对象;独立存储,互不覆盖。
import AsyncStorage from '@react-native-async-storage/async-storage';

const STORAGE_KEY_PREFIX = 'healthyduoduo.';

export const CURRENT_MEMBER_STORAGE_KEY = `${STORAGE_KEY_PREFIX}currentMemberId`;
export const CURRENT_CHILD_STORAGE_KEY = `${STORAGE_KEY_PREFIX}currentChildId`;

function parseStoredMemberId(stored: string | null): number | undefined {
  if (!stored) {
    return undefined;
  }
  const id = Number(stored);
  return Number.isSafeInteger(id) && id > 0 ? id : undefined;
}

export async function loadCurrentMemberId(): Promise<number | undefined> {
  return parseStoredMemberId(await AsyncStorage.getItem(CURRENT_MEMBER_STORAGE_KEY));
}

export async function saveCurrentMemberId(memberId: number): Promise<void> {
  await AsyncStorage.setItem(CURRENT_MEMBER_STORAGE_KEY, String(memberId));
}

export async function clearCurrentMemberId(): Promise<void> {
  await AsyncStorage.removeItem(CURRENT_MEMBER_STORAGE_KEY);
}

export async function loadCurrentChildId(): Promise<number | undefined> {
  return parseStoredMemberId(await AsyncStorage.getItem(CURRENT_CHILD_STORAGE_KEY));
}

export async function saveCurrentChildId(memberId: number): Promise<void> {
  await AsyncStorage.setItem(CURRENT_CHILD_STORAGE_KEY, String(memberId));
}

export async function clearCurrentChildId(): Promise<void> {
  await AsyncStorage.removeItem(CURRENT_CHILD_STORAGE_KEY);
}
