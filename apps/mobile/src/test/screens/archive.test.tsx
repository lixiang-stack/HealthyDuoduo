import AsyncStorage from '@react-native-async-storage/async-storage';
import { screen } from '@testing-library/react-native';

import { resolveApiClient } from '../../api/resolveClient';
import { CURRENT_MEMBER_STORAGE_KEY } from '../../members/currentMemberStore';
import { ArchiveScreen } from '../../reports/ArchiveScreen';
import { createFakeApiClient, TEST_BASE_URL } from '../fakeApi';
import {
  makeFlagCounts,
  makeMember,
  makePanelSummary,
  makeReportSummary,
} from '../fixtures';
import { renderScreen } from '../renderScreen';
import { mockUseLocalSearchParams } from '../routerMock';

jest.mock('../../api/resolveClient', () => ({
  resolveApiClient: jest.fn(),
}));

const mockedResolveApiClient = jest.mocked(resolveApiClient);

beforeEach(async () => {
  jest.clearAllMocks();
  await AsyncStorage.clear();
  await AsyncStorage.setItem(CURRENT_MEMBER_STORAGE_KEY, '1');
  mockUseLocalSearchParams.mockReturnValue({});
  mockedResolveApiClient.mockResolvedValue({
    client: createFakeApiClient({
      GET: {
        '/members': () => ({ members: [makeMember({ member_id: 1, name: '小明' })] }),
        '/reports': () => ({
          reports: [
            makeReportSummary({
              report_id: 'r1',
              report_date: '2026-10-01',
              status: 'partial',
              unreviewed_low_confidence_count: 1,
              flag_counts: makeFlagCounts({ high: 1 }),
            }),
            makeReportSummary({
              report_id: 'r2',
              report_date: '2026-09-01',
              status: 'failed',
              fail_reason: '图像无法识别',
              panels: [],
            }),
          ],
        }),
        '/members/{member_id}/panels': () => ({
          panels: [makePanelSummary({ report_count: 2, item_count: 3, abnormal_count: 1 })],
        }),
      },
    }),
    baseUrl: TEST_BASE_URL,
  });
});

describe('ArchiveScreen', () => {
  it('renders pending entries and the timeline for the current member', async () => {
    await renderScreen(<ArchiveScreen view="time" />);

    expect(await screen.findByTestId('archive-pending-partial')).toHaveTextContent(
      /待核对 · 1 份 · 1 项/,
    );
    expect(screen.getByTestId('archive-pending-failed')).toHaveTextContent(/识别失败 · 1 份/);
    expect(screen.getByTestId('report-timeline')).toBeTruthy();
    expect(screen.getByTestId('report-card-r1')).toBeTruthy();
    expect(screen.getByTestId('report-card-r2')).toBeTruthy();
    expect(screen.getByTestId('upload-fab')).toBeTruthy();
  });

  it('renders the panel browser with abnormal summary and panel rows', async () => {
    await renderScreen(<ArchiveScreen view="panel" />);

    expect(await screen.findByTestId('panel-row-liver')).toBeTruthy();
    expect(screen.getByTestId('panel-row-meta-liver')).toHaveTextContent(
      '2 份 · 1 项异常 · 3 项指标',
    );
    expect(screen.getByTestId('archive-all-abnormal')).toHaveTextContent(
      /全部异常 1 项 · 1 个项目/,
    );
  });

  it('asks for a member when none is selected', async () => {
    await AsyncStorage.clear();

    await renderScreen(<ArchiveScreen view="time" />);

    expect(await screen.findByTestId('archive-unselected')).toBeTruthy();
    expect(screen.getByText('选择一个成员查看其档案。')).toBeTruthy();
  });
});
