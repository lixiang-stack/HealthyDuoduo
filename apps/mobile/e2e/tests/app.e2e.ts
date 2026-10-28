// App E2E 主用例集:一条串行核心链路(设置 → 成员 → 时间线 → 上传 → 核对),
// 报告内容来自专用栈的 golden 回放(见 e2e/support.ts);破坏性生命周期与错误态
// 在独立文件,避免连坐跳过。运行入口:apps/mobile `npm run e2e`(专用栈见 e2e/run.sh)。
import { beforeAll, describe, test } from '@e2e-dev/web';
import { expect } from 'e2e';
import type { Screen } from 'e2e';

import { parseNumericForm } from '../../src/reports/refRange';
import { relativeDateLabel } from '../../src/reports/relativeDate';
import { EXAM_NAME_KIND, SECTION_LABELS, examNameText } from '../../src/reports/sections';
import { reportDateLabel, uploadResultText } from '../../src/reports/status';
import { flagReasonText, isBackfillItem, reviewStatusLabel } from '../../src/review/rules';
import {
  API_BASE_URL,
  openConfigured,
  samplePath,
  sampleReportId,
  seedData,
  stubFileChooser,
  testID,
  type Member,
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
    await expect(
      screen.getByTestId(testID.headerSettingsFamily, { visible: true }),
    ).toBeVisible();
  });

  test('创建并切换家庭成员', async ({ app, browser, screen }) => {
    await openConfigured(app, browser, '/members');
    await screen.getByTestId('members-name-input').fill('E2E 宝宝');
    const responsePromise = browser.waitForResponse(/\/api\/v1\/members$/);
    await screen.getByTestId('members-create').tap();
    const response = await responsePromise;
    expect(response.status).toBe(201);
    const baby = await response.json<Member>();
    expect(baby.name).toBe('E2E 宝宝');

    await expect(screen.getByTestId(testID.memberSelect(baby.member_id))).toBeVisible();
    await screen.getByTestId(testID.memberSelect(baby.member_id)).tap();
    await expect(screen.getByTestId(testID.memberRow(baby.member_id))).toContainText('当前成员');
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
    // cbc_03 是 success 但含未核对低置信项:未知徽标为 0,待核对徽标应显示。
    await expect(
      screen.getByTestId(testID.reportBadge(seed.reviewReport.report_id, 'unknown')),
    ).toHaveCount(0);
    await expect(
      screen.getByTestId(testID.reportBadge(seed.reviewReport.report_id, 'unreviewed')),
    ).toBeVisible();

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

  test('家庭首页成员卡、双 Tab、设置 header 与上传 FAB', async ({
    app,
    browser,
    screen,
  }) => {
    const latestRelative = relativeDateLabel('2024-09-15', new Date());

    // 以无报告的成员为当前成员打开,点小明卡后应切换到小明并进档案。
    await openConfigured(app, browser, '/', { memberId: seed.uploadMember.member_id });
    await expect(
      screen.getByTestId(testID.headerSettingsFamily, { visible: true }),
    ).toBeVisible();
    await expect(screen.getByTestId(testID.tabFamily)).toBeVisible();
    await expect(screen.getByTestId(testID.tabArchive)).toBeVisible();
    await expect(screen.getByTestId(testID.uploadFab)).toHaveCount(0);

    const xiaomingCard = screen.getByTestId(testID.familyMember(seed.xiaoming.member_id));
    await expect(xiaomingCard).toContainText('E2E 小明');
    await expect(xiaomingCard).toContainText('孩子');
    await expect(
      screen.getByTestId(testID.familyMemberPending(seed.xiaoming.member_id)),
    ).toContainText('待核对 1 份 · 1 项');
    await expect(
      screen.getByTestId(testID.familyMemberLatest(seed.xiaoming.member_id)),
    ).toContainText(`最近：影像 · 2024-09-15 · ${latestRelative}`);
    await expect(
      screen.getByTestId(testID.familyMemberAbnormal(seed.xiaoming.member_id)),
    ).toContainText('异常项目：血常规 1 项');
    await expect(
      screen.getByTestId(testID.familyMemberFlags(seed.xiaoming.member_id)),
    ).toContainText('最近报告 偏高 0 · 偏低 0');
    await expect(
      screen.getByTestId(testID.familyMemberCount(seed.xiaoming.member_id)),
    ).toContainText('报告 3 份');

    const mamaCard = screen.getByTestId(testID.familyMember(seed.mama.member_id));
    await expect(mamaCard).toContainText('E2E 妈妈');
    await expect(
      screen.getByTestId(testID.familyMemberPending(seed.mama.member_id)),
    ).toContainText('待核对 1 份 · 1 项');
    await expect(
      screen.getByTestId(testID.familyMemberLatest(seed.mama.member_id)),
    ).toContainText('最近：血常规 · 2024-08-19');
    await expect(
      screen.getByTestId(testID.familyMemberAbnormal(seed.mama.member_id)),
    ).toContainText('异常项目：血常规 7 项');
    await expect(
      screen.getByTestId(testID.familyMemberFlags(seed.mama.member_id)),
    ).toContainText('最近报告 偏高 2 · 偏低 5');
    await expect(
      screen.getByTestId(testID.familyMemberCount(seed.mama.member_id)),
    ).toContainText('报告 1 份');

    // 无报告成员:保留成员卡,只显示空文案,不显示待核对 / 异常项目 / 总数。
    const emptyCard = screen.getByTestId(testID.familyMember(seed.uploadMember.member_id));
    await expect(emptyCard).toContainText('这位成员还没有检查单');
    await expect(
      screen.getByTestId(testID.familyMemberPending(seed.uploadMember.member_id)),
    ).toHaveCount(0);
    await expect(
      screen.getByTestId(testID.familyMemberAbnormal(seed.uploadMember.member_id)),
    ).toHaveCount(0);
    await expect(
      screen.getByTestId(testID.familyMemberCount(seed.uploadMember.member_id)),
    ).toHaveCount(0);
    await expect(screen.getByTestId(testID.familyManageMembers)).toBeVisible();

    // 点卡 = 选中该成员并进档案;档案页显示该成员报告与上传 FAB。
    await xiaomingCard.tap();
    await browser.waitForURL(/\/timeline$/);
    await expect(screen.getByTestId(testID.reportCard(seed.reviewReport.report_id))).toBeVisible();
    await expect(screen.getByTestId(testID.reportCard(seed.healReport.report_id))).toHaveCount(0);
    await expect(
      screen.getByTestId(testID.headerSettingsArchive, { visible: true }),
    ).toBeVisible();
    await expect(screen.getByTestId(testID.uploadFab)).toBeVisible();
    await expect(screen.getByTestId(testID.archivePendingPartial)).toContainText('待核对 · 1 份');

    // 双 Tab 切换保持 URL / 当前成员;回档案默认按时间。
    await screen.getByTestId(testID.tabFamily).tap();
    await browser.waitForURL(/\/$/);
    await expect(screen.getByTestId(testID.familyMember(seed.xiaoming.member_id))).toBeVisible();
    await screen.getByTestId(testID.tabArchive).tap();
    await browser.waitForURL(/\/timeline$/);
    await expect(screen.getByTestId(testID.reportCard(seed.reviewReport.report_id))).toBeVisible();
    await expect(screen.getByTestId(testID.archivePendingPartial)).toBeVisible();

    await screen.getByTestId(testID.uploadFab).tap();
    await browser.waitForURL(/\/upload$/);
    await expect(screen.getByText('归属成员')).toBeVisible();
  });

  test('档案未选成员提示回家庭;header 设置入口', async ({ app, browser, screen }) => {
    await openConfigured(app, browser, '/timeline', { memberId: 999999999 });
    await expect(screen.getByTestId(testID.archiveUnselected)).toContainText('选择一个成员查看其档案。');
    await expect(screen.getByTestId(testID.uploadFab)).toHaveCount(0);

    await screen.getByTestId(testID.archiveBackHome).tap();
    await browser.waitForURL(/\/$/);
    await expect(screen.getByTestId(testID.familyManageMembers)).toBeVisible();

    await screen.getByTestId(testID.headerSettingsFamily, { visible: true }).tap();
    await browser.waitForURL(/\/settings$/);
    await expect(screen.getByText('后端 Base URL')).toBeVisible();
  });

  test('全部档案与筛选', async ({ app, browser, screen }) => {
    await openConfigured(app, browser, '/timeline', {
      memberId: seed.mama.member_id,
    });
    await expect(screen.getByTestId(testID.archivePendingPartial)).toContainText(
      '待核对 · 1 份 · 1 项',
    );
    await expect(screen.getByTestId(testID.archivePendingFailed)).toHaveCount(0);

    await screen.getByTestId(testID.archivePendingPartial).tap();
    await browser.waitForURL(/\/timeline\?filter=partial$/);
    await expect(screen.getByTestId(testID.archiveFilterBanner)).toContainText('1 份');
    await expect(screen.getByTestId(testID.reportCard(seed.healReport.report_id))).toBeVisible();

    await screen.getByTestId(testID.archiveFilterClear).tap();
    await expect(screen.getByTestId(testID.archiveFilterBanner)).toHaveCount(0);
    await expect(screen.getByTestId(testID.reportCard(seed.healReport.report_id))).toBeVisible();

    await openConfigured(app, browser, '/timeline?filter=failed', {
      memberId: seed.xiaoming.member_id,
    });
    await expect(screen.getByTestId(testID.archivePendingFailed)).toHaveCount(0);
    await expect(screen.getByTestId(testID.archiveFilterBanner)).toContainText('0 份');
    await expect(screen.getByTestId(testID.archiveEmpty)).toBeVisible();
    await screen.getByTestId(testID.archiveShowAll).tap();
    await expect(screen.getByTestId(testID.reportCard(seed.reviewReport.report_id))).toBeVisible();
    await expect(screen.getByTestId(testID.reportCard(seed.imaging01.report_id))).toBeVisible();
    await expect(screen.getByTestId(testID.reportCard(seed.imaging03.report_id))).toBeVisible();
  });

  test('按项目:影像行直接进最新报告详情', async ({ app, browser, screen }) => {
    await openConfigured(app, browser, '/timeline', { memberId: seed.xiaoming.member_id });
    await screen.getByTestId(testID.archiveViewPanel).tap();
    await expect(screen.getByTestId(testID.panelRow('cbc'), { visible: true })).toBeVisible();
    const imagingRow = screen.getByTestId(testID.panelRow('imaging'));
    await expect(imagingRow).toContainText('叙述型报告');
    await imagingRow.tap();
    // 影像项目不做项目详情:直接打开最新影像报告(us_03 有日期,晚于 us_01)。
    await browser.waitForURL(new RegExp(`/report/${seed.imaging03.report_id}$`));
    await expect(screen.getByTestId(testID.reportExamName)).toBeVisible();
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

  test('一次选择多张图:逐张识别并按行给出核对入口', async ({ app, browser, screen }) => {
    const successReportId = await sampleReportId('glu_03.jpeg');
    const reviewReportId = await sampleReportId('glu_01.png');
    await stubFileChooser(browser);
    await openConfigured(app, browser, '/upload', { memberId: seed.uploadMember.member_id });
    await screen.getByTestId('upload-pick-library').tap();
    await browser
      .locator('[data-testid="file-input"]')
      .setInputFiles([samplePath('glu_03.jpeg'), samplePath('glu_01.png')]);

    await expect(screen.getByTestId(testID.uploadSelectedCount)).toContainText('2/9');
    await expect(screen.getByText('glu_03.jpeg')).toBeVisible();
    await expect(screen.getByText('glu_01.png')).toBeVisible();
    await expect(screen.getByTestId(testID.uploadRowRemove(0))).toBeVisible();

    await screen.getByTestId('upload-submit').tap();
    await expect(screen.getByTestId('upload-result')).toContainText(
      '共 2 张:1 张已归档,1 张待核对',
    );
    await expect(screen.getByTestId(testID.uploadRowStatus(0))).toContainText('已归档');
    await expect(screen.getByTestId(testID.uploadRowStatus(1))).toContainText('待核对');
    await expect(screen.getByTestId(testID.uploadRowReview(1))).toBeVisible();
    await expect(screen.getByTestId(testID.uploadRowOpen(0))).toBeVisible();
    await expect(screen.getByTestId(testID.uploadRetry)).toHaveCount(0);
    await expect(screen.getByTestId(testID.uploadContinue)).toBeVisible();

    await screen.getByTestId(testID.uploadOpenTimeline).tap();
    await expect(screen.getByTestId(testID.reportCard(successReportId))).toBeVisible();
    await expect(screen.getByTestId(testID.reportCard(reviewReportId))).toBeVisible();
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
