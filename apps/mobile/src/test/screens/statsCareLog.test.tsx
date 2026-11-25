import AsyncStorage from '@react-native-async-storage/async-storage';
import * as Notifications from 'expo-notifications';
import { act, fireEvent, screen, waitFor } from '@testing-library/react-native';

import { resolveApiClient } from '../../api/resolveClient';
import { InMemoryCareEventRepository } from '../../care/careEventRepository';
import { DailyPlaceholder } from '../../daily/DailyPlaceholder';
import { CURRENT_CHILD_STORAGE_KEY } from '../../members/currentMemberStore';
import { REMINDER_SETTINGS_STORAGE_KEY } from '../../reminders/reminderSettings';
import { createFakeApiClient, TEST_BASE_URL } from '../fakeApi';
import {
  makeDiaperEvent,
  makeFeedEvent,
  makeMember,
  makeSleepEvent,
} from '../fixtures';
import { renderScreen } from '../renderScreen';
import { mockRouter } from '../routerMock';

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

function at(daysAgo: number, hour: number, minute = 0): string {
  const date = new Date();
  date.setDate(date.getDate() - daysAgo);
  date.setHours(hour, minute, 0, 0);
  return date.toISOString();
}

function minutesAgo(minutes: number): string {
  return new Date(Date.now() - minutes * 60000).toISOString();
}

/** RefreshControl is a native child of the ScrollView; trigger the JS handler directly. */
async function pullToRefresh(testID: string) {
  const scroll = screen.getByTestId(testID);
  await act(async () => {
    scroll.props.refreshControl.props.onRefresh();
  });
}

function statsEvents(memberId = doudou.member_id) {
  return [
    makeFeedEvent({ id: 1, member_id: memberId, started_at: at(0, 10), amount_ml: 120 }),
    makeFeedEvent({
      id: 2,
      member_id: memberId,
      started_at: at(0, 12),
      method: 'breast',
      amount_ml: null,
      duration_min: 15,
    }),
    makeFeedEvent({ id: 3, member_id: memberId, started_at: at(1, 9), amount_ml: 90 }),
    makeDiaperEvent({ id: 4, member_id: memberId, started_at: at(0, 8) }),
    makeSleepEvent({
      id: 5,
      member_id: memberId,
      started_at: at(0, 13),
      ended_at: at(0, 14, 30),
      duration_min: 90,
    }),
  ];
}

beforeEach(async () => {
  jest.clearAllMocks();
  await AsyncStorage.clear();
  await AsyncStorage.setItem(CURRENT_CHILD_STORAGE_KEY, String(doudou.member_id));
  mockedNotifications.scheduleNotificationAsync.mockResolvedValue('notification-1');
  mockedNotifications.cancelAllScheduledNotificationsAsync.mockResolvedValue();
});

