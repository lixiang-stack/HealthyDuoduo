// 异常指标趋势用例:首页入口 → 多点列表 → 详情选点 → 单点成员 → 无异常 → 无报告。
// 独立 seed/成员/lft fixture,与主链路不共享数据(样张按 sha256 全局去重)。
import { beforeAll, describe, test } from '@e2e-dev/web';
import { expect } from 'e2e';

import { openConfigured, seedTrendData, testID, type TrendSeed } from '../support';

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
});
