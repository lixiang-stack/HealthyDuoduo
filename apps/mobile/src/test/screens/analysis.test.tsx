import AsyncStorage from '@react-native-async-storage/async-storage';
import { act, fireEvent, screen, waitFor } from '@testing-library/react-native';

import { resolveApiClient } from '../../api/resolveClient';
import AnalysisScreen from '../../app/analysis/[analysis_id]';
import ReportDetailScreen from '../../app/report/[report_id]';
import PanelDetailScreen from '../../app/panel/[panel]';
import {
  ANALYSIS_CONSENT_STORAGE_KEY,
  ANALYSIS_SOURCE_STATEMENT,
} from '../../analysis/consent';
import { CURRENT_MEMBER_STORAGE_KEY } from '../../members/currentMemberStore';
import { createFakeApiClient, TEST_BASE_URL, type FakeApiOptions } from '../fakeApi';
import {
  makeAnalysisFacts,
  makeAnalysisHistory,
  makeAnalysisResult,
  makeMember,
  makePanelDetail,
  makeReportDetail,
} from '../fixtures';
import { renderScreen } from '../renderScreen';
import { mockRouter, mockUseLocalSearchParams } from '../routerMock';

jest.mock('../../api/resolveClient', () => ({
  resolveApiClient: jest.fn(),
}));

const mockedResolveApiClient = jest.mocked(resolveApiClient);

function mockApi(routes: Parameters<typeof createFakeApiClient>[0]) {
  mockedResolveApiClient.mockResolvedValue({
    client: createFakeApiClient(routes),
    baseUrl: TEST_BASE_URL,
  });
}

/** 分析入口现在先弹时间范围;默认 12 个月,可改 6 / 24。 */
async function chooseRange(months: 6 | 12 | 24 = 12) {
  expect(await screen.findByTestId('analysis-range')).toBeTruthy();
  if (months !== 12) {
    await act(async () => {
      fireEvent.press(screen.getByTestId(`analysis-range-${months}`));
    });
  }
  await act(async () => {
    fireEvent.press(screen.getByTestId('analysis-range-confirm'));
  });
}

beforeEach(async () => {
  jest.clearAllMocks();
  await AsyncStorage.clear();
  mockUseLocalSearchParams.mockReturnValue({});
});

