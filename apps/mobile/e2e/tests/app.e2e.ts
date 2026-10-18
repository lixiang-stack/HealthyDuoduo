// App E2E 主用例集:一条串行核心链路(设置 → 成员 → 时间线 → 上传 → 核对 → 自愈)
// 加一条后端不可达错误态。全部走确定性 locator/expect;无 agent/模型调用。
// 运行入口:apps/mobile `npm run e2e`(专用栈启停见 e2e/run.sh)。
import { beforeAll, describe, test } from '@e2e-dev/web';
import { expect } from 'e2e';

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
    await expect(screen.getByText(API_BASE_URL)).toBeVisible();
    await expect(screen.getByText(/已连通 · 版本/)).toBeVisible();
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

  test('时间线按成员过滤报告', async ({ app, browser, screen }) => {
    await openConfigured(app, browser, '/timeline', { memberId: seed.xiaoming.member_id });
    await expect(screen.getByTestId(testID.reportCard(seed.reviewReport.report_id))).toBeVisible();
    await expect(screen.getByTestId(testID.reportCard(seed.healReport.report_id))).toHaveCount(0);

    await screen.getByTestId(testID.memberChip(seed.mama.member_id)).tap();
    await expect(screen.getByTestId(testID.reportCard(seed.healReport.report_id))).toBeVisible();
    await expect(screen.getByTestId(testID.reportCard(seed.reviewReport.report_id))).toHaveCount(0);
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

    await screen.getByText('去时间线查看').tap();
    await expect(screen.getByTestId(testID.reportCard(payload.report.report_id))).toBeVisible();
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

    await openConfigured(app, browser, `/report/${seed.reviewReport.report_id}`, {
      memberId: seed.xiaoming.member_id,
    });
    await expect(screen.getByTestId(testID.reviewItem(pending!.item_id))).toContainText('已确认');

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

    await screen.getByText('去时间线查看').tap();
    await expect(screen.getByTestId(testID.reportCard(payload.report.report_id))).toBeVisible();
  });

  test('后端不可达时首页显示失败态', async ({ app, browser, screen }) => {
    await openConfigured(app, browser, '/', { baseUrl: 'http://127.0.0.1:1/api/v1' });
    await expect(screen.getByText('无法连接后端')).toBeVisible();
  });
});
