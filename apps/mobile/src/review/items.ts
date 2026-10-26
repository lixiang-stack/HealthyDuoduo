// 核对闭环纯逻辑(票 09):触发条件、待核对项、单响应合并与修正草稿 → 请求。
// 不含 UI/网络(组件与页面消费);映射与合并口径可直测(spec:客户端纯逻辑直测)。
import type {
  CorrectionRequest,
  FlagCounts,
  ReportDetail,
  ReportItem,
  ReviewResult,
} from '../api/reports';
import { flagReasonText, isBackfillItem, reviewStatusLabel } from './rules';

export { flagReasonText, isBackfillItem, reviewStatusLabel };

/** 核对触发条件(上传返回分岔与详情入口共用):partial、不可忽略的可操作未知项,
 *  或未核对的低置信项;干净 success(全量高置信、无可操作未知)免核对直落;
 *  failed 无可核对项(展示图像与原因)。 */
export function needsReview(report: Pick<ReportDetail, 'status' | 'items'>): boolean {
  return (
    report.status === 'partial' ||
    report.items.some(
      (item) =>
        (item.low_confidence && item.review_status === 'unreviewed' && item.flag !== 'not_tested') ||
        (item.flag === 'unknown' && isActionableUnknown(item.flag_reason)),
    )
  );
}

/** 待核对项 = 未核对的低置信项 + 可操作未知项(「剩余 N 项」的同一口径;
 *  行序保持报告原序)。可操作未知确认后仍是 unknown,只有修正值/参考范围才消解,
 *  所以即使 review_status=confirmed 也保留。未检测项是已知的“本次未做”,不是核对任务。 */
export function pendingItems(items: ReportItem[]): ReportItem[] {
  return items.filter(
    (item) =>
      item.flag !== 'not_tested' &&
      ((item.review_status === 'unreviewed' && item.low_confidence) ||
        (item.flag === 'unknown' && isActionableUnknown(item.flag_reason))),
  );
}

/** 原图定位可见性:有 OCR 行链接,且(用户展开或正在修正)。低置信项默认展开;
 *  高置信项默认收起,但无需先点「修正」即可展开;无行链接不渲染空视口。 */
export function shouldShowZoom(
  item: Pick<ReportItem, 'low_confidence' | 'ocr_lines'>,
  state: { open?: boolean; editing: boolean },
): boolean {
  return item.ocr_lines.length > 0 && (state.editing || (state.open ?? item.low_confidence));
}

/** flag 聚合徽标计数(与服务端 FlagCounts 同口径:normal 不计,缺档即 0)。
 *  ReviewResult 只带报告级状态/计数与本次指标项,整屏刷新时徽标由客户端按
 *  更新后的全量 items 重算(单响应刷新整屏)。 */
/** 可操作未知 = 抽取/比较失败，需要家长补录或修正；无参考范围/信息项/叙述体/
 *  定性不支持等不需要处理，不计入「未知」与待核对。 */
export function isActionableUnknown(reason: ReportItem['flag_reason']): boolean {
  return (
    reason !== 'no_reference' &&
    reason !== 'qualitative_unsupported' &&
    reason !== 'informational' &&
    reason !== 'narrative'
  );
}

export function countFlagCounts(items: ReportItem[]): FlagCounts {
  const counts: FlagCounts = { high: 0, low: 0, abnormal: 0, unknown: 0 };
  for (const item of items) {
    if (item.flag === 'high') {
      counts.high += 1;
    } else if (item.flag === 'low') {
      counts.low += 1;
    } else if (item.flag === 'abnormal') {
      counts.abnormal += 1;
    } else if (item.flag === 'unknown' && isActionableUnknown(item.flag_reason)) {
      counts.unknown += 1;
    }
  }
  return counts;
}

/** 有值、无参考范围、且不是信息/叙述/待核实项:允许家长确认“纸面没有参考范围”。 */
export function canMarkNoReference(item: ReportItem): boolean {
  return (
    item.flag === 'unknown' &&
    item.value !== null &&
    item.ref_range === null &&
    item.flag_reason !== 'no_reference' &&
    item.flag_reason !== 'informational' &&
    item.flag_reason !== 'narrative' &&
    item.flag_reason !== 'unverified' &&
    item.review_status !== 'confirmed'
  );
}

export type ReviewActions = {
  confirm: boolean;
  primary: 'confirm' | 'backfill' | null;
  correctLabel: '修正' | '再修正' | '补录缺失信息';
  helper?: string;
};

/** 卡片动作:可补录 → 只给补录主操作;可操作未知 → 不给确认(确认不改变 unknown);
 *  低置信非可操作未知且未核对 → 确认;其余 → 修正/再修正。
 *  每卡最多一个 filled primary(票 02 口径)。 */
export function reviewActions(item: ReportItem): ReviewActions {
  if (item.flag === 'not_tested') {
    return { confirm: false, primary: null, correctLabel: '修正' };
  }
  if (isBackfillItem(item)) {
    return {
      confirm: false,
      primary: 'backfill',
      correctLabel: '补录缺失信息',
      helper: '照原图补齐后按值×范围重算。',
    };
  }
  if (
    item.low_confidence &&
    item.review_status === 'unreviewed' &&
    !(item.flag === 'unknown' && isActionableUnknown(item.flag_reason))
  ) {
    return { confirm: true, primary: 'confirm', correctLabel: '修正' };
  }
  return {
    confirm: false,
    primary: null,
    correctLabel: item.review_status === 'corrected' ? '再修正' : '修正',
  };
}

/** 应用单次核对响应到详情状态(单响应刷新整屏):替换本次指标项、更新报告级派生
 *  (status / 未核对计数)、按最新 items 重算 flag 徽标。 */
export function applyReviewResult(report: ReportDetail, result: ReviewResult): ReportDetail {
  const items = report.items.map((item) =>
    item.item_id === result.item.item_id ? result.item : item,
  );
  return {
    ...report,
    status: result.status,
    unreviewed_low_confidence_count: result.unreviewed_low_confidence_count,
    flag_counts: countFlagCounts(items),
    not_tested_count: items.filter((item) => item.flag === 'not_tested').length,
    items,
  };
}

/** 修正草稿 = 四项纸面事实的编辑态(初值取自当前生效值,null 显示为空串)。 */
export type CorrectionDraft = { name: string; value: string; unit: string; ref_range: string };

/** 当前生效值 → 编辑草稿。 */
export function draftFromItem(item: ReportItem): CorrectionDraft {
  return {
    name: item.name,
    value: item.value ?? '',
    unit: item.unit ?? '',
    ref_range: item.ref_range ?? '',
  };
}

/** 草稿 → 修正请求:只带与当前生效值不同的字段(去空白后比较与发送;name 空 → 无效,
 *  四项无差异 → 不产生请求——不给 no-op 修正造修正对)。 */
export function buildCorrection(item: ReportItem, draft: CorrectionDraft): CorrectionRequest | undefined {
  const name = draft.name.trim();
  if (name === '') {
    return undefined;
  }
  const request: CorrectionRequest = { action: 'correct' };
  let changed = false;
  const apply = (field: keyof CorrectionDraft, current: string | null) => {
    const next = draft[field].trim();
    if (next !== (current ?? '')) {
      request[field] = next;
      changed = true;
    }
  };
  apply('name', item.name);
  apply('value', item.value);
  apply('unit', item.unit);
  apply('ref_range', item.ref_range);
  return changed ? request : undefined;
}