describe('AnalysisScreen', () => {
  it('renders only the facts layer without framework or explanation and shows the source statement', async () => {
    const result = makeAnalysisResult({
      layers: {
        facts: makeAnalysisFacts({
          deviations: [
            {
              evidence_id: 'ev-a-1',
              report_id: 'a'.repeat(64),
              report_date: '2026-10-01',
              report_type: '生化',
              trend_key: '1500104A',
              name: '丙氨酸转氨酶测定',
              unit: 'U/L',
              unit_display: 'U/L',
              value: '83.5',
              ref_range: '7-40',
              flag: 'high',
            },
          ],
          history: [
            makeAnalysisHistory({
              change: 'up',
              point_count: 2,
              previous: {
                evidence_id: 'ev-a-0',
                report_id: 'a'.repeat(64),
                report_date: '2026-06-01',
                report_type: '生化',
                trend_key: '1500104A',
                name: '丙氨酸转氨酶测定',
                unit: 'U/L',
                unit_display: 'U/L',
                value: '27.4',
                ref_range: '7-40',
                flag: 'normal',
              },
            }),
          ],
          excluded: [
            {
              report_id: 'b'.repeat(64),
              report_date: '2026-08-01',
              trend_key: '1500104A',
              name: '丙氨酸转氨酶测定',
              reason: 'unverified_unit',
            },
          ],
        }),
      },
    });
    mockUseLocalSearchParams.mockReturnValue({ analysis_id: '12' });
    mockApi({ GET: { '/analyses/{analysis_id}': () => result } });

    await renderScreen(<AnalysisScreen />);

    expect(await screen.findByTestId('analysis-heading')).toHaveTextContent('肝功分析');
    expect(screen.getByTestId('analysis-scope')).toHaveTextContent(/纳入 1 份报告/);
    expect(screen.getByTestId('analysis-deviations')).toHaveTextContent(/丙氨酸转氨酶测定/);
    expect(screen.getByTestId('analysis-deviations')).toHaveTextContent(/83\.5 U\/L/);
    expect(screen.getByTestId('analysis-history')).toHaveTextContent(/较上次升高/);
    expect(screen.getByTestId('analysis-notes')).toHaveTextContent(/缺铁性贫血/);
    expect(screen.getByTestId('analysis-notes')).toHaveTextContent(/补铁治疗，一个月后复查。/);
    expect(screen.getByTestId('analysis-excluded')).toHaveTextContent(/单位未核实/);
    expect(screen.getByTestId('analysis-source-statement')).toHaveTextContent(ANALYSIS_SOURCE_STATEMENT);
  });

  it('renders the insufficient-evidence state without deviations', async () => {
    const result = makeAnalysisResult({
      status: 'insufficient_evidence',
      layers: {
        facts: makeAnalysisFacts({
          deviations: [],
          history: [],
          notes: [],
          excluded: [],
          insufficient_evidence: true,
        }),
      },
    });
    mockUseLocalSearchParams.mockReturnValue({ analysis_id: '12' });
    mockApi({ GET: { '/analyses/{analysis_id}': () => result } });

    await renderScreen(<AnalysisScreen />);

    expect(await screen.findByTestId('analysis-insufficient')).toHaveTextContent(
      '证据不足，请先核对',
    );
    expect(screen.queryByTestId('analysis-deviations')).toBeNull();
  });

  it('shows the indicator-to-panel expansion notice and stale marker', async () => {
    const result = makeAnalysisResult({
      stale: true,
      layers: {
        facts: makeAnalysisFacts({
          entry: 'indicator',
          indicator_name: '丙氨酸转氨酶',
          expanded_from_indicator: true,
        }),
      },
    });
    mockUseLocalSearchParams.mockReturnValue({ analysis_id: '12' });
    mockApi({ GET: { '/analyses/{analysis_id}': () => result } });

    await renderScreen(<AnalysisScreen />);

    expect(await screen.findByTestId('analysis-entry-notice')).toHaveTextContent(
      '已从指标「丙氨酸转氨酶」扩展至项目「肝功」',
    );
    expect(screen.getByTestId('analysis-stale')).toHaveTextContent(/数据已更新/);
  });
});

