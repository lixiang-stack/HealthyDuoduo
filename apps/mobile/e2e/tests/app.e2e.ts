// App E2E 主用例集:一条串行核心链路(设置 → 成员 → 时间线 → 上传 → 核对 → 自愈)
// 加一条后端不可达错误态。全部走确定性 locator/expect;无 agent/模型调用。
// 运行入口:apps/mobile `npm run e2e`(专用栈启停见 e2e/run.sh)。
import { beforeAll, describe, test } from '@e2e-dev/web';
import { expect } from 'e2e';
import type { Screen } from 'e2e';

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
async function expectImagingDetail(screen: Screen): Promise<void> {
  await expect(screen.getByText(/指标项/)).toHaveCount(0);
  await expect(screen.getByText(/未知/)).toHaveCount(0);
  await expect(screen.getByText(/待核对/)).toHaveCount(0);
  await expect(screen.getByText(/逐项核对/)).toHaveCount(0);
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
    const imaging03Card = screen.getByTestId(testID.reportCard(seed.imaging03.report_id));
    await expect(imaging01Card).toContainText('检查单未印日期');
    await expect(imaging01Card).not.toContainText(/未知|待核对/);
    await expect(imaging03Card).not.toContainText(/未知|待核对/);

    await screen.getByTestId(testID.memberChip(seed.mama.member_id)).tap();
    await expect(screen.getByTestId(testID.reportCard(seed.healReport.report_id))).toBeVisible();
    await expect(screen.getByTestId(testID.reportCard(seed.reviewReport.report_id))).toHaveCount(0);
    // 检验单卡片保持原样:未知徽标与非零待核对徽标仍显示。
    await expect(screen.getByTestId(testID.reportCard(seed.healReport.report_id))).toContainText(
      /未知 \d+/,
    );
    await expect(screen.getByTestId(testID.reportCard(seed.healReport.report_id))).toContainText(
      /待核对 \d+ 项/,
    );
  });

  test('成员概览与待处理入口', async ({ app, browser, screen }) => {
    await openConfigured(app, browser, '/', { memberId: seed.xiaoming.member_id });
    await expect(screen.getByText('查看谁的档案')).toBeVisible();
    // cbc_03 实际是 success;小明待核对 / 失败都为 0 → 空态,不显示入口。
    await expect(screen.getByTestId(testID.overviewPendingEmpty)).toContainText('没有待处理的报告。');
    await expect(screen.getByTestId(testID.overviewPendingPartial)).toHaveCount(0);
    await expect(screen.getByTestId(testID.overviewPendingFailed)).toHaveCount(0);
    await expect(screen.getByTestId(testID.overviewLatest(seed.reviewReport.report_id))).toBeVisible();
    await expect(screen.getByTestId(testID.overviewLatest(seed.imaging03.report_id))).toBeVisible();
    await expect(screen.getByTestId(testID.overviewLatest(seed.imaging01.report_id))).toHaveCount(0);
    await expect(screen.getByText('全部档案（3 份）')).toBeVisible();

    await screen.getByTestId(testID.memberChip(seed.mama.member_id)).tap();
    await expect(screen.getByTestId(testID.overviewPendingPartial)).toContainText(
      /待核对 · 1 份 · \d+ 项/,
    );
    await expect(screen.getByTestId(testID.overviewPendingFailed)).toHaveCount(0);
    await expect(screen.getByTestId(testID.overviewPendingEmpty)).toHaveCount(0);
    await expect(screen.getByTestId(testID.overviewLatest(seed.healReport.report_id))).toBeVisible();
    await expect(screen.getByTestId(testID.overviewLatest(seed.reviewReport.report_id))).toHaveCount(0);
    await expect(screen.getByTestId(testID.overviewLatest(seed.imaging03.report_id))).toHaveCount(0);
    await expect(screen.getByTestId(testID.overviewLatest(seed.imaging01.report_id))).toHaveCount(0);
    await expect(screen.getByText('全部档案（1 份）')).toBeVisible();

    await screen.getByTestId(testID.overviewPendingPartial).tap();
    await browser.waitForURL(/\/timeline\?filter=partial$/);
    await expect(screen.getByTestId(testID.archiveFilterBanner)).toContainText(
      '只看待核对 · 1 份',
    );
    await expect(screen.getByTestId(testID.reportCard(seed.healReport.report_id))).toBeVisible();

    await openConfigured(app, browser, '/', { memberId: seed.xiaoming.member_id });
    await screen.getByText('全部档案（3 份）').tap();
    await browser.waitForURL(/\/timeline$/);
    await expect(screen.getByTestId(testID.reportCard(seed.reviewReport.report_id))).toBeVisible();
    await expect(screen.getByTestId(testID.reportCard(seed.imaging01.report_id))).toBeVisible();
    await expect(screen.getByTestId(testID.reportCard(seed.imaging03.report_id))).toBeVisible();
  });

  test('全部档案与筛选', async ({ app, browser, screen }) => {
    // cbc_03 实际是 success(单条低置信项不翻转报告状态);真实 partial 是妈妈的 cbc_04。
    await openConfigured(app, browser, '/timeline?filter=partial', {
      memberId: seed.mama.member_id,
    });
    await expect(screen.getByTestId(testID.archiveFilterBanner)).toContainText(
      '只看待核对 · 1 份',
    );
    await expect(screen.getByTestId(testID.reportCard(seed.healReport.report_id))).toBeVisible();

    await screen.getByTestId(testID.archiveFilterClear).tap();
    await expect(screen.getByTestId(testID.archiveFilterBanner)).toHaveCount(0);
    await expect(screen.getByTestId(testID.reportCard(seed.healReport.report_id))).toBeVisible();

    await openConfigured(app, browser, '/timeline?filter=failed', {
      memberId: seed.xiaoming.member_id,
    });
    await expect(screen.getByTestId(testID.archiveFilterBanner)).toContainText(
      '只看识别失败 · 0 份',
    );
    await expect(screen.getByTestId(testID.archiveEmpty)).toContainText('没有符合筛选条件的报告。');
    await screen.getByText('查看全部档案').tap();
    await expect(screen.getByTestId(testID.reportCard(seed.reviewReport.report_id))).toBeVisible();
    await expect(screen.getByTestId(testID.reportCard(seed.imaging01.report_id))).toBeVisible();
    await expect(screen.getByTestId(testID.reportCard(seed.imaging03.report_id))).toBeVisible();
  });

  test('上传样张后时间线出现新报告', async ({ app, browser, screen }) => {
    await stubFileChooser(browser);
    await openConfigured(app, browser, '/upload', { memberId: seed.xiaoming.member_id });
    await screen.getByTestId('upload-pick-library').tap();
    await browser.locator('[data-testid="file-input"]').setInputFiles(samplePath('glu_02.jpeg'));
    await expect(screen.getByText('glu_02.jpeg')).toBeVisible();

    const responsePromise = browser.waitForResponse(/\/api\/v1\/reports$/);
    await screen.getByTestId('upload-submit').tap();
    const payload = await (await responsePromise).json<UploadResult>();
    expect(payload.duplicate).toBe(false);
    await expect(screen.getByTestId('upload-result')).toContainText('已归档');

    await screen.getByText('去档案查看').tap();
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
    expect(report.report_date).toBe('2024-09-15');
    expect(sections.some((section) => section.kind === 'findings')).toBe(false);
    // 影像单不再产出指标项(票 04);详情只呈现段落 + 原图。
    expect(report.items.length).toBe(0);

    await openConfigured(app, browser, `/report/${report.report_id}`, {
      memberId: seed.xiaoming.member_id,
    });
    await expect(screen.getByTestId(testID.reportExamName)).toContainText('检查项目');
    await expect(screen.getByTestId(testID.reportExamName)).toContainText(
      '产科胎儿彩超检查（中孕）',
    );
    await expect(screen.getByText('2024-09-15')).toBeVisible();
    await expect(screen.getByText('检查所见')).toHaveCount(0);
    await expect(screen.getByTestId(testID.reportSection(impressionIndex))).toContainText(
      '超声提示',
    );
    await expect(screen.getByTestId(testID.reportSection(impressionIndex))).toContainText(
      impression.text,
    );

    await expectImagingDetail(screen);
  });

  test('影像单 us_01:检查项目 / 检查所见 / 超声提示三段完整,段落卡可缩放原图', async ({
    app,
    browser,
    screen,
  }) => {
    const report = seed.imaging01;
    const sections = report.sections ?? [];
    const examName = sections.find((section) => section.kind === 'exam_name');
    const findings = sections.find((section) => section.kind === 'findings')!;
    const findingsIndex = sections.indexOf(findings);
    const impression = sections.find((section) => section.kind === 'impression')!;
    const impressionIndex = sections.indexOf(impression);
    expect(examName).toBeDefined();
    expect(report.report_date).toBeNull();
    expect(report.items.length).toBe(0);

    await openConfigured(app, browser, `/report/${report.report_id}`, {
      memberId: seed.xiaoming.member_id,
    });
    await expect(screen.getByTestId(testID.reportExamName)).toContainText('检查项目');
    await expect(screen.getByTestId(testID.reportExamName)).toContainText(examName!.text);
    // 影像单未印日期:明确「未印」,不显示检验单口径的「日期未知」(票 03)。
    await expect(screen.getByText('检查单未印日期')).toBeVisible();
    await expect(screen.getByText('日期未知')).toHaveCount(0);
    await expect(screen.getByTestId(testID.reportSection(findingsIndex))).toContainText('检查所见');
    await expect(screen.getByTestId(testID.reportSection(findingsIndex))).toContainText(findings.text);
    await expect(screen.getByTestId(testID.reportSection(impressionIndex))).toContainText('超声提示');
    await expect(screen.getByTestId(testID.reportSection(impressionIndex))).toContainText(
      impression.text,
    );

    await expectImagingDetail(screen);

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
    await screen.getByText('去档案查看').tap();
    await browser.waitForURL(/\/timeline$/);

    await openConfigured(app, browser, `/report/${seed.reviewReport.report_id}`, {
      memberId: seed.xiaoming.member_id,
    });
    await expect(screen.getByTestId(testID.reviewItem(pending!.item_id))).toContainText('已确认');
    // 检验单详情保持原路径:指标项标题仍在(票 03 只改影像单)。
    await expect(screen.getByText(/指标项/)).toBeVisible();

    const target = seed.reviewReport.items.find(
      (item) => item.item_id !== pending!.item_id && item.value !== null,
    );
    expect(target).toBeDefined();
    await screen.getByTestId(testID.reviewCorrect(target!.item_id)).tap();
    await screen.getByTestId(testID.reviewValue(target!.item_id)).fill(`${target!.value}0`);
    await screen.getByTestId(testID.reviewSave(target!.item_id)).tap();
    await expect(screen.getByTestId(testID.reviewItem(target!.item_id))).toContainText('已修正');
  });

  test('改归属、删除与同图重传', async ({ app, browser, screen }) => {
    await openConfigured(app, browser, `/report/${seed.healReport.report_id}`, {
      memberId: seed.mama.member_id,
    });
    // 检验单详情保持原路径:待核对 / 未知徽标、指标项与逐项核对入口仍在(票 03 只改影像单)。
    await expect(screen.getByText('待核对')).toBeVisible();
    await expect(screen.getByText(/未知 \d+/)).toBeVisible();
    const unknownItem = seed.healReport.items.find((item) => item.flag === 'unknown');
    expect(unknownItem).toBeDefined();
    await expect(screen.getByTestId(testID.reviewItem(unknownItem!.item_id))).toContainText('未知');
    await expect(screen.getByText(/指标项/)).toBeVisible();
    await expect(screen.getByText(/逐项核对/)).toBeVisible();

    await screen.getByTestId('report-manage-reassign').tap();
    await screen.getByTestId(testID.memberChip(seed.xiaoming.member_id)).tap();
    await expect(screen.getByText('把报告从「E2E 妈妈」改派给「E2E 小明」?')).toBeVisible();
    await screen.getByTestId('report-manage-confirm-reassign').tap();
    await expect(screen.getByTestId('report-manage-notice')).toContainText('已改派给「E2E 小明」');

    await screen.getByTestId('report-manage-delete').tap();
    await screen.getByTestId('report-manage-confirm-delete').tap();
    await browser.waitForURL(/\/timeline$/);
    await screen.getByTestId(testID.memberChip(seed.xiaoming.member_id)).tap();
    await expect(screen.getByTestId(testID.reportCard(seed.healReport.report_id))).toHaveCount(0);

    await stubFileChooser(browser);
    await openConfigured(app, browser, '/upload', { memberId: seed.xiaoming.member_id });
    await screen.getByTestId('upload-pick-library').tap();
    await browser.locator('[data-testid="file-input"]').setInputFiles(samplePath('cbc_04.png'));
    await expect(screen.getByText('cbc_04.png')).toBeVisible();

    const responsePromise = browser.waitForResponse(/\/api\/v1\/reports$/);
    await screen.getByTestId('upload-submit').tap();
    const payload = await (await responsePromise).json<UploadResult>();
    expect(payload.duplicate).toBe(false);
    // report_id 即图像 sha256:同图重传内容身份不变,新在归档行(指标项 id 全换)。
    expect(payload.report.report_id).toBe(seed.healReport.report_id);
    expect(payload.report.items.length).toBeGreaterThan(0);
    expect(payload.report.items[0]!.item_id).not.toBe(seed.healReport.items[0]!.item_id);
    await expect(screen.getByTestId('upload-result')).toContainText('已归档');

    await screen.getByText('去档案查看').tap();
    await expect(screen.getByTestId(testID.reportCard(payload.report.report_id))).toBeVisible();
  });

  test('后端不可达时首页显示失败态', async ({ app, browser, screen }) => {
    await openConfigured(app, browser, '/', { baseUrl: 'http://127.0.0.1:1/api/v1' });
    await expect(screen.getByText(/加载失败/)).toBeVisible();
  });
});
