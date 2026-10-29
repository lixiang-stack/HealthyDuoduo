import type { ReportItem, ReportSummary, TrendPoint } from '../api/reports';

import cbc05Json from '../../../../samples/expected/report/cbc_05.json';
import cbc07Json from '../../../../samples/expected/report/cbc_07.json';
import lft02Json from '../../../../samples/expected/report/lft_02.json';
import lft04Json from '../../../../samples/expected/report/lft_04.json';
import lft05Json from '../../../../samples/expected/report/lft_05.json';
import lft06Json from '../../../../samples/expected/report/lft_06.json';
import {
  chartLayout,
  defaultSelectedIndex,
  formatSeriesMeta,
  isAbnormalFlag,
  isFilledPoint,
  selectSeries,
  trendPointRows,
  trendUnavailableReason,
} from './trends';

// 测试数据直接来自真实 golden(samples/expected/report),不凭空造数值;
// 只补 HTTP 侧字段(report_id / item_id / review_status 等)。
type GoldenItem = {
  name: string;
  panel: ReportItem['panel'];
  value: string | null;
  unit: string | null;
  ref_range: string | null;
  method: string | null;
  flag: ReportItem['flag'];
  low_confidence: boolean;
  raw_text: string;
  ocr_line_numbers: number[];
};

type GoldenReport = {
  report_type: string;
  report_date: string | null;
  status: ReportSummary['status'];
  panels: ReportSummary['panels'];
  items: GoldenItem[];
};

const golden = (json: unknown) => json as GoldenReport;

const lft05 = golden(lft05Json);
const lft02 = golden(lft02Json);
const lft04 = golden(lft04Json);
const lft06 = golden(lft06Json);
const cbc05 = golden(cbc05Json);
const cbc07 = golden(cbc07Json);

function toReportItem(item: GoldenItem, itemId: number): ReportItem {
  return {
    item_id: itemId,
    name: item.name,
    panel: item.panel,
    value: item.value,
    unit: item.unit,
    ref_range: item.ref_range,
    method: item.method,
    flag: item.flag,
    low_confidence: item.low_confidence,
    raw_text: item.raw_text,
    review_status: 'unreviewed',
    ocr_lines: [],
    prev: null,
  };
}

function itemOf(file: GoldenReport, name: string): ReportItem {
  const index = file.items.findIndex((item) => item.name === name);
  if (index < 0) {
    throw new Error(`golden 缺少指标项:${name}`);
  }
  return toReportItem(file.items[index], index + 1);
}

function pointOf(
  file: GoldenReport,
  name: string,
  reportId: string,
  overrides: Partial<TrendPoint> = {},
): TrendPoint {
  const item = itemOf(file, name);
  return {
    report_id: reportId,
    report_date: file.report_date,
    value: item.value,
    unit: item.unit,
    ref_range: item.ref_range,
    method: item.method,
    flag: item.flag,
    low_confidence: item.low_confidence,
    reviewed: false,
    ...overrides,
  };
}

function latestOf(
  file: GoldenReport,
  name: string,
): { reportDate: string | null; value: string | null } {
  return { reportDate: file.report_date, value: itemOf(file, name).value };
}

const altLow = () => pointOf(lft05, '丙氨酸转氨酶', 'lft_05', { reviewed: false });
const altHigh = () => pointOf(lft02, '丙氨酸转氨酶', 'lft_02', { reviewed: true });

describe('isAbnormalFlag', () => {
  it('只认 high / low / abnormal', () => {
    expect(isAbnormalFlag('high')).toBe(true);
    expect(isAbnormalFlag('low')).toBe(true);
    expect(isAbnormalFlag('abnormal')).toBe(true);
    expect(isAbnormalFlag('normal')).toBe(false);
    expect(isAbnormalFlag('unknown')).toBe(false);
  });
});

