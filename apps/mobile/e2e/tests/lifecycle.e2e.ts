// 破坏性生命周期用例:改归属 → 删除 → 同图重传。独立 seed/成员/cbc_02 fixture,
// 与主链路不共享数据;失败不会连坐跳过其他文件。
import { beforeAll, test } from '@e2e-dev/web';
import { expect } from 'e2e';

import { reassignConfirmText } from '../../src/reports/manage';
import { itemFlagLabel, reportStatusLabel, uploadResultText } from '../../src/reports/status';
import {
  openConfigured,
  samplePath,
  seedLifecycleData,
  stubFileChooser,
  testID,
  type LifecycleSeed,
  type UploadResult,
} from '../support';

let seed!: LifecycleSeed;

beforeAll(async () => {
  seed = await seedLifecycleData();
});

test('改归属、删除与同图重传', { timeout: 300_000 }, async ({ app, browser, screen }) => {
  const { member, target, report } = seed;
  const unknown = report.items.find((item) => item.flag === 'unknown')!;

  await openConfigured(app, browser, `/report/${report.report_id}`, {
    memberId: member.member_id,
  });
  await expect(screen.getByText(reportStatusLabel(report)!)).toBeVisible();
  await expect(screen.getByTestId(testID.reportBadge(report.report_id, 'unknown'))).toBeVisible();
  await expect(screen.getByTestId(testID.reviewItem(unknown.item_id))).toContainText(
    itemFlagLabel(unknown),
  );
  await expect(screen.getByTestId(testID.reportItemsTitle)).toBeVisible();
  await expect(screen.getByTestId(testID.reportPendingEntry)).toBeVisible();

  await screen.getByTestId('report-manage-reassign').tap();
  await screen.getByTestId(testID.memberChip(target.member_id)).tap();
  await expect(screen.getByText(reassignConfirmText(member.name, target.name))).toBeVisible();
  await screen.getByTestId('report-manage-confirm-reassign').tap();
  await expect(screen.getByTestId('report-manage-notice')).toContainText(target.name);

  await screen.getByTestId('report-manage-delete').tap();
  await screen.getByTestId('report-manage-confirm-delete').tap();
  await browser.waitForURL(/\/timeline$/);
  await screen.getByTestId(testID.memberChip(target.member_id)).tap();
  await expect(screen.getByTestId(testID.reportCard(report.report_id))).toHaveCount(0);

  await stubFileChooser(browser);
  await openConfigured(app, browser, '/upload', { memberId: target.member_id });
  await screen.getByTestId('upload-pick-library').tap();
  await browser.locator('[data-testid="file-input"]').setInputFiles(samplePath('cbc_02.png'));
  await expect(screen.getByText('cbc_02.png')).toBeVisible();

  const responsePromise = browser.waitForResponse(/\/api\/v1\/reports$/);
  await screen.getByTestId('upload-submit').tap();
  const payload = await (await responsePromise).json<UploadResult>();
  expect(payload.duplicate).toBe(false);
  // report_id 即图像 sha256:同图重传内容身份不变,新在归档行(指标项 id 全换)。
  expect(payload.report.report_id).toBe(report.report_id);
  expect(payload.report.items.length).toBeGreaterThan(0);
  expect(payload.report.items[0]!.item_id).not.toBe(report.items[0]!.item_id);
  await expect(screen.getByTestId('upload-result')).toContainText(uploadResultText(payload));

  await screen.getByTestId(testID.uploadOpenTimeline).tap();
  await expect(screen.getByTestId(testID.reportCard(payload.report.report_id))).toBeVisible();
});
