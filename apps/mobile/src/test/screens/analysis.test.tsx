import AsyncStorage from '@react-native-async-storage/async-storage';
import { act, fireEvent, screen, waitFor } from '@testing-library/react-native';

import type { AnalysisExplanation } from '../../api/analyses';
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
  makeAnalysisFramework,
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

  it('renders framework labels with fixed explanations and sources', async () => {
    const result = makeAnalysisResult();
    mockUseLocalSearchParams.mockReturnValue({ analysis_id: '12' });
    mockApi({ GET: { '/analyses/{analysis_id}': () => result } });

    await renderScreen(<AnalysisScreen />);

    expect(await screen.findByTestId('analysis-framework')).toHaveTextContent(/转氨酶升高：轻度/);
    expect(screen.getByTestId('analysis-framework')).toHaveTextContent(/转氨酶升高 2–5×ULN/);
    expect(screen.getByTestId('analysis-framework')).toHaveTextContent(/依据 ACG 2016/);
  });

  it('renders CBC morphology, Hb cutoff and BMI percentile labels', async () => {
    const framework = makeAnalysisFramework({
      labels: [
        {
          code: 'cbc_hemoglobin_cutoff',
          label: '血红蛋白低于该年龄段切点',
          explanation: '按 WHO 2024 年龄与性别血红蛋白切点；单据参考范围 115-150。切点标签不是诊断。',
          value: '108 g/L（WHO 切点 115 g/L）',
          evidence_ids: ['ev-a-1'],
          sources: [{ standard: 'WHO 2024', note: '血红蛋白贫血切点', url: null }],
        },
        {
          code: 'cbc_anemia_morphology',
          label: '小细胞低色素性形态，RDW 升高',
          explanation: 'MCV 偏低；MCH / MCHC 偏低。这是形态学描述，不是诊断。',
          value: 'MCV 72 fL · MCH 24 pg',
          evidence_ids: ['ev-a-2'],
          sources: [{ standard: 'WS/T 405 / WS/T 779', note: '血细胞分析参考区间', url: null }],
        },
        {
          code: 'bmi_z_score',
          label: 'BMI 年龄别 z 值与百分位',
          explanation: '按 WHO 2007 5–19 岁 BMI 年龄别参考计算。',
          value: 'z = +1.30（第 90 百分位）',
          evidence_ids: [],
          sources: [{ standard: 'WHO 2007', note: 'BMI-for-age', url: null }],
        },
      ],
    });
    mockUseLocalSearchParams.mockReturnValue({ analysis_id: '12' });
    mockApi({
      GET: {
        '/analyses/{analysis_id}': () =>
          makeAnalysisResult({
            layers: { facts: makeAnalysisFacts({ panel: 'cbc' }), framework },
          }),
      },
    });

    await renderScreen(<AnalysisScreen />);

    expect(await screen.findByTestId('analysis-framework')).toHaveTextContent(/小细胞低色素性形态/);
    expect(screen.getByTestId('analysis-framework')).toHaveTextContent(/WHO 切点 115 g\/L/);
    expect(screen.getByTestId('analysis-framework')).toHaveTextContent(/第 90 百分位/);
  });

  it('renders the explanation bridge text and marks truncation', async () => {
    const result = makeAnalysisResult({
      layers: {
        facts: makeAnalysisFacts(),
        explanation: {
          sections: [
            {
              entry_id: 'liver-hepatocellular-mild',
              evidence_ids: ['ev-aaaaaaaaaaaa-1'],
              bridge_text: '先看本次转氨酶与碱性磷酸酶的关系。',
              explanation: '转氨酶升高而碱性磷酸酶和胆红素正常，通常提示肝细胞受累，而不是胆道淤积。',
              check_directions: ['复查肝功能，观察是否持续升高'],
              general_advice: ['规律作息'],
              track_items: ['ALT', 'AST'],
              source_ids: [{ standard: 'AASLD', note: 'R 值分型', url: null }],
            },
          ],
          truncated: true,
        },
      },
    });
    mockUseLocalSearchParams.mockReturnValue({ analysis_id: '12' });
    mockApi({ GET: { '/analyses/{analysis_id}': () => result } });

    await renderScreen(<AnalysisScreen />);

    expect(await screen.findByTestId('analysis-explanation')).toHaveTextContent(/先看本次转氨酶/);
    const section = screen.getByTestId('analysis-explanation-section-liver-hepatocellular-mild');
    expect(section).toHaveTextContent(/可能原因/);
    expect(section).toHaveTextContent(/通常提示肝细胞受累/);
    expect(section).toHaveTextContent(/进一步检查/);
    expect(section).toHaveTextContent(/复查肝功能/);
    expect(section).toHaveTextContent(/生活建议/);
    expect(section).toHaveTextContent(/规律作息/);
    expect(section).toHaveTextContent(/长期跟踪/);
    expect(section).toHaveTextContent(/建议持续记录：ALT、AST/);
    expect(screen.getByTestId('analysis-explanation')).toHaveTextContent(/已截断/);
    expect(screen.queryByTestId('analysis-explanation-sources-detail-liver-hepatocellular-mild')).toBeNull();

    fireEvent.press(screen.getByTestId('analysis-explanation-sources-liver-hepatocellular-mild'));
    expect(
      await screen.findByTestId('analysis-explanation-sources-detail-liver-hepatocellular-mild'),
    ).toHaveTextContent(/AASLD · R 值分型/);
  });

  it('renders legacy explanation sections that predate the four-part fields', async () => {
    const legacy = {
      entry_id: 'liver-hepatocellular-mild',
      evidence_ids: [],
      bridge_text: '旧分析只有连接语。',
      source_ids: [],
    } as unknown as AnalysisExplanation['sections'][number];
    const result = makeAnalysisResult({
      layers: {
        facts: makeAnalysisFacts(),
        explanation: { sections: [legacy], truncated: false },
      },
    });
    mockUseLocalSearchParams.mockReturnValue({ analysis_id: '12' });
    mockApi({ GET: { '/analyses/{analysis_id}': () => result } });

    await renderScreen(<AnalysisScreen />);

    expect(await screen.findByTestId('analysis-explanation')).toHaveTextContent(/旧分析只有连接语/);
    expect(screen.queryByText('可能原因')).toBeNull();
  });

  it('shows the quota message in the explanation position without disabling the result', async () => {
    const result = makeAnalysisResult({
      layers: { facts: makeAnalysisFacts() },
      quota: {
        used_today: 20,
        limit_today: 20,
        explanation_available: false,
        explanation_reason: 'quota_exhausted',
      },
    });
    mockUseLocalSearchParams.mockReturnValue({ analysis_id: '12' });
    mockApi({ GET: { '/analyses/{analysis_id}': () => result } });

    await renderScreen(<AnalysisScreen />);

    expect(await screen.findByTestId('analysis-explanation-unavailable')).toHaveTextContent(
      /今日额度已用完，事实与判读仍可用/,
    );
    expect(screen.getByTestId('analysis-quota-remaining')).toHaveTextContent(/今日剩余解读次数 0 次/);
    expect(screen.getByTestId('analysis-feedback-open')).toBeTruthy();
  });

  it('records a "this is wrong" feedback with the optional reason', async () => {
    const postHandler = jest.fn((_options?: FakeApiOptions) => ({
      feedback_id: 7,
      created_at: '2026-11-03T12:00:00Z',
    }));
    mockUseLocalSearchParams.mockReturnValue({ analysis_id: '12' });
    mockApi({
      GET: { '/analyses/{analysis_id}': () => makeAnalysisResult() },
      POST: { '/analyses/{analysis_id}/feedback': postHandler },
    });

    await renderScreen(<AnalysisScreen />);
    fireEvent.press(await screen.findByTestId('analysis-feedback-open'));
    const reasonInput = await screen.findByTestId('analysis-feedback-reason');
    fireEvent.changeText(reasonInput, '解释和数值对不上');
    await waitFor(() =>
      expect(screen.getByTestId('analysis-feedback-reason').props.value).toBe('解释和数值对不上'),
    );
    fireEvent.press(screen.getByTestId('analysis-feedback-submit'));

    await waitFor(() => expect(postHandler).toHaveBeenCalledTimes(1));
    expect(postHandler.mock.calls[0][0]).toMatchObject({
      params: { path: { analysis_id: 12 } },
      body: { reason: '解释和数值对不上' },
    });
    expect(await screen.findByTestId('analysis-feedback-done')).toHaveTextContent(/已记录/);
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
