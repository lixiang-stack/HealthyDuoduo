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
    created_at: '2026-01-01T00:00:00Z',
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
