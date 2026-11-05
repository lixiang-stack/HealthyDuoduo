// 分析结果页展示口径(llm-analysis 票 04,分析结果 UX 方案 A):
// 结果页只回答“这意味着什么、下一步做什么”。事实层只陈述数据与病历原文,
// 不生成医学判断;历史趋势、全部指标与体格发育回到项目页 / 成员页。
import type {
  AnalysisEvidencePoint,
  AnalysisExplanation,
  AnalysisFacts,
  AnalysisSource,
} from '../api/analyses';
import { panelLabel, type PanelCode } from '../reports/panels';
import { flagLabel, refRangeLabel } from '../reports/status';
import { displayItemUnit } from '../review/items';

export const INSUFFICIENT_EVIDENCE_TEXT = '证据不足，请先核对';
export const NO_DEVIATION_TEXT = '本次没有偏离参考范围的指标。';
export const NO_NOTES_TEXT = '本次窗口内没有可引用的病历诊断或处理。';
export const UNCOVERED_DEVIATION_TEXT =
  '「暂无对应解读」表示数据在日期范围内，只是还没有经过审核的解释条目；请结合医生意见。';
export const NOTE_ATTRIBUTION_WARNING =
  '以下是当时病历原文，不是本次分析的建议；请勿按旧处方自行用药。';
export const NOT_DIAGNOSIS_TEXT = '这是解读，不是诊断。';

export function windowText(facts: AnalysisFacts): string {
  const base = `${facts.window.from} 至 ${facts.window.to}（${facts.window.months} 个月）`;
  return facts.window.sparse
    ? `${base}；部分指标历史数据较少，已按指标向前扩展参考，具体见各指标。`
    : base;
}

export function entryNotice(facts: AnalysisFacts): string | undefined {
  if (!facts.expanded_from_indicator) {
    return undefined;
  }
  const name = facts.indicator_name ?? '单个指标';
  return `已从指标「${name}」扩展至项目「${panelLabel(facts.panel as PanelCode)}」`;
}

/** 分析快照生成时间:按设备本地时区展示到分钟;无效/缺失时返回 undefined。 */
export function analysisCreatedAtText(value: string | null | undefined): string | undefined {
  if (!value) {
    return undefined;
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return undefined;
  }
  const pad = (number: number) => String(number).padStart(2, '0');
  return `生成于 ${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(
    date.getHours(),
  )}:${pad(date.getMinutes())}`;
}

export function evidenceValue(point: AnalysisEvidencePoint): string {
  if (point.value === null) {
    return '—';
  }
  const unit = displayItemUnit(point);
  return unit && unit !== '1' ? `${point.value} ${unit}` : point.value;
}

export function evidenceFlag(point: AnalysisEvidencePoint): string {
  return flagLabel(point.flag as Parameters<typeof flagLabel>[0]);
}

export function evidenceRef(point: AnalysisEvidencePoint): string {
  return refRangeLabel(point.ref_range);
}

/** 解释层的 evidence_ids 与偏离项求交集,得到本次解读覆盖的异常项。 */
export function deviationCoverage(
  explanation: AnalysisExplanation | undefined,
  deviations: AnalysisEvidencePoint[],
): {
  covered: number;
  total: number;
  coveredIds: Set<string>;
  uncoveredNames: string[];
} {
  const coveredIds = new Set<string>();
  if (explanation) {
    for (const section of explanation.sections) {
      for (const id of section.evidence_ids ?? []) {
        coveredIds.add(id);
      }
    }
  }
  const uncovered = deviations.filter((point) => !coveredIds.has(point.evidence_id));
  return {
    covered: deviations.length - uncovered.length,
    total: deviations.length,
    coveredIds,
    uncoveredNames: uncovered.map((point) => shortIndicatorName(point.name)),
  };
}

/** 结果页文案里用短名,避免“总蛋白测定（TP）”这类完整检验名占满一行。 */
export function shortIndicatorName(name: string): string {
  return name.replace(/测定$/, '');
}