describe('selectSeries', () => {
  it('同单位过滤,日期升序,dateless 沉底;跨单位只计数不混入', () => {
    const points = [
      altHigh(),
      pointOf(lft06, '丙氨酸转氨酶', 'lft_06'),
      altLow(),
    ];
    const series = selectSeries(points, 'U/L', 'lft_02');

    expect(series.listPoints.map((point) => point.report_id)).toEqual(['lft_05', 'lft_02']);
    expect(series.chartPoints.map((point) => point.report_id)).toEqual(['lft_05', 'lft_02']);
    expect(series.otherUnitCount).toBe(1);
    expect(series.totalCount).toBe(3);
    expect(series.selectedPoint?.report_id).toBe('lft_02');
  });

  it('null / 空单位归一为同一序列,并计入其他单位份数', () => {
    const points = [
      pointOf(cbc05, '单核细胞数', 'cbc_05', { unit: null }),
      pointOf(cbc05, '单核细胞数', 'cbc_05_empty', { unit: '', report_date: '2019-09-01' }),
      pointOf(cbc07, '单核细胞数', 'cbc_07'),
    ];
    const series = selectSeries(points, null, 'cbc_05');

    expect(series.listPoints.map((point) => point.report_id)).toEqual(['cbc_05', 'cbc_05_empty']);
    expect(series.unit).toBeNull();
    expect(series.otherUnitCount).toBe(1);
  });

  it('按 report_id 去重,保留排序后的第一份', () => {
    const points = [
      pointOf(lft02, '丙氨酸转氨酶', 'same', { report_date: '2021-08-02', value: '835' }),
      pointOf(lft05, '丙氨酸转氨酶', 'same', { report_date: '2021-08-02', value: '27.40' }),
    ];
    const series = selectSeries(points, 'U/L', 'same');
    expect(series.listPoints).toHaveLength(1);
    expect(series.listPoints[0].value).toBe('835');
    expect(series.totalCount).toBe(2);
  });

  it('缺日期计入 undatedCount 并沉底,仍留在 listPoints 与逐点数据里', () => {
    const dateless = pointOf(lft02, '丙氨酸转氨酶', 'dateless', { report_date: null });
    const series = selectSeries([altLow(), dateless, altHigh()], 'U/L', 'lft_02');

    expect(series.listPoints.map((point) => point.report_date)).toEqual([
      '2017-11-18',
      '2021-08-02',
      null,
    ]);
    expect(series.undatedCount).toBe(1);
    expect(series.chartPoints).toHaveLength(2);
  });

  it('区间 / 不等式 / 文本 / null 不进入 chartPoints,计入非数值', () => {
    const points = [
      pointOf(lft02, '丙氨酸转氨酶', 'scalar', { report_date: '2021-08-02' }),
      pointOf(lft05, '丙氨酸转氨酶', 'interval', {
        report_date: '2017-11-18',
        value: '70-80',
      }),
      pointOf(cbc07, 'C反应蛋白', 'bound', {
        report_date: '2020-01-01',
        value: '<2.00',
        unit: 'U/L',
      }),
      pointOf(cbc05, '单核细胞数', 'text', {
        report_date: '2019-08-07',
        value: '未见',
        unit: 'U/L',
      }),
      pointOf(cbc05, '单核细胞数', 'null', {
        report_date: '2019-08-08',
        value: null,
        unit: 'U/L',
      }),
    ];
    const series = selectSeries(points, 'U/L', 'scalar');

    expect(series.chartPoints.map((point) => point.report_id)).toEqual(['scalar']);
    expect(series.nonNumericCount).toBe(4);
    expect(series.listPoints).toHaveLength(5);
  });

  it('候选点缺失时回落到最新有日期点', () => {
    const series = selectSeries(
      [altLow(), altHigh(), pointOf(lft04, '丙氨酸转氨酶', 'dateless', { report_date: null })],
      'U/L',
      'missing',
    );
    expect(series.selectedPoint?.report_id).toBe('lft_02');
  });

  it('单点序列:selectedPoint 即该点', () => {
    const point = pointOf(lft04, '总胆红素', 'lft_04');
    const series = selectSeries([point], 'μmol/L', 'lft_04');
    expect(series.listPoints).toHaveLength(1);
    expect(series.chartPoints).toHaveLength(1);
    expect(series.selectedPoint).toBe(point);
  });

  it('改单位后序列分裂:只接同单位点', () => {
    const points = [
      altLow(),
      altHigh(),
      pointOf(lft06, '丙氨酸转氨酶', 'lft_06'),
    ];
    const ul = selectSeries(points, 'U/L', 'lft_02');
    const iu = selectSeries(points, 'IU/L', 'lft_06');

    expect(ul.listPoints.map((point) => point.report_id)).toEqual(['lft_05', 'lft_02']);
    expect(iu.listPoints.map((point) => point.report_id)).toEqual(['lft_06']);
    expect(ul.otherUnitCount).toBe(1);
    expect(iu.otherUnitCount).toBe(2);
  });
});

