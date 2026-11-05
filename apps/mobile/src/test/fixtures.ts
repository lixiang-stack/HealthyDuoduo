import type {
  AnalysisEvidencePoint,
  AnalysisFacts,
  AnalysisFramework,
  AnalysisHistory,
  AnalysisResult,
} from '../api/analyses';
import type { GrowthRecord } from '../api/growth';
import type { Member } from '../api/members';
import type {
  FlagCounts,
  PanelDetail,
  PanelItem,
  PanelSummary,
  ReportDetail,
  ReportItem,
  ReportSummary,
  ReviewResult,
  TrendPoint,
} from '../api/reports';

// Values below follow the real lft_05 golden (samples/expected/report/lft_05.json).
// IDs and review state are test-local so each case can describe one interaction.

export function makeFlagCounts(overrides: Partial<FlagCounts> = {}): FlagCounts {
  return { high: 0, low: 0, abnormal: 0, unknown: 0, ...overrides };
}

export function makeMember(overrides: Partial<Member> = {}): Member {
  return {
    member_id: 1,
    name: '小明',
    is_child: true,
    sex: 'female',
    birth_date: '2020-01-01',
    created_at: '2026-01-01T00:00:00Z',
    latest_growth: null,
    ...overrides,
  };
}

export function makeGrowthRecord(overrides: Partial<GrowthRecord> = {}): GrowthRecord {
  return {
    growth_id: 1,
    member_id: 1,
    measured_on: '2026-10-01',
    height_cm: 118.5,
    weight_kg: 22.4,
    source: 'measured',
    bmi: 16,
    weight_status: null,
    ...overrides,
  };
}

export function makeReportItem(overrides: Partial<ReportItem> = {}): ReportItem {
  return {
    item_id: 1,
    name: '丙氨酸转氨酶',
    panel: 'liver',
    value: '27.40',
    unit: 'U/L',
    ref_range: '7-40',
    method: null,
    code: null,
    loinc: null,
    unit_display: null,
    unit_status: null,
    raw_value: null,
    raw_unit: null,
    raw_ref_range: null,
    ref_kind: null,
    result_kind: null,
    result_basis: null,
    flag: 'normal',
    low_confidence: false,
    raw_text: '丙氨酸氨基转移酶  27.40  U/L  7-40',
    review_status: 'unreviewed',
    ocr_lines: [],
    prev: null,
    ...overrides,
  };
}

export function makeReportSummary(overrides: Partial<ReportSummary> = {}): ReportSummary {
  return {
    report_id: 'r1',
    member_id: 1,
    report_type: '生化',
    report_date: '2026-10-01',
    status: 'success',
    fail_reason: null,
    flag_counts: makeFlagCounts(),
    panels: ['liver'],
    not_tested_count: 0,
    unreviewed_low_confidence_count: 0,
    ...overrides,
  };
}

export function makeReportDetail(overrides: Partial<ReportDetail> = {}): ReportDetail {
  return {
    ...makeReportSummary(),
    sections: null,
    items: [makeReportItem()],
    ...overrides,
  };
}

export function makeTrendPoint(overrides: Partial<TrendPoint> = {}): TrendPoint {
  return {
    report_id: 'r1',
    report_date: '2026-10-01',
    value: '27.40',
    unit: 'U/L',
    unit_display: 'U/L',
    unit_status: 'verified',
    ref_range: '7-40',
    method: null,
    flag: 'normal',
    low_confidence: false,
    reviewed: true,
    ...overrides,
  };
}

export function makePanelItem(overrides: Partial<PanelItem> = {}): PanelItem {
  const latest = makeTrendPoint();
  return {
    key: '1500104A',
    name: '丙氨酸转氨酶',
    unit: 'U/L',
    ref_range: '7-40',
    latest,
    points: [latest],
    report_count: 1,
    not_tested_count: 0,
    latest_report_not_tested: false,
    ...overrides,
  };
}

export function makePanelSummary(overrides: Partial<PanelSummary> = {}): PanelSummary {
  return {
    panel: 'liver',
    report_count: 1,
    item_count: 1,
    abnormal_count: 0,
    latest_report_date: '2026-10-01',
    latest_report_id: 'r1',
    report_types: ['生化'],
    narrative: false,
    ...overrides,
  };
}

