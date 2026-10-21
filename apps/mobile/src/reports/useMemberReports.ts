// 成员档案加载(概览与档案共用):解析后端 → 取成员 → 取当前成员报告摘要;
// 聚焦重载、失败重试。四态由页面分别呈现;当前成员是否存在随 ready 返回。
import { useFocusEffect } from 'expo-router';
import { useCallback, useState } from 'react';

import { listMembers, type Member } from '../api/members';
import { listReports, type ReportSummary } from '../api/reports';
import { resolveApiClient } from '../api/resolveClient';

export type MemberReportsState =
  | { status: 'loading' }
  | { status: 'unconfigured' }
  | { status: 'error'; detail: string }
  | { status: 'ready'; members: Member[]; reports: ReportSummary[]; hasCurrentMember: boolean };

export function useMemberReports(currentMemberId: number | undefined): {
  state: MemberReportsState;
  reload: () => Promise<void>;
} {
  const [state, setState] = useState<MemberReportsState>({ status: 'loading' });

  const reload = useCallback(async () => {
    const resolved = await resolveApiClient();
    if (!resolved) {
      setState({ status: 'unconfigured' });
      return;
    }
    try {
      const members = await listMembers(resolved.client);
      const selected = members.find((m) => m.member_id === currentMemberId);
      const reports = selected ? await listReports(resolved.client, selected.member_id) : [];
      setState({ status: 'ready', members, reports, hasCurrentMember: selected !== undefined });
    } catch (error) {
      setState({ status: 'error', detail: error instanceof Error ? error.message : String(error) });
    }
  }, [currentMemberId]);

  useFocusEffect(
    useCallback(() => {
      void reload();
    }, [reload]),
  );

  return { state, reload };
}