describe('defaultSelectedIndex', () => {
  it('优先候选报告对应的点', () => {
    const series = selectSeries([altLow(), altHigh()], 'U/L', 'lft_05');
    expect(defaultSelectedIndex(series)).toBe(0);

    const latest = selectSeries([altLow(), altHigh()], 'U/L', 'lft_02');
    expect(defaultSelectedIndex(latest)).toBe(1);
  });

  it('候选点缺失时回落最新点;空序列取 0', () => {
    const dateless = pointOf(lft02, '丙氨酸转氨酶', 'dateless', { report_date: null });
    const series = selectSeries([altLow(), altHigh(), dateless], 'U/L', 'missing');
    expect(defaultSelectedIndex(series)).toBe(1);
    expect(defaultSelectedIndex(selectSeries([], 'U/L', 'missing'))).toBe(0);
  });

  it('候选点是缺日期项时仍选它', () => {
    const dateless = pointOf(lft02, '丙氨酸转氨酶', 'dateless', { report_date: null });
    const series = selectSeries([altLow(), dateless], 'U/L', 'dateless');
    expect(defaultSelectedIndex(series)).toBe(1);
  });
});

describe('isFilledPoint', () => {
  it('实心 = reviewed || !low_confidence', () => {
    expect(isFilledPoint({ reviewed: true, low_confidence: true })).toBe(true);
    expect(isFilledPoint({ reviewed: true, low_confidence: false })).toBe(true);
    expect(isFilledPoint({ reviewed: false, low_confidence: false })).toBe(true);
    expect(isFilledPoint({ reviewed: false, low_confidence: true })).toBe(false);
  });
});

