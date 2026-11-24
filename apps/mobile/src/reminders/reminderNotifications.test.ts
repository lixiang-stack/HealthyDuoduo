import * as Notifications from 'expo-notifications';
import { IosAuthorizationStatus } from 'expo-notifications';
import { Platform } from 'react-native';

import { makeNotificationPermission } from '../test/fixtures';
import {
  cancelReminderNotifications,
  ensureReminderPermission,
  syncReminderNotification,
} from './reminderNotifications';
import type { ReminderSettings } from './reminderSettings';

const mocked = jest.mocked(Notifications);

const ENABLED: ReminderSettings = { enabled: true, intervalHours: 3 };
const NOW = new Date('2026-11-24T15:00:00+08:00');

beforeEach(() => {
  jest.clearAllMocks();
  mocked.getPermissionsAsync.mockResolvedValue(makeNotificationPermission());
  mocked.requestPermissionsAsync.mockResolvedValue(makeNotificationPermission());
  mocked.scheduleNotificationAsync.mockResolvedValue('notification-1');
  mocked.cancelAllScheduledNotificationsAsync.mockResolvedValue();
});

describe('notification permission', () => {
  it('reuses an already granted permission without prompting again', async () => {
    await expect(ensureReminderPermission()).resolves.toBe('granted');

    expect(mocked.requestPermissionsAsync).not.toHaveBeenCalled();
  });

  it('requests permission when it has not been granted yet', async () => {
    mocked.getPermissionsAsync.mockResolvedValue(
      makeNotificationPermission({ status: 'undetermined', granted: false }),
    );

    await expect(ensureReminderPermission()).resolves.toBe('granted');

    expect(mocked.requestPermissionsAsync).toHaveBeenCalledTimes(1);
  });

  it('reports a denied permission instead of pretending the reminder is on', async () => {
    mocked.getPermissionsAsync.mockResolvedValue(
      makeNotificationPermission({ status: 'undetermined', granted: false }),
    );
    mocked.requestPermissionsAsync.mockResolvedValue(
      makeNotificationPermission({ status: 'denied', granted: false }),
    );

    await expect(ensureReminderPermission()).resolves.toBe('denied');
  });

  it('accepts an iOS provisional permission', async () => {
    mocked.getPermissionsAsync.mockResolvedValue(
      makeNotificationPermission({
        granted: false,
        ios: { status: IosAuthorizationStatus.PROVISIONAL },
      }),
    );

    await expect(ensureReminderPermission()).resolves.toBe('granted');
  });
});

describe('reminder scheduling', () => {
  it('schedules one local notification from the last feed interval', async () => {
    await syncReminderNotification(ENABLED, '2026-11-24T14:00:00+08:00', NOW);

    expect(mocked.cancelAllScheduledNotificationsAsync).toHaveBeenCalledTimes(1);
    expect(mocked.scheduleNotificationAsync).toHaveBeenCalledWith({
      content: {
        title: '距上次喂养已 3 小时',
        body: '提醒只陈述时间，不提供喂养建议。',
      },
      trigger: {
        type: 'date',
        date: new Date('2026-11-24T17:00:00+08:00'),
        channelId: 'feed-reminders',
      },
    });
  });

  it('cancels without scheduling when reminders are off', async () => {
    await syncReminderNotification({ ...ENABLED, enabled: false }, undefined, NOW);

    expect(mocked.cancelAllScheduledNotificationsAsync).toHaveBeenCalledTimes(1);
    expect(mocked.scheduleNotificationAsync).not.toHaveBeenCalled();
  });

  it('cancels without scheduling when there is no feed or the interval already passed', async () => {
    await syncReminderNotification(ENABLED, undefined, NOW);
    await syncReminderNotification(ENABLED, '2026-11-24T09:00:00+08:00', NOW);

    expect(mocked.cancelAllScheduledNotificationsAsync).toHaveBeenCalledTimes(2);
    expect(mocked.scheduleNotificationAsync).not.toHaveBeenCalled();
  });

  it('cancels every scheduled notification', async () => {
    await cancelReminderNotifications();

    expect(mocked.cancelAllScheduledNotificationsAsync).toHaveBeenCalledTimes(1);
  });

  it('still schedules without a channel when Android channel creation is unavailable', async () => {
    const platform = jest.replaceProperty(Platform, 'OS', 'android');
    mocked.setNotificationChannelAsync.mockRejectedValue(new Error('channel unavailable'));

    try {
      await syncReminderNotification(ENABLED, '2026-11-24T14:00:00+08:00', NOW);

      expect(mocked.scheduleNotificationAsync).toHaveBeenCalledWith({
        content: {
          title: '距上次喂养已 3 小时',
          body: '提醒只陈述时间，不提供喂养建议。',
        },
        trigger: {
          type: 'date',
          date: new Date('2026-11-24T17:00:00+08:00'),
        },
      });
    } finally {
      platform.restore();
    }
  });
});
