// 按项目浏览用例:项目列表 → 项目详情 → 趋势层 → 「包含该项目的报告」筛选,
// 以及尿常规归尿常规(不串肝功 / 血糖)与 failed 报告不进项目列表 / 筛选。
// 独立 seed/成员,与其它文件不共享图片(样张按 sha256 全局去重)。
import { beforeAll, describe, test } from '@e2e-dev/web';
import { expect } from 'e2e';

import { PANEL_ORDER } from '../../src/reports/panels';
import {
  abnormalItemCount,
  panelItemIsAbnormal,
  reportItemGroups,
} from '../../src/reports/panelOverview';
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

  // 掩码语料样本无原图,经 e2e/replay 替身图回放 golden:混合生化 / 凝血 / 其他。
  test('混合报告分组、凝血项目与其他项目来源', { timeout: 300_000 }, async ({
    app,
    browser,
    screen,
  }) => {
    const mixedPanels = await listMemberPanels(seed.mixed.member_id);
    // 同日期按 §5.3 固定优先级;期望顺序由 App 的 PANEL_ORDER 派生。
    const expectedPanels = PANEL_ORDER.filter((panel) =>
      mixedPanels.some((summary) => summary.panel === panel),
    );
    expect(mixedPanels.map((panel) => panel.panel)).toEqual(expectedPanels);
    expect(mixedPanels.find((panel) => panel.panel === 'liver')!.report_count).toBe(3);
    expect(mixedPanels.find((panel) => panel.panel === 'kidney')!.report_count).toBe(2);
    expect(mixedPanels.find((panel) => panel.panel === 'lipid')!.report_count).toBe(1);
    expect(mixedPanels.find((panel) => panel.panel === 'glucose')!.report_count).toBe(1);

    // 报告详情按 panels 分组:lft_08 同时含肝功 / 肾功 / 尿酸 / 电解质,
    // 组顺序与每组「N 项 · M 项异常」由报告 golden 派生。
    const reportGroups = reportItemGroups(seed.lft08);
    expect(reportGroups.reduce((sum, group) => sum + group.items.length, 0)).toBe(
      seed.lft08.items.length,
    );
    await openConfigured(app, browser, `/report/${seed.lft08.report_id}`, {
      memberId: seed.mixed.member_id,
    });
    await expect(screen.getByTestId(testID.reportItemsTitle)).toBeVisible();
    const groupBoxes: { y: number }[] = [];
    for (const group of reportGroups) {
      const groupNode = screen.getByTestId(testID.reportPanelGroup(group.panel!));
      await expect(groupNode).toContainText(group.label);
      await expect(groupNode).toContainText(
        `${group.items.length} 项 · ${abnormalItemCount(group.items)} 项异常`,
      );
      const box = await groupNode.boundingBox();
      expect(box).toBeDefined();
      groupBoxes.push(box!);
    }
    // DOM 纵向顺序 = report.panels 顺序。
    for (let index = 1; index < groupBoxes.length; index += 1) {
      expect(groupBoxes[index].y).toBeGreaterThan(groupBoxes[index - 1].y);
    }

    // 项目列表覆盖混合生化的全部项目(含血糖 / 电解质)。
    await openConfigured(app, browser, '/timeline', { memberId: seed.mixed.member_id });
    await screen.getByTestId(testID.archiveViewPanel).tap();
    for (const panel of expectedPanels) {
      await expect(screen.getByTestId(testID.panelRow(panel), { visible: true })).toBeVisible();
    }

    // ?panel=lipid 只保留 lft_06。
    const lipid = mixedPanels.find((panel) => panel.panel === 'lipid')!;
    await openConfigured(app, browser, '/timeline?panel=lipid', {
      memberId: seed.mixed.member_id,
    });
    await expect(screen.getByTestId(testID.archivePanelBanner)).toContainText(
      `只看血脂 · ${lipid.report_count} 份`,
    );
    await expect(screen.getByTestId(testID.reportCard(seed.lft06.report_id))).toBeVisible();
    await expect(screen.getByTestId(testID.reportCard(seed.lft08.report_id))).toHaveCount(0);
    await expect(screen.getByTestId(testID.reportCard(seed.lft09.report_id))).toHaveCount(0);

    // 其他:三个不同 report_type 的样本归同一行,来源类别合并展示。
    const source = '其他 · 临检、免疫、其他专项';
    const otherPanels = await listMemberPanels(seed.other.member_id);
    expect(otherPanels.map((panel) => panel.panel)).toEqual(['other']);
    const otherPanel = otherPanels[0];
    expect(otherPanel.report_count).toBe(3);
    await openConfigured(app, browser, '/timeline', { memberId: seed.other.member_id });
    await screen.getByTestId(testID.archiveViewPanel).tap();
    await expect(screen.getByTestId(testID.panelRow('other'), { visible: true })).toContainText(
      source,
    );
    await expect(screen.getByTestId(testID.panelRowMeta('other'))).toContainText(
      `${otherPanel.report_count} 份`,
    );
    await screen.getByTestId(testID.panelRow('other')).tap();
    await browser.waitForURL(/\/panel\/other$/);
    await expect(screen.getByTestId(testID.panelDetailHeading)).toContainText('其他');
    await expect(screen.getByTestId(testID.panelDetailMeta)).toContainText(
      `${otherPanel.report_count} 份报告`,
    );
    await expect(screen.getByTestId(testID.panelSourceLabel)).toContainText(source);
    await screen.getByTestId(testID.panelReportsLink).tap();
    await browser.waitForURL(/\/timeline\?panel=other$/);
    await expect(screen.getByTestId(testID.archivePanelBanner)).toContainText(
      `只看其他 · ${otherPanel.report_count} 份`,
    );
    await expect(screen.getByTestId(testID.reportCard(seed.bloodtype01.report_id))).toBeVisible();
    await expect(screen.getByTestId(testID.reportCard(seed.fibrosis01.report_id))).toBeVisible();
    await expect(screen.getByTestId(testID.reportCard(seed.unknown01.report_id))).toBeVisible();
    await expect(screen.getByTestId(testID.reportCard(seed.coag01.report_id))).toHaveCount(0);

    // 凝血 / 感染筛查:独立项目行与详情。
    const coagPanels = await listMemberPanels(seed.coag.member_id);
    expect(coagPanels.map((panel) => panel.panel)).toEqual(['coag']);
    await openConfigured(app, browser, '/timeline', { memberId: seed.coag.member_id });
    await screen.getByTestId(testID.archiveViewPanel).tap();
    await screen.getByTestId(testID.panelRow('coag'), { visible: true }).tap();
    await browser.waitForURL(/\/panel\/coag$/);
    await expect(screen.getByTestId(testID.panelDetailHeading)).toContainText('凝血');
    await expect(screen.getByTestId(testID.panelDetailMeta)).toContainText(
      `${coagPanels[0].report_count} 份报告`,
    );

    const infectionPanels = await listMemberPanels(seed.infection.member_id);
    expect(infectionPanels.map((panel) => panel.panel)).toEqual(['infection']);
    await openConfigured(app, browser, '/timeline', { memberId: seed.infection.member_id });
    await screen.getByTestId(testID.archiveViewPanel).tap();
    await expect(screen.getByTestId(testID.panelRow('infection'), { visible: true })).toBeVisible();
  });
});
