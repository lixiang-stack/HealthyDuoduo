// 核对闭环纯逻辑(票 09):触发条件、待核对项、单响应合并与修正草稿 → 请求。
// 不含 UI/网络(组件与页面消费);映射与合并口径可直测(spec:客户端纯逻辑直测)。
import type {
  CorrectionRequest,
  FlagCounts,
  ReportDetail,
  ReportItem,
  ReviewResult,
} from '../api/reports';

/** 核对触发条件(上传返回分岔与详情入口共用):partial,或未核对的低置信项
 *  (含低置信项触发);干净 success(全量高置信/低置信项均已确信)免核对直落;
 *  failed 无可核对项(展示图像与原因)。 */
export function needsReview(report: Pick<ReportDetail, 'status' | 'items'>): boolean {
  return (
    report.status === 'partial' ||
    report.items.some((item) => item.low_confidence && item.review_status === 'unreviewed')
  );
}

/** 待核对项 = 未核对的低置信项(「剩余 N 项」的同一口径;行序保持报告原序)。 */
export function pendingItems(items: ReportItem[]): ReportItem[] {
  return items.filter((item) => item.review_status === 'unreviewed' && item.low_confidence);
}

/** 原图定位可见性:有 OCR 行链接,且(用户展开或正在修正)。低置信项默认展开;
 *  高置信项默认收起,但无需先点「修正」即可展开;无行链接不渲染空视口。 */
export function shouldShowZoom(
  item: Pick<ReportItem, 'low_confidence' | 'ocr_lines'>,
  state: { open?: boolean; editing: boolean },
): boolean {
  return item.ocr_lines.length > 0 && (state.editing || (state.open ?? item.low_confidence));
}

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

/** flag 聚合徽标计数(与服务端 FlagCounts 同口径:normal 不计,缺档即 0)。
 *  ReviewResult 只带报告级状态/计数与本次指标项,整屏刷新时徽标由客户端按
 *  更新后的全量 items 重算(单响应刷新整屏)。 */
export function countFlagCounts(items: ReportItem[]): FlagCounts {
  const counts: FlagCounts = { high: 0, low: 0, unknown: 0 };
  for (const item of items) {
    if (item.flag === 'high') {
      counts.high += 1;
    } else if (item.flag === 'low') {
      counts.low += 1;
    } else if (item.flag === 'unknown') {
      counts.unknown += 1;
    }
  }
  return counts;
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