describe('Stats care log', () => {
  it('shows the four metrics, three KPIs, a today-highlighted bar chart and no reminder card', async () => {
    useApi();

    await renderScreen(<DailyPlaceholder variant="stats" />, {
      careEvents: new InMemoryCareEventRepository(statsEvents()),
    });

    expect(await screen.findByTestId('stats-care-log')).toBeTruthy();
    for (const metric of ['feeds', 'amount', 'sleep', 'diaper']) {
      expect(screen.getByTestId(`stats-metric-${metric}`)).toBeTruthy();
    }
    expect(screen.getByTestId('stats-metric-feeds').props.accessibilityState.selected).toBe(
      true,
    );
    expect(screen.getByTestId('stats-kpi-0')).toHaveTextContent(/7 天喂养/);
    expect(screen.getByTestId('stats-kpi-0')).toHaveTextContent(/3/);
    expect(screen.getByTestId('stats-kpi-1')).toHaveTextContent(/日均/);
    expect(screen.getByTestId('stats-kpi-2')).toHaveTextContent(/平均间隔/);
    expect(screen.getByTestId('stats-chart-bar')).toBeTruthy();
    expect(screen.getByTestId('stats-bar-today')).toBeTruthy();
    expect(screen.getAllByText('今天').length).toBeGreaterThan(0);
    expect(screen.queryByTestId('today-reminder')).toBeNull();
    expect(screen.queryByText(/喂奶间隔提醒/)).toBeNull();
    expect(screen.getByTestId('stats-caption')).toHaveTextContent(
      /来自家长记录，不构成建议。/,
    );
  });

  it('switches metrics and chart types without leaving the stats view', async () => {
    useApi();

    await renderScreen(<DailyPlaceholder variant="stats" />, {
      careEvents: new InMemoryCareEventRepository(statsEvents()),
    });
    await screen.findByTestId('stats-care-log');

    await fireEvent.press(screen.getByTestId('stats-metric-amount'));
    expect(screen.getByTestId('stats-kpi-0')).toHaveTextContent(/7 天奶量/);
    expect(screen.getByTestId('stats-kpi-0')).toHaveTextContent(/210/);
    expect(screen.getByTestId('stats-kpi-1')).toHaveTextContent(/ml/);

    await fireEvent.press(screen.getByTestId('stats-chart-tab-stacked'));
    expect(screen.getByTestId('stats-chart-stacked')).toBeTruthy();
    expect(screen.queryByTestId('stats-chart-bar')).toBeNull();
    expect(screen.getByTestId('stats-segment-breast')).toHaveTextContent(/母乳/);
    expect(screen.getByTestId('stats-segment-formula')).toHaveTextContent(/奶粉/);

    await fireEvent.press(screen.getByTestId('stats-chart-tab-line'));
    expect(screen.getByTestId('stats-chart-line')).toBeTruthy();
    expect(screen.queryByTestId('stats-chart-stacked')).toBeNull();

    await fireEvent.press(screen.getByTestId('stats-metric-sleep'));
    expect(screen.getByTestId('stats-kpi-0')).toHaveTextContent(/日均睡眠/);
    expect(screen.getByTestId('stats-kpi-0')).toHaveTextContent(/0.2 小时/);
    expect(screen.getByTestId('stats-chart-line')).toBeTruthy();

    await fireEvent.press(screen.getByTestId('stats-metric-diaper'));
    expect(screen.getByTestId('stats-kpi-0')).toHaveTextContent(/7 天尿布/);
    expect(screen.getByTestId('stats-kpi-0')).toHaveTextContent(/1/);
  });

  it('shows an empty state with a way back to today when the metric has no data', async () => {
    useApi();

    await renderScreen(<DailyPlaceholder variant="stats" />, {
      careEvents: new InMemoryCareEventRepository(),
    });

    expect(await screen.findByTestId('stats-empty')).toHaveTextContent(/还没有记录/);
    expect(screen.queryByTestId('stats-chart-bar')).toBeNull();
    await fireEvent.press(screen.getByTestId('stats-go-today'));
    expect(mockRouter.replace).toHaveBeenCalledWith('/care/today');
  });

  it('renders a single recorded day without hiding the chart', async () => {
    useApi();

    await renderScreen(<DailyPlaceholder variant="stats" />, {
      careEvents: new InMemoryCareEventRepository([
        makeFeedEvent({ id: 1, started_at: at(0, 10), amount_ml: 120 }),
      ]),
    });

    expect(await screen.findByTestId('stats-chart-bar')).toBeTruthy();
    expect(screen.getByTestId('stats-day-value-6')).toHaveTextContent('1');
    expect(screen.getByTestId('stats-day-value-0')).toHaveTextContent('0');
    expect(screen.getByTestId('stats-bar-today')).toBeTruthy();
    await fireEvent.press(screen.getByTestId('stats-chart-tab-line'));
    expect(screen.getByTestId('stats-chart-line')).toBeTruthy();
  });

  it('reloads the stats for a newly selected child', async () => {
    useApi();
    const repository = new InMemoryCareEventRepository([
      makeFeedEvent({ id: 1, started_at: at(0, 10) }),
      makeFeedEvent({ id: 2, member_id: second.member_id, started_at: at(0, 11) }),
      makeFeedEvent({ id: 3, member_id: second.member_id, started_at: at(0, 12) }),
    ]);

    await renderScreen(<DailyPlaceholder variant="stats" />, { careEvents: repository });
    expect(await screen.findByTestId('stats-kpi-0')).toHaveTextContent(/1/);

    await fireEvent.press(screen.getByTestId('daily-child-switcher'));
    await fireEvent.press(await screen.findByTestId('child-option-2'));

    await waitFor(() =>
      expect(screen.getByTestId('stats-kpi-0')).toHaveTextContent(/2/),
    );
    expect(screen.getByTestId('stats-metric-feeds').props.accessibilityState.selected).toBe(
      true,
    );
  });

  it('shows a load failure instead of an empty stats state', async () => {
    useApi();
    const repository = new InMemoryCareEventRepository();
    jest.spyOn(repository, 'list').mockRejectedValue(new Error('backend down'));

    await renderScreen(<DailyPlaceholder variant="stats" />, { careEvents: repository });

    expect(await screen.findByTestId('stats-error')).toHaveTextContent(/backend down/);
    expect(screen.queryByTestId('stats-empty')).toBeNull();
  });

  it('reloads the seven-day read model on pull-to-refresh', async () => {
    useApi();
    const repository = new InMemoryCareEventRepository(statsEvents());
    const list = jest.spyOn(repository, 'list');

    await renderScreen(<DailyPlaceholder variant="stats" />, { careEvents: repository });
    await screen.findByTestId('stats-care-log');
    expect(list).toHaveBeenCalledTimes(1);

    await pullToRefresh('daily-stats');

    await waitFor(() => expect(list).toHaveBeenCalledTimes(2));
  });

  it('keeps the previous stats when a refresh fails', async () => {
    useApi();
    const repository = new InMemoryCareEventRepository(statsEvents());
    await renderScreen(<DailyPlaceholder variant="stats" />, { careEvents: repository });
    expect(await screen.findByTestId('stats-kpi-0')).toHaveTextContent(/3/);
    jest.spyOn(repository, 'list').mockRejectedValue(new Error('backend down'));

    await pullToRefresh('daily-stats');

    expect(await screen.findByTestId('stats-error')).toHaveTextContent(/backend down/);
    expect(screen.getByTestId('stats-kpi-0')).toHaveTextContent(/3/);
    expect(screen.queryByTestId('stats-empty')).toBeNull();
  });

  it('re-arms the reminder for the child selected on the stats page', async () => {
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

    await renderScreen(<DailyPlaceholder variant="stats" />, {
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
});
