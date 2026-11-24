import AsyncStorage from '@react-native-async-storage/async-storage';

import {
  DEFAULT_REMINDER_SETTINGS,
  REMINDER_SETTINGS_STORAGE_KEY,
  loadReminderSettings,
  parseReminderSettings,
  saveReminderSettings,
} from './reminderSettings';

beforeEach(async () => {
  jest.clearAllMocks();
  await AsyncStorage.clear();
});

describe('reminder settings', () => {
  it('defaults to off with a 3-hour interval', () => {
    expect(DEFAULT_REMINDER_SETTINGS).toEqual({ enabled: false, intervalHours: 3 });
    expect(parseReminderSettings(null)).toEqual(DEFAULT_REMINDER_SETTINGS);
  });

  it('reads a stored enabled setting', () => {
    expect(parseReminderSettings('{"enabled":true,"intervalHours":2}')).toEqual({
      enabled: true,
      intervalHours: 2,
    });
  });

  it.each([
    'not json',
    '{}',
    '{"enabled":true,"intervalHours":5}',
    '{"enabled":"yes","intervalHours":3}',
    'null',
  ])('falls back to defaults for invalid storage %s', (raw) => {
    expect(parseReminderSettings(raw)).toEqual(DEFAULT_REMINDER_SETTINGS);
  });

  it('persists a setting and loads it again', async () => {
    await saveReminderSettings({ enabled: true, intervalHours: 4 });

    expect(AsyncStorage.setItem).toHaveBeenCalledWith(
      REMINDER_SETTINGS_STORAGE_KEY,
      '{"enabled":true,"intervalHours":4}',
    );
    await expect(loadReminderSettings()).resolves.toEqual({
      enabled: true,
      intervalHours: 4,
    });
  });
});