export function makePanelDetail(overrides: Partial<PanelDetail> = {}): PanelDetail {
  return {
    panel: 'liver',
    member_id: 1,
    report_count: 1,
    latest_report_date: '2026-10-01',
    latest_report_id: 'r1',
    report_types: ['生化'],
    narrative: false,
    items: [makePanelItem()],
    ...overrides,
  };
}

export function makeReviewResult(
  item: ReportItem,
  overrides: Partial<ReviewResult> = {},
): ReviewResult {
  return {
    report_id: 'r1',
    status: 'success',
    unreviewed_low_confidence_count: 0,
    item,
    ...overrides,
  };
}

export function makeAnalysisEvidencePoint(
  overrides: Partial<AnalysisEvidencePoint> = {},
): AnalysisEvidencePoint {
  return {
    evidence_id: 'ev-aaaaaaaaaaaa-1',
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
    ...overrides,
  };
}

export function makeAnalysisHistory(overrides: Partial<AnalysisHistory> = {}): AnalysisHistory {
  const latest = makeAnalysisEvidencePoint();
  return {
    trend_key: '1500104A',
    name: '丙氨酸转氨酶测定',
    unit: 'U/L',
    unit_display: 'U/L',
    points: [latest],
    latest,
    previous: null,
    change: 'unknown',
    highest: true,
    lowest: false,
    point_count: 1,
    sparse: false,
    comparable: true,
    ...overrides,
  };
}

export function makeAnalysisFacts(overrides: Partial<AnalysisFacts> = {}): AnalysisFacts {
  return {
    entry: 'panel',
    panel: 'liver',
    indicator_name: null,
    expanded_from_indicator: false,
    window: {
      months: 12,
      from: '2025-11-03',
      to: '2026-11-03',
      sparse: false,
    },
    reports: [
      { report_id: 'a'.repeat(64), report_date: '2026-10-01', report_type: '生化' },
    ],
    notes: [
      {
        report_id: 'b'.repeat(64),
        report_date: '2026-09-20',
        outside_window: false,
        sections: [
          { kind: 'diagnosis', text: '缺铁性贫血' },
          { kind: 'treatment', text: '补铁治疗，一个月后复查。' },
        ],
      },
    ],
    deviations: [makeAnalysisEvidencePoint()],
    history: [makeAnalysisHistory()],
    uncomparable: [],
    excluded: [],
    insufficient_evidence: false,
    ...overrides,
  };
}


export function makeAnalysisFramework(overrides: Partial<AnalysisFramework> = {}): AnalysisFramework {
  return {
    labels: [
      {
        code: 'liver_transaminase_grade',
        label: '转氨酶升高：轻度',
        explanation: '转氨酶升高 2–5×ULN，按 ACG 2016 为轻度。',
        value: 'ALT 2.1×ULN',
        evidence_ids: ['ev-aaaaaaaaaaaa-1'],
        sources: [
          {
            standard: 'ACG 2016',
            note: '转氨酶升高分级',
            url: 'https://doi.org/10.1038/ajg.2016.517',
          },
        ],
      },
    ],
    ...overrides,
  };
}

export function makeAnalysisResult(overrides: Partial<AnalysisResult> = {}): AnalysisResult {
  return {
    analysis_id: 12,
    created_at: '2026-11-05T06:20:00Z',
    status: 'success',
    layers: { facts: makeAnalysisFacts(), framework: makeAnalysisFramework() },
    snapshot: {
      member_id: 1,
      member_name: '小明',
      panel: 'liver',
      entry: 'panel',
      indicator_name: null,
      window_months: 12,
      window_from: '2025-11-03',
      window_to: '2026-11-03',
      rule_version: 'llm-analysis-v3',
      data_version: 'v1',
      points: [
        {
          evidence_id: 'ev-aaaaaaaaaaaa-1',
          report_id: 'a'.repeat(64),
          report_date: '2026-10-01',
          trend_key: '1500104A',
          unit: 'U/L',
          value: '83.5',
          flag: 'high',
        },
      ],
      notes: [
        {
          report_id: 'b'.repeat(64),
          report_date: '2026-09-20',
          outside_window: false,
          section_kinds: ['diagnosis', 'treatment'],
        },
      ],
    },
    sources: [],
    quota: { used_today: 0, limit_today: 20, explanation_available: true },
    stale: false,
    ...overrides,
  };
}