type IndicatorName = {
  match: RegExp;
  full: string;
  abbreviation: string;
};

const INDICATOR_NAMES: IndicatorName[] = [
  { match: /丙氨酸/, full: '丙氨酸氨基转移酶', abbreviation: 'ALT' },
  { match: /天门冬氨酸|天冬氨酸|谷草转氨酶/, full: '天门冬氨酸氨基转移酶', abbreviation: 'AST' },
  { match: /碱性磷酸酶/, full: '碱性磷酸酶', abbreviation: 'ALP' },
  {
    match: /γ-谷氨酰|谷氨酰转肽酶|谷氨酰基转移酶/,
    full: 'γ-谷氨酰基转移酶',
    abbreviation: 'GGT',
  },
  { match: /总胆红素/, full: '总胆红素', abbreviation: 'TBIL' },
  { match: /直接胆红素/, full: '直接胆红素', abbreviation: 'DBIL' },
  { match: /间接胆红素/, full: '间接胆红素', abbreviation: 'IBIL' },
  { match: /总蛋白/, full: '总蛋白', abbreviation: 'TP' },
  { match: /前白蛋白/, full: '前白蛋白', abbreviation: 'PA' },
  { match: /白蛋白/, full: '白蛋白', abbreviation: 'ALB' },
];

function matchIndicatorName(name: string): IndicatorName | undefined {
  if (name.includes('/')) {
    return undefined;
  }
  return INDICATOR_NAMES.find((item) => item.match.test(name));
}

/** 指标展示名:首次出现给中文全称 + 缩写,避免正文只出现裸 ALT/AST。 */
export function indicatorDisplayName(name: string): string {
  const matched = matchIndicatorName(name);
  if (!matched || name.includes(matched.abbreviation) || name.includes('（')) {
    return name;
  }
  return `${matched.full}（${matched.abbreviation}）`;
}

/** 按页面出现顺序生成指标展示名:同一缩写首次全称、后文统一缩写。 */
export function indicatorLabels(names: string[]): Map<string, string> {
  const labels = new Map<string, string>();
  const seen = new Set<string>();
  for (const name of names) {
    if (labels.has(name)) {
      continue;
    }
    const matched = matchIndicatorName(name);
    if (!matched) {
      labels.set(name, name);
      continue;
    }
    if (seen.has(matched.abbreviation)) {
      labels.set(name, matched.abbreviation);
    } else {
      labels.set(name, `${matched.full}（${matched.abbreviation}）`);
      seen.add(matched.abbreviation);
    }
  }
  return labels;
}

const TRACK_ITEM_NAMES: Record<string, string> = Object.fromEntries(
  INDICATOR_NAMES.map((item) => [item.abbreviation, `${item.full}（${item.abbreviation}）`]),
);

/** 长期跟踪条目映射到展示名,不只显示裸缩写。 */
export function trackItemLabel(item: string): string {
  return TRACK_ITEM_NAMES[item.trim().toUpperCase()] ?? item;
}

/** 把模型 / 框架层固定文案里的内部时间标记与 ULN 换成家长能读懂的语言。 */
export function humanizeClinicalText(text: string): string {
  return text
    .replace(/T-0\s*时?/g, '最近一次')
    .replace(/T-(\d+)m\s*时?/g, '约 $1 个月前')
    .replace(/(\d+(?:\.\d+)?)\s*[×xX]\s*ULN\s*/g, '$1 倍正常上限')
    .replace(/ULN/g, '正常上限')
    .replace(/umol\/l/gi, 'μmol/L');
}

export type SourceDisplay = {
  title: string;
  org: string;
  description: string;
};

