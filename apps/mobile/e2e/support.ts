// App E2E 支撑:契约类型直接复用 App 的 OpenAPI 生成物;API、样张、storage 键、
// testID 约定单点定义(用例只消费,不散落裸字符串)。
import { readFile } from 'node:fs/promises';
import { basename, resolve } from 'node:path';

import type { Browser } from '@e2e-dev/web';
import type { App } from 'e2e';

import type { components } from '../src/api/schema';
import { isImagingReport } from '../src/reports/category';
import { parseNumericForm } from '../src/reports/refRange';
import type { FlagBadgeKind } from '../src/reports/status';
import { flagReasonText, isBackfillItem } from '../src/review/rules';

export type Member = components['schemas']['Member'];
export type ReportDetail = components['schemas']['ReportDetail'];
export type UploadResult = components['schemas']['UploadResult'];

/** 专用栈 API 基址;run.sh 以 E2E_API_BASE_URL 导出(与 HDD_SERVE_PORT 同源)。 */
export const API_BASE_URL = process.env.E2E_API_BASE_URL ?? 'http://127.0.0.1:58080/api/v1';

const REPO_ROOT = resolve(import.meta.dirname, '../../..');
const SAMPLES = resolve(REPO_ROOT, 'samples');

export const samplePath = (name: string) => resolve(SAMPLES, name);

// 与 App 侧 settings/baseUrlStore.ts、members/currentMemberStore.ts 的键逐字一致
// (Web 下 AsyncStorage = localStorage;预置后各屏免走设置页)。
const BASE_URL_STORAGE_KEY = 'healthyduoduo.baseUrl';
const CURRENT_MEMBER_STORAGE_KEY = 'healthyduoduo.currentMemberId';

/** 预置后端地址/当前成员后打开页面;每次调用追加一段页面初始化脚本。 */
export async function openConfigured(
  app: App,
  browser: Browser,
  path: string,
  options: { memberId?: number; baseUrl?: string } = {},
): Promise<void> {
  const lines = [
    `localStorage.setItem(${JSON.stringify(BASE_URL_STORAGE_KEY)}, ${JSON.stringify(options.baseUrl ?? API_BASE_URL)});`,
  ];
  if (options.memberId !== undefined) {
    lines.push(`localStorage.setItem(${JSON.stringify(CURRENT_MEMBER_STORAGE_KEY)}, '${options.memberId}');`);
  }
  await browser.addInitScript(lines.join('\n'));
  await app.open(path);
}

/** Expo Web 相册通道经 `input.dispatchEvent(new MouseEvent('click'))` 打开原生文件选择器;
 *  headless 下 chooser 被自动取消,expo 随即移除 input,setInputFiles 就找不到节点。
 *  屏蔽这次 click、保留 input,再由测试用 setInputFiles 注入文件(change 事件照常触发)。 */
export async function stubFileChooser(browser: Browser): Promise<void> {
  await browser.addInitScript(`
    const originalDispatchEvent = HTMLInputElement.prototype.dispatchEvent;
    HTMLInputElement.prototype.dispatchEvent = function (event) {
      if (this.type === 'file' && event && event.type === 'click') {
        return true;
      }
      return originalDispatchEvent.call(this, event);
    };
  `);
}

type BadgeKind = FlagBadgeKind | 'unreviewed';

