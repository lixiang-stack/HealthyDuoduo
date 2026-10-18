// 报告屏数据挂载(票 09 起详情页与核对页共用):解析后端客户端 → 按路由报告 id
// 取详情 → 聚焦重载 → 核对响应就地合并(单响应刷新整屏)。只共享数据流,
// 四态(loading/unconfigured/error/ready)呈现各屏自行渲染。
import { useFocusEffect } from 'expo-router';
import { useCallback, useState } from 'react';

import type { ApiClient } from '../api/client';
import { getReport, reportImageUrl, type ReportDetail, type ReviewResult } from '../api/reports';
import { resolveApiClient } from '../api/resolveClient';
import { applyReviewResult } from '../review/items';

export type ReportDataState =
  | { status: 'loading' }
  | { status: 'unconfigured' }
  | { status: 'error'; detail: string }
  | { status: 'ready'; client: ApiClient; report: ReportDetail; imageUrl: string };

export function useReportData(reportId: string | undefined): {
  state: ReportDataState;
  reload: () => Promise<void>;
  applyReview: (result: ReviewResult) => void;
} {
  const [state, setState] = useState<ReportDataState>({ status: 'loading' });

  const reload = useCallback(async () => {
    if (!reportId) {
      setState({ status: 'error', detail: '缺少报告编号' });
      return;
    }
    const resolved = await resolveApiClient();
    if (!resolved) {
      setState({ status: 'unconfigured' });
      return;
    }
    try {
      const report = await getReport(resolved.client, reportId);
      setState({
        status: 'ready',
        client: resolved.client,
        report,
        imageUrl: reportImageUrl(resolved.baseUrl, reportId),
      });
    } catch (error) {
      setState({ status: 'error', detail: error instanceof Error ? error.message : String(error) });
    }
  }, [reportId]);

  useFocusEffect(
    useCallback(() => {
      void reload();
    }, [reload]),
  );

  const applyReview = useCallback((result: ReviewResult) => {
    setState((prev) =>
      prev.status === 'ready' ? { ...prev, report: applyReviewResult(prev.report, result) } : prev,
    );
  }, []);

  return { state, reload, applyReview };
}
