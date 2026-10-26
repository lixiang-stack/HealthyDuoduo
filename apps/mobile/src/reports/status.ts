// 报告状态与徽标文案(纯逻辑,spec 固定映射):success→识别完成、partial→待核对、
// failed→识别失败;检验单 success 含可操作未知项时也按待核对展示(未知项可修正)。
// 影像单的 partial 没有对应核对动作,不显示状态文案。
// 徽标按 flag 聚合计数 + 「待核对 N 项」;影像单不呈现指标项,未知与待核对徽标都关闭。
// 文案中性事实侧,不做解读。
import type { ReportItem, ReportSummary, UploadResult } from '../api/reports';
import { pendingItems } from '../review/items';

import { isImagingReport } from './category';

export type ReportStatus = ReportSummary['status'];
export type Flag = ReportItem['flag'];

const STATUS_LABELS: Record<ReportStatus, string> = {
  success: '识别完成',
  partial: '待核对',
  failed: '识别失败',
};

/** 报告展示状态:检验单 success 但含可操作未知项时按待核对展示;
 *  影像单不因未知项降级(指标项不呈现)。 */
export function reportStatusKind(
  report: Pick<ReportSummary, 'status' | 'report_type' | 'flag_counts'>,
): ReportStatus {
  if (!isImagingReport(report) && report.status === 'success' && report.flag_counts.unknown > 0) {
    return 'partial';
  }
  return report.status;
}

/** 状态文案按报告类别取:检验单 partial 或 success+可操作未知为「待核对」;影像单
 *  partial 无核对动作,不显示。 */
export function reportStatusLabel(
  report: Pick<ReportSummary, 'status' | 'report_type' | 'flag_counts'>,
): string | undefined {
  if (isImagingReport(report) && report.status === 'partial') {
    return undefined;
  }
  return STATUS_LABELS[reportStatusKind(report)];
}

export const DATE_UNKNOWN_LABEL = '日期未知';
const IMAGING_DATE_UNPRINTED_LABEL = '检查单未印日期';

/** 参考范围缺失时的中性事实文案:只陈述数据事实,不推断是纸面未印还是抽取/OCR 漏掉。 */
export const MISSING_REF_RANGE_LABEL = '无参考范围';

/** 参考范围展示文案:有值写「参考 …」,缺失用中性事实文案。 */
export function refRangeLabel(refRange: string | null | undefined): string {
  return refRange ? `参考 ${refRange}` : MISSING_REF_RANGE_LABEL;
}

/** 日期文案按报告类别取:影像单未印日期时明确说「未印」,不暗示识别失败。 */
export function reportDateLabel(
  report: Pick<ReportSummary, 'report_date' | 'report_type'>,
): string {
  if (report.report_date) {
    return report.report_date;
  }
  return isImagingReport(report) ? IMAGING_DATE_UNPRINTED_LABEL : DATE_UNKNOWN_LABEL;
}

const FLAG_LABELS: Record<Flag, string> = {
  normal: '正常',
  high: '偏高',
  low: '偏低',
  abnormal: '异常',
  unknown: '未知',
  not_tested: '未检测',
};

/** 指标项 flag 文案(颜色之外的第二通道:只陈述与参考范围的比较结果)。 */
export function flagLabel(flag: Flag): string {
  return FLAG_LABELS[flag];
}

/** 指标项最终标签:unknown 不一律写「未知」,按 reason 给中性事实文案。 */
export function itemFlagLabel(item: Pick<ReportItem, 'flag' | 'flag_reason'>): string {
  if (item.flag !== 'unknown') {
    return flagLabel(item.flag);
  }
  switch (item.flag_reason) {
    case 'informational':
      return '信息项';
    case 'no_reference':
      return MISSING_REF_RANGE_LABEL;
    case 'qualitative_unsupported':
      return '定性结果';
    case 'unverified':
      return '待核实';
    case 'narrative':
      return '叙述体';
    case 'indeterminate':
      return '不确定';
    default:
      return '未知';
  }
}