/** 确定性 testID 生成;与 App 侧各组件模板一一对应。 */export const testID = {
  memberChip: (memberId: number) => `member-chip-${memberId}`,
  memberRow: (memberId: number) => `member-row-${memberId}`,
  memberSelect: (memberId: number) => `member-select-${memberId}`,
  reportCard: (reportId: string) => `report-card-${reportId}`,
  reportBadge: (reportId: string, badge: BadgeKind) => `report-badge-${reportId}-${badge}`,
  reportItemsTitle: 'report-items-title',
  reportPendingEntry: 'report-pending-entry',
  archiveFilterBanner: 'archive-filter-banner',
  archiveFilterClear: 'archive-filter-clear',
  archiveEmpty: 'archive-empty',
  archiveShowAll: 'archive-show-all',
  overviewAllReports: 'overview-all-reports',
  overviewPendingPartial: 'overview-pending-partial',
  overviewPendingFailed: 'overview-pending-failed',
  overviewPendingEmpty: 'overview-pending-empty',
  overviewTrends: 'overview-trends',
  overviewLatest: (reportId: string) => `overview-latest-${reportId}`,
  trendCategory: (index: number) => `trend-category-${index}`,
  trendIndicator: (index: number) => `trend-indicator-${index}`,
  trendSparkline: (index: number) => `trend-sparkline-${index}`,
  trendEmptyAbnormal: 'trend-empty-abnormal',
  trendEmptyReports: 'trend-empty-reports',
  trendDetail: 'trend-detail',
  trendDetailChart: 'trend-detail-chart',
  trendDetailClose: 'trend-detail-close',
  trendPoint: (index: number) => `trend-point-${index}`,
  reportExamName: 'report-exam-name',
  reportSection: (index: number) => `report-section-${index}`,
  reportSectionZoom: (index: number) => `report-section-zoom-${index}`,
  reportImageToggle: 'report-image-toggle',
  reportImage: 'report-image',
  reviewItem: (itemId: number) => `review-item-${itemId}`,
  reviewReason: (itemId: number) => `review-reason-${itemId}`,
  reviewConfirm: (itemId: number) => `review-confirm-${itemId}`,
  reviewBackfill: (itemId: number) => `review-backfill-${itemId}`,
  reviewCorrect: (itemId: number) => `review-correct-${itemId}`,
  reviewZoomToggle: (itemId: number) => `review-zoom-toggle-${itemId}`,
  reviewZoom: (itemId: number) => `review-zoom-${itemId}`,
  reviewValue: (itemId: number) => `review-field-value-${itemId}`,
  reviewSave: (itemId: number) => `review-save-${itemId}`,
  uploadOpenTimeline: 'upload-open-timeline',
  reviewOpenTimeline: 'review-open-timeline',
} as const;

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${path}`, init);
  if (!response.ok) {
    const detail = await response.text().catch(() => '');
    throw new Error(`API ${init?.method ?? 'GET'} ${path}: HTTP ${response.status} ${detail}`);
  }
  return (await response.json()) as T;
}

export async function listMembers(): Promise<Member[]> {
  const payload = await api<{ members: Member[] }>('/members');
  return payload.members;
}

export async function createMember(name: string, isChild: boolean): Promise<Member> {
  return api<Member>('/members', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ name, is_child: isChild }),
  });
}

const MIME_BY_SUFFIX: Record<string, string> = {
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.webp': 'image/webp',
};

/** 经真实上传端点入档样张(同步识别);duplicate=true 说明栈未重置,直接失败。 */
export async function uploadSample(name: string, memberId: number): Promise<ReportDetail> {
  const path = samplePath(name);
  const bytes = await readFile(path);
  const suffix = path.slice(path.lastIndexOf('.')).toLowerCase();
  const form = new FormData();
  form.append('member_id', String(memberId));
  form.append(
    'image',
    new Blob([new Uint8Array(bytes)], { type: MIME_BY_SUFFIX[suffix] ?? 'application/octet-stream' }),
    basename(path),
  );
  const result = await api<UploadResult>('/reports', {
    method: 'POST',
    body: form,
    signal: AbortSignal.timeout(120_000),
  });
  if (result.duplicate) {
    throw new Error(`样张 ${name} 已在库:专用栈未重置,请用 e2e/run.sh 重跑`);
  }
  return result.report;
}

export type Seed = {
  xiaoming: Member;
  mama: Member;
  uploadMember: Member;
  reviewReport: ReportDetail;
  healReport: ReportDetail;
  imaging01: ReportDetail;
  imaging03: ReportDetail;
};

function fixtureError(sample: string, detail: string): never {
  throw new Error(`E2E fixture ${sample}: ${detail}(专用栈需经 e2e/run.sh 启用 golden 回放)`);
}

function sectionKinds(report: ReportDetail): string[] {
  return (report.sections ?? []).map((section) => section.kind);
}

function assertReviewFixture(report: ReportDetail): void {
  if (report.status !== 'success') {
    fixtureError('cbc_03', `expected status success, got ${report.status}`);
  }
  if (isImagingReport(report)) {
    fixtureError('cbc_03', `expected a lab report, got ${report.report_type}`);
  }
  if (report.flag_counts.unknown !== 0) {
    fixtureError('cbc_03', `expected no unknown items, got ${report.flag_counts.unknown}`);
  }
  const pending = report.items.filter(
    (item) => item.low_confidence && item.review_status === 'unreviewed',
  );
  if (pending.length === 0) {
    fixtureError('cbc_03', 'expected an unreviewed low-confidence item');
  }
  if (isBackfillItem(pending[0])) {
    fixtureError('cbc_03', 'expected a confirmable (non-backfill) low-confidence item');
  }
  if (
    !report.items.some(
      (item) =>
        item.item_id !== pending[0].item_id &&
        item.value !== null &&
        parseNumericForm(item.value).kind === 'scalar',
    )
  ) {
    fixtureError('cbc_03', 'expected another numeric item for correction');
  }
}

function assertPartialUnknownFixture(sample: string, report: ReportDetail): void {
  if (report.status !== 'partial') {
    fixtureError(sample, `expected status partial, got ${report.status}`);
  }
  if (isImagingReport(report)) {
    fixtureError(sample, `expected a lab report, got ${report.report_type}`);
  }
  if (report.flag_counts.unknown === 0) {
    fixtureError(sample, 'expected unknown items');
  }
  if (!report.items.some((item) => item.flag === 'unknown')) {
    fixtureError(sample, 'expected an unknown item');
  }
  if (report.unreviewed_low_confidence_count === 0) {
    fixtureError(sample, 'expected unreviewed low-confidence items');
  }
}

function assertHealFixture(report: ReportDetail): void {
  assertPartialUnknownFixture('cbc_04', report);
  const backfill = report.items.find((item) => item.flag_reason !== null && isBackfillItem(item));
  if (!backfill) {
    fixtureError('cbc_04', 'expected a backfill item (isBackfillItem)');
  }
  if (!flagReasonText(backfill.flag_reason)) {
    fixtureError('cbc_04', 'expected backfill item reason text');
  }
  if (
    !report.items.some(
      (item) => item.flag === 'unknown' && !item.low_confidence && item.ocr_lines.length > 0,
    )
  ) {
    fixtureError('cbc_04', 'expected a high-confidence unknown with OCR lines');
  }
  if (!report.items.some((item) => item.low_confidence && item.ocr_lines.length > 0)) {
    fixtureError('cbc_04', 'expected a low-confidence item with OCR lines');
  }
}

function assertImagingFixture(
  sample: string,
  report: ReportDetail,
  expected: {
    status: 'success' | 'partial' | 'failed';
    date: string | null;
    required: string[];
    forbidden?: string[];
  },
): void {
  if (!isImagingReport(report)) {
    fixtureError(sample, `expected an imaging report, got ${report.report_type}`);
  }
  if (report.status !== expected.status) {
    fixtureError(sample, `expected status ${expected.status}, got ${report.status}`);
  }
  if (report.items.length !== 0) {
    fixtureError(sample, `expected no items, got ${report.items.length}`);
  }
  if (report.report_date !== expected.date) {
    fixtureError(sample, `expected report_date ${expected.date}, got ${report.report_date}`);
  }
  const kinds = sectionKinds(report);
  for (const kind of expected.required) {
    if (!kinds.includes(kind)) {
      fixtureError(sample, `expected section ${kind}, got [${kinds.join(', ')}]`);
    }
  }
  for (const kind of expected.forbidden ?? []) {
    if (kinds.includes(kind)) {
      fixtureError(sample, `unexpected section ${kind}`);
    }
  }
}

/** 种子:三成员 + 固定 golden 报告(核对 cbc_03、自愈 cbc_04、影像单 us_01/us_03)
 *  + 仅用于上传用例的独立成员(新增报告不影响概览/档案计数)。
 *  用例前提在此显式断言:失败在 beforeAll 点名 fixture 与缺口,而不是用例内 undefined。 */
export async function seedData(): Promise<Seed> {
  const xiaoming = await createMember('E2E 小明', true);
  const mama = await createMember('E2E 妈妈', false);
  const uploadMember = await createMember('E2E 上传', false);

  const reviewReport = await uploadSample('cbc_03.png', xiaoming.member_id);
  assertReviewFixture(reviewReport);
  const healReport = await uploadSample('cbc_04.png', mama.member_id);
  assertHealFixture(healReport);
  const imaging01 = await uploadSample('us_01.webp', xiaoming.member_id);
  assertImagingFixture('us_01', imaging01, {
    status: 'success',
    date: null,
    required: ['exam_name', 'findings', 'impression'],
  });
  const imaging03 = await uploadSample('us_03.jpeg', xiaoming.member_id);
  assertImagingFixture('us_03', imaging03, {
    status: 'success',
    date: '2024-09-15',
    required: ['exam_name', 'impression'],
    forbidden: ['findings'],
  });
  return { xiaoming, mama, uploadMember, reviewReport, healReport, imaging01, imaging03 };
}

export type LifecycleSeed = {
  member: Member;
  target: Member;
  report: ReportDetail;
};

/** 破坏性用例的独立种子:专用成员 + cbc_02(partial、有 unknown/待核对),
 *  与主链路 fixture 不共享数据,避免删除/重传影响其他用例。 */
export async function seedLifecycleData(): Promise<LifecycleSeed> {
  const member = await createMember('E2E 生命周期', false);
  const target = await createMember('E2E 生命周期目标', false);
  const report = await uploadSample('cbc_02.png', member.member_id);
  assertPartialUnknownFixture('cbc_02', report);
  return { member, target, report };
}
