// 按项目浏览用例:项目列表 → 项目详情 → 趋势层 → 「包含该项目的报告」筛选,
// 以及尿常规归尿常规(不串肝功 / 血糖)与 failed 报告不进项目列表 / 筛选。
// 独立 seed/成员,与其它文件不共享图片(样张按 sha256 全局去重)。
import { beforeAll, describe, test } from '@e2e-dev/web';
import { expect } from 'e2e';

import { panelItemIsAbnormal } from '../../src/reports/panelOverview';
import { reportStatusLabel } from '../../src/reports/status';
import {
  getPanelDetail,
  listMemberPanels,
  openConfigured,
  seedPanelData,
  testID,
  type PanelSeed,
} from '../support';

let seed!: PanelSeed;

describe('按项目浏览', { serial: true, timeout: 300_000 }, () => {
  beforeAll(async () => {
    seed = await seedPanelData();
  });

  test('项目列表、详情、趋势层与报告筛选', { timeout: 300_000 }, async ({
    app,
    browser,
    screen,
  }) => {
    const panels = await listMemberPanels(seed.thyroid.member_id);
    expect(panels.map((panel) => panel.panel)).toEqual(['thyroid']);
    const thyroid = panels[0];
    expect(thyroid.narrative).toBe(false);
    expect(thyroid.report_count).toBe(1);
    const detail = await getPanelDetail(seed.thyroid.member_id, 'thyroid');
    const abnormal = detail.items.filter((item) => panelItemIsAbnormal(item));
    expect(abnormal).toHaveLength(0);

    await openConfigured(app, browser, '/timeline', { memberId: seed.thyroid.member_id });
    await screen.getByTestId(testID.archiveViewPanel).tap();
    await expect(screen.getByTestId(testID.panelRow('thyroid'), { visible: true })).toBeVisible();
    await expect(screen.getByTestId(testID.panelRowMeta('thyroid'))).toContainText(
      `${thyroid.report_count} 份 · ${thyroid.abnormal_count} 项异常 · ${thyroid.item_count} 项指标`,
    );
    await expect(screen.getByTestId(testID.panelRow('liver'))).toHaveCount(0);
    await expect(screen.getByTestId(testID.panelRow('glucose'))).toHaveCount(0);

    await screen.getByTestId(testID.panelRow('thyroid')).tap();
    await browser.waitForURL(/\/panel\/thyroid$/);
    await expect(screen.getByTestId(testID.panelDetailHeading)).toContainText('甲状腺');
    await expect(screen.getByTestId(testID.panelDetailMeta)).toContainText(
      `${detail.report_count} 份报告`,
    );
    await expect(screen.getByTestId(testID.panelDetailMeta)).toContainText('0 项异常');
    await expect(screen.getByTestId(testID.panelAbnormalTitle)).toHaveCount(0);
    await expect(screen.getByTestId(testID.panelRestTitle)).toContainText(
      `其余 ${detail.items.length} 项`,
    );
    const first = detail.items[0];
    await expect(screen.getByTestId(testID.panelItem(0))).toContainText(first.name);

    // 正常项点按打开趋势层,使用已加载的 points,不额外请求。
    await screen.getByTestId(testID.panelItem(0)).tap();
    await expect(screen.getByTestId(testID.trendDetail)).toBeVisible();
    await expect(screen.getByTestId(testID.trendDetailChart)).toBeVisible();
    await screen.getByTestId(testID.trendDetailClose).tap();
    await expect(screen.getByTestId(testID.trendDetail)).toHaveCount(0);

    // 「包含该项目的报告」→ /timeline?panel=;数量与项目列表一致。
    await screen.getByTestId(testID.panelReportsLink).tap();
    await browser.waitForURL(/\/timeline\?panel=thyroid$/);
    await expect(screen.getByTestId(testID.archivePanelBanner)).toContainText(
      `只看甲状腺 · ${thyroid.report_count} 份`,
    );
    await expect(screen.getByTestId(testID.reportCard(seed.tft01.report_id))).toBeVisible();
    await screen.getByTestId(testID.archivePanelClear).tap();
    await expect(screen.getByTestId(testID.archivePanelBanner)).toHaveCount(0);
    await expect(screen.getByTestId(testID.reportCard(seed.tft01.report_id))).toBeVisible();
  });

  test('尿常规不串肝功 / 血糖;failed 报告不进项目视图与筛选', {
    timeout: 300_000,
  }, async ({ app, browser, screen }) => {
    const panels = await listMemberPanels(seed.urinalysis.member_id);
    expect(panels.map((panel) => panel.panel)).toEqual(['urinalysis']);

    await openConfigured(app, browser, '/timeline', { memberId: seed.urinalysis.member_id });
    await screen.getByTestId(testID.archiveViewPanel).tap();
    await expect(screen.getByTestId(testID.panelRow('urinalysis'), { visible: true })).toBeVisible();
    await expect(screen.getByTestId(testID.panelRow('liver'))).toHaveCount(0);
    await expect(screen.getByTestId(testID.panelRow('glucose'))).toHaveCount(0);

    // failed 报告:项目视图空态;按时间视图仍可见,?panel= 不包含它(即使 panels=[other])。
    await openConfigured(app, browser, '/timeline', { memberId: seed.failed.member_id });
    await screen.getByTestId(testID.archiveViewPanel).tap();
    await expect(screen.getByTestId(testID.archivePanelsEmpty, { visible: true })).toContainText(
      '还没有可归类的报告。',
    );
    await screen.getByTestId(testID.archiveShowTime).tap();
    await expect(screen.getByTestId(testID.reportCard(seed.failedReport.report_id))).toContainText(
      reportStatusLabel(seed.failedReport)!,
    );

    await openConfigured(app, browser, '/timeline?panel=other', {
      memberId: seed.failed.member_id,
    });
    await expect(screen.getByTestId(testID.archivePanelBanner)).toContainText('只看其他 · 0 份');
    await expect(screen.getByTestId(testID.archiveEmpty)).toBeVisible();
    await expect(screen.getByTestId(testID.reportCard(seed.failedReport.report_id))).toHaveCount(0);
  });
});
