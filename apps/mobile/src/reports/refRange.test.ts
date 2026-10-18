import { parseNumericForm, parseRefRange, stripLayout } from './refRange';

// 条带 POC 定稿规则(票 07;真实数据形态取自共享栈 report_items 的 ref_range/value):
// 区间 3-10 / 0.4-8.0 / 3.5--5.5、单侧 <5 / <9.0 / 男:0-15、
// 值标量 10.8 / 区间 70-80 / 带不等号 <2.00 / 文本 未见。

describe('parseNumericForm', () => {
  it('标量:纯数字(含小数、首尾空白)', () => {
    expect(parseNumericForm('10.8')).toEqual({ kind: 'scalar', value: 10.8 });
    expect(parseNumericForm('135')).toEqual({ kind: 'scalar', value: 135 });
    expect(parseNumericForm(' 0.43 ')).toEqual({ kind: 'scalar', value: 0.43 });
  });

  it('区间:短横线/双短横线/波浪号四种连接形', () => {
    expect(parseNumericForm('70-80')).toEqual({ kind: 'interval', low: 70, high: 80 });
    expect(parseNumericForm('3.5--5.5')).toEqual({ kind: 'interval', low: 3.5, high: 5.5 });
    expect(parseNumericForm('0.4~8.0')).toEqual({ kind: 'interval', low: 0.4, high: 8.0 });
    expect(parseNumericForm('0--0.1')).toEqual({ kind: 'interval', low: 0, high: 0.1 });
  });

  it('区间降序(如 80-70)按升序归一,条带不出现反序', () => {
    expect(parseNumericForm('80-70')).toEqual({ kind: 'interval', low: 70, high: 80 });
  });

  it('带不等号:< ≤ > ≥ 四形(容许空格)', () => {
    expect(parseNumericForm('<2.00')).toEqual({ kind: 'bound', op: '<', bound: 2 });
    expect(parseNumericForm('≤5')).toEqual({ kind: 'bound', op: '<=', bound: 5 });
    expect(parseNumericForm('> 3.5')).toEqual({ kind: 'bound', op: '>', bound: 3.5 });
    expect(parseNumericForm('≥10')).toEqual({ kind: 'bound', op: '>=', bound: 10 });
  });

  it('null / 空 / 文本 / 杂格式 → none(退化纯文本,不猜)', () => {
    expect(parseNumericForm(null)).toEqual({ kind: 'none' });
    expect(parseNumericForm(undefined)).toEqual({ kind: 'none' });
    expect(parseNumericForm('')).toEqual({ kind: 'none' });
    expect(parseNumericForm('未见')).toEqual({ kind: 'none' });
    expect(parseNumericForm('10.8x')).toEqual({ kind: 'none' });
    expect(parseNumericForm('18.30↓20.00')).toEqual({ kind: 'none' });
  });
});

describe('parseRefRange', () => {
  it('性别前缀(男:/女:/全角冒号)剥离后解析', () => {
    expect(parseRefRange('男:0-15')).toEqual({ kind: 'interval', low: 0, high: 15 });
    expect(parseRefRange('女：5-10')).toEqual({ kind: 'interval', low: 5, high: 10 });
  });

  it('单侧范围与常规区间、普通文本', () => {
    expect(parseRefRange('<5')).toEqual({ kind: 'bound', op: '<', bound: 5 });
    expect(parseRefRange('<9.0')).toEqual({ kind: 'bound', op: '<', bound: 9 });
    expect(parseRefRange('316-354')).toEqual({ kind: 'interval', low: 316, high: 354 });
    expect(parseRefRange('正常')).toEqual({ kind: 'none' });
    expect(parseRefRange(null)).toEqual({ kind: 'none' });
  });
});

