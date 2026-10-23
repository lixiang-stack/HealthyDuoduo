// 核对卡片的纯规则与事实文案(核对状态、unknown 原因、可补录判定):
// 只依赖契约类型与 ref_range 解析,不引入 API/平台模块,App 与 E2E 可共用同一份口径。
import type { components } from '../api/schema';
import { isReversedInterval, parseNumericForm, parseRefRange } from '../reports/refRange';

type ReportItem = components['schemas']['ReportItem'];

/** 指标项核对状态文案(纯事实侧;unreviewed 无标签,显示即「未处理」)。 */
export function reviewStatusLabel(status: ReportItem['review_status']): string | undefined {
  switch (status) {
    case 'confirmed':
      return '已确认';
    case 'corrected':
      return '已修正';
    default:
      return undefined;
  }
}

/** unknown 卡片的可解释文案:说明发生了什么,并给出下一步提示。 */
const FLAG_REASON_TEXT: Record<NonNullable<ReportItem['flag_reason']>, { text: string; hint: string }> = {
  ocr_miss: { text: 'OCR 输出里没有可直接配对的完整值/范围', hint: '可修正或查看原图定位' },
  ocr_merge: { text: 'OCR 把相邻列内容粘在一起', hint: '可修正或查看原图定位' },
  extractor_pairing: { text: '识别到了值/范围，但未归到该项', hint: '可修正或查看原图定位' },
  no_reference: { text: '纸面没有参考范围', hint: '无需处理；可在原图核对' },
  qualitative_unsupported: { text: '定性结果，暂未建立比较规则', hint: '暂不做数值比较' },
  unverified: { text: '暂无原图，尚未核实', hint: '可查看原图定位或重拍' },
  narrative: { text: '叙述体报告，不做指标比较', hint: '无需处理' },
  informational: { text: '信息项，不做正常/异常比较', hint: '无需处理' },
  indeterminate: { text: '结果跨参考边界，暂不能判定', hint: '可查看原图定位或修正' },
  scalar_ref: { text: '参考范围是标量，暂不支持比较', hint: '可修正或查看原图定位' },
  other: { text: '暂无法与参考范围比较', hint: '可查看原图定位' },
};

export function flagReasonText(reason: ReportItem['flag_reason']): string | undefined {
  if (!reason) {
    return undefined;
  }
  const entry = FLAG_REASON_TEXT[reason];
  return entry ? `${entry.text}；${entry.hint}` : undefined;
}

const BACKFILL_REASONS = new Set<NonNullable<ReportItem['flag_reason']>>([
  'ocr_miss',
  'ocr_merge',
  'extractor_pairing',
]);

/** 可补录未知:缺值 + 可解析范围,或数值 + 上下界写反;且原因必须是抽取类。
 *  informational / no_reference / unverified 等不是补录任务。 */
export function isBackfillItem(item: ReportItem): boolean {
  if (item.flag !== 'unknown') {
    return false;
  }
  if (item.flag_reason && !BACKFILL_REASONS.has(item.flag_reason)) {
    return false;
  }
  const ref = parseRefRange(item.ref_range);
  if (item.value === null) {
    return ref.kind === 'interval' || ref.kind === 'bound';
  }
  const value = parseNumericForm(item.value);
  return value.kind === 'scalar' && isReversedInterval(item.ref_range);
}
