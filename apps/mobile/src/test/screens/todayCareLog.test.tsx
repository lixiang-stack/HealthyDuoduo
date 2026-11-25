import AsyncStorage from '@react-native-async-storage/async-storage';
import * as Notifications from 'expo-notifications';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react-native';

import { resolveApiClient } from '../../api/resolveClient';
import { InMemoryCareEventRepository } from '../../care/careEventRepository';
import { DailyPlaceholder } from '../../daily/DailyPlaceholder';
import { CURRENT_CHILD_STORAGE_KEY } from '../../members/currentMemberStore';
import { REMINDER_SETTINGS_STORAGE_KEY } from '../../reminders/reminderSettings';
import { createFakeApiClient, TEST_BASE_URL } from '../fakeApi';
import { makeDiaperEvent, makeFeedEvent, makeMember, makeSleepEvent } from '../fixtures';
import { renderScreen } from '../renderScreen';
import { mockRouter } from '../routerMock';

jest.mock('../../api/resolveClient', () => ({
  resolveApiClient: jest.fn(),
}));

const mockedResolveApiClient = jest.mocked(resolveApiClient);
const mockedNotifications = jest.mocked(Notifications);

const doudou = makeMember({ member_id: 1, name: '豆豆', is_child: true });
const second = makeMember({ member_id: 2, name: '老二', is_child: true });
const mom = makeMember({ member_id: 9, name: '妈妈', is_child: false });

function useApi(members = [doudou, second, mom]) {
  mockedResolveApiClient.mockResolvedValue({
    client: createFakeApiClient({ GET: { '/members': () => ({ members }) } }),
    baseUrl: TEST_BASE_URL,
  });
}

function minutesAgo(minutes: number): string {
  return new Date(Date.now() - minutes * 60000).toISOString();
}

function localAt(hour: number, minute: number, dayOffset = 0): string {
  const date = new Date();
  date.setDate(date.getDate() + dayOffset);
  date.setHours(hour, minute, 0, 0);
  return date.toISOString();
}

/** RefreshControl is a native child of the ScrollView; trigger the JS handler directly. */
async function pullToRefresh(testID: string) {
  const scroll = screen.getByTestId(testID);
  await act(async () => {
    scroll.props.refreshControl.props.onRefresh();
  });
}

beforeEach(async () => {
  jest.clearAllMocks();
  await AsyncStorage.clear();
  mockedNotifications.scheduleNotificationAsync.mockResolvedValue('notification-1');
  mockedNotifications.cancelAllScheduledNotificationsAsync.mockResolvedValue();
});

