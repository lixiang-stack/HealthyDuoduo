// App E2E 主用例集:一条串行核心链路(设置 → 成员 → 时间线 → 上传 → 核对),
// 报告内容来自专用栈的 golden 回放(见 e2e/support.ts);破坏性生命周期与错误态
// 在独立文件,避免连坐跳过。运行入口:apps/mobile `npm run e2e`(专用栈见 e2e/run.sh)。
import { beforeAll, describe, test } from '@e2e-dev/web';
import { expect } from 'e2e';
import type { Screen } from 'e2e';

import { latestByCategory, pendingSummary } from '../../src/reports/overview';
import { parseNumericForm } from '../../src/reports/refRange';
import { EXAM_NAME_KIND, SECTION_LABELS, examNameText } from '../../src/reports/sections';
import { reportDateLabel, uploadResultText } from '../../src/reports/status';
import { flagReasonText, isBackfillItem, reviewStatusLabel } from '../../src/review/rules';
import {
  API_BASE_URL,
  listMembers,
  openConfigured,
  samplePath,
  seedData,
  stubFileChooser,
  testID,
  type Seed,
  type UploadResult,
} from '../support';

let seed!: Seed;

// 影像单详情主体(票 03/04):指标项 / 未知 / 待核对 / 逐项核对不出现;原图入口可展开。
async function expectImagingDetail(screen: Screen, reportId: string): Promise<void> {
  await expect(screen.getByTestId(testID.reportItemsTitle)).toHaveCount(0);
  await expect(screen.getByTestId(testID.reportBadge(reportId, 'unknown'))).toHaveCount(0);
  await expect(screen.getByTestId(testID.reportBadge(reportId, 'unreviewed'))).toHaveCount(0);
  await expect(screen.getByTestId(testID.reportPendingEntry)).toHaveCount(0);
  await expect(screen.getByTestId(testID.reportImageToggle)).toBeVisible();
  await expect(screen.getByTestId(testID.reportImage)).toHaveCount(0);
  await screen.getByTestId(testID.reportImageToggle).tap();
  await expect(screen.getByTestId(testID.reportImage)).toBeVisible();
}