describe('stripLayout', () => {
  it('区间参考 + 标量值:条为参考区间、点为值位置(3-10 / 10.8 偏高 → 点在条右端外)', () => {
    const layout = stripLayout(parseRefRange('3-10'), parseNumericForm('10.8'))!;
    expect(layout.band.start).toBeCloseTo(0.0689655, 6);
    expect(layout.band.end).toBeCloseTo(0.8426172, 6);
    expect(layout.marker).toEqual({ kind: 'dot', at: expect.any(Number), open: false });
    if (layout.marker?.kind !== 'dot') throw new Error('want dot');
    expect(layout.marker.at).toBeCloseTo(0.9310345, 6);
    expect(layout.marker.at).toBeGreaterThan(layout.band.end);
  });

  it('区间参考 + 值居中:点落在中点(0-10 / 5)', () => {
    const layout = stripLayout(parseRefRange('0-10'), parseNumericForm('5'))!;
    expect(layout.band.start).toBeCloseTo(0.0689655, 6);
    expect(layout.band.end).toBeCloseTo(0.9310345, 6);
    if (layout.marker?.kind !== 'dot') throw new Error('want dot');
    expect(layout.marker.at).toBeCloseTo(0.5, 6);
    expect(layout.marker.open).toBe(false);
  });

  it('区间参考 + 值区间:跨度条、不画点(70-80)', () => {
    const layout = stripLayout(parseRefRange('60-90'), parseNumericForm('70-80'))!;
    if (layout.marker?.kind !== 'span') throw new Error('want span');
    expect(layout.marker.start).toBeCloseTo(0.3563218, 6);
    expect(layout.marker.end).toBeCloseTo(0.6436782, 6);
    expect(layout.marker.start).toBeGreaterThan(layout.band.start);
    expect(layout.marker.end).toBeLessThan(layout.band.end);
  });

  it('单侧参考 <5:正常区从左缘画到 5;值 3 落在区内', () => {
    const layout = stripLayout(parseRefRange('<5'), parseNumericForm('3'))!;
    expect(layout.band.start).toBe(0);
    expect(layout.band.end).toBeCloseTo(0.9310345, 6);
    expect(layout.marker).toEqual({ kind: 'dot', at: expect.any(Number), open: false });
    if (layout.marker?.kind !== 'dot') throw new Error('want dot');
    expect(layout.marker.at).toBeCloseTo(0.0689655, 6);
  });

  it('单侧参考 >5:正常区从 5 画到右缘', () => {
    const layout = stripLayout(parseRefRange('>5'), parseNumericForm('6'))!;
    expect(layout.band.end).toBe(1);
    expect(layout.band.start).toBeCloseTo(0.0689655, 6);
    if (layout.marker?.kind !== 'dot') throw new Error('want dot');
    expect(layout.marker.at).toBeCloseTo(0.9310345, 6);
  });

  it('标量参考:细基准线(起止同点),值照常落点', () => {
    const layout = stripLayout(parseRefRange('18.40'), parseNumericForm('18.2'))!;
    expect(layout.band.start).toBe(layout.band.end);
    expect(layout.band.start).toBeCloseTo(0.9310345, 6);
    if (layout.marker?.kind !== 'dot') throw new Error('want dot');
    expect(layout.marker.at).toBeCloseTo(0.0689655, 6);
  });

  it('带不等号的值:<2.00 空心点置于条低端外;>40 对称在高端外', () => {
    const low = stripLayout(parseRefRange('0-30'), parseNumericForm('<2.00'))!;
    if (low.marker?.kind !== 'dot') throw new Error('want dot');
    expect(low.marker.open).toBe(true);
    expect(low.marker.at).toBeCloseTo(low.band.start - 0.03, 6);

    const high = stripLayout(parseRefRange('0-30'), parseNumericForm('>40'))!;
    if (high.marker?.kind !== 'dot') throw new Error('want dot');
    expect(high.marker.open).toBe(true);
    expect(high.marker.at).toBeCloseTo(high.band.end + 0.03, 6);
    expect(high.marker.at).toBeGreaterThan(high.band.end);
  });

  it('值 null/文本:不画标记,条照常', () => {
    const layout = stripLayout(parseRefRange('3-10'), parseNumericForm('未见'))!;
    expect(layout.marker).toBeNull();
    expect(layout.band.start).toBeLessThan(layout.band.end);
  });

  it('参考范围解析失败/null:整条不画(返回 null,退化为纯文本)', () => {
    expect(stripLayout(parseRefRange('正常'), parseNumericForm('10.8'))).toBeNull();
    expect(stripLayout(parseRefRange(null), parseNumericForm('10.8'))).toBeNull();
  });

  it('退化边界:同值区间 / 单侧参考+无值不产生 NaN', () => {
    const line = stripLayout(parseRefRange('5-5'), parseNumericForm('5'))!;
    expect(Number.isFinite(line.band.start)).toBe(true);
    expect(line.band.start).toBe(line.band.end);
    expect(line.band.start).toBeCloseTo(0.5, 6);

    const open = stripLayout(parseRefRange('≥5'), parseNumericForm(null))!;
    expect(open.band.start).toBeCloseTo(0.5, 6);
    expect(open.band.end).toBe(1);
    expect(open.marker).toBeNull();
  });
});