describe('Panel analysis entry', () => {
  function setupPanelApi(postHandler: (options?: FakeApiOptions) => unknown) {
    mockApi({
      GET: {
        '/members/{member_id}/panels/{panel}': () => makePanelDetail(),
        '/members': () => ({ members: [makeMember()] }),
        '/reports': () => ({ reports: [] }),
      },
      POST: { '/analyses': postHandler },
    });
  }

  beforeEach(async () => {
    mockUseLocalSearchParams.mockReturnValue({ panel: 'liver' });
    await AsyncStorage.setItem(CURRENT_MEMBER_STORAGE_KEY, '1');
  });

  it('asks for data-destination consent once, then creates the panel analysis and navigates', async () => {
    const postHandler = jest.fn((_options?: FakeApiOptions) => makeAnalysisResult());
    setupPanelApi(postHandler);

    await renderScreen(<PanelDetailScreen />);
    expect(await screen.findByTestId('panel-detail-heading')).toBeTruthy();

    fireEvent.press(screen.getByTestId('panel-analyze'));
    await chooseRange(12);
    expect(await screen.findByTestId('analysis-consent')).toHaveTextContent(/分析会把本次涉及的检验数值/);
    expect(postHandler).not.toHaveBeenCalled();

    fireEvent.press(screen.getByTestId('analysis-consent-confirm'));
    await waitFor(() => expect(postHandler).toHaveBeenCalledTimes(1));
    expect(postHandler.mock.calls[0][0]).toMatchObject({
      body: {
        member_id: 1,
        panel: 'liver',
        entry: 'panel',
        window_months: 12,
      },
    });
    expect(mockRouter.push).toHaveBeenCalledWith({
      pathname: '/analysis/[analysis_id]',
      params: { analysis_id: '12' },
    });
    expect(await AsyncStorage.getItem(ANALYSIS_CONSENT_STORAGE_KEY)).toBe('granted');
  });

  it('offers analysis from the trend sheet after consent was granted', async () => {
    await AsyncStorage.setItem(ANALYSIS_CONSENT_STORAGE_KEY, 'granted');
    const postHandler = jest.fn((_options?: FakeApiOptions) => makeAnalysisResult());
    setupPanelApi(postHandler);

    await renderScreen(<PanelDetailScreen />);
    expect(await screen.findByTestId('panel-detail-heading')).toBeTruthy();

    fireEvent.press(screen.getByTestId('panel-item-0'));
    expect(await screen.findByTestId('trend-analyze')).toBeTruthy();
    fireEvent.press(screen.getByTestId('trend-analyze'));
    await chooseRange(12);

    await waitFor(() => expect(postHandler).toHaveBeenCalledTimes(1));
    expect(postHandler.mock.calls[0][0]).toMatchObject({
      body: {
        member_id: 1,
        panel: 'liver',
        entry: 'indicator',
        indicator_name: '丙氨酸转氨酶',
        window_months: 12,
      },
    });
  });

  it('starts a clinical-note analysis from the note detail page', async () => {
    await AsyncStorage.setItem(ANALYSIS_CONSENT_STORAGE_KEY, 'granted');
    await AsyncStorage.setItem(CURRENT_MEMBER_STORAGE_KEY, '1');
    mockUseLocalSearchParams.mockReturnValue({ report_id: 'note1' });
    const postHandler = jest.fn((_options?: FakeApiOptions) => makeAnalysisResult());
    mockApi({
      GET: {
        '/reports/{report_id}': () =>
          makeReportDetail({
            report_id: 'note1',
            report_type: 'clinical_note',
            panels: ['clinical_note'],
            items: [],
            sections: [
              {
                kind: 'diagnosis',
                text: '缺铁性贫血',
                ocr_lines: [],
              },
            ],
          }),
        '/members': () => ({ members: [makeMember()] }),
      },
      POST: { '/analyses': postHandler },
    });

    await renderScreen(<ReportDetailScreen />);
    expect(await screen.findByTestId('report-analyze-note')).toBeTruthy();

    fireEvent.press(screen.getByTestId('report-analyze-note'));
    await chooseRange(12);
    await waitFor(() => expect(postHandler).toHaveBeenCalledTimes(1));
    expect(postHandler.mock.calls[0][0]).toMatchObject({
      body: {
        member_id: 1,
        panel: 'clinical_note',
        entry: 'clinical_note',
        report_sha: 'note1',
        window_months: 12,
      },
    });
  });

  it('skips the consent after it was granted and sends a single-indicator entry', async () => {
    await AsyncStorage.setItem(ANALYSIS_CONSENT_STORAGE_KEY, 'granted');
    const postHandler = jest.fn((_options?: FakeApiOptions) => makeAnalysisResult());
    setupPanelApi(postHandler);

    await renderScreen(<PanelDetailScreen />);
    expect(await screen.findByTestId('panel-detail-heading')).toBeTruthy();

    fireEvent.press(screen.getByTestId('panel-item-analyze-0'));
    await chooseRange(6);
    await waitFor(() => expect(postHandler).toHaveBeenCalledTimes(1));
    expect(screen.queryByTestId('analysis-consent')).toBeNull();
    expect(postHandler.mock.calls[0][0]).toMatchObject({
      body: {
        member_id: 1,
        panel: 'liver',
        entry: 'indicator',
        indicator_name: '丙氨酸转氨酶',
        window_months: 6,
      },
    });
  });
});
