// 异常指标页数据挂载(panel-browse 票 04):只对 abnormal_count > 0 的项目取详情,
// 行数据直接复用 PanelItem.latest / points;失败整页 error,不做逐行局部错误。
// 不缓存全局;useMemberPanels 聚焦/切换成员后 panels 换新,本 hook 随之重载。
import { useEffect, useState } from 'react';

import { getPanelDetail, type PanelSummary } from '../api/reports';
import { resolveApiClient } from '../api/resolveClient';
import { abnormalPanelGroups, type AbnormalPanelGroup } from './panelOverview';

type LoadedState = Exclude<AbnormalPanelGroupsState, { status: 'idle' }>;

export type AbnormalPanelGroupsState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'unconfigured' }
  | { status: 'error'; detail: string }
  | { status: 'ready'; groups: AbnormalPanelGroup[] };

type Loaded = {
  memberId: number;
  panels: PanelSummary[];
  state: LoadedState;
};

function hasAbnormalItems(panel: PanelSummary): boolean {
  return panel.abnormal_count > 0;
}

export function useAbnormalPanelGroups(
  memberId: number | undefined,
  panels: PanelSummary[] | undefined,
): AbnormalPanelGroupsState {
  const [loaded, setLoaded] = useState<Loaded>();

  useEffect(() => {
    if (memberId === undefined || panels === undefined) {
      return;
    }
    const targets = panels.filter(hasAbnormalItems);
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
        setLoaded({ memberId, panels, state: { status: 'unconfigured' } });
        return;
      }
      try {
        const details = await Promise.all(
          targets.map((panel) => getPanelDetail(resolved.client, memberId, panel.panel)),
        );
        if (!active) {
          return;
        }
        setLoaded({
          memberId,
          panels,
          state: { status: 'ready', groups: abnormalPanelGroups(details) },
        });
      } catch (error) {
        if (active) {
          setLoaded({
            memberId,
            panels,
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
  }, [memberId, panels]);

  if (memberId === undefined || panels === undefined) {
    return { status: 'idle' };
  }
  if (!panels.some(hasAbnormalItems)) {
    return { status: 'ready', groups: [] };
  }
  if (!loaded || loaded.memberId !== memberId || loaded.panels !== panels) {
    return { status: 'loading' };
  }
  return loaded.state;
}
