// 异常指标趋势数据挂载(票 03):只对异常类别的最新非 failed 报告取详情,
// 再按唯一 name+unit 取趋势点;失败整页 error,不做逐行局部错误。
// 不缓存全局;useMemberReports 聚焦/切换成员后 reports 换新,本 hook 随之重载。
import { useEffect, useState } from 'react';

import { getReport, getTrend, type ReportSummary, type TrendPoint } from '../api/reports';
import { resolveApiClient } from '../api/resolveClient';
import {
  latestAbnormalByCategory,
  selectAbnormalCandidates,
  selectSeries,
  trendKey,
  type TrendCandidate,
  type TrendSeries,
} from './trends';

export type TrendRow = {
  candidate: TrendCandidate;
  series: TrendSeries;
};

export type TrendGroup = {
  reportType: string;
  reportId: string;
  reportDate: string | null;
  rows: TrendRow[];
};

type LoadedState = Exclude<AbnormalTrendsState, { status: 'idle' }>;

export type AbnormalTrendsState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'unconfigured' }
  | { status: 'error'; detail: string }
  | { status: 'ready'; groups: TrendGroup[] };

type Loaded = {
  memberId: number;
  reports: ReportSummary[];
  state: LoadedState;
};

function groupCandidates(
  candidates: TrendCandidate[],
  pointsByKey: Map<string, TrendPoint[]>,
): TrendGroup[] {
  const groups = new Map<string, TrendGroup>();
  for (const candidate of candidates) {
    let group = groups.get(candidate.reportId);
    if (!group) {
      group = {
        reportType: candidate.reportType,
        reportId: candidate.reportId,
        reportDate: candidate.reportDate,
        rows: [],
      };
      groups.set(candidate.reportId, group);
    }
    group.rows.push({
      candidate,
      series: selectSeries(
        pointsByKey.get(trendKey(candidate.item.name, candidate.item.unit)) ?? [],
        candidate.item.unit,
        candidate.reportId,
      ),
    });
  }
  return [...groups.values()];
}

export function useAbnormalTrends(
  memberId: number | undefined,
  reports: ReportSummary[] | undefined,
): AbnormalTrendsState {
  const [loaded, setLoaded] = useState<Loaded>();

  useEffect(() => {
    if (memberId === undefined || reports === undefined) {
      return;
    }
    const targets = latestAbnormalByCategory(reports);
    if (targets.length === 0) {
      return;
    }

    let active = true;
    void (async () => {
      const resolved = await resolveApiClient();
      if (!active) {
        return;
      }
      if (!resolved) {
        setLoaded({ memberId, reports, state: { status: 'unconfigured' } });
        return;
      }
      try {
        const details = await Promise.all(
          targets.map((report) => getReport(resolved.client, report.report_id)),
        );
        const candidates = selectAbnormalCandidates(reports, details);

        const requestNames = new Map<string, string>();
        for (const candidate of candidates) {
          const key = trendKey(candidate.item.name, candidate.item.unit);
          if (!requestNames.has(key)) {
            requestNames.set(key, candidate.item.name);
          }
        }
        const entries = [...requestNames.entries()];
        const trends = await Promise.all(
          entries.map(([, name]) => getTrend(resolved.client, memberId, name)),
        );
        const pointsByKey = new Map(
          entries.map(([key], index) => [key, trends[index].points] as const),
        );

        if (!active) {
          return;
        }
        setLoaded({
          memberId,
          reports,
          state: { status: 'ready', groups: groupCandidates(candidates, pointsByKey) },
        });
      } catch (error) {
        if (active) {
          setLoaded({
            memberId,
            reports,
            state: {
              status: 'error',
              detail: error instanceof Error ? error.message : String(error),
            },
          });
        }
      }
    })();

    return () => {
      active = false;
    };
  }, [memberId, reports]);

  if (memberId === undefined || reports === undefined) {
    return { status: 'idle' };
  }
  if (latestAbnormalByCategory(reports).length === 0) {
    return { status: 'ready', groups: [] };
  }
  if (!loaded || loaded.memberId !== memberId || loaded.reports !== reports) {
    return { status: 'loading' };
  }
  return loaded.state;
}
