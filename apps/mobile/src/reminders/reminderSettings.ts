// Device-local feeding-interval reminder settings. Stored per device because the
// reminder is a local notification capability (no account / cloud sync, ADR-0006).
import AsyncStorage from '@react-native-async-storage/async-storage';

export const REMINDER_INTERVAL_OPTIONS = [2, 3, 4] as const;
export type ReminderIntervalHours = (typeof REMINDER_INTERVAL_OPTIONS)[number];

export type ReminderSettings = {
  enabled: boolean;
  intervalHours: ReminderIntervalHours;
};

export const DEFAULT_REMINDER_SETTINGS: ReminderSettings = {
  enabled: false,
  intervalHours: 3,
};

export const REMINDER_SETTINGS_STORAGE_KEY = 'healthyduoduo.reminder';

export function parseReminderSettings(raw: string | null): ReminderSettings {
  if (raw === null) {
    return DEFAULT_REMINDER_SETTINGS;
  }
  try {
    const parsed: unknown = JSON.parse(raw);
    if (typeof parsed !== 'object' || parsed === null) {
      return DEFAULT_REMINDER_SETTINGS;
    }
    const { enabled, intervalHours } = parsed as Record<string, unknown>;
    if (
      typeof enabled !== 'boolean' ||
      !REMINDER_INTERVAL_OPTIONS.includes(intervalHours as ReminderIntervalHours)
    ) {
      return DEFAULT_REMINDER_SETTINGS;
    }
    return { enabled, intervalHours: intervalHours as ReminderIntervalHours };
  } catch {
    return DEFAULT_REMINDER_SETTINGS;
  }
}

export async function loadReminderSettings(): Promise<ReminderSettings> {
  return parseReminderSettings(await AsyncStorage.getItem(REMINDER_SETTINGS_STORAGE_KEY));
}

export async function saveReminderSettings(settings: ReminderSettings): Promise<void> {
  await AsyncStorage.setItem(REMINDER_SETTINGS_STORAGE_KEY, JSON.stringify(settings));
}
