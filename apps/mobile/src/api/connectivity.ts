// 后端连通状态判定:纯逻辑 seam(无网络、无框架),探测层与 UI 共用。

/** health 判定的最终结果(UI 直接消费)。 */
export type Connectivity =
  | { status: 'ok'; version: string }
  | { status: 'unreachable'; detail: string }
  | { status: 'error'; detail: string };

/** 页面级健康视图状态:探测中的过渡态 + 未配置态 + 判定结果。 */
export type HealthState = { status: 'checking' } | { status: 'unconfigured' } | Connectivity;

/** 一次 health 探测的原始结果(探测层产出,判定层消费)。 */
export type HealthProbe =
  | { kind: 'ok'; version: string }
  | { kind: 'http'; status: number }
  | { kind: 'network'; detail: string };

export function resolveConnectivity(probe: HealthProbe): Connectivity {
  switch (probe.kind) {
    case 'ok':
      return { status: 'ok', version: probe.version };
    case 'http':
      return { status: 'error', detail: `HTTP ${probe.status}` };
    case 'network':
      return { status: 'unreachable', detail: probe.detail };
  }
}
