import AsyncStorage from '@react-native-async-storage/async-storage';
import * as Notifications from 'expo-notifications';
import { fireEvent, screen, waitFor } from '@testing-library/react-native';

import { resolveApiClient } from '../../api/resolveClient';
import ReminderScreen from '../../app/reminder';
import { InMemoryCareEventRepository } from '../../care/careEventRepository';
import { CURRENT_CHILD_STORAGE_KEY } from '../../members/currentMemberStore';
import {
  REMINDER_SETTINGS_STORAGE_KEY,
  loadReminderSettings,
} from '../../reminders/reminderSettings';
import { createFakeApiClient, TEST_BASE_URL } from '../fakeApi';
import { makeFeedEvent, makeMember, makeNotificationPermission } from '../fixtures';
import { renderScreen } from '../renderScreen';

jest.mock('../../api/resolveClient', () => ({
  resolveApiClient: jest.fn(),
}));

const mockedResolveApiClient = jest.mocked(resolveApiClient);
const mockedNotifications = jest.mocked(Notifications);
const doudou = makeMember({ member_id: 1, name: '豆豆', is_child: true });
const second = makeMember({ member_id: 2, name: '老二', is_child: true });

function useApi() {
  mockedResolveApiClient.mockResolvedValue({
    client: createFakeApiClient({ GET: { '/members': () => ({ members: [doudou, second] }) } }),
    baseUrl: TEST_BASE_URL,
  });
}

function minutesAgo(minutes: number): string {
  return new Date(Date.now() - minutes * 60000).toISOString();
}

beforeEach(async () => {
  jest.clearAllMocks();
  await AsyncStorage.clear();
  await AsyncStorage.setItem(CURRENT_CHILD_STORAGE_KEY, String(doudou.member_id));
  mockedNotifications.getPermissionsAsync.mockResolvedValue(makeNotificationPermission());
  mockedNotifications.requestPermissionsAsync.mockResolvedValue(makeNotificationPermission());
  mockedNotifications.scheduleNotificationAsync.mockResolvedValue('notification-1');
  mockedNotifications.cancelAllScheduledNotificationsAsync.mockResolvedValue();
  mockedNotifications.setNotificationChannelAsync.mockResolvedValue(null);
});

