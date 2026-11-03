// 分析入口(llm-analysis 票 04):点击分析后先选时间范围(默认 12 个月),
// 首次再展示数据去向确认,之后直接创建分析并跳转。
// 创建后由结果页 GET /analyses/{id} 读取已保存快照;这里只负责入口、范围与确认。
import { router } from 'expo-router';
import { useCallback, useState } from 'react';

import {
  ANALYSIS_DEFAULT_WINDOW_MONTHS,
  createAnalysis,
  type AnalysisEntry,
  type AnalysisWindowMonths,
} from '../api/analyses';
import { resolveApiClient } from '../api/resolveClient';
import type { PanelCode } from '../reports/panels';
import { grantAnalysisConsent, hasAnalysisConsent } from './consent';

type Pending = {
  entry: AnalysisEntry;
  indicatorName?: string;
  reportSHA?: string;
  windowMonths: AnalysisWindowMonths;
};

export function useStartAnalysis(
  memberId: number | undefined,
  panel: PanelCode | undefined,
) {
  const [pending, setPending] = useState<Pending>();
  const [rangeOpen, setRangeOpen] = useState(false);
  const [pendingConsent, setPendingConsent] = useState(false);
  const [starting, setStarting] = useState(false);
  const [error, setError] = useState<string>();

  const create = useCallback(
    async (next: Pending) => {
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
          entry: next.entry,
          ...(next.indicatorName ? { indicator_name: next.indicatorName } : {}),
          ...(next.reportSHA ? { report_sha: next.reportSHA } : {}),
          window_months: next.windowMonths,
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
    [memberId, panel],
  );

  /** 点击分析入口:先打开时间范围选择,不立即请求。 */
  const start = useCallback(
    (entry: AnalysisEntry, indicatorName?: string, reportSHA?: string) => {
      setError(undefined);
      setPending({
        entry,
        indicatorName,
        reportSHA,
        windowMonths: ANALYSIS_DEFAULT_WINDOW_MONTHS,
      });
      setRangeOpen(true);
    },
    [],
  );

  /** 选定时间范围后:已有确认直接分析;首次先弹数据去向确认。 */
  const confirmRange = useCallback(
    async (windowMonths: AnalysisWindowMonths) => {
      if (!pending) {
        return;
      }
      const next = { ...pending, windowMonths };
      setPending(next);
      setRangeOpen(false);
      if (await hasAnalysisConsent()) {
        await create(next);
        return;
      }
      setPendingConsent(true);
    },
    [pending, create],
  );

  const confirmConsent = useCallback(async () => {
    if (!pending) {
      return;
    }
    setPendingConsent(false);
    await grantAnalysisConsent();
    await create(pending);
  }, [pending, create]);

  const cancelRange = useCallback(() => {
    setRangeOpen(false);
    setPending(undefined);
  }, []);

  const cancelConsent = useCallback(() => {
    setPendingConsent(false);
    setPending(undefined);
  }, []);

  return {
    start,
    confirmRange,
    cancelRange,
    rangeOpen,
    pendingConsent,
    confirmConsent,
    cancelConsent,
    starting,
    error,
  };
}
