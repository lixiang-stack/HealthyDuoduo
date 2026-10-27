// 异常指标趋势用例:首页入口 → 多点列表 → 详情选点 → 单点成员 → 无异常 → 无报告。
// 独立 seed/成员/lft fixture,与主链路不共享数据(样张按 sha256 全局去重)。
import { beforeAll, describe, test } from '@e2e-dev/web';
import { expect } from 'e2e';

import {
  getPanelDetail,
  listMemberPanels,
  openConfigured,
  sampleReportId,
  seedTrendData,
  testID,
  type TrendSeed,
} from '../support';

const SINGLE_REPORT_META = '2011-07-02 · 仅 1 份报告';

let seed!: TrendSeed;

describe('异常指标趋势', { serial: true, timeout: 300_000 }, () => {
  beforeAll(async () => {
    seed = await seedTrendData();
  });

  test('列表、详情、单点、无异常与无报告', { timeout: 300_000 }, async ({
    app,
    browser,
    screen,
  }) => {
    const selectMember = async (memberId: number) => {
      await screen.getByTestId(testID.memberChip(memberId), { visible: true }).tap();
    };

    await openConfigured(app, browser, '/', { memberId: seed.multiPoint.member_id });
    await expect(screen.getByTestId(testID.overviewTrends)).toBeVisible();
    await screen.getByTestId(testID.overviewTrends).tap();
    await browser.waitForURL(/\/trends$/);

    const category = screen.getByTestId(testID.trendCategory(0), { visible: true });
    await expect(category).toContainText('生化 · 2021-08-02 · 6 项异常');
    const altRow = screen.getByTestId(testID.trendIndicator(0), { visible: true });
    await expect(altRow).toContainText('丙氨酸转氨酶');
    await expect(altRow).toContainText('835 U/L');
    await expect(altRow).toContainText('偏高');
    await expect(altRow).toContainText('2 份报告');
    await expect(screen.getByTestId(testID.trendSparkline(0), { visible: true })).toBeVisible();
    await expect(screen.getByTestId(testID.trendIndicator(5), { visible: true })).toBeVisible();
    await expect(screen.getByTestId(testID.trendIndicator(6))).toHaveCount(0);
    await expect(category.getByText('总胆红素')).toHaveCount(0);

    await altRow.tap();
    const detail = screen.getByTestId(testID.trendDetail);
    await expect(detail).toBeVisible();
    await expect(screen.getByTestId(testID.trendDetailChart)).toBeVisible();
    await expect(detail).toContainText('最新：生化 · U/L');
    await expect(detail).toContainText('方法：—');
    await expect(screen.getByTestId(testID.trendPoint(0))).toBeVisible();
    await expect(screen.getByTestId(testID.trendPoint(1))).toBeVisible();
    await expect(screen.getByTestId(testID.trendPoint(2))).toHaveCount(0);
    await expect(screen.getByTestId(testID.trendPoint(0))).toContainText('2017-11-18');
    await expect(screen.getByTestId(testID.trendPoint(1))).toContainText('2021-08-02');

    await screen.getByTestId(testID.trendPoint(0)).tap();
    // 点卡切换后，27.40 / 正常同时出现在点卡与逐点行；835 只剩逐点行。
    await expect(detail.getByText('27.40 U/L')).toHaveCount(2);
    await expect(detail.getByText('正常')).toHaveCount(2);
    await expect(detail.getByText('835 U/L')).toHaveCount(1);

    await screen.getByTestId(testID.trendDetailClose).tap();
    await expect(screen.getByTestId(testID.trendDetail)).toHaveCount(0);
    await expect(screen.getByTestId(testID.trendIndicator(0), { visible: true })).toBeVisible();

    await selectMember(seed.singlePoint.member_id);
    const singleCategory = screen.getByTestId(testID.trendCategory(0), { visible: true });
    await expect(singleCategory).toContainText('生化 · 2011-07-02 · 3 项异常');
    const bilirubinRow = screen.getByTestId(testID.trendIndicator(0), { visible: true });
    await expect(bilirubinRow).toContainText('总胆红素');
    await expect(bilirubinRow).toContainText('41.00 μmol/L');
    await expect(bilirubinRow).toContainText('偏高');
    await expect(bilirubinRow).toContainText(SINGLE_REPORT_META);
    await bilirubinRow.tap();
    const singleDetail = screen.getByTestId(testID.trendDetail);
    await expect(singleDetail).toContainText('最新：生化 · μmol/L');
    await expect(singleDetail).toContainText(SINGLE_REPORT_META);
    await expect(screen.getByTestId(testID.trendPoint(0))).toBeVisible();
    await expect(screen.getByTestId(testID.trendPoint(1))).toHaveCount(0);
    await screen.getByTestId(testID.trendDetailClose).tap();
    await expect(screen.getByTestId(testID.trendDetail)).toHaveCount(0);

    await selectMember(seed.noAbnormal.member_id);
    await expect(screen.getByTestId(testID.trendEmptyAbnormal, { visible: true })).toContainText(
      '这位成员暂时没有异常指标。',
    );

    await selectMember(seed.noReports.member_id);
    await expect(screen.getByTestId(testID.trendEmptyReports, { visible: true })).toContainText(
      '这位成员还没有检查单。',
    );
  });

  // panel-browse 票 03:混合报告在多项目下出现;项目详情与 ?panel= 筛选同源。
  // 复用本文件的 lft_02 / lft_05 fixture,不新增图片(样张按 sha256 全局去重)。
  test('按项目:混合报告、项目详情与 ?panel= 筛选', { timeout: 300_000 }, async ({
    app,
    browser,
    screen,
  }) => {
    const memberId = seed.multiPoint.member_id;
    const panels = await listMemberPanels(memberId);
    expect(panels.map((panel) => panel.panel)).toEqual([
      'liver',
      'kidney',
      'uric_acid',
      'lipid',
    ]);
    const kidney = panels.find((panel) => panel.panel === 'kidney')!;
    const liver = panels.find((panel) => panel.panel === 'liver')!;
    expect(kidney.report_count).toBe(1);
    expect(liver.report_count).toBe(2);

    await openConfigured(app, browser, '/timeline', { memberId });
    await screen.getByTestId(testID.archiveViewPanel).tap();
    for (const panel of panels) {
      await expect(screen.getByTestId(testID.panelRow(panel.panel), { visible: true })).toBeVisible();
    }
    await expect(screen.getByTestId(testID.panelRowMeta('kidney'))).toContainText(
      `${kidney.report_count} 份 · ${kidney.abnormal_count} 项异常 · ${kidney.item_count} 项指标`,
    );

    await screen.getByTestId(testID.panelRow('kidney')).tap();
    await browser.waitForURL(/\/panel\/kidney$/);
    const detail = await getPanelDetail(memberId, 'kidney');
    expect(detail.items.map((item) => item.name)).toEqual([
      '尿素',
      '肌酐',
      'eGFR',
      '血清胱抑素C',
    ]);
    await expect(screen.getByTestId(testID.panelDetailHeading)).toContainText('肾功');
    await expect(screen.getByTestId(testID.panelDetailMeta)).toContainText('1 份报告');
    await expect(screen.getByTestId(testID.panelDetailMeta)).toContainText('0 项异常');
    await expect(screen.getByTestId(testID.panelItem(0))).toContainText('尿素');
    await screen.getByTestId(testID.panelItem(0)).tap();
    await expect(screen.getByTestId(testID.trendDetail)).toBeVisible();
    await screen.getByTestId(testID.trendDetailClose).tap();

    await screen.getByTestId(testID.panelReportsLink).tap();
    await browser.waitForURL(/\/timeline\?panel=kidney$/);
    await expect(screen.getByTestId(testID.archivePanelBanner)).toContainText(
      `只看肾功 · ${kidney.report_count} 份`,
    );
    await expect(screen.getByTestId(testID.reportCard(await sampleReportId('lft_02.jpg')))).toBeVisible();
    await expect(screen.getByTestId(testID.reportCard(await sampleReportId('lft_05.jpeg')))).toHaveCount(0);

    // 有异常的项目:异常项内联迷你趋势,报告份数与项目列表一致。
    await openConfigured(app, browser, '/panel/liver', { memberId });
    await expect(screen.getByTestId(testID.panelDetailMeta)).toContainText(
      `${liver.report_count} 份报告`,
    );
    await expect(screen.getByTestId(testID.panelAbnormalTitle)).toContainText(
      `异常 ${liver.abnormal_count} 项`,
    );
    await expect(screen.getByTestId(testID.panelItemSparkline(0), { visible: true })).toBeVisible();
    await expect(screen.getByTestId(testID.panelReportsLink)).toContainText(
      `${liver.report_count} 份`,
    );
  });
});