/** 异常/信息徽标档位(normal 不计);kind 供 UI 生成稳定 testID,label 为展示文案。 */
export type FlagBadgeKind = Exclude<Flag, 'normal'> | 'not_tested';
export type FlagBadge = { kind: FlagBadgeKind; label: string };

/** 异常徽标(偏高/偏低/异常/可操作未知;normal 不计,为 0 的档不出现)
 *  + 未检测信息徽标。影像单不呈现指标项,未知档按报告类别关闭。 */
export function flagBadges(
  report: Pick<ReportSummary, 'report_type' | 'flag_counts' | 'not_tested_count'>,
): FlagBadge[] {
  const { high, low, abnormal, unknown } = report.flag_counts;
  const badges: FlagBadge[] = [];
  if (high > 0) {
    badges.push({ kind: 'high', label: `偏高 ${high}` });
  }
  if (low > 0) {
    badges.push({ kind: 'low', label: `偏低 ${low}` });
  }
  if (abnormal > 0) {
    badges.push({ kind: 'abnormal', label: `异常 ${abnormal}` });
  }
  if (!isImagingReport(report) && unknown > 0) {
    badges.push({ kind: 'unknown', label: `未知 ${unknown}` });
  }
  if (report.not_tested_count > 0) {
    badges.push({ kind: 'not_tested', label: `未检测 ${report.not_tested_count}` });
  }
  return badges;
}

/** 「待核对 N 项」低置信徽标;N=0 不出徽标。success 的可操作未知项通过
 *  状态文案「待核对」与「未知 N」展示,不混入低置信计数。 */
export function unreviewedBadge(count: number): string | undefined {
  return count > 0 ? `待核对 ${count} 项` : undefined;
}

export type ReportBadgeLabels = { flags: FlagBadge[]; unreviewed?: string };

/** 报告级徽标(时间线/详情共用):影像单不呈现指标项、没有核对入口,
 * 未知与待核对徽标都不显示;其余类别保持原口径。 */
export function reportBadgeLabels(
  report: Pick<
    ReportSummary,
    'report_type' | 'flag_counts' | 'not_tested_count' | 'unreviewed_low_confidence_count'
  >,
): ReportBadgeLabels {
  return {
    flags: flagBadges(report),
    unreviewed: isImagingReport(report)
      ? undefined
      : unreviewedBadge(report.unreviewed_low_confidence_count),
  };
}

/** 上传等待达到此毫秒数后提示核对后端可达性(NF-04 预期 <3s,此为超时提示阈值)。 */
export const SLOW_UPLOAD_HINT_MS = 10_000;

/** 上传等待文案:秒级等待可感知;超时阈值后提示确认后端与识别服务可达。 */
export function uploadWaitText(elapsedMs: number): string {
  return elapsedMs >= SLOW_UPLOAD_HINT_MS
    ? '识别用时较长,请确认后端与识别服务可达(可离开本页,时间线刷新后可见结果)'
    : '正在上传并识别(通常几秒)…';
}

/** 上传结果轻提示(同步响应驱动):重复/成功/待核对/失败各一分岔,failed 展示后端失败原因。
 *  待核对项数与核对页同口径(低置信 + 可操作未知)。 */
export function uploadResultText(result: UploadResult): string {
  if (result.duplicate) {
    return '这张图已经归档过,未重复上传';
  }
  const { status, fail_reason: failReason } = result.report;
  const pending = pendingItems(result.report.items).length;
  switch (status) {
    case 'success':
      return pending > 0 ? `已归档,有 ${pending} 项待核对` : '识别完成,已归档';
    case 'partial':
      return `已归档,有 ${pending} 项待核对`;
    case 'failed':
      return `识别失败:${failReason ?? '未识别出指标项'}(已留痕,可重新拍照)`;
  }
}
