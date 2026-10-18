// 报告自愈纯逻辑(票 10):改归属响应的就地合并与「点选目标成员是否需要改派」判定。
// 不含 UI/网络(管理卡与数据挂载消费);映射口径可直测(spec:客户端纯逻辑直测)。
import type { ReportDetail, ReportSummary } from '../api/reports';

/** 改归属响应(报告头部)→ 详情就地合并:只并归属(改派不动报告内容,其余字段保持本地现值)。 */
export function applyReassignResult(report: ReportDetail, summary: ReportSummary): ReportDetail {
  return { ...report, member_id: summary.member_id };
}

/** 点选目标成员是否需要改派:当前归属相同 = 保持原样(不产生请求);无主(null)必改派。 */
export function needsReassign(currentMemberId: number | null, targetMemberId: number): boolean {
  return currentMemberId !== targetMemberId;
}

/** 改派确认行文案:显式给出 from→to(无主报告只给目标侧;成员名是身份标注,确认即自校验)。 */
export function reassignConfirmText(fromName: string | null, toName: string): string {
  if (fromName === null) {
    return `把该报告改派给「${toName}」?`;
  }
  return `把报告从「${fromName}」改派给「${toName}」?`;
}
