import AsyncStorage from '@react-native-async-storage/async-storage';
import * as Linking from 'expo-linking';
import { act, fireEvent, screen, waitFor } from '@testing-library/react-native';

import type { AnalysisEvidencePoint, AnalysisExplanation } from '../../api/analyses';
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
  makeAnalysisEvidencePoint,
  makeAnalysisFacts,
  makeAnalysisFramework,
  makeAnalysisResult,
  makeMember,
  makePanelDetail,
  makeReportDetail,
} from '../fixtures';
import { renderScreen } from '../renderScreen';
import { mockRouter, mockUseLocalSearchParams } from '../routerMock';

jest.mock('expo-linking', () => ({ openURL: jest.fn() }));

jest.mock('../../api/resolveClient', () => ({
  resolveApiClient: jest.fn(),
}));

const mockedResolveApiClient = jest.mocked(resolveApiClient);
const mockedOpenURL = jest.mocked(Linking.openURL);

const REPORT_A = 'a'.repeat(64);
const REPORT_B = 'b'.repeat(64);

function mockApi(routes: Parameters<typeof createFakeApiClient>[0]) {
  mockedResolveApiClient.mockResolvedValue({
    client: createFakeApiClient(routes),
    baseUrl: TEST_BASE_URL,
  });
}

function point(overrides: Partial<AnalysisEvidencePoint> = {}): AnalysisEvidencePoint {
  return makeAnalysisEvidencePoint(overrides);
}