describe('App E2E', { serial: true, timeout: 300_000 }, () => {
  beforeAll(async () => {
    seed = await seedData();
  });

  test('设置后端地址并通过健康检查', async ({ app, screen }) => {
    await app.open('/settings');
    await expect(screen.getByText('后端 Base URL')).toBeVisible();
    await screen.getByTestId('settings-base-url').fill(API_BASE_URL);
    await screen.getByTestId('settings-save').tap();
    await expect(screen.getByText(/已连通 · 版本/)).toBeVisible();

    await app.open('/');
    await expect(screen.getByText('查看谁的档案')).toBeVisible();
  });

  test('创建并切换家庭成员', async ({ app, browser, screen }) => {
    await openConfigured(app, browser, '/members');
    await screen.getByTestId('members-name-input').fill('E2E 宝宝');
    await screen.getByTestId('members-create').tap();

    const baby = (await listMembers()).find((member) => member.name === 'E2E 宝宝');
    expect(baby).toBeDefined();
    await expect(screen.getByTestId(testID.memberSelect(baby!.member_id))).toBeVisible();
    await screen.getByTestId(testID.memberSelect(baby!.member_id)).tap();
    await expect(screen.getByTestId(testID.memberRow(baby!.member_id))).toContainText('当前成员');
  });

  test('档案按成员过滤报告', async ({ app, browser, screen }) => {
    await openConfigured(app, browser, '/timeline', { memberId: seed.xiaoming.member_id });
    await expect(screen.getByTestId(testID.reportCard(seed.reviewReport.report_id))).toBeVisible();
    await expect(screen.getByTestId(testID.reportCard(seed.healReport.report_id))).toHaveCount(0);
    // 影像单不呈现指标项:卡片无未知 / 待核对徽标,未印日期文案按类别区分(票 03)。
    const imaging01Card = screen.getByTestId(testID.reportCard(seed.imaging01.report_id));
    await expect(imaging01Card).toContainText(reportDateLabel(seed.imaging01));
    await expect(
      screen.getByTestId(testID.reportBadge(seed.imaging01.report_id, 'unknown')),
    ).toHaveCount(0);
    await expect(
      screen.getByTestId(testID.reportBadge(seed.imaging01.report_id, 'unreviewed')),
    ).toHaveCount(0);
    await expect(
      screen.getByTestId(testID.reportBadge(seed.imaging03.report_id, 'unknown')),
    ).toHaveCount(0);
    await expect(
      screen.getByTestId(testID.reportBadge(seed.imaging03.report_id, 'unreviewed')),
    ).toHaveCount(0);
    // cbc_03 是 success 且无 unknown:核对项不产生时间线徽标。
    await expect(
      screen.getByTestId(testID.reportBadge(seed.reviewReport.report_id, 'unknown')),
    ).toHaveCount(0);
    await expect(
      screen.getByTestId(testID.reportBadge(seed.reviewReport.report_id, 'unreviewed')),
    ).toHaveCount(0);

    await screen.getByTestId(testID.memberChip(seed.mama.member_id)).tap();
    await expect(screen.getByTestId(testID.reportCard(seed.healReport.report_id))).toBeVisible();
    await expect(screen.getByTestId(testID.reportCard(seed.reviewReport.report_id))).toHaveCount(0);
    // 检验单卡片保持原样:未知徽标与非零待核对徽标仍显示(文案由 fixture 派生)。
    await expect(
      screen.getByTestId(testID.reportBadge(seed.healReport.report_id, 'unknown')),
    ).toBeVisible();
    await expect(
      screen.getByTestId(testID.reportBadge(seed.healReport.report_id, 'unreviewed')),
    ).toBeVisible();
  });

  test('成员概览与待处理入口', async ({ app, browser, screen }) => {
    const xiaomingReports = [seed.reviewReport, seed.imaging01, seed.imaging03];
    const xiaomingLatest = latestByCategory(xiaomingReports);

    await openConfigured(app, browser, '/', { memberId: seed.xiaoming.member_id });
    await expect(screen.getByText('查看谁的档案')).toBeVisible();
    // cbc_03 是 success;小明待核对 / 失败都为 0 → 空态,不显示入口。
    await expect(screen.getByTestId(testID.overviewPendingEmpty)).toBeVisible();
    await expect(screen.getByTestId(testID.overviewPendingPartial)).toHaveCount(0);
    await expect(screen.getByTestId(testID.overviewPendingFailed)).toHaveCount(0);
    for (const report of xiaomingReports) {
      const latest = xiaomingLatest.some((entry) => entry.report_id === report.report_id);
      await expect(screen.getByTestId(testID.overviewLatest(report.report_id))).toHaveCount(
        latest ? 1 : 0,
      );
    }
    await expect(screen.getByTestId(testID.overviewAllReports)).toContainText(
      `${xiaomingReports.length} 份`,
    );

    const mamaReports = [seed.healReport];
    const mamaPending = pendingSummary(mamaReports);
    await screen.getByTestId(testID.memberChip(seed.mama.member_id)).tap();
    await expect(screen.getByTestId(testID.overviewPendingPartial)).toContainText(
      `${mamaPending.partialReports} 份`,
    );
    await expect(screen.getByTestId(testID.overviewPendingPartial)).toContainText(
      `${mamaPending.unreviewedItems} 项`,
    );
    await expect(screen.getByTestId(testID.overviewPendingFailed)).toHaveCount(0);
    await expect(screen.getByTestId(testID.overviewPendingEmpty)).toHaveCount(0);
    for (const report of mamaReports) {
      await expect(screen.getByTestId(testID.overviewLatest(report.report_id))).toBeVisible();
    }
    await expect(screen.getByTestId(testID.overviewLatest(seed.imaging03.report_id))).toHaveCount(0);
    await expect(screen.getByTestId(testID.overviewAllReports)).toContainText(
      `${mamaReports.length} 份`,
    );

    await screen.getByTestId(testID.overviewPendingPartial).tap();
    await browser.waitForURL(/\/timeline\?filter=partial$/);
    await expect(screen.getByTestId(testID.archiveFilterBanner)).toContainText(
      `${mamaPending.partialReports} 份`,
    );
    await expect(screen.getByTestId(testID.reportCard(seed.healReport.report_id))).toBeVisible();

    await openConfigured(app, browser, '/', { memberId: seed.xiaoming.member_id });
    await screen.getByTestId(testID.overviewAllReports).tap();
    await browser.waitForURL(/\/timeline$/);
    await expect(screen.getByTestId(testID.reportCard(seed.reviewReport.report_id))).toBeVisible();
    await expect(screen.getByTestId(testID.reportCard(seed.imaging01.report_id))).toBeVisible();
    await expect(screen.getByTestId(testID.reportCard(seed.imaging03.report_id))).toBeVisible();
  });

  test('全部档案与筛选', async ({ app, browser, screen }) => {
    const mamaPending = pendingSummary([seed.healReport]);
    await openConfigured(app, browser, '/timeline?filter=partial', {
      memberId: seed.mama.member_id,
    });
    await expect(screen.getByTestId(testID.archiveFilterBanner)).toContainText(
      `${mamaPending.partialReports} 份`,
    );
    await expect(screen.getByTestId(testID.reportCard(seed.healReport.report_id))).toBeVisible();

    await screen.getByTestId(testID.archiveFilterClear).tap();
    await expect(screen.getByTestId(testID.archiveFilterBanner)).toHaveCount(0);
    await expect(screen.getByTestId(testID.reportCard(seed.healReport.report_id))).toBeVisible();

    const xiaomingReports = [seed.reviewReport, seed.imaging01, seed.imaging03];
    const xiaomingPending = pendingSummary(xiaomingReports);
    await openConfigured(app, browser, '/timeline?filter=failed', {
      memberId: seed.xiaoming.member_id,
    });
    await expect(screen.getByTestId(testID.archiveFilterBanner)).toContainText(
      `${xiaomingPending.failedReports} 份`,
    );
    await expect(screen.getByTestId(testID.archiveEmpty)).toBeVisible();
    await screen.getByTestId(testID.archiveShowAll).tap();
    await expect(screen.getByTestId(testID.reportCard(seed.reviewReport.report_id))).toBeVisible();
    await expect(screen.getByTestId(testID.reportCard(seed.imaging01.report_id))).toBeVisible();
    await expect(screen.getByTestId(testID.reportCard(seed.imaging03.report_id))).toBeVisible();
  });

  test('上传样张后时间线出现新报告', async ({ app, browser, screen }) => {
    await stubFileChooser(browser);
    await openConfigured(app, browser, '/upload', { memberId: seed.uploadMember.member_id });
    await screen.getByTestId('upload-pick-library').tap();
    await browser.locator('[data-testid="file-input"]').setInputFiles(samplePath('glu_02.jpeg'));
    await expect(screen.getByText('glu_02.jpeg')).toBeVisible();

    const responsePromise = browser.waitForResponse(/\/api\/v1\/reports$/);
    await screen.getByTestId('upload-submit').tap();
    const payload = await (await responsePromise).json<UploadResult>();
    expect(payload.duplicate).toBe(false);
    expect(payload.report.items.length).toBeGreaterThan(0);
    await expect(screen.getByTestId('upload-result')).toContainText(uploadResultText(payload));

    await screen.getByTestId(testID.uploadOpenTimeline).tap();
    await expect(screen.getByTestId(testID.reportCard(payload.report.report_id))).toBeVisible();
  });

  test('影像单 us_03:检查项目 / 超声提示 / 日期 / 原图,不呈现指标项与未知', async ({
    app,
    browser,
    screen,
  }) => {
    const report = seed.imaging03;
    const sections = report.sections ?? [];
    const impression = sections.find((section) => section.kind === 'impression')!;
    const impressionIndex = sections.indexOf(impression);
    const examName = examNameText(report.sections);
    expect(examName).toBeDefined();
    expect(report.items.length).toBe(0);

    await openConfigured(app, browser, `/report/${report.report_id}`, {
      memberId: seed.xiaoming.member_id,
    });
    await expect(screen.getByTestId(testID.reportExamName)).toContainText(
      SECTION_LABELS[EXAM_NAME_KIND],
    );
    await expect(screen.getByTestId(testID.reportExamName)).toContainText(examName!);
    await expect(screen.getByText(reportDateLabel(report))).toBeVisible();
    await expect(screen.getByText(SECTION_LABELS.findings)).toHaveCount(0);
    await expect(screen.getByTestId(testID.reportSection(impressionIndex))).toContainText(
      SECTION_LABELS.impression,
    );
    await expect(screen.getByTestId(testID.reportSection(impressionIndex))).toContainText(
      impression.text,
    );

    await expectImagingDetail(screen, report.report_id);
  });

  test('影像单 us_01:检查项目 / 检查所见 / 超声提示三段完整,段落卡可缩放原图', async ({
    app,
    browser,
    screen,
  }) => {
    const report = seed.imaging01;
    const sections = report.sections ?? [];
    const examName = examNameText(report.sections);
    const findings = sections.find((section) => section.kind === 'findings')!;
    const findingsIndex = sections.indexOf(findings);
    const impression = sections.find((section) => section.kind === 'impression')!;
    const impressionIndex = sections.indexOf(impression);
    expect(examName).toBeDefined();
    expect(report.items.length).toBe(0);

    await openConfigured(app, browser, `/report/${report.report_id}`, {
      memberId: seed.xiaoming.member_id,
    });
    await expect(screen.getByTestId(testID.reportExamName)).toContainText(
      SECTION_LABELS[EXAM_NAME_KIND],
    );
    await expect(screen.getByTestId(testID.reportExamName)).toContainText(examName!);
    // 影像单未印日期:明确「未印」,不显示检验单口径的「日期未知」(票 03)。
    await expect(screen.getByText(reportDateLabel(report))).toBeVisible();
    await expect(
      screen.getByText(reportDateLabel({ ...report, report_type: seed.reviewReport.report_type })),
    ).toHaveCount(0);
    await expect(screen.getByTestId(testID.reportSection(findingsIndex))).toContainText(
      SECTION_LABELS.findings,
    );
    await expect(screen.getByTestId(testID.reportSection(findingsIndex))).toContainText(
      findings.text,
    );
    await expect(screen.getByTestId(testID.reportSection(impressionIndex))).toContainText(
      SECTION_LABELS.impression,
    );
    await expect(screen.getByTestId(testID.reportSection(impressionIndex))).toContainText(
      impression.text,
    );

    await expectImagingDetail(screen, report.report_id);

    await screen.getByTestId(testID.reportSection(findingsIndex)).tap();
    await expect(screen.getByTestId(testID.reportSectionZoom(findingsIndex))).toBeVisible();
    await screen.getByTestId(testID.reportSection(findingsIndex)).tap();
    await expect(screen.getByTestId(testID.reportSectionZoom(findingsIndex))).toHaveCount(0);
  });

  test('核对低置信项并修正指标项', async ({ app, browser, screen }) => {
    const pending = seed.reviewReport.items.find(
      (item) => item.low_confidence && item.review_status === 'unreviewed',
    );
    expect(pending).toBeDefined();

    await openConfigured(app, browser, `/review/${seed.reviewReport.report_id}`, {
      memberId: seed.xiaoming.member_id,
    });
    await expect(screen.getByTestId(testID.reviewItem(pending!.item_id))).toBeVisible();
    await screen.getByTestId(testID.reviewConfirm(pending!.item_id)).tap();
    await expect(screen.getByTestId(testID.reviewItem(pending!.item_id))).toHaveCount(0);

    // 核对页常驻「去档案查看」:有剩余项时也能一次回到 /timeline。
    await screen.getByTestId(testID.reviewOpenTimeline).tap();
    await browser.waitForURL(/\/timeline$/);

    await openConfigured(app, browser, `/report/${seed.reviewReport.report_id}`, {
      memberId: seed.xiaoming.member_id,
    });
    await expect(screen.getByTestId(testID.reviewItem(pending!.item_id))).toContainText(
      reviewStatusLabel('confirmed')!,
    );
    // 检验单详情保持原路径:指标项标题仍在(票 03 只改影像单)。
    await expect(screen.getByTestId(testID.reportItemsTitle)).toBeVisible();

    const target = seed.reviewReport.items.find(
      (item) =>
        item.item_id !== pending!.item_id &&
        item.value !== null &&
        parseNumericForm(item.value).kind === 'scalar',
    );
    expect(target).toBeDefined();
    await screen.getByTestId(testID.reviewCorrect(target!.item_id)).tap();
    await screen.getByTestId(testID.reviewValue(target!.item_id)).fill(`${target!.value}0`);
    await screen.getByTestId(testID.reviewSave(target!.item_id)).tap();
    await expect(screen.getByTestId(testID.reviewItem(target!.item_id))).toContainText(
      reviewStatusLabel('corrected')!,
    );
  });

  test('可补录未知只给补录主操作,并显示原因', async ({ app, browser, screen }) => {
    const item = seed.healReport.items.find(
      (entry) => entry.flag_reason !== null && isBackfillItem(entry),
    );
    expect(item).toBeDefined();
    const reason = flagReasonText(item!.flag_reason);
    expect(reason).toBeDefined();
    await openConfigured(app, browser, `/report/${seed.healReport.report_id}`, {
      memberId: seed.mama.member_id,
    });
    // ref_range=null 只陈述数据事实,不推断纸面未印或抽取/OCR 漏掉(票 04)。
    const missingRef = seed.healReport.items.find(
      (entry) => entry.name === '血清淀粉样蛋白A',
    );
    expect(missingRef).toBeDefined();
    await expect(screen.getByTestId(testID.reviewItem(missingRef!.item_id))).toContainText(
      '无参考范围',
    );
    await expect(screen.getByTestId(testID.reviewReason(item!.item_id))).toContainText(reason!);
    await expect(screen.getByTestId(testID.reviewBackfill(item!.item_id))).toBeVisible();
    await expect(screen.getByTestId(testID.reviewConfirm(item!.item_id))).toHaveCount(0);
  });

  test('高置信未知项无需修改即可展开原图定位', async ({ app, browser, screen }) => {
    const unknown = seed.healReport.items.find(
      (item) =>
        item.flag === 'unknown' && !item.low_confidence && item.ocr_lines.length > 0,
    );
    const lowConfidence = seed.healReport.items.find(
      (item) => item.low_confidence && item.ocr_lines.length > 0,
    );
    expect(unknown).toBeDefined();
    expect(lowConfidence).toBeDefined();

    await openConfigured(app, browser, `/report/${seed.healReport.report_id}`, {
      memberId: seed.mama.member_id,
    });
    // 高置信未知项默认收起;无需先点「修正」即可展开(票 03)。
    await expect(screen.getByTestId(testID.reviewZoom(unknown!.item_id))).toHaveCount(0);
    await expect(screen.getByTestId(testID.reviewZoomToggle(unknown!.item_id))).toBeVisible();
    await screen.getByTestId(testID.reviewZoomToggle(unknown!.item_id)).tap();
    await expect(screen.getByTestId(testID.reviewZoom(unknown!.item_id))).toBeVisible();
    await screen.getByTestId(testID.reviewZoomToggle(unknown!.item_id)).tap();
    await expect(screen.getByTestId(testID.reviewZoom(unknown!.item_id))).toHaveCount(0);
    // 低置信项保持默认展开。
    await expect(screen.getByTestId(testID.reviewZoom(lowConfidence!.item_id))).toBeVisible();
  });
});
