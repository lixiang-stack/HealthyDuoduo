// 当前成员选择的持久化单测:保存/读回往返 + 未存与损坏值一律回落 undefined。
import AsyncStorage from '@react-native-async-storage/async-storage';

import { CURRENT_MEMBER_STORAGE_KEY, loadCurrentMemberId, saveCurrentMemberId } from './currentMemberStore';

jest.mock('@react-native-async-storage/async-storage', () =>
  require('@react-native-async-storage/async-storage/jest/async-storage-mock'),
);

describe('currentMemberStore', () => {
  beforeEach(async () => {
    await AsyncStorage.clear();
  });

  it('保存后可读回同一成员 id', async () => {
    await saveCurrentMemberId(42);
    expect(await loadCurrentMemberId()).toBe(42);
  });

  it('未保存过时为 undefined', async () => {
    expect(await loadCurrentMemberId()).toBeUndefined();
  });

  it('存储值损坏/非法(非数字、0、负数、非整数)时为 undefined', async () => {
    for (const bad of ['abc', '0', '-3', '1.5']) {
      await AsyncStorage.setItem(CURRENT_MEMBER_STORAGE_KEY, bad);
      expect(await loadCurrentMemberId()).toBeUndefined();
    }
  });
});