describe('ReminderScreen', () => {
  it('starts off, offers 2 / 3 / 4 hour intervals and explains the local-only limits', async () => {
    useApi();

    await renderScreen(<ReminderScreen />, { careEvents: new InMemoryCareEventRepository() });

    expect(
      (await screen.findByTestId('reminder-switch')).props.accessibilityState.checked,
    ).toBe(false);
    expect(screen.getByTestId('reminder-status')).toHaveTextContent(/默认关闭/);
    for (const hours of [2, 3, 4]) {
      expect(screen.getByTestId(`reminder-interval-${hours}`)).toBeTruthy();
    }
    expect(
      screen.getByTestId('reminder-interval-3').props.accessibilityState.selected,
    ).toBe(true);
    expect(screen.getByTestId('reminder-limitations')).toHaveTextContent(/不承诺准时/);
    expect(screen.getByTestId('reminder-limitations')).toHaveTextContent(/省电/);
  });

  it('requests permission, enables the reminder and schedules it from the last feed', async () => {
    useApi();
    const lastFeed = makeFeedEvent({ id: 1, started_at: minutesAgo(30) });
    const repository = new InMemoryCareEventRepository([lastFeed]);
    mockedNotifications.getPermissionsAsync.mockResolvedValue(
      makeNotificationPermission({ status: 'undetermined', granted: false }),
    );

    await renderScreen(<ReminderScreen />, { careEvents: repository });
    await screen.findByTestId('reminder-last-feed');
    await fireEvent.press(screen.getByTestId('reminder-switch'));

    expect(await screen.findByTestId('reminder-preview')).toHaveTextContent(
      /距上次喂养已 3 小时/,
    );
    expect(screen.getByTestId('reminder-switch').props.accessibilityState.checked).toBe(
      true,
    );
    expect(mockedNotifications.requestPermissionsAsync).toHaveBeenCalledTimes(1);
    expect(mockedNotifications.scheduleNotificationAsync).toHaveBeenCalledTimes(1);
    const request = mockedNotifications.scheduleNotificationAsync.mock.calls[0][0];
    expect(request.content.title).toBe('距上次喂养已 3 小时');
    expect(request.trigger).toMatchObject({ type: 'date', channelId: 'feed-reminders' });
    expect((request.trigger as { date: Date }).date.getTime()).toBe(
      Date.parse(lastFeed.started_at) + 3 * 60 * 60 * 1000,
    );
    await expect(loadReminderSettings()).resolves.toEqual({
      enabled: true,
      intervalHours: 3,
    });
  });

  it('reschedules when the interval changes while enabled', async () => {
    useApi();
    await AsyncStorage.setItem(
      REMINDER_SETTINGS_STORAGE_KEY,
      '{"enabled":true,"intervalHours":3}',
    );
    const lastFeed = makeFeedEvent({ id: 1, started_at: minutesAgo(30) });

    await renderScreen(<ReminderScreen />, {
      careEvents: new InMemoryCareEventRepository([lastFeed]),
    });
    expect(
      (await screen.findByTestId('reminder-switch')).props.accessibilityState.checked,
    ).toBe(true);
    await fireEvent.press(screen.getByTestId('reminder-interval-2'));

    await waitFor(() =>
      expect(mockedNotifications.scheduleNotificationAsync).toHaveBeenCalledTimes(1),
    );
    const request = mockedNotifications.scheduleNotificationAsync.mock.calls[0][0];
    expect((request.trigger as { date: Date }).date.getTime()).toBe(
      Date.parse(lastFeed.started_at) + 2 * 60 * 60 * 1000,
    );
    expect(
      screen.getByTestId('reminder-interval-2').props.accessibilityState.selected,
    ).toBe(true);
    await expect(loadReminderSettings()).resolves.toEqual({
      enabled: true,
      intervalHours: 2,
    });
  });

  it('keeps the reminder off and explains when notification permission is denied', async () => {
    useApi();
    mockedNotifications.getPermissionsAsync.mockResolvedValue(
      makeNotificationPermission({ status: 'undetermined', granted: false }),
    );
    mockedNotifications.requestPermissionsAsync.mockResolvedValue(
      makeNotificationPermission({ status: 'denied', granted: false }),
    );

    await renderScreen(<ReminderScreen />, {
      careEvents: new InMemoryCareEventRepository([
        makeFeedEvent({ id: 1, started_at: minutesAgo(30) }),
      ]),
    });
    await screen.findByTestId('reminder-last-feed');
    await fireEvent.press(screen.getByTestId('reminder-switch'));

    expect(await screen.findByTestId('reminder-permission-error')).toHaveTextContent(
      /通知权限/,
    );
    expect(screen.getByTestId('reminder-switch').props.accessibilityState.checked).toBe(
      false,
    );
    expect(mockedNotifications.scheduleNotificationAsync).not.toHaveBeenCalled();
    await expect(loadReminderSettings()).resolves.toEqual({
      enabled: false,
      intervalHours: 3,
    });
  });

  it('cancels every scheduled notification when switched off', async () => {
    useApi();
    await AsyncStorage.setItem(
      REMINDER_SETTINGS_STORAGE_KEY,
      '{"enabled":true,"intervalHours":3}',
    );

    await renderScreen(<ReminderScreen />, {
      careEvents: new InMemoryCareEventRepository([
        makeFeedEvent({ id: 1, started_at: minutesAgo(30) }),
      ]),
    });
    expect(
      (await screen.findByTestId('reminder-switch')).props.accessibilityState.checked,
    ).toBe(true);
    await fireEvent.press(screen.getByTestId('reminder-switch'));

    await waitFor(() =>
      expect(mockedNotifications.cancelAllScheduledNotificationsAsync).toHaveBeenCalled(),
    );
    expect(screen.getByTestId('reminder-switch').props.accessibilityState.checked).toBe(
      false,
    );
    await expect(loadReminderSettings()).resolves.toEqual({
      enabled: false,
      intervalHours: 3,
    });
  });

  it('enables without inventing a time when the child has no feed yet', async () => {
    useApi();

    await renderScreen(<ReminderScreen />, { careEvents: new InMemoryCareEventRepository() });
    await fireEvent.press(await screen.findByTestId('reminder-switch'));

    expect(await screen.findByTestId('reminder-status')).toHaveTextContent(
      /开启后从上次喂养开始计时/,
    );
    expect(mockedNotifications.scheduleNotificationAsync).not.toHaveBeenCalled();
  });
});
