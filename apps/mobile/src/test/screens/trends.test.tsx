import AsyncStorage from '@react-native-async-storage/async-storage';
import { fireEvent, screen, waitFor } from '@testing-library/react-native';

import { resolveApiClient } from '../../api/resolveClient';
import TrendsScreen from '../../app/trends';
import { CURRENT_MEMBER_STORAGE_KEY } from '../../members/currentMemberStore';
import { createFakeApiClient, TEST_BASE_URL } from '../fakeApi';
import {
  makeMember,
  makePanelDetail,
  makePanelItem,
  makePanelSummary,
  makeReportSummary,
  makeTrendPoint,
} from '../fixtures';
import { renderScreen } from '../renderScreen';

jest.mock('../../api/resolveClient', () => ({
  resolveApiClient: jest.fn(),
}));

const mockedResolveApiClient = jest.mocked(resolveApiClient);

beforeEach(async () => {
  jest.clearAllMocks();
  await AsyncStorage.clear();
  await AsyncStorage.setItem(CURRENT_MEMBER_STORAGE_KEY, '1');

  const oldPoint = makeTrendPoint({
    report_id: 'r2',
    report_date: '2026-09-01',
    value: '55.00',
    unit: 'umol/L',
    unit_display: 'μmol/L',
    flag: 'high',
  });
  const latestPoint = makeTrendPoint({
    report_id: 'r1',
    report_date: '2026-10-01',
    value: '70.00',
    unit: 'umol/L',
    unit_display: 'μmol/L',
    flag: 'high',
  });

  mockedResolveApiClient.mockResolvedValue({
    client: createFakeApiClient({
      GET: {
        '/members': () => ({ members: [makeMember({ member_id: 1, name: '小明' })] }),
        '/reports': () => ({
          reports: [
            makeReportSummary({ report_id: 'r1', report_date: '2026-10-01' }),
            makeReportSummary({ report_id: 'r2', report_date: '2026-09-01' }),
          ],
        }),
        '/members/{member_id}/panels': () => ({
          panels: [makePanelSummary({ abnormal_count: 1, item_count: 1 })],
        }),
        '/members/{member_id}/panels/{panel}': () =>
          makePanelDetail({
            items: [
              makePanelItem({
                unit: 'umol/L',
                latest: latestPoint,
                points: [oldPoint, latestPoint],
                report_count: 2,
              }),
            ],
          }),
      },
    }),
    baseUrl: TEST_BASE_URL,
  });
});

describe('TrendsScreen', () => {
  it('lists abnormal indicators and opens the trend detail sheet', async () => {
    await renderScreen(<TrendsScreen />);

    expect(await screen.findByTestId('trends-summary')).toHaveTextContent(
      '异常指标 1 项 · 1 个项目',
    );
    expect(screen.getByTestId('trend-indicator-0')).toHaveTextContent(/丙氨酸转氨酶/);
    expect(screen.getByTestId('trend-indicator-0')).toHaveTextContent(/μmol\/L/);
    expect(screen.getByTestId('trend-indicator-0')).not.toHaveTextContent(/umol\/L/);

    await fireEvent.press(screen.getByTestId('trend-indicator-0'));

    expect(await screen.findByTestId('trend-detail')).toBeTruthy();
    expect(screen.getByTestId('trend-point-0')).toBeTruthy();
    expect(screen.getByTestId('trend-point-1')).toBeTruthy();

    await fireEvent.press(screen.getByTestId('trend-detail-close'));
    await waitFor(() => expect(screen.queryByTestId('trend-detail')).toBeNull());
  });

  it('shows the empty-abnormal state without opening the sheet', async () => {
    mockedResolveApiClient.mockResolvedValue({
      client: createFakeApiClient({
        GET: {
          '/members': () => ({ members: [makeMember({ member_id: 1, name: '小明' })] }),
          '/reports': () => ({
            reports: [makeReportSummary({ report_id: 'r1', report_date: '2026-10-01' })],
          }),
          '/members/{member_id}/panels': () => ({ panels: [] }),
        },
      }),
      baseUrl: TEST_BASE_URL,
    });

    await renderScreen(<TrendsScreen />);

    expect(await screen.findByTestId('trend-empty-abnormal')).toBeTruthy();
    expect(screen.queryByTestId('trend-detail')).toBeNull();
  });
});