describe('chartLayout', () => {
  it('两点按日期横向展开;y 域含参考范围,非负从 0 起', () => {
    const series = selectSeries([altLow(), altHigh()], 'U/L', 'lft_02');
    const layout = chartLayout(series, 320, 64, { axes: false });

    expect(layout.points).toHaveLength(2);
    expect(layout.points[0].x).toBeLessThan(layout.points[1].x);
    expect(layout.points[0].x).toBe(8);
    expect(layout.points[1].x).toBe(312);
    expect(layout.domain.min).toBe(0);
    expect(layout.domain.max).toBeGreaterThan(835);
    expect(layout.line).not.toBe('');
    expect(layout.points[1].latest).toBe(true);
    expect(layout.points[0].latest).toBe(false);
  });

  it('单点居中,不画线;参考带横跨绘图区', () => {
    const series = selectSeries([pointOf(lft04, '总胆红素', 'lft_04')], 'μmol/L', 'lft_04');
    const layout = chartLayout(series, 320, 64, { axes: false });

    expect(layout.points).toHaveLength(1);
    expect(layout.points[0].x).toBe(160);
    expect(layout.line).toBe('');
    expect(layout.bands[0].x).toBe(8);
    expect(layout.bands[0].width).toBe(304);
  });

  it('同日期点微偏移,不重叠', () => {
    const series = selectSeries(
      [
        pointOf(lft02, '丙氨酸转氨酶', 'a'),
        pointOf(lft02, '天冬氨酸转氨酶', 'b'),
      ],
      'U/L',
      'a',
    );
    const layout = chartLayout(series, 320, 64, { axes: false });

    expect(layout.points[0].x).not.toBe(layout.points[1].x);
    expect(layout.points[0].x).toBeLessThan(160);
    expect(layout.points[1].x).toBeGreaterThan(160);
  });

  it('参考带按点分段;无参考的点不画带', () => {
    const series = selectSeries([altLow(), altHigh()], 'U/L', 'lft_02');
    const layout = chartLayout(series, 320, 64, { axes: false });
    expect(layout.bands).toHaveLength(2);
    expect(layout.bands[0].x).toBe(layout.points[0].x);
    expect(layout.bands[0].width).toBeCloseTo(layout.points[1].x - layout.points[0].x, 6);
    expect(layout.bands[0].height).toBeGreaterThan(0);

    const noRef = selectSeries(
      [
        pointOf(lft05, '丙氨酸转氨酶', 'lft_05', { ref_range: null }),
        altHigh(),
      ],
      'U/L',
      'lft_02',
    );
    expect(chartLayout(noRef, 320, 64, { axes: false }).bands).toHaveLength(1);
  });

  it('单侧参考从 y 域底部起;标量参考退化为 1px 基准线', () => {
    const oneSided = selectSeries(
      [
        pointOf(lft05, '总胆红素', 'lft_05'),
        pointOf(lft02, '总胆红素', 'lft_02'),
      ],
      'μmol/L',
      'lft_02',
    );
    const bounded = chartLayout(oneSided, 320, 64, { axes: false });
    expect(bounded.bands).toHaveLength(2);
    expect(bounded.bands[1].y + bounded.bands[1].height).toBeCloseTo(56, 6);
    expect(bounded.bands[1].height).toBeGreaterThan(1);

    const scalarRef = selectSeries(
      [pointOf(lft05, '岩藻糖苷酶', 'lft_05', { ref_range: '18.40' })],
      'U/L',
      'lft_05',
    );
    const scalarBand = chartLayout(scalarRef, 320, 64, { axes: false }).bands[0];
    expect(scalarBand.height).toBe(1);
  });

  it('实心 = reviewed || !low_confidence;空心只用于未核对低置信', () => {
    const series = selectSeries(
      [
        pointOf(lft05, '丙氨酸转氨酶', 'normal_high_confidence', { reviewed: false }),
        pointOf(lft02, '丙氨酸转氨酶', 'hollow', {
          reviewed: false,
          low_confidence: true,
          report_date: '2020-01-01',
        }),
        pointOf(lft02, '丙氨酸转氨酶', 'reviewed', {
          reviewed: true,
          low_confidence: true,
          report_date: '2021-08-02',
        }),
      ],
      'U/L',
      'reviewed',
    );
    const layout = chartLayout(series, 320, 64, { axes: false });

    expect(layout.points.map((point) => point.filled)).toEqual([true, false, true]);
  });

  it('无可绘制点返回空几何', () => {
    const series = selectSeries(
      [pointOf(cbc07, 'C反应蛋白', 'bound', { value: '<2.00', unit: 'U/L' })],
      'U/L',
      'bound',
    );
    expect(chartLayout(series, 320, 64, { axes: false })).toEqual({
      points: [],
      bands: [],
      line: '',
      yTicks: [],
      xLabels: [],
      plot: { left: 0, right: 0, top: 0, bottom: 0 },
      domain: { min: 0, max: 0 },
    });
  });
});

