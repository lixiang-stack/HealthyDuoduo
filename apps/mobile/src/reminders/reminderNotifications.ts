// Local-notification adapter for the feeding-interval reminder. Only local scheduling
// is used: no push tokens, no remote server and no account (ADR-0006). Android needs a
// channel before the OS permission prompt can appear; iOS uses the system prompt.
import * as Notifications from 'expo-notifications';
import { AndroidImportance, IosAuthorizationStatus } from 'expo-notifications';
import { Platform } from 'react-native';

import { reminderNotificationContent, reminderTiming } from './reminderSchedule';
import type { ReminderSettings } from './reminderSettings';

export const REMINDER_CHANNEL_ID = 'feed-reminders';

Notifications.setNotificationHandler({
  handleNotification: async () => ({
    shouldShowBanner: true,
    shouldShowList: true,
    shouldPlaySound: false,
    shouldSetBadge: false,
  }),
});

export type ReminderPermission = 'granted' | 'denied';

function allowsNotifications(status: Notifications.NotificationPermissionsStatus): boolean {
  const iosStatus = status.ios?.status;
  return (
    status.granted ||
    iosStatus === IosAuthorizationStatus.AUTHORIZED ||
    iosStatus === IosAuthorizationStatus.PROVISIONAL ||
    iosStatus === IosAuthorizationStatus.EPHEMERAL
  );
}

// Expo Go on Android can reject channel creation while still allowing local
// notifications through the fallback channel; a missing channel must not block the
// reminder. A real build normally creates it once and reuses it.
async function prepareAndroidChannel(): Promise<boolean> {
  if (Platform.OS !== 'android') {
    return true;
  }
  try {
    await Notifications.setNotificationChannelAsync(REMINDER_CHANNEL_ID, {
      name: '喂奶间隔提醒',
      importance: AndroidImportance.DEFAULT,
    });
    return true;
  } catch {
    return false;
  }
}

export async function ensureReminderPermission(): Promise<ReminderPermission> {
  await prepareAndroidChannel();
  const current = await Notifications.getPermissionsAsync();
  if (allowsNotifications(current)) {
    return 'granted';
  }
  const requested = await Notifications.requestPermissionsAsync();
  return allowsNotifications(requested) ? 'granted' : 'denied';
}

export async function cancelReminderNotifications(): Promise<void> {
  await Notifications.cancelAllScheduledNotificationsAsync();
}

/** Replace the previous reminder with one derived from the latest feed. Overdue
 *  intervals are not scheduled retroactively; the next feed starts a fresh window.
 *  `isCurrent` is re-checked right before scheduling so a superseded sync (rapid
 *  child switch) cannot re-arm the previous child's reminder. */
export async function syncReminderNotification(
  settings: ReminderSettings,
  lastFeedStartedAt: string | undefined,
  now: Date,
  isCurrent: () => boolean = () => true,
): Promise<void> {
  await cancelReminderNotifications();
  if (!settings.enabled || !isCurrent()) {
    return;
  }
  const timing = reminderTiming(lastFeedStartedAt, settings.intervalHours, now);
  if (timing.status !== 'scheduled') {
    return;
  }
  const hasChannel = await prepareAndroidChannel();
  if (!isCurrent()) {
    return;
  }
  await Notifications.scheduleNotificationAsync({
    content: reminderNotificationContent(settings.intervalHours),
    trigger: {
      type: Notifications.SchedulableTriggerInputTypes.DATE,
      date: timing.at,
      ...(hasChannel ? { channelId: REMINDER_CHANNEL_ID } : {}),
    },
  });
}