const SOURCE_COPY: Record<string, SourceDisplay> = {
  'ACG 2016': {
    title: '转氨酶升高分级',
    org: 'ACG 2016 · 美国胃肠病学会临床指南',
    description:
      '按超过正常上限的倍数分级，2–5 倍为轻度。',
  },
  AASLD: {
    title: 'AST/ALT 比值',
    org: 'AASLD · 美国肝病学会转氨酶升高指南',
    description:
      'AST 除以 ALT 的比值，需结合病史和其他指标理解。',
  },
  'Duvall 2023': {
    title: '总蛋白与白蛋白解读',
    org: 'Duvall 2023 · 检验医学综述',
    description:
      '总蛋白由白蛋白和球蛋白组成；偏低可能与合成、丢失、稀释或炎症有关。',
  },
  'ACG 2017': {
    title: '异常肝功能评估',
    org: 'ACG 2017 · 美国胃肠病学会指南',
    description: '白蛋白由肝脏合成；显著疾病或炎症也会让它降低，单次异常通常需复查。',
  },
  'ASPEN 2021': {
    title: '白蛋白不是营养标志',
    org: 'ASPEN 2021 · 美国肠外肠内营养学会',
    description: '白蛋白更多反映炎症和疾病状态，不能单独判断营养状况。',
  },
  'KDIGO 2021': {
    title: '蛋白尿与肾脏丢失',
    org: 'KDIGO 2021 · 国际肾脏病指南',
    description:
      '肾病综合征需要肾病范围蛋白尿，常伴低白蛋白或水肿。',
  },
  'WS/T 578.1—2017': {
    title: '能量与蛋白质参考摄入量',
    org: 'WS/T 578.1—2017 · 中国卫生行业标准',
    description:
      '按年龄、性别和生理状况给出能量与蛋白质参考摄入量。',
  },
  '中国居民膳食指南（2022）': {
    title: '平衡膳食建议',
    org: '中国营养学会 · 中国居民膳食指南（2022）',
    description:
      '食物多样、规律进餐，适量动物性食物、奶和豆制品。',
  },
  'WHO 2024': {
    title: '血红蛋白贫血切点',
    org: 'WHO 2024 · 世界卫生组织',
    description: '按年龄与性别血红蛋白切点判断贫血；切点标签不是诊断。',
  },
  'WS/T 405 / WS/T 779': {
    title: '血细胞分析参考区间',
    org: 'WS/T 405 / WS/T 779 · 中国卫生行业标准',
    description: '血细胞分析项目的参考区间；用于判断数值是否在参考范围内。',
  },
  'WHO 2007': {
    title: 'BMI 年龄别 z 值',
    org: 'WHO 2007 · 世界卫生组织生长参考',
    description: '5–19 岁 BMI 年龄别参考，用于计算 z 值与百分位。',
  },
  'WS/T 586—2018': {
    title: '儿童 BMI 筛查界值',
    org: 'WS/T 586—2018 · 中国卫生行业标准',
    description: '6–18 岁性别年龄别 BMI 超重 / 肥胖筛查界值。',
  },
};

export function sourceDisplay(source: AnalysisSource): SourceDisplay {
  return (
    SOURCE_COPY[source.standard] ?? {
      title: source.note || source.standard,
      org: source.standard,
      description: source.note || '这条来源用于解释本次指标分级。',
    }
  );
}

/** 来源行的人类可读引用,例如「转氨酶升高分级（ACG 2016）」。 */
export function sourceReferenceText(source: AnalysisSource): string {
  return `${sourceDisplay(source).title}（${source.standard}）`;
}

/** 病历段落的归属标签:明确是「医生当时」的原文,不是本次分析建议。 */
export function noteSectionAttributionLabel(kind: string): string {
  switch (kind) {
    case 'diagnosis':
      return '医生当时诊断（原文）';
    case 'treatment':
      return '医生当时处理（原文，不是建议）';
    default:
      return '医生记录（原文）';
  }
}

const UNCERTAIN_NOTE_WORDS = ['考虑', '可能'];

/** 病历原文含「考虑 / 可能」时加引号,避免读成系统自己的断言。 */
export function quoteNoteText(text: string): string {
  return UNCERTAIN_NOTE_WORDS.some((word) => text.includes(word)) ? `「${text}」` : text;
}

export function noteDate(date: string | null): string {
  return date ?? '日期未知';
}
