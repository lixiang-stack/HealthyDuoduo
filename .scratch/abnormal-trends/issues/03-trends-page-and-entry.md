# 03: `/trends` 列表、迷你趋势图与首页入口

**What to build:** App 侧第一条可用纵切：当前成员的异常指标趋势列表。按 `report_type` 分组，每类取最新非 failed 报告、只列 `high/low/abnormal`；每行显示最新值/flag、同单位迷你趋势、报告份数。后端已就绪（票 02）；本票不做详情层，也不渲染 `查看趋势` 入口（票 04 接上，避免 dead link）。

**Blocked by:** 02。

**Status:** ready-for-agent

## 验收清单

### API / 纯逻辑

- [ ] `apps/mobile/src/api/reports.ts` 增加 `TrendPoint` / `TrendResult` 类型与 `getTrend(client, memberId, name, unit?)`；路径参数走生成的 `name`，`unit` 为空时省略 query。本票 hook 不传 `unit`（取全部单位，供 `selectSeries` 过滤/计数）。
- [ ] `apps/mobile/src/reports/trends.ts` 新增纯逻辑：
  - `isAbnormalFlag(flag)` = `high | low | abnormal`。
  - `selectAbnormalCandidates(reports, details)`：先过滤 `status !== 'failed'` → `latestByCategory` → 只保留 `flag_counts.high + low + abnormal > 0` 的类别 → 对应 detail 的异常项 → 按 `name + unit` 去重（保持类别日期序 / item 序）→ `{reportType, reportId, reportDate, item}`；类别用 `report_type`，不硬编码；候选数 = 组头计数，0 候选的组不输出。
  - `selectSeries(points, unit, candidateReportId)`：同单位（null / 空归一）→ 按 `report_id` 去重 → 日期升序 / dateless 沉底 → 返回 `listPoints`、`chartPoints`（dated + 标量）、`undatedCount`、`nonNumericCount`、`otherUnitCount`、`totalCount`、`selectedPoint`（候选 report_id 对应点；找不到时回落最新点）。
  - `chartLayout(series, width, height, {axes: false})`：迷你图几何；x 按日期，单点居中；y 含 ref；非负从 0 起；band 分段；单侧 / 标量 ref 复用 `parseRefRange` / `stripLayout` 口径；`filled = reviewed || !low_confidence`。
  - `formatSeriesMeta(series, candidate)`：`YYYY-MM-DD – YYYY-MM-DD · N 份报告` / `仅 1 份报告` / `日期未知` + 缺日期 / 非数值指标项 / 单位不同追加。
- [ ] `apps/mobile/src/reports/trends.test.ts` 覆盖：flag 过滤、最新非 failed 报告、name+unit 去重、0 候选组、组头计数 = 行数、同单位 / 跨单位 / null 单位 / `otherUnitCount`、report_id 去重、单点、缺日期、非标量、band 分段（含单侧 / 标量）、实心/空心、元信息文案、改名/改单位后序列分裂。测试数据用真实 golden（lft_05 / lft_02 / lft_04 / cbc_05 / cbc_07）。

### 数据加载

- [ ] `apps/mobile/src/reports/useAbnormalTrends.ts`：
  - 输入 `(memberId, reports)`；无成员/无 reports → idle；无报告 → ready + 空 groups。
  - 只对异常类别请求 `getReport`；候选按 `name+unit` 去重后每个键只请求一次 `getTrend`（不带 `unit`）；按类别组装 groups（0 候选的组不输出）。
  - 状态 `idle | loading | unconfigured | error | ready`；页面级错误；`active` flag 防 stale 写入。
  - 不缓存全局；`useMemberReports` 聚焦/切换成员触发重载。
- [ ] 请求失败整页 `error` + 重试；不做逐行局部错误。

### UI

