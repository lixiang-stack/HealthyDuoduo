// 项目详情加载(panel-browse 票 03):按当前成员 + 项目组取全部指标与跨报告点;
// 聚焦重载,四态由页面呈现。未选成员 / 非法 code 不请求(返回 idle)。
import { useFocusEffect } from 'expo-router';
import { useCallback, useRef, useState } from 'react';

import { listMembers } from '../api/members';
import { getPanelDetail, listReports, type PanelDetail } from '../api/reports';
import { resolveApiClient } from '../api/resolveClient';
import { latestFailedReportAfter } from './panelOverview';
import type { PanelCode } from './panels';

export type PanelDetailState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'unconfigured' }
  | { status: 'error'; detail: string }
  | {
      status: 'ready';
      detail: PanelDetail;
      memberName?: string;
      /** report_id → report_type,用于趋势层按最新点来源标注(不猜项目组为类别)。 */
      reportTypeById: Record<string, string>;
      /** 成员最新一份报告为 failed 时,头部提示「另有更新的一份识别失败报告」。 */
      latestFailedReportId?: string;
    };

export function usePanelDetail(
  memberId: number | undefined,
  panel: PanelCode | undefined,
): { state: PanelDetailState; reload: () => Promise<void> } {
  const [state, setState] = useState<PanelDetailState>({ status: 'idle' });
  const requestId = useRef(0);

  const reload = useCallback(async () => {
    if (memberId === undefined || panel === undefined) {
      setState({ status: 'idle' });
      return;
    }
    const request = ++requestId.current;
    const resolved = await resolveApiClient();
    if (request !== requestId.current) {
      return;
    }
    if (!resolved) {
      setState({ status: 'unconfigured' });
      return;
    }
    try {
      const [detail, members, reports] = await Promise.all([
        getPanelDetail(resolved.client, memberId, panel),
        // 成员名 / 最新失败报告只是头部补充;取不到不影响项目详情主链路。
        listMembers(resolved.client).catch(() => []),
        listReports(resolved.client, memberId).catch(() => []),
      ]);
      if (request !== requestId.current) {
        return;
      }
      const memberName = members.find((member) => member.member_id === memberId)?.name;
      const latestFailed = latestFailedReportAfter(
        reports,
        detail.latest_report_date,
        detail.latest_report_id,
      );
      setState({
        status: 'ready',
        detail,
        memberName,
        reportTypeById: Object.fromEntries(
          reports.map((report) => [report.report_id, report.report_type]),
        ),
        latestFailedReportId: latestFailed?.report_id,
      });
    } catch (error) {
      if (request !== requestId.current) {
        return;
      }
      setState({ status: 'error', detail: error instanceof Error ? error.message : String(error) });
    }
  }, [memberId, panel]);

  useFocusEffect(
    useCallback(() => {
      void reload();
    }, [reload]),
  );

  return { state, reload };
}