describe('chartLayout detail mode', () => {
  it('y 轴取整为 0/500/1000,给出 3 个刻度与首尾日期标签', () => {
    const series = selectSeries([altLow(), altHigh()], 'U/L', 'lft_02');
    const layout = chartLayout(series, 320, 180, { axes: true });

    expect(layout.yTicks.map((tick) => tick.value)).toEqual([0, 500, 1000]);
    expect(layout.yTicks.map((tick) => tick.label)).toEqual(['0', '500', '1000']);
    expect(layout.yTicks[0].y).toBeGreaterThan(layout.yTicks[1].y);
    expect(layout.yTicks[1].y).toBeGreaterThan(layout.yTicks[2].y);
    expect(layout.plot).toEqual({ left: 38, right: 310, top: 10, bottom: 156 });
    expect(layout.xLabels).toEqual([
      { x: 38, label: '2017-11-18', anchor: 'start' },
      { x: 310, label: '2021-08-02', anchor: 'end' },
    ]);
  });

  it('单点居中,只给一个居中日期标签', () => {
    const series = selectSeries([pointOf(lft04, '总胆红素', 'lft_04')], 'μmol/L', 'lft_04');
    const layout = chartLayout(series, 320, 180, { axes: true });

    expect(layout.points[0].x).toBe(174);
    expect(layout.xLabels).toEqual([{ x: 174, label: '2011-07-02', anchor: 'middle' }]);
    expect(layout.yTicks).toHaveLength(3);
  });

  it('同日期点微偏移', () => {
    const series = selectSeries(
      [pointOf(lft02, '丙氨酸转氨酶', 'a'), pointOf(lft02, '天冬氨酸转氨酶', 'b')],
      'U/L',
      'a',
    );
    const layout = chartLayout(series, 320, 180, { axes: true });

    expect(layout.points[0].x).not.toBe(layout.points[1].x);
    expect(layout.points[0].x).toBeLessThan(174);
    expect(layout.points[1].x).toBeGreaterThan(174);
    expect(layout.xLabels).toHaveLength(1);
  });

  it('selectedIndex 放大选中点;缺省放大最新点', () => {
    const series = selectSeries([altLow(), altHigh()], 'U/L', 'lft_02');
    const first = chartLayout(series, 320, 180, { axes: true, selectedIndex: 0 });
    expect(first.points.map((point) => point.selected)).toEqual([true, false]);

    const fallback = chartLayout(series, 320, 180, { axes: true });
    expect(fallback.points.map((point) => point.selected)).toEqual([false, true]);

    const none = chartLayout(series, 320, 180, { axes: true, selectedIndex: null });
    expect(none.points.some((point) => point.selected)).toBe(false);
  });

  it('详情模式同样按点分段参考带;单侧从绘图区底部起,标量参考退化为 1px', () => {
    const oneSided = selectSeries(
      [pointOf(lft05, '总胆红素', 'lft_05'), pointOf(lft02, '总胆红素', 'lft_02')],
      'μmol/L',
      'lft_02',
    );
    const bounded = chartLayout(oneSided, 320, 180, { axes: true });
    expect(bounded.bands).toHaveLength(2);
    expect(bounded.bands[1].y + bounded.bands[1].height).toBeCloseTo(156, 6);

    const noRef = selectSeries(
      [pointOf(lft05, '丙氨酸转氨酶', 'lft_05', { ref_range: null }), altHigh()],
      'U/L',
      'lft_02',
    );
    expect(chartLayout(noRef, 320, 180, { axes: true }).bands).toHaveLength(1);

    const scalarRef = selectSeries(
      [pointOf(lft05, '岩藻糖苷酶', 'lft_05', { ref_range: '18.40' })],
      'U/L',
      'lft_05',
    );
    expect(chartLayout(scalarRef, 320, 180, { axes: true }).bands[0].height).toBe(1);
  });

  it('实心 / 空心与迷你图同口径', () => {
    const series = selectSeries(
      [
        pointOf(lft05, '丙氨酸转氨酶', 'normal_high_confidence', { reviewed: false }),
        pointOf(lft02, '丙氨酸转氨酶', 'hollow', {
          reviewed: false,
          low_confidence: true,
          report_date: '2020-01-01',
        }),
        pointOf(lft02, '丙氨酸转氨酶', 'reviewed', {
          reviewed: true,
          low_confidence: true,
          report_date: '2021-08-02',
        }),
      ],
      'U/L',
      'reviewed',
    );
    expect(chartLayout(series, 320, 180, { axes: true }).points.map((point) => point.filled)).toEqual(
      [true, false, true],
    );
  });
});