- [ ] `apps/mobile/src/reports/TrendChart.tsx` 的 `TrendSparkline`：`react-native-svg`，高 64，`onLayout` 测宽（默认 320）；只画 dated scalar；参考带分段；实心/空心；无轴；放大最新点；无障碍标签 `{name}趋势，{N} 个数值点`。
- [ ] `apps/mobile/src/app/trends.tsx`：
  - `useCurrentMember` + `useMemberReports` + `useAbnormalTrends`；成员 chips。
  - 分组标题 `{report_type} · {最新报告日期} · {N} 项异常`（N = 实际行数；日期用 `reportDateLabel` 兜底）；类别按最新非 failed 报告日期降序。
  - 行：名称 / 最新值+单位 / `参考 ...` 或 `无参考范围` / flag 文案 / 低置信标签（最新未核对低置信）/ 迷你图或 `暂无可用数值趋势` / 元信息；本票不渲染 `查看趋势 ›`、不做 onPress（票 04 接详情层时再加，避免 dead link）。
  - 七态文案与入口（见 spec：loading / unconfigured / error / 无成员 / 未选成员 / 无报告 / 无异常）。
- [ ] `apps/mobile/src/app/index.tsx` 在「待处理」之后、「各类最近一次」之前加「异常指标趋势」入口（testID `overview-trends`，`Link href="/trends"`）；当前成员有报告时显示。
- [ ] `apps/mobile/src/app/_layout.tsx` 加 `trends` 标题「异常指标趋势」。
- [ ] `FLAG_COLORS` 从 `review/components.tsx` 提到 `reports/components.tsx`（已有 `STATUS_COLORS`）或新建 `reports/colors.ts`；不放进纯逻辑的 `reports/status.ts`；两处共用；不改颜色语义（normal 绿 / high 红 / low 蓝 / abnormal 红 / unknown 灰）。
- [ ] `apps/mobile/e2e/support.ts` 增加计划中的 testID：`overview-trends`、`trend-category-${index}`、`trend-indicator-${index}`、`trend-sparkline-${index}`、`trend-empty-abnormal`、`trend-empty-reports`（详情层 testID 在票 04 加）。

### 验证

- [ ] `npm run typecheck`、`npx expo lint`、`npm test` 全绿。
- [ ] 手工（Expo Web）：首页入口 → 列表分组（含日期）→ 异常项/正常项 → 单点/多点 → 缺日期 / 非数值 → 跨单位不连线 + 中性计数 → failed 最新报告降级 → 切换成员。
- [ ] 不新增除 `react-native-svg` 外的依赖（Expo SDK 57 已包含在 Expo Go，无需 dev build/config plugin）；本票不接详情层、不做完整图。

## 实现提示

- 复用 `reports/overview.ts` 的 `latestByCategory` 与 `useMemberReports`；调用前先过滤 `status !== 'failed'`；不要另写报告排序/类别选择逻辑。
- `ReportSummary` 的 `flag_counts` 用来决定「是否请求详情」；不要为了概览把所有报告详情都拉下来。
- `ReportDetail.items` 的顺序就是报告内 item 序；候选行序保持该顺序；组头计数用候选行数，不用 `flag_counts`。
- `parseNumericForm` 已在 `reports/refRange.ts`，只把 `scalar` 点画进图；`bound / interval / none` 计入非标量；`parseRefRange` / `stripLayout` 已在同文件。
- 迷你图不显示轴/日期标签；元信息负责日期范围。单点画一个点，不画线。
- `TrendChart` 的宽度用 `onLayout` 测；不要在页面里写死屏幕宽度（卡片有 padding/border）。
- 请求预算是「异常类别数 + 唯一 name+unit 数」；在真实成员上观察首屏时间，超过约 2s 再考虑批量端点（本期不做）。
- 文案保持中性：不出现「好转 / 恶化 / 风险 / 预警 / 建议」；跨单位只写「另有 N 份单位不同（未连线）」。
- 详情层本票不做；票 04 接详情层时再加 `查看趋势` 入口；不要提前实现完整图，避免票 03/04 交错。

## Comments

（待实现）