function explanation(
  overrides: Partial<AnalysisExplanation['sections'][number]> = {},
): AnalysisExplanation {
  return {
    sections: [
      {
        entry_id: 'liver-hepatocellular-mild',
        evidence_ids: ['ev-aaaaaaaaaaaa-1'],
        bridge_text: 'T-0 时先看本次转氨酶与碱性磷酸酶的关系。',
        explanation: '转氨酶升高而碱性磷酸酶和胆红素正常，通常提示肝细胞受累，而不是胆道淤积。',
        check_directions: ['复查肝功能，观察是否持续升高'],
        general_advice: ['规律作息'],
        track_items: ['ALT', 'AST'],
        source_ids: [],
        ...overrides,
      },
    ],
    truncated: false,
  };
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

/** 断言结论页区块在渲染树中的先后顺序。 */
function analysisSectionOrder(): string[] {
  return screen
    .getAllByTestId(
      /^analysis-(explanation|framework|deviations|notes|data-note)$/,
    )
    .map((node) => String(node.props.testID));
}

beforeEach(async () => {
  jest.clearAllMocks();
  mockedOpenURL.mockResolvedValue(true);
  await AsyncStorage.clear();
  mockUseLocalSearchParams.mockReturnValue({});
});

describe('AnalysisScreen', () => {
  it('puts the conclusion first and keeps only the conclusion-page sections', async () => {
    const result = makeAnalysisResult({
      sources: [
        {
          standard: 'ACG 2016',
          note: '转氨酶升高分级',
          url: 'https://example.com/acg',
        },
      ],
      layers: {
        framework: makeAnalysisFramework(),
        facts: makeAnalysisFacts({
          deviations: [
            point({ evidence_id: 'ev-a-1', name: '丙氨酸转氨酶测定', value: '83.5' }),
          ],
        }),
        explanation: explanation({ evidence_ids: ['ev-a-1'] }),
      },
    });
    mockUseLocalSearchParams.mockReturnValue({ analysis_id: '12' });
    mockApi({ GET: { '/analyses/{analysis_id}': () => result } });

    await renderScreen(<AnalysisScreen />);

    expect(await screen.findByTestId('analysis-meta')).toHaveTextContent('小明 · 肝功');
    expect(screen.getByTestId('analysis-created-at')).toHaveTextContent(
      /^生成于 \d{4}-\d{2}-\d{2} \d{2}:\d{2}$/,
    );
    expect(screen.queryByTestId('analysis-heading')).toBeNull();
    expect(screen.getByTestId('analysis-explanation')).toHaveTextContent(
      /1 项异常里，本次解读解释了 1 项/,
    );
    expect(screen.getByTestId('analysis-framework')).toHaveTextContent(/指标分级/);
    expect(screen.getByTestId('analysis-deviations')).toHaveTextContent(/异常 1 项/);
    expect(screen.getByTestId('analysis-deviations')).toHaveTextContent(/丙氨酸氨基转移酶（ALT）/);
    expect(screen.getByTestId('analysis-deviations')).toHaveTextContent(/83\.5 U\/L/);
    expect(screen.getByTestId('analysis-notes')).toHaveTextContent(/医生当时诊断（原文）/);
    expect(screen.getByTestId('analysis-notes')).toHaveTextContent(/医生当时处理（原文，不是建议）/);
    expect(screen.getByTestId('analysis-notes')).toHaveTextContent(/请勿按旧处方自行用药/);
    expect(screen.getByTestId('analysis-data-note')).toHaveTextContent(/数据说明/);
    expect(screen.getByTestId('analysis-source-statement')).toHaveTextContent(ANALYSIS_SOURCE_STATEMENT);

    expect(screen.queryByTestId('analysis-history')).toBeNull();
    expect(screen.queryByTestId('analysis-scope')).toBeNull();
    expect(screen.queryByTestId('analysis-excluded')).toBeNull();
    expect(screen.queryByTestId('analysis-body-growth-card')).toBeNull();
    expect(screen.queryByTestId('analysis-snapshot-version')).toBeNull();
    expect(analysisSectionOrder()).toEqual([
      'analysis-explanation',
      'analysis-framework',
      'analysis-deviations',
      'analysis-notes',
      'analysis-data-note',
    ]);
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
      /证据不足，请先核对/,
    );
    expect(screen.getByTestId('analysis-insufficient')).toHaveTextContent(/去核对报告/);
    expect(screen.queryByTestId('analysis-deviations')).toBeNull();
  });

  it('renders framework labels with plain source references and no raw ULN', async () => {
    const result = makeAnalysisResult();
    mockUseLocalSearchParams.mockReturnValue({ analysis_id: '12' });
    mockApi({ GET: { '/analyses/{analysis_id}': () => result } });

    await renderScreen(<AnalysisScreen />);

    expect(await screen.findByTestId('analysis-framework')).toHaveTextContent(/指标分级/);
    expect(screen.getByTestId('analysis-framework')).toHaveTextContent(/转氨酶升高：轻度/);
    expect(screen.getByTestId('analysis-framework')).toHaveTextContent(/2–5 倍正常上限/);
    expect(screen.getByTestId('analysis-framework')).not.toHaveTextContent(/×ULN/);
    expect(screen.getByTestId('analysis-framework')).toHaveTextContent(
      /参考：转氨酶升高分级（ACG 2016）/,
    );
  });

  it('does not show BMI / body-growth labels on the liver result page', async () => {
    const framework = makeAnalysisFramework({
      labels: [
        ...makeAnalysisFramework().labels,
        {
          code: 'bmi',
          label: 'BMI 23.1',
          explanation: '按测量时点的身高体重计算。',
          value: '23.1 kg/m²（测量日期 2026-10-01）',
          evidence_ids: [],
          sources: [{ standard: 'WS/T 586—2018', note: 'BMI 界值', url: null }],
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
          makeAnalysisResult({ layers: { facts: makeAnalysisFacts(), framework } }),
      },
    });

    await renderScreen(<AnalysisScreen />);

    expect(await screen.findByTestId('analysis-framework')).toHaveTextContent(/指标分级/);
    expect(screen.queryByText('体格发育')).toBeNull();
    expect(screen.queryByText(/BMI 23\.1/)).toBeNull();
    expect(screen.queryByText(/第 90 百分位/)).toBeNull();
  });

  it('shows coverage and explains that TP / albumin are in range but have no reviewed entry', async () => {
    const deviations = [
      point({ evidence_id: 'ev-1', name: '丙氨酸氨基转移酶测定', value: '83.5', flag: 'high' }),
      point({ evidence_id: 'ev-2', name: '天门冬氨酸氨基转移酶测定', value: '45', flag: 'high' }),
      point({ evidence_id: 'ev-3', name: '总蛋白测定', value: '60.70', flag: 'low' }),
      point({ evidence_id: 'ev-4', name: '白蛋白测定', value: '35.50', flag: 'low' }),
    ];
    const result = makeAnalysisResult({
      layers: {
        facts: makeAnalysisFacts({ deviations }),
        explanation: explanation({ evidence_ids: ['ev-1', 'ev-2'] }),
      },
    });
    mockUseLocalSearchParams.mockReturnValue({ analysis_id: '12' });
    mockApi({ GET: { '/analyses/{analysis_id}': () => result } });

    await renderScreen(<AnalysisScreen />);

    const coverage = await screen.findByTestId('analysis-explanation-coverage');
    expect(coverage).toHaveTextContent(/4 项异常里，本次解读解释了 2 项/);
    expect(coverage).toHaveTextContent(
      /另外 2 项（总蛋白、白蛋白）在日期范围内，但暂无对应的审核解读条目/,
    );
    expect(screen.getByTestId('analysis-deviation-coverage-ev-1')).toHaveTextContent('已解读');
    expect(screen.getByTestId('analysis-deviation-coverage-ev-2')).toHaveTextContent('已解读');
    expect(screen.getByTestId('analysis-deviation-coverage-ev-3')).toHaveTextContent(
      '暂无对应解读',
    );
    expect(screen.getByTestId('analysis-deviation-coverage-ev-4')).toHaveTextContent(
      '暂无对应解读',
    );
    expect(screen.getByTestId('analysis-deviations-uncovered')).toHaveTextContent(
      /数据在日期范围内.*没有经过审核的解读条目/,
    );
    expect(screen.getByTestId('analysis-explanation')).toHaveTextContent(/这是解读，不是诊断/);
  });

  it('shows 4/4 coverage when the low-protein entry explains TP and albumin', async () => {
    const deviations = [
      point({ evidence_id: 'ev-1', name: '丙氨酸氨基转移酶测定', value: '83.5', flag: 'high' }),
      point({ evidence_id: 'ev-2', name: '天门冬氨酸氨基转移酶测定', value: '45', flag: 'high' }),
      point({ evidence_id: 'ev-3', name: '总蛋白测定', value: '60.70', flag: 'low' }),
      point({ evidence_id: 'ev-4', name: '白蛋白测定', value: '35.50', flag: 'low' }),
    ];
    const lowSources = [
      { standard: 'Duvall 2023', note: '总蛋白与白蛋白解读', url: 'https://example.com/duvall' },
      { standard: 'ACG 2017', note: '异常肝功能评估', url: 'https://example.com/acg2017' },
      { standard: 'ASPEN 2021', note: '白蛋白不是营养标志', url: 'https://example.com/aspen' },
      { standard: 'KDIGO 2021', note: '蛋白尿与肾脏丢失', url: 'https://example.com/kdigo' },
      { standard: 'WS/T 578.1—2017', note: '能量与蛋白质参考摄入量', url: null },
      { standard: '中国居民膳食指南（2022）', note: '平衡膳食建议', url: null },
    ];
    const result = makeAnalysisResult({
      sources: [
        ...lowSources,
        { standard: 'ACG 2016', note: '转氨酶升高分级', url: 'https://example.com/acg2016' },
        { standard: 'AASLD', note: 'R 值分型', url: 'https://example.com/aasld' },
      ],
      layers: {
        framework: makeAnalysisFramework(),
        facts: makeAnalysisFacts({ deviations }),
        explanation: {
          sections: [
            {
              ...explanation().sections[0],
              evidence_ids: ['ev-1', 'ev-2'],
              source_ids: [
                { standard: 'AASLD', note: 'R 值分型', url: 'https://example.com/aasld' },
              ],
            },
            {
              ...explanation().sections[0],
              entry_id: 'liver-low-protein-albumin',
              evidence_ids: ['ev-3', 'ev-4'],
              explanation:
                '总蛋白由白蛋白和球蛋白组成，总蛋白低不一定代表白蛋白低。白蛋白偏低时，可能与肝脏制造蛋白质减少、蛋白质从肠道或尿液丢失、长期吃得不够或吸收不好、近期生病或炎症有关；单次结果不能确定原因。',
              check_directions: ['把结果和症状、近期体重变化带给医生看'],
              general_advice: ['按本人年龄和身体状况保持日常均衡饮食'],
              track_items: ['TP', 'ALB'],
              source_ids: lowSources,
            },
          ],
          truncated: false,
        },
      },
    });
    mockUseLocalSearchParams.mockReturnValue({ analysis_id: '12' });
    mockApi({ GET: { '/analyses/{analysis_id}': () => result } });

    await renderScreen(<AnalysisScreen />);

    const coverage = await screen.findByTestId('analysis-explanation-coverage');
    expect(coverage).toHaveTextContent(/4 项异常里，本次解读解释了 4 项/);
    expect(coverage).not.toHaveTextContent(/另外/);
    expect(screen.getByTestId('analysis-deviation-coverage-ev-3')).toHaveTextContent('已解读');
    expect(screen.getByTestId('analysis-deviation-coverage-ev-4')).toHaveTextContent('已解读');
    expect(screen.queryByText('暂无对应解读')).toBeNull();
    expect(screen.queryByTestId('analysis-deviations-uncovered')).toBeNull();

    const summary = screen.getByTestId('analysis-explanation-summary');
    expect(summary).toHaveTextContent(/转氨酶升高而碱性磷酸酶和胆红素正常/);
    expect(summary).toHaveTextContent(/总蛋白由白蛋白和球蛋白组成/);
    expect(summary).toHaveTextContent(/进一步检查/);
    expect(summary).toHaveTextContent(/生活建议/);
    expect(summary).toHaveTextContent(/长期跟踪/);
    expect(summary).toHaveTextContent(/总蛋白（TP）/);
    expect(summary).toHaveTextContent(/白蛋白（ALB）/);
    expect(screen.getByTestId('analysis-explanation')).toHaveTextContent(
      /参考：总蛋白与白蛋白解读（Duvall 2023）/,
    );
  });

  it('renders the four-part explanation with a bullet, no basis toggle and no raw URL', async () => {
    const result = makeAnalysisResult({
      sources: [
        {
          standard: 'ACG 2016',
          note: '转氨酶升高分级',
          url: 'https://example.com/acg',
        },
        {
          standard: 'AASLD',
          note: 'R 值分型',
          url: 'https://example.com/aasld',
        },
      ],
      layers: {
        facts: makeAnalysisFacts(),
        framework: makeAnalysisFramework(),
        explanation: {
          ...explanation(),
          sections: [
            {
              ...explanation().sections[0],
              source_ids: [
                {
                  standard: 'AASLD',
                  note: 'R 值分型',
                  url: 'https://example.com/aasld',
                },
              ],
            },
          ],
          truncated: true,
        },
      },
    });
    mockUseLocalSearchParams.mockReturnValue({ analysis_id: '12' });
    mockApi({ GET: { '/analyses/{analysis_id}': () => result } });

    await renderScreen(<AnalysisScreen />);

    expect(await screen.findByTestId('analysis-explanation')).toHaveTextContent(/本次解读/);
    expect(screen.queryByTestId('analysis-explanation-basis-liver-hepatocellular-mild')).toBeNull();
    const section = screen.getByTestId('analysis-explanation-summary');
    expect(section).toHaveTextContent(/可能原因/);
    expect(section).toHaveTextContent(/· 转氨酶升高而碱性磷酸酶和胆红素正常/);
    expect(section).toHaveTextContent(/进一步检查/);
    expect(section).toHaveTextContent(/复查肝功能/);
    expect(section).toHaveTextContent(/生活建议/);
    expect(section).toHaveTextContent(/规律作息/);
    expect(section).toHaveTextContent(/长期跟踪/);
    expect(section).toHaveTextContent(/丙氨酸氨基转移酶（ALT）/);
    expect(section).toHaveTextContent(/天门冬氨酸氨基转移酶（AST）/);
    expect(section).toHaveTextContent(/反馈这段内容/);
    expect(screen.getByTestId('analysis-explanation-truncated')).toHaveTextContent(
      /只影响解读部分/,
    );
    expect(screen.getByTestId('analysis-explanation')).toHaveTextContent(
      /参考：转氨酶升高分级（ACG 2016）等 2 份/,
    );
    expect(screen.queryByText('https://example.com/acg')).toBeNull();

    fireEvent.press(screen.getByTestId('analysis-sources-open'));
    const sheet = await screen.findByTestId('analysis-source-sheet');
    expect(sheet).toHaveTextContent(/这些参考来源是什么？/);
    expect(screen.getByTestId('analysis-source-list')).toBeTruthy();
    expect(sheet).toHaveTextContent(/ACG 2016 · 美国胃肠病学会临床指南/);
    expect(sheet).toHaveTextContent(/AASLD · 美国肝病学会转氨酶升高指南/);
    expect(sheet).not.toHaveTextContent(/https:\/\//);

    await act(async () => {
      fireEvent.press(screen.getByTestId('analysis-source-open-0'));
    });
    await waitFor(() =>
      expect(mockedOpenURL).toHaveBeenCalledWith('https://example.com/acg'),
    );
  });

  it('shows the quota message in the conclusion position without disabling the result', async () => {
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
      /今日额度已用完，事实与分级仍可用/,
    );
    expect(screen.getByTestId('analysis-explanation-unavailable')).toHaveTextContent(/明天再试/);
    expect(screen.getByTestId('analysis-quota-remaining')).toHaveTextContent(/今日剩余解读次数 0 次/);
    expect(screen.getByTestId('analysis-feedback-open')).toBeTruthy();
  });

  it('shows the cloud-failure state with a next step without offering a new snapshot', async () => {
    const result = makeAnalysisResult({
      layers: { facts: makeAnalysisFacts() },
      quota: {
        used_today: 2,
        limit_today: 20,
        explanation_available: false,
        explanation_reason: 'llm_timeout',
      },
    });
    mockUseLocalSearchParams.mockReturnValue({ analysis_id: '12' });
    mockApi({ GET: { '/analyses/{analysis_id}': () => result } });

    await renderScreen(<AnalysisScreen />);

    expect(await screen.findByTestId('analysis-explanation-unavailable')).toHaveTextContent(
      /本次解读暂不可用/,
    );
    expect(screen.getByTestId('analysis-explanation-unavailable')).toHaveTextContent(
      /可稍后重新解读/,
    );
    expect(screen.queryByTestId('analysis-rerun')).toBeNull();
  });

  it('shows the quota-unknown state with a retry-later hint', async () => {
    const result = makeAnalysisResult({
      layers: { facts: makeAnalysisFacts() },
      quota: {
        used_today: 0,
        limit_today: 20,
        explanation_available: true,
        explanation_reason: 'quota_unavailable',
      },
    });
    mockUseLocalSearchParams.mockReturnValue({ analysis_id: '12' });
    mockApi({ GET: { '/analyses/{analysis_id}': () => result } });

    await renderScreen(<AnalysisScreen />);

    expect(await screen.findByTestId('analysis-explanation-unavailable')).toHaveTextContent(
      /额度状态暂不可用/,
    );
    expect(screen.getByTestId('analysis-explanation-unavailable')).toHaveTextContent(/稍后重试/);
  });

  it('shows the no-entries guidance in the conclusion position', async () => {
    const result = makeAnalysisResult({
      layers: { facts: makeAnalysisFacts() },
      quota: {
        used_today: 2,
        limit_today: 20,
        explanation_available: false,
        explanation_reason: 'no_entries',
      },
    });
    mockUseLocalSearchParams.mockReturnValue({ analysis_id: '12' });
    mockApi({ GET: { '/analyses/{analysis_id}': () => result } });

    await renderScreen(<AnalysisScreen />);

    expect(await screen.findByTestId('analysis-explanation-unavailable')).toHaveTextContent(
      /本次没有可引用的解读条目/,
    );
    expect(screen.getByTestId('analysis-explanation-unavailable')).toHaveTextContent(/问医生/);
  });

  it('records result-level feedback with a fixed category and neutral wording', async () => {
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
    expect(await screen.findByTestId('analysis-feedback-open')).toHaveTextContent('反馈问题');
    fireEvent.press(screen.getByTestId('analysis-feedback-open'));
    expect(await screen.findByText('解读结果哪里需要改进？')).toBeTruthy();
    await act(async () => {
      fireEvent.press(screen.getByTestId('analysis-feedback-reason-explanation_wrong'));
    });
    await act(async () => {
      fireEvent.press(screen.getByTestId('analysis-feedback-submit'));
    });

    await waitFor(() => expect(postHandler).toHaveBeenCalledTimes(1));
    expect(postHandler.mock.calls[0][0]).toMatchObject({
      params: { path: { analysis_id: 12 } },
      body: { reason: 'explanation_wrong' },
    });
    expect(await screen.findByTestId('analysis-feedback-done')).toHaveTextContent(/已记录/);
  });

  it('records merged-card feedback without an entry id', async () => {
    const postHandler = jest.fn((_options?: FakeApiOptions) => ({
      feedback_id: 8,
      created_at: '2026-11-03T12:00:00Z',
    }));
    const result = makeAnalysisResult({
      layers: {
        facts: makeAnalysisFacts(),
        explanation: explanation(),
      },
    });
    mockUseLocalSearchParams.mockReturnValue({ analysis_id: '12' });
    mockApi({
      GET: { '/analyses/{analysis_id}': () => result },
      POST: { '/analyses/{analysis_id}/feedback': postHandler },
    });

    await renderScreen(<AnalysisScreen />);
    expect(await screen.findByTestId('analysis-explanation-feedback')).toHaveTextContent(
      '反馈这段内容',
    );
    fireEvent.press(screen.getByTestId('analysis-explanation-feedback'));
    expect(await screen.findByText('解读结果哪里需要改进？')).toBeTruthy();
    await act(async () => {
      fireEvent.press(screen.getByTestId('analysis-feedback-reason-explanation_wrong'));
    });
    await act(async () => {
      fireEvent.press(screen.getByTestId('analysis-feedback-submit'));
    });

    await waitFor(() => expect(postHandler).toHaveBeenCalledTimes(1));
    expect(postHandler.mock.calls[0][0]).toMatchObject({
      body: { reason: 'explanation_wrong' },
    });
  });

  it('shows the indicator-to-panel expansion notice, stale rerun action and creates a new snapshot', async () => {
    const postHandler = jest.fn((_options?: FakeApiOptions) =>
      makeAnalysisResult({ analysis_id: 99 }),
    );
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
    mockApi({
      GET: { '/analyses/{analysis_id}': () => result },
      POST: { '/analyses': postHandler },
    });

    await renderScreen(<AnalysisScreen />);

    expect(await screen.findByTestId('analysis-entry-notice')).toHaveTextContent(
      '已从指标「丙氨酸转氨酶」扩展至项目「肝功」',
    );
    expect(screen.getByTestId('analysis-stale')).toHaveTextContent(/数据已更新/);
    await act(async () => {
      fireEvent.press(screen.getByTestId('analysis-rerun'));
    });

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
    expect(mockRouter.push).toHaveBeenCalledWith({
      pathname: '/analysis/[analysis_id]',
      params: { analysis_id: '99' },
    });
  });

  it('switches the window from the result page and keeps the old snapshot', async () => {
    const postHandler = jest.fn((_options?: FakeApiOptions) =>
      makeAnalysisResult({ analysis_id: 77 }),
    );
    mockUseLocalSearchParams.mockReturnValue({ analysis_id: '12' });
    mockApi({
      GET: { '/analyses/{analysis_id}': () => makeAnalysisResult() },
      POST: { '/analyses': postHandler },
    });

    await renderScreen(<AnalysisScreen />);
    fireEvent.press(await screen.findByTestId('analysis-range-open'));
    await chooseRange(24);

    await waitFor(() => expect(postHandler).toHaveBeenCalledTimes(1));
    expect(postHandler.mock.calls[0][0]).toMatchObject({
      body: { member_id: 1, panel: 'liver', entry: 'panel', window_months: 24 },
    });
    expect(mockRouter.push).toHaveBeenCalledWith({
      pathname: '/analysis/[analysis_id]',
      params: { analysis_id: '77' },
    });
  });

  it('links a clinical-note result back to the note and hides snapshot actions', async () => {
    const result = makeAnalysisResult({
      stale: true,
      layers: {
        facts: makeAnalysisFacts({ entry: 'clinical_note', panel: 'clinical_note' }),
      },
    });
    mockUseLocalSearchParams.mockReturnValue({ analysis_id: '12' });
    mockApi({ GET: { '/analyses/{analysis_id}': () => result } });

    await renderScreen(<AnalysisScreen />);

    expect(await screen.findByTestId('analysis-stale')).toHaveTextContent(
      /回到原病历详情页重新发起/,
    );
    expect(screen.queryByTestId('analysis-rerun')).toBeNull();
    expect(screen.queryByTestId('analysis-range-open')).toBeNull();
    expect(screen.getByTestId('analysis-panel-link')).toHaveTextContent(/查看病历原文/);
  });

  it('explains the data window and links to the existing panel page', async () => {
    const result = makeAnalysisResult({
      layers: {
        facts: makeAnalysisFacts({
          reports: [
            { report_id: REPORT_A, report_date: '2026-11-04', report_type: '生化' },
            { report_id: REPORT_B, report_date: '2026-09-04', report_type: '生化' },
            { report_id: 'c'.repeat(64), report_date: '2026-05-16', report_type: '生化' },
          ],
          excluded: [
            {
              report_id: 'd'.repeat(64),
              report_date: '2025-09-28',
              trend_key: 'TBIL',
              name: '总胆红素测定',
              reason: 'outside_window',
            },
            {
              report_id: 'e'.repeat(64),
              report_date: '2025-09-24',
              trend_key: 'GGT',
              name: 'γ-谷氨酰基转移酶测定',
              reason: 'outside_window',
            },
          ],
        }),
      },
    });
    mockUseLocalSearchParams.mockReturnValue({ analysis_id: '15' });
    mockApi({ GET: { '/analyses/{analysis_id}': () => result } });

    await renderScreen(<AnalysisScreen />);

    const dataNote = await screen.findByTestId('analysis-data-note');
    expect(dataNote).toHaveTextContent(/本次判断使用 2025-11-03 之后的 3 份报告、1 份病历/);
    expect(dataNote).toHaveTextContent(
      /2025-09-28、2025-09-24 的旧点只用于历史趋势，不参与本次异常判断/,
    );
    expect(dataNote).toHaveTextContent(/历史趋势在「肝功」项目页查看/);
    expect(dataNote).toHaveTextContent(/体格发育（BMI 等）在成员页查看/);
    expect(screen.getByTestId('analysis-panel-link')).toHaveTextContent(
      /查看肝功全部指标与趋势/,
    );
    expect(screen.queryByTestId('analysis-history')).toBeNull();
    expect(screen.queryByTestId('analysis-scope')).toBeNull();
    expect(screen.queryByTestId('analysis-excluded')).toBeNull();
  });

  it('hides a unit of 1 and uses the display unit', async () => {
    const result = makeAnalysisResult({
      layers: {
        facts: makeAnalysisFacts({
          deviations: [
            point({
              evidence_id: 'ev-ratio',
              name: '白蛋白/球蛋白',
              value: '1.41',
              unit: '1',
              unit_display: null,
              flag: 'normal',
            }),
          ],
        }),
      },
    });
    mockUseLocalSearchParams.mockReturnValue({ analysis_id: '12' });
    mockApi({ GET: { '/analyses/{analysis_id}': () => result } });

    await renderScreen(<AnalysisScreen />);

    expect(await screen.findByTestId('analysis-deviation-ev-ratio')).toHaveTextContent(/1\.41/);
    expect(screen.getByTestId('analysis-deviation-ev-ratio')).not.toHaveTextContent(/1\.41 1/);
  });
});

describe('Panel analysis entry', () => {
  function setupPanelApi(postHandler: (options?: FakeApiOptions) => unknown) {
    mockApi({
      GET: {
        '/members/{member_id}/panels/{panel}': () => makePanelDetail(),
        '/members': () => ({ members: [makeMember()] }),
        '/reports': () => ({ reports: [] }),
        '/analyses/quota': () => ({
          used_today: 18,
          limit_today: 20,
          explanation_available: true,
        }),
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
    expect(screen.getByTestId('panel-analyze')).toHaveTextContent('解读这个项目');
    expect(await screen.findByTestId('panel-quota-remaining')).toHaveTextContent(
      '今日剩余解读次数 2 次',
    );

    fireEvent.press(screen.getByTestId('panel-analyze'));
    await chooseRange(12);
    expect(await screen.findByTestId('analysis-consent')).toHaveTextContent(/解读会把本次涉及的检验数值/);
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
