// 家庭首页数据(票 06):一次取成员列表,再按成员并发取报告摘要与项目列表;
// 首版 N+1 可接受(spec §11),超时再议家庭聚合端点。聚焦重载,四态由页面呈现。
import { useFocusEffect } from 'expo-router';
import { useCallback, useRef, useState } from 'react';

import { listMembers, type Member } from '../api/members';
import {
  listMemberPanels,
  listReports,
  type PanelSummary,
  type ReportSummary,
} from '../api/reports';
import { resolveApiClient } from '../api/resolveClient';
import { sortMembers } from '../members/order';

export type FamilyMemberOverview = {
  member: Member;
  reports: ReportSummary[];
  panels: PanelSummary[];
};

export type FamilyOverviewState =
  | { status: 'loading' }
  | { status: 'unconfigured' }
  | { status: 'error'; detail: string }
  | { status: 'ready'; members: FamilyMemberOverview[] };

export function useFamilyOverview(): {
  state: FamilyOverviewState;
  reload: () => Promise<void>;
} {
  const [state, setState] = useState<FamilyOverviewState>({ status: 'loading' });
  // 聚焦重载可能并发;只允许最后一次启动的请求写状态。
  const requestId = useRef(0);

  const reload = useCallback(async () => {
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
      const members = sortMembers(await listMembers(resolved.client));
      const overviews = await Promise.all(
        members.map(async (member) => {
          const [reports, panels] = await Promise.all([
            listReports(resolved.client, member.member_id),
            listMemberPanels(resolved.client, member.member_id),
          ]);
          return { member, reports, panels };
        }),
      );
      if (request !== requestId.current) {
        return;
      }
      setState({ status: 'ready', members: overviews });
    } catch (error) {
      if (request !== requestId.current) {
        return;
      }
      setState({ status: 'error', detail: error instanceof Error ? error.message : String(error) });
    }
  }, []);

  useFocusEffect(
    useCallback(() => {
      void reload();
    }, [reload]),
  );

  return { state, reload };
}
