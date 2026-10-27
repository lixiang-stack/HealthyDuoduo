// 当前成员项目列表加载(panel-browse 票 03):只在「按项目」视图挂载时请求;
// 聚焦 / 切换成员重载,四态由档案页的按项目视图分别呈现。
import { useFocusEffect } from 'expo-router';
import { useCallback, useRef, useState } from 'react';

import { listMemberPanels, type PanelSummary } from '../api/reports';
import { resolveApiClient } from '../api/resolveClient';

export type MemberPanelsState =
  | { status: 'loading' }
  | { status: 'unconfigured' }
  | { status: 'error'; detail: string }
  | { status: 'ready'; panels: PanelSummary[] };

export function useMemberPanels(memberId: number): {
  state: MemberPanelsState;
  reload: () => Promise<void>;
} {
  const [state, setState] = useState<MemberPanelsState>({ status: 'loading' });
  // 切换成员会并发起多次 reload;只允许最后一次启动的请求写状态。
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
      const panels = await listMemberPanels(resolved.client, memberId);
      if (request !== requestId.current) {
        return;
      }
      setState({ status: 'ready', panels });
    } catch (error) {
      if (request !== requestId.current) {
        return;
      }
      setState({ status: 'error', detail: error instanceof Error ? error.message : String(error) });
    }
  }, [memberId]);

  useFocusEffect(
    useCallback(() => {
      void reload();
    }, [reload]),
  );

  return { state, reload };
}
