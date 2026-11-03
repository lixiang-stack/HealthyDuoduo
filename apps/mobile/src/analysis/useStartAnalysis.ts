// 分析入口(llm-analysis 票 04):首次点击先展示数据去向确认,之后直接创建分析并跳转。
// 创建后由结果页 GET /analyses/{id} 读取已保存快照;这里只负责入口与确认。
import { router } from 'expo-router';
import { useCallback, useState } from 'react';

import {
  createAnalysis,
  type AnalysisEntry,
  type AnalysisWindowMonths,
} from '../api/analyses';
import { resolveApiClient } from '../api/resolveClient';
import type { PanelCode } from '../reports/panels';
import { grantAnalysisConsent, hasAnalysisConsent } from './consent';

type Pending = { entry: AnalysisEntry; indicatorName?: string; reportSHA?: string };

export function useStartAnalysis(
  memberId: number | undefined,
  panel: PanelCode | undefined,
  windowMonths: AnalysisWindowMonths,
) {
  const [pending, setPending] = useState<Pending>();
  const [starting, setStarting] = useState(false);
  const [error, setError] = useState<string>();

  const create = useCallback(
    async (entry: AnalysisEntry, indicatorName?: string, reportSHA?: string) => {
      if (memberId === undefined || !panel) {
        setError('请选择成员与项目后再分析');
        return;
      }
      setStarting(true);
      setError(undefined);
      try {
        const resolved = await resolveApiClient();
        if (!resolved) {
          setError('尚未配置后端地址，请先到设置填写');
          return;
        }
        const result = await createAnalysis(resolved.client, {
          member_id: memberId,
          panel,
          entry,
          ...(indicatorName ? { indicator_name: indicatorName } : {}),
          ...(reportSHA ? { report_sha: reportSHA } : {}),
          window_months: windowMonths,
        });
        router.push({
          pathname: '/analysis/[analysis_id]',
          params: { analysis_id: String(result.analysis_id) },
        });
      } catch (caught) {
        setError(caught instanceof Error ? caught.message : String(caught));
      } finally {
        setStarting(false);
      }
    },
    [memberId, panel, windowMonths],
  );

  const start = useCallback(
    async (entry: AnalysisEntry, indicatorName?: string, reportSHA?: string) => {
      if (await hasAnalysisConsent()) {
        await create(entry, indicatorName, reportSHA);
        return;
      }
      setPending({ entry, indicatorName, reportSHA });
    },
    [create],
  );

  const confirmConsent = useCallback(async () => {
    const next = pending;
    setPending(undefined);
    if (!next) {
      return;
    }
    await grantAnalysisConsent();
    await create(next.entry, next.indicatorName, next.reportSHA);
  }, [pending, create]);

  return {
    start,
    confirmConsent,
    cancelConsent: () => setPending(undefined),
    pendingConsent: pending !== undefined,
    starting,
    error,
  };
}