describe('Today care log', () => {
  it('shows the hero, quick actions, today summary and only the current child’s timeline', async () => {
    await AsyncStorage.setItem(CURRENT_CHILD_STORAGE_KEY, String(doudou.member_id));
    useApi();
    const repository = new InMemoryCareEventRepository([
      makeFeedEvent({ id: 1, started_at: minutesAgo(30), note: '剩下 20 ml' }),
      makeDiaperEvent({ id: 2, started_at: minutesAgo(20) }),
      makeSleepEvent({ id: 4, started_at: minutesAgo(10) }),
      makeSleepEvent({
        id: 5,
        started_at: localAt(23, 50, -1),
        ended_at: localAt(6, 20),
        duration_min: 390,
      }),
      makeFeedEvent({ id: 3, member_id: second.member_id, started_at: minutesAgo(10) }),
    ]);

    await renderScreen(<DailyPlaceholder variant="today" />, { careEvents: repository });

    expect(await screen.findByTestId('today-hero-eyebrow')).toHaveTextContent(
      '豆豆 · 距上次喂养',
    );
    for (const type of ['feed', 'diaper', 'sleep', 'solid']) {
      expect(screen.getByTestId(`today-quick-${type}`)).toBeTruthy();
    }
    expect(screen.getByTestId('today-quick-sleep')).toHaveTextContent('睡眠');
    expect(screen.getByTestId('today-summary-feeds')).toHaveTextContent(/1/);
    expect(screen.getByTestId('today-summary-amount')).toHaveTextContent(/120/);
    expect(screen.getByTestId('today-summary-diapers')).toHaveTextContent(/1/);
    const timeline = await screen.findByTestId('today-timeline');
    expect(within(timeline).getByTestId('today-now')).toHaveTextContent(/现在 \d{2}:\d{2}/);
    expect(within(timeline).getByTestId('today-event-1')).toHaveTextContent(/奶粉 120 ml/);
    expect(within(timeline).getByTestId('today-event-2')).toHaveTextContent(/尿布 · 尿尿/);
    expect(within(timeline).getByTestId('today-event-node-1')).toBeTruthy();
    expect(within(timeline).getByTestId('today-event-4')).toHaveTextContent(/进行中/);
    expect(within(timeline).getByTestId('today-event-4')).toHaveTextContent(/已睡 \d+ 分钟/);
    expect(within(timeline).getByTestId('today-event-5')).toHaveTextContent(/昨天/);
    expect(within(timeline).getByTestId('today-event-5')).toHaveTextContent(/跨夜/);
    expect(within(timeline).queryByTestId('today-event-3')).toBeNull();
    expect(within(timeline).queryByTestId('today-hero')).toBeNull();
    expect(within(timeline).getByTestId('today-event-1')).toHaveTextContent(/家长记录/);
    expect(within(timeline).getByTestId('today-event-2')).toHaveTextContent(/家长记录/);
    expect(screen.queryByText('妈妈')).toBeNull();
  });

  it('opens the matching sheet from a quick action and refreshes the timeline, hero and summary after saving', async () => {
    await AsyncStorage.setItem(CURRENT_CHILD_STORAGE_KEY, String(doudou.member_id));
    useApi();
    const repository = new InMemoryCareEventRepository();

    await renderScreen(<DailyPlaceholder variant="today" />, { careEvents: repository });
    expect(await screen.findByTestId('today-empty')).toBeTruthy();
    expect(
      within(screen.getByTestId('today-timeline')).getByTestId('today-empty'),
    ).toBeTruthy();

    await fireEvent.press(screen.getByTestId('today-quick-feed'));
    expect(await screen.findByTestId('record-title')).toHaveTextContent('记录喂奶');
    expect(screen.getByTestId('record-target')).toHaveTextContent('记录给 豆豆');

    await fireEvent.press(screen.getByTestId('record-method-formula'));
    await fireEvent.changeText(screen.getByTestId('record-amount'), '90');
    await fireEvent.press(screen.getByTestId('record-submit'));

    await waitFor(() => expect(screen.queryByTestId('record-sheet')).toBeNull());
    expect(await screen.findByTestId('today-last-feed')).toHaveTextContent('刚刚');
    expect(screen.getByTestId('today-event-1')).toHaveTextContent(/奶粉 90 ml/);
    expect(screen.getByTestId('today-summary-feeds')).toHaveTextContent(/1/);
    expect(screen.getByTestId('today-summary-amount')).toHaveTextContent(/90/);
    expect(screen.queryByTestId('today-empty')).toBeNull();
  });

  it('edits an existing event with its original owner and refreshes after deletion', async () => {
    await AsyncStorage.setItem(CURRENT_CHILD_STORAGE_KEY, String(doudou.member_id));
    useApi();
    const repository = new InMemoryCareEventRepository([
      makeFeedEvent({ id: 1, started_at: minutesAgo(30) }),
    ]);

    await renderScreen(<DailyPlaceholder variant="today" />, { careEvents: repository });
    await fireEvent.press(await screen.findByTestId('today-event-1'));
    expect(await screen.findByTestId('record-title')).toHaveTextContent('修改记录');
    expect(screen.getByTestId('record-target')).toHaveTextContent('记录给 豆豆');

    await fireEvent.changeText(screen.getByTestId('record-note'), '换边');
    await fireEvent.press(screen.getByTestId('record-submit'));
    await waitFor(() => expect(screen.queryByTestId('record-sheet')).toBeNull());
    expect(await screen.findByText('换边')).toBeTruthy();

    await fireEvent.press(screen.getByTestId('today-event-1'));
    await fireEvent.press(await screen.findByTestId('record-delete'));
    await waitFor(() => expect(screen.queryByTestId('record-sheet')).toBeNull());
    expect(await screen.findByTestId('today-empty')).toBeTruthy();
  });

  it('reloads the timeline for a newly selected child', async () => {
    await AsyncStorage.setItem(CURRENT_CHILD_STORAGE_KEY, String(doudou.member_id));
    useApi();
    const repository = new InMemoryCareEventRepository([
      makeFeedEvent({ id: 1, started_at: minutesAgo(30) }),
      makeDiaperEvent({ id: 2, member_id: second.member_id, started_at: minutesAgo(10) }),
    ]);

    await renderScreen(<DailyPlaceholder variant="today" />, { careEvents: repository });
    expect(await screen.findByTestId('today-event-1')).toBeTruthy();

    await fireEvent.press(screen.getByTestId('daily-child-switcher'));
    await fireEvent.press(await screen.findByTestId('child-option-2'));

    expect(await screen.findByTestId('today-hero-eyebrow')).toHaveTextContent(
      '老二 · 距上次喂养',
    );
    expect(await screen.findByTestId('today-event-2')).toBeTruthy();
    expect(screen.queryByTestId('today-event-1')).toBeNull();
  });

  it('shows a load failure instead of an empty today state', async () => {
    await AsyncStorage.setItem(CURRENT_CHILD_STORAGE_KEY, String(doudou.member_id));
    useApi();
    const repository = new InMemoryCareEventRepository();
    jest.spyOn(repository, 'list').mockRejectedValue(new Error('backend down'));

    await renderScreen(<DailyPlaceholder variant="today" />, { careEvents: repository });

    expect(await screen.findByTestId('today-error')).toHaveTextContent(/backend down/);
    expect(screen.getByTestId('today-last-feed-detail')).toHaveTextContent(
      '护理记录加载失败',
    );
    expect(screen.queryByTestId('today-summary')).toBeNull();
    expect(screen.queryByTestId('today-empty')).toBeNull();
  });

  it('reloads the timeline on pull-to-refresh', async () => {
    await AsyncStorage.setItem(CURRENT_CHILD_STORAGE_KEY, String(doudou.member_id));
    useApi();
    const repository = new InMemoryCareEventRepository([
      makeFeedEvent({ id: 1, started_at: minutesAgo(30) }),
    ]);
    const list = jest.spyOn(repository, 'list');

    await renderScreen(<DailyPlaceholder variant="today" />, { careEvents: repository });
    await screen.findByTestId('today-event-1');
    expect(list).toHaveBeenCalledTimes(1);

    await pullToRefresh('today-timeline');

    await waitFor(() => expect(list).toHaveBeenCalledTimes(2));
  });

  it('keeps the previous timeline and summary when a refresh fails', async () => {
    await AsyncStorage.setItem(CURRENT_CHILD_STORAGE_KEY, String(doudou.member_id));
    useApi();
    const repository = new InMemoryCareEventRepository([
      makeFeedEvent({ id: 1, started_at: minutesAgo(30) }),
    ]);
    await renderScreen(<DailyPlaceholder variant="today" />, { careEvents: repository });
    expect(await screen.findByTestId('today-event-1')).toBeTruthy();
    const list = jest.spyOn(repository, 'list').mockRejectedValue(new Error('backend down'));

    await pullToRefresh('today-timeline');

    expect(await screen.findByTestId('today-error')).toHaveTextContent(/backend down/);
    expect(screen.getByTestId('today-event-1')).toBeTruthy();
    expect(screen.getByTestId('today-summary-feeds')).toHaveTextContent(/1/);
    expect(list).toHaveBeenCalledTimes(1);
  });

  it('shows the reminder card, opens its settings and stays off by default', async () => {
    await AsyncStorage.setItem(CURRENT_CHILD_STORAGE_KEY, String(doudou.member_id));
    useApi();

    await renderScreen(<DailyPlaceholder variant="today" />, {
      careEvents: new InMemoryCareEventRepository(),
    });

    expect(await screen.findByTestId('today-reminder-meta')).toHaveTextContent('未开启');
    await fireEvent.press(screen.getByTestId('today-reminder'));
    expect(mockRouter.push).toHaveBeenCalledWith('/reminder');
  });

  it('shows the last and next reminder time from the latest feed', async () => {
    await AsyncStorage.setItem(CURRENT_CHILD_STORAGE_KEY, String(doudou.member_id));
    await AsyncStorage.setItem(
      REMINDER_SETTINGS_STORAGE_KEY,
      '{"enabled":true,"intervalHours":3}',
    );
    useApi();

    await renderScreen(<DailyPlaceholder variant="today" />, {
      careEvents: new InMemoryCareEventRepository([
        makeFeedEvent({ id: 1, started_at: minutesAgo(30) }),
      ]),
    });

    expect(await screen.findByTestId('today-reminder-meta')).toHaveTextContent(
      /每 3 小时 · 上次 \d{2}:\d{2} · 下次 \d{2}:\d{2}/,
    );
  });

  it('replaces the scheduled reminder after a new feed is saved', async () => {
    await AsyncStorage.setItem(CURRENT_CHILD_STORAGE_KEY, String(doudou.member_id));
    await AsyncStorage.setItem(
      REMINDER_SETTINGS_STORAGE_KEY,
      '{"enabled":true,"intervalHours":3}',
    );
    useApi();

    await renderScreen(<DailyPlaceholder variant="today" />, {
      careEvents: new InMemoryCareEventRepository([
        makeFeedEvent({ id: 1, started_at: minutesAgo(120) }),
      ]),
    });
    const beforeOpen = Date.now();
    await fireEvent.press(await screen.findByTestId('today-quick-feed'));
    await fireEvent.press(screen.getByTestId('record-method-formula'));
    await fireEvent.changeText(screen.getByTestId('record-amount'), '90');
    await fireEvent.press(screen.getByTestId('record-submit'));

    // One schedule from the cold-start resync, one after the new feed.
    await waitFor(() =>
      expect(mockedNotifications.scheduleNotificationAsync).toHaveBeenCalledTimes(2),
    );
    expect(mockedNotifications.cancelAllScheduledNotificationsAsync).toHaveBeenCalled();
    const request = mockedNotifications.scheduleNotificationAsync.mock.calls[1][0];
    const scheduledAt = (request.trigger as { date: Date }).date.getTime();
    expect(scheduledAt).toBeGreaterThanOrEqual(beforeOpen - 60_000 + 3 * 60 * 60 * 1000);
    expect(scheduledAt).toBeLessThanOrEqual(Date.now() + 3 * 60 * 60 * 1000);
  });

  it('re-arms the reminder from persisted events on a cold start', async () => {
    await AsyncStorage.setItem(CURRENT_CHILD_STORAGE_KEY, String(doudou.member_id));
    await AsyncStorage.setItem(
      REMINDER_SETTINGS_STORAGE_KEY,
      '{"enabled":true,"intervalHours":3}',
    );
    useApi();
    const feed = makeFeedEvent({ id: 1, started_at: minutesAgo(30) });

    await renderScreen(<DailyPlaceholder variant="today" />, {
      careEvents: new InMemoryCareEventRepository([feed]),
    });

    await waitFor(() =>
      expect(mockedNotifications.scheduleNotificationAsync).toHaveBeenCalledTimes(1),
    );
    const request = mockedNotifications.scheduleNotificationAsync.mock.calls[0][0];
    expect((request.trigger as { date: Date }).date.getTime()).toBe(
      Date.parse(feed.started_at) + 3 * 60 * 60 * 1000,
    );
  });

  it('re-arms the reminder for the newly selected child', async () => {
    await AsyncStorage.setItem(CURRENT_CHILD_STORAGE_KEY, String(doudou.member_id));
    await AsyncStorage.setItem(
      REMINDER_SETTINGS_STORAGE_KEY,
      '{"enabled":true,"intervalHours":3}',
    );
    useApi();
    const secondFeed = makeFeedEvent({
      id: 2,
      member_id: second.member_id,
      started_at: minutesAgo(10),
    });

    await renderScreen(<DailyPlaceholder variant="today" />, {
      careEvents: new InMemoryCareEventRepository([
        makeFeedEvent({ id: 1, started_at: minutesAgo(30) }),
        secondFeed,
      ]),
    });
    await waitFor(() =>
      expect(mockedNotifications.scheduleNotificationAsync).toHaveBeenCalledTimes(1),
    );

    await fireEvent.press(screen.getByTestId('daily-child-switcher'));
    await fireEvent.press(await screen.findByTestId('child-option-2'));

    await waitFor(() =>
      expect(mockedNotifications.scheduleNotificationAsync).toHaveBeenCalledTimes(2),
    );
    const request = mockedNotifications.scheduleNotificationAsync.mock.calls[1][0];
    expect((request.trigger as { date: Date }).date.getTime()).toBe(
      Date.parse(secondFeed.started_at) + 3 * 60 * 60 * 1000,
    );
  });

  it('cancels the reminder when the current child has no feed', async () => {
    await AsyncStorage.setItem(CURRENT_CHILD_STORAGE_KEY, String(doudou.member_id));
    await AsyncStorage.setItem(
      REMINDER_SETTINGS_STORAGE_KEY,
      '{"enabled":true,"intervalHours":3}',
    );
    useApi();

    await renderScreen(<DailyPlaceholder variant="today" />, {
      careEvents: new InMemoryCareEventRepository(),
    });

    await waitFor(() =>
      expect(mockedNotifications.cancelAllScheduledNotificationsAsync).toHaveBeenCalled(),
    );
    expect(mockedNotifications.scheduleNotificationAsync).not.toHaveBeenCalled();
  });

  it('cancels any stale schedule without scheduling when reminders are off', async () => {
    await AsyncStorage.setItem(CURRENT_CHILD_STORAGE_KEY, String(doudou.member_id));
    useApi();

    await renderScreen(<DailyPlaceholder variant="today" />, {
      careEvents: new InMemoryCareEventRepository([
        makeFeedEvent({ id: 1, started_at: minutesAgo(30) }),
      ]),
    });
    await screen.findByTestId('today-reminder');
    await act(async () => undefined);

    expect(mockedNotifications.cancelAllScheduledNotificationsAsync).toHaveBeenCalledTimes(1);
    expect(mockedNotifications.scheduleNotificationAsync).not.toHaveBeenCalled();
  });

  it('cancels the old child’s reminder when no care child is selected', async () => {
    await AsyncStorage.setItem(
      REMINDER_SETTINGS_STORAGE_KEY,
      '{"enabled":true,"intervalHours":3}',
    );
    useApi();

    await renderScreen(<DailyPlaceholder variant="today" />, {
      careEvents: new InMemoryCareEventRepository([
        makeFeedEvent({ id: 1, started_at: minutesAgo(30) }),
      ]),
    });

    expect(await screen.findByTestId('daily-needs-child')).toBeTruthy();
    await waitFor(() =>
      expect(mockedNotifications.cancelAllScheduledNotificationsAsync).toHaveBeenCalled(),
    );
    expect(mockedNotifications.scheduleNotificationAsync).not.toHaveBeenCalled();
  });

  it('keeps one schedule when the same child and feed mount again', async () => {
    await AsyncStorage.setItem(CURRENT_CHILD_STORAGE_KEY, String(doudou.member_id));
    await AsyncStorage.setItem(
      REMINDER_SETTINGS_STORAGE_KEY,
      '{"enabled":true,"intervalHours":3}',
    );
    useApi();
    const repository = new InMemoryCareEventRepository([
      makeFeedEvent({ id: 1, started_at: minutesAgo(30) }),
    ]);

    const first = await renderScreen(<DailyPlaceholder variant="today" />, {
      careEvents: repository,
    });
    await waitFor(() =>
      expect(mockedNotifications.scheduleNotificationAsync).toHaveBeenCalledTimes(1),
    );
    first.unmount();

    await renderScreen(<DailyPlaceholder variant="today" />, { careEvents: repository });
    await screen.findByTestId('today-reminder');
    await act(async () => undefined);

    expect(mockedNotifications.scheduleNotificationAsync).toHaveBeenCalledTimes(1);
  });
});
