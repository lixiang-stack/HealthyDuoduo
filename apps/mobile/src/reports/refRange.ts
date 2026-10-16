// 参考范围 / 当前值 → 条带几何(票 07;spec 5.3「解析逻辑客户端实现一份、集中放置」)。
// 画法 POC 定稿(维护者确认):
//   参考范围:区间 → 画带;单侧 <b / ≤b → 从左缘画到 b(b 为正常区上界;> / ≥ 对称);
//             标量 → 细基准线;解析失败 / null → 不画条(退化为纯文本)。
//   当前值:  标量 → 实心点;带不等号 → 空心点置于条低端外(<)或高端外(>)
//             (spec「点置于条端外」);区间 → 跨度条、不画点(spec「区间无点」);
//             null / 文本 → 不画点。
// 几何口径:坐标轴 = 锚点(参考端点 + 数值值)最小/最大再外扩 8% 边距;锚点同值
// (跨度 0)取 |锚点| 或 1 兜底,保证不产生 NaN。只出几何,颜色语义由调用方按 flag 处理。

export type BoundOp = '<' | '<=' | '>' | '>=';

export type NumericForm =
  | { kind: 'scalar'; value: number }
  | { kind: 'interval'; low: number; high: number }
  | { kind: 'bound'; op: BoundOp; bound: number }
  | { kind: 'none' };

export type StripLayout = {
  /** 参考范围条(归一化 0..1;start === end 表示标量基准线)。 */
  band: { start: number; end: number };
  /** 当前值标记:dot 实心/空心点、span 跨度条、null 不画。 */
  marker:
    | { kind: 'dot'; at: number; open: boolean }
    | { kind: 'span'; start: number; end: number }
    | null;
};

const NUM = String.raw`\d+(?:\.\d+)?`;
const INTERVAL_RE = new RegExp(`^(${NUM})\\s*[-–—~]{1,2}\\s*(${NUM})$`);
const BOUND_RE = new RegExp(`^([<≤>≥])\\s*(${NUM})$`);
const SCALAR_RE = new RegExp(`^(${NUM})$`);
/** 性别条件范围前缀(如 男:0-15);只剥离前缀,显示仍用原文。 */
const SEX_PREFIX_RE = /^[男女][:：]\s*/;

const BOUND_OPS: Record<string, BoundOp> = { '<': '<', '≤': '<=', '>': '>', '≥': '>=' };

/** 数值形态解析(值 / 参考范围共用);非数值文本或 null → none(不猜)。 */
export function parseNumericForm(text: string | null | undefined): NumericForm {
  const s = text?.trim();
  if (!s) {
    return { kind: 'none' };
  }
  const interval = INTERVAL_RE.exec(s);
  if (interval) {
    const a = Number(interval[1]);
    const b = Number(interval[2]);
    return { kind: 'interval', low: Math.min(a, b), high: Math.max(a, b) };
  }
  const bound = BOUND_RE.exec(s);
  if (bound) {
    return { kind: 'bound', op: BOUND_OPS[bound[1]], bound: Number(bound[2]) };
  }
  const scalar = SCALAR_RE.exec(s);
  if (scalar) {
    return { kind: 'scalar', value: Number(scalar[1]) };
  }
  return { kind: 'none' };
}

/** 参考范围解析:剥离性别条件前缀(如 男:0-15)后与数值形态同口径。 */
export function parseRefRange(text: string | null | undefined): NumericForm {
  if (text == null) {
    return { kind: 'none' };
  }
  return parseNumericForm(text.trim().replace(SEX_PREFIX_RE, ''));
}

/** 参考范围条 + 当前值点位的几何;参考范围无数值形态时为 null(退化纯文本)。 */
export function stripLayout(ref: NumericForm, value: NumericForm): StripLayout | null {
  if (ref.kind === 'none') {
    return null;
  }
  const anchors: number[] = [];
  if (ref.kind === 'interval') {
    anchors.push(ref.low, ref.high);
  } else if (ref.kind === 'scalar') {
    anchors.push(ref.value);
  } else {
    anchors.push(ref.bound);
  }
  if (value.kind === 'scalar') {
    anchors.push(value.value);
  } else if (value.kind === 'interval') {
    anchors.push(value.low, value.high);
  } else if (value.kind === 'bound') {
    anchors.push(value.bound);
  }

  const lo = Math.min(...anchors);
  const hi = Math.max(...anchors);
  const span = hi - lo || Math.max(Math.abs(lo), 1);
  const margin = 0.08 * span;
  const min = lo - margin;
  const max = hi + margin;
  const frac = (x: number) => (x - min) / (max - min);

  let band: StripLayout['band'];
  if (ref.kind === 'interval') {
    band = { start: frac(ref.low), end: frac(ref.high) };
  } else if (ref.kind === 'scalar') {
    band = { start: frac(ref.value), end: frac(ref.value) };
  } else if (ref.op === '<' || ref.op === '<=') {
    band = { start: 0, end: frac(ref.bound) };
  } else {
    band = { start: frac(ref.bound), end: 1 };
  }

  let marker: StripLayout['marker'] = null;
  if (value.kind === 'scalar') {
    marker = { kind: 'dot', at: frac(value.value), open: false };
  } else if (value.kind === 'interval') {
    marker = { kind: 'span', start: frac(value.low), end: frac(value.high) };
  } else if (value.kind === 'bound') {
    // 点置于条端外:值带不等号时精确值未知,空心点贴低端/高端外缘表达方向;
    // 单侧参考(条自窗口边缘起)没有外侧空间时夹在边缘(clamp),不越出轨道。
    const outsideGap = 0.03;
    const at =
      value.op === '<' || value.op === '<='
        ? Math.max(0, band.start - outsideGap)
        : Math.min(1, band.end + outsideGap);
    marker = { kind: 'dot', at, open: true };
  }
  return { band, marker };
}
