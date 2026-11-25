import AsyncStorage from '@react-native-async-storage/async-storage';
import * as Notifications from 'expo-notifications';

import { InMemoryCareEventRepository } from '../care/careEventRepository';
import { makeFeedEvent } from '../test/fixtures';
import { REMINDER_SETTINGS_STORAGE_KEY } from './reminderSettings';
import { syncReminderToLatestFeed } from './syncReminderToLatestFeed';

const mocked = jest.mocked(Notifications);
const NOW = new Date('2026-11-24T15:00:00+08:00');
const ENABLED = '{"enabled":true,"intervalHours":3}';

function minutesBefore(minutes: number): string {
  return new Date(NOW.getTime() - minutes * 60_000).toISOString();
}

beforeEach(async () => {
  jest.clearAllMocks();
  await AsyncStorage.clear();
  mocked.scheduleNotificationAsync.mockResolvedValue('notification-1');
  mocked.cancelAllScheduledNotificationsAsync.mockResolvedValue();
});

describe('syncReminderToLatestFeed', () => {
  it('schedules from the latest feed when reminders are enabled', async () => {
    await AsyncStorage.setItem(REMINDER_SETTINGS_STORAGE_KEY, ENABLED);
    const feed = makeFeedEvent({ id: 1, started_at: minutesBefore(30) });

    await syncReminderToLatestFeed({
      repository: new InMemoryCareEventRepository([feed]),
      memberId: 1,
      now: NOW,
    });

    expect(mocked.cancelAllScheduledNotificationsAsync).toHaveBeenCalledTimes(1);
    expect(mocked.scheduleNotificationAsync).toHaveBeenCalledTimes(1);
    expect(
      (mocked.scheduleNotificationAsync.mock.calls[0][0].trigger as { date: Date }).date,
    ).toEqual(new Date(Date.parse(feed.started_at) + 3 * 60 * 60 * 1000));
  });

  it('does not reschedule the same child and latest feed again', async () => {
    await AsyncStorage.setItem(REMINDER_SETTINGS_STORAGE_KEY, ENABLED);
    const repository = new InMemoryCareEventRepository([
      makeFeedEvent({ id: 1, started_at: minutesBefore(30) }),
    ]);

    await syncReminderToLatestFeed({ repository, memberId: 1, now: NOW });
    await syncReminderToLatestFeed({ repository, memberId: 1, now: NOW });

    expect(mocked.cancelAllScheduledNotificationsAsync).toHaveBeenCalledTimes(1);
    expect(mocked.scheduleNotificationAsync).toHaveBeenCalledTimes(1);
  });

  it('reschedules when the latest feed changes', async () => {
    await AsyncStorage.setItem(REMINDER_SETTINGS_STORAGE_KEY, ENABLED);
    const repository = new InMemoryCareEventRepository([
      makeFeedEvent({ id: 1, started_at: minutesBefore(30) }),
    ]);
    await syncReminderToLatestFeed({ repository, memberId: 1, now: NOW });

    await repository.create({
      member_id: 1,
      type: 'feed',
      started_at: minutesBefore(5),
      ended_at: null,
      note: null,
      method: 'formula',
      amount_ml: 90,
      duration_min: null,
    });
    await syncReminderToLatestFeed({ repository, memberId: 1, now: NOW });

    expect(mocked.cancelAllScheduledNotificationsAsync).toHaveBeenCalledTimes(2);
    expect(mocked.scheduleNotificationAsync).toHaveBeenCalledTimes(2);
  });

  it('does not re-arm a superseded child after a newer switch', async () => {
    await AsyncStorage.setItem(REMINDER_SETTINGS_STORAGE_KEY, ENABLED);
    const first = makeFeedEvent({ id: 1, member_id: 1, started_at: minutesBefore(30) });
    const second = makeFeedEvent({ id: 2, member_id: 2, started_at: minutesBefore(10) });
    const repository = new InMemoryCareEventRepository([first, second]);
    const originalList = repository.list.bind(repository);
    let releaseSecond: () => void = () => undefined;
    const secondGate = new Promise<void>((resolve) => {
      releaseSecond = resolve;
    });
    jest.spyOn(repository, 'list').mockImplementation(async (query) => {
      if (query.member_id === 2) {
        await secondGate;
      }
      return originalList(query);
    });

    const superseded = syncReminderToLatestFeed({
      repository,
      memberId: 2,
      now: NOW,
      cancelBeforeRead: true,
    });
    const latest = syncReminderToLatestFeed({
      repository,
      memberId: 1,
      now: NOW,
      cancelBeforeRead: true,
    });
    await latest;
    releaseSecond();
    await superseded;

    expect(mocked.scheduleNotificationAsync).toHaveBeenCalledTimes(1);
    const request = mocked.scheduleNotificationAsync.mock.calls[0][0];
    expect((request.trigger as { date: Date }).date).toEqual(
      new Date(Date.parse(first.started_at) + 3 * 60 * 60 * 1000),
    );
  });

  it('cancels before reading on a child switch so a failed read stays cancelled', async () => {
    await AsyncStorage.setItem(REMINDER_SETTINGS_STORAGE_KEY, ENABLED);
    const repository = new InMemoryCareEventRepository([
      makeFeedEvent({ id: 1, started_at: minutesBefore(30) }),
    ]);
    const list = jest.spyOn(repository, 'list').mockRejectedValue(new Error('backend down'));

    await expect(
      syncReminderToLatestFeed({
        repository,
        memberId: 2,
        now: NOW,
        cancelBeforeRead: true,
      }),
    ).rejects.toThrow('backend down');

    expect(mocked.cancelAllScheduledNotificationsAsync).toHaveBeenCalledTimes(1);
    expect(mocked.scheduleNotificationAsync).not.toHaveBeenCalled();
    expect(list).toHaveBeenCalledTimes(1);
  });

  it('keeps the existing schedule when a cold-start read fails', async () => {
    await AsyncStorage.setItem(REMINDER_SETTINGS_STORAGE_KEY, ENABLED);
    const repository = new InMemoryCareEventRepository();
    jest.spyOn(repository, 'list').mockRejectedValue(new Error('backend down'));

    await expect(
      syncReminderToLatestFeed({ repository, memberId: 1, now: NOW }),
    ).rejects.toThrow('backend down');

    expect(mocked.cancelAllScheduledNotificationsAsync).not.toHaveBeenCalled();
    expect(mocked.scheduleNotificationAsync).not.toHaveBeenCalled();
  });

  it('cancels any stale schedule without scheduling when reminders are off', async () => {
    const repository = new InMemoryCareEventRepository([
      makeFeedEvent({ id: 1, started_at: minutesBefore(30) }),
    ]);
    const list = jest.spyOn(repository, 'list');

    await syncReminderToLatestFeed({ repository, memberId: 1, now: NOW });

    expect(list).not.toHaveBeenCalled();
    expect(mocked.cancelAllScheduledNotificationsAsync).toHaveBeenCalledTimes(1);
    expect(mocked.scheduleNotificationAsync).not.toHaveBeenCalled();
  });

  it('cancels without scheduling when there is no current child', async () => {
    await AsyncStorage.setItem(REMINDER_SETTINGS_STORAGE_KEY, ENABLED);

    await syncReminderToLatestFeed({
      repository: new InMemoryCareEventRepository(),
      memberId: undefined,
      now: NOW,
    });

    expect(mocked.cancelAllScheduledNotificationsAsync).toHaveBeenCalledTimes(1);
    expect(mocked.scheduleNotificationAsync).not.toHaveBeenCalled();
  });

  it('cancels without scheduling when there is no feed in the read window', async () => {
    await AsyncStorage.setItem(REMINDER_SETTINGS_STORAGE_KEY, ENABLED);

    await syncReminderToLatestFeed({
      repository: new InMemoryCareEventRepository(),
      memberId: 1,
      now: NOW,
    });

    expect(mocked.cancelAllScheduledNotificationsAsync).toHaveBeenCalledTimes(1);
    expect(mocked.scheduleNotificationAsync).not.toHaveBeenCalled();
  });
});