describe('trendUnavailableReason', () => {
  const altLatest = latestOf(lft02, '丙氨酸转氨酶');

  it('有可绘制点时不返回原因', () => {
    const series = selectSeries([altLow(), altHigh()], 'U/L', 'lft_02');
    expect(trendUnavailableReason(series, altLatest)).toBeUndefined();
  });

  it('最近一次为非数值结果', () => {
    const series = selectSeries(
      [pointOf(cbc07, 'C反应蛋白', 'cbc_07', { unit: 'U/L' })],
      'U/L',
      'cbc_07',
    );
    expect(trendUnavailableReason(series, latestOf(cbc07, 'C反应蛋白'))).toBe(
      '最近一次为非数值结果',
    );
  });

  it('最近一次缺日期', () => {
    const series = selectSeries([], 'U/L', 'lft_02');
    expect(
      trendUnavailableReason(series, { ...altLatest, reportDate: null }),
    ).toBe('最近一次缺日期');
  });

  it('没有可绘制的数值点', () => {
    const series = selectSeries([], 'U/L', 'lft_02');
    expect(trendUnavailableReason(series, altLatest)).toBe('没有可绘制的数值点');
  });
});

describe('formatSeriesMeta', () => {
  const latest = latestOf(lft02, '丙氨酸转氨酶');

  it('多点:日期范围 + 份数', () => {
    const series = selectSeries([altLow(), altHigh()], 'U/L', 'lft_02');
    expect(formatSeriesMeta(series, latest)).toBe('2017-11-18 – 2021-08-02 · 2 份报告');
  });

  it('单点:日期 + 仅 1 份报告', () => {
    const series = selectSeries(
      [pointOf(lft04, '总胆红素', 'lft_04')],
      'μmol/L',
      'lft_04',
    );
    expect(formatSeriesMeta(series, latestOf(lft04, '总胆红素'))).toBe(
      '2011-07-02 · 仅 1 份报告',
    );
  });

  it('无日期:日期未知 + 份数', () => {
    const series = selectSeries(
      [
        pointOf(lft02, '丙氨酸转氨酶', 'a', { report_date: null }),
        pointOf(lft05, '丙氨酸转氨酶', 'b', { report_date: null }),
      ],
      'U/L',
      'lft_02',
    );
    expect(formatSeriesMeta(series, latest)).toBe('日期未知 · 2 份报告');
  });

  it('缺日期 / 非数值 / 单位不同条件独立追加', () => {
    const series = selectSeries(
      [
        altLow(),
        altHigh(),
        pointOf(lft02, '丙氨酸转氨酶', 'undated', { report_date: null }),
        pointOf(lft02, '丙氨酸转氨酶', 'bad', { value: '<2.00' }),
        pointOf(lft06, '丙氨酸转氨酶', 'other-unit'),
      ],
      'U/L',
      'lft_02',
    );
    expect(formatSeriesMeta(series, latest)).toBe(
      '2017-11-18 – 2021-08-02 · 4 份报告 · 另有 1 份缺日期 · 另有 1 个非数值指标项 · 另有 1 份单位不同（未连线）',
    );
  });

  it('同单位点缺失时候选报告日期兜底,不虚构份数', () => {
    const series = selectSeries([], 'U/L', 'lft_02');
    expect(formatSeriesMeta(series, latest)).toBe('2021-08-02 · 暂无趋势数据');
  });
});

describe('trendPointRows', () => {
  it('同日多份报告:首行显示日期,后续行标「同日」', () => {
    const rows = trendPointRows([
      pointOf(lft05, '丙氨酸转氨酶', 'a'),
      pointOf(lft02, '丙氨酸转氨酶', 'b', { report_date: lft05.report_date }),
      pointOf(lft02, '丙氨酸转氨酶', 'c'),
    ]);

    expect(rows.map((row) => row.dateLabel)).toEqual([
      '2017-11-18',
      '同日',
      '2021-08-02',
    ]);
    expect(rows).toHaveLength(3);
  });

  it('缺日期点只在首行显示「日期未知」,不重复标签', () => {
    const rows = trendPointRows([
      pointOf(lft02, '丙氨酸转氨酶', 'a', { report_date: null }),
      pointOf(lft02, '丙氨酸转氨酶', 'b', { report_date: null }),
    ]);

    expect(rows.map((row) => row.dateLabel)).toEqual(['日期未知', undefined]);
  });

  it('单点保留日期标签', () => {
    const rows = trendPointRows([pointOf(lft04, '总胆红素', 'lft_04')]);
    expect(rows.map((row) => row.dateLabel)).toEqual(['2011-07-02']);
  });
});
