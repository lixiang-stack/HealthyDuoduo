# 04: 异常指标按项目分组

**Spec:** ../spec.md（§8.5）

**What to build:** 「异常指标」页从按 `report_type` 分组改成按项目组分组：只列每个项目最近一次结果里的 high / low / abnormal 指标；点行打开趋势层；无异常 / 无报告 / 错误态与现有页面同口径。

**Blocked by:** 02（报告项目组纵切）、03（按项目浏览）

**Status:** resolved

- [x] 页面按项目分组，组头显示项目 label、最新报告日期、异常项数；不再出现 `report_type` 分组。
- [x] 只列 latest flag 为 high / low / abnormal 的指标；normal / unknown / not_tested 不列。
- [x] 行内显示最新值、单位、参考范围、flag、低置信标签与迷你趋势；点行打开趋势层，使用已加载的点，不额外请求。
- [x] 无异常时显示「这位成员暂时没有异常指标。」+ 查看全部档案；无报告 / 错误 / 未配置 / 未选成员态与现有同口径。
- [x] 现有趋势单位门控、信任标记、趋势详情层行为不回归。
- [x] 趋势页 E2E 更新；`npm test`、`npm run typecheck`、`npx expo lint` 全绿。

## Comments

**实现（2026-10-28，分支 `panel-trends-impl`）**

- 数据源：新增 `useAbnormalPanelGroups(memberId, panels)`；先读 `GET /members/{id}/panels`，只对 `abnormal_count > 0` 的项目并发 `GET /members/{id}/panels/{panel}`。行数据直接取 `PanelItem.latest` / `points`，迷你图与详情层复用 `selectSeries`，点行只读已加载 series，不再逐指标请求 `indicators/{name}/trend`。
- 分组口径：`panelOverview.abnormalPanelGroups` 按项目列表顺序输出（服务端已按最近日期 / §5.3 排序），只保留 latest flag ∈ high/low/abnormal 的指标；没有异常指标的项目不输出空组；组头计数取实际行数。
- 页面：`/trends` 移除 `report_type` 分组与 `useAbnormalTrends`；标题改为「异常指标」；组头为 `项目 label · 最新报告日期 · N 项异常`。详情层 `reportType` 按 `latest.report_id` 从当前成员报告摘要映射，`other` 多来源项目不会用来源集合冒充单点类别；`latest_report_not_tested` 时数值旁标「最近一次未检测」（与项目详情同口径）。
- 复用：项目详情与异常指标页共用的 `PanelTrend` / `panelTrendOf` 收进 `panelOverview.ts`，两页不再各自复制「latest → series / reportType」映射。
- 空态 / 无报告 / 错误 / 未配置 / 未选成员沿用原文案与重试口径；全部 failed / 有报告但无异常都落到「这位成员暂时没有异常指标。」
- 清理：删除 `useAbnormalTrends`、`selectAbnormalCandidates` / `latestAbnormalByCategory` / `trendKey`，以及 App 侧不再使用的 `getTrend` / `TrendResult`；契约端点本身保留。`formatSeriesMeta` / `trendUnavailableReason` 改为结构化最新点参数，避免继续携带已下线的 `TrendCandidate`。
- 单测：`panelOverview.test.ts` 用真实 golden 构造项目详情，覆盖 lft_08 跨项目（肝功 2 项 / 尿酸 1 项，肾功 / 电解质空组不输出）、lft_02（肝功 6 行）、tft_01（全 normal / unknown 返回空）。
- E2E：`trends.e2e.ts` 改为 `肝功 · 2021-08-02 · 6 项异常` 与单点 `肝功 · 2011-07-02 · 3 项异常`，断言 0 异常指标的项目不出组、组内不列正常项；testID `trend-category` → `trend-panel`；`docs/agents/app-e2e.md` 同步口径。
- 验证：`npm test` 20 suites / 191 tests；`npm run typecheck`；`npx expo lint`（0 error，仅既有 `review/items.test.ts:212` warning）；`npm run e2e` 7 files / 21 tests 全绿；`python3 scripts/check-docs.py` 86 files OK。

**已知边界（沿用票 03）：** `PanelItem` 只有规范名，没有 `raw_text` / `review_status`，异常指标行使用规范名；如需与报告详情「原文印名优先」统一，需扩契约。现有趋势 seed 成员各只有 1 个异常项目，多项目分组的顺序 / 计数由 lft_08 单测覆盖；带原图的多异常项目回放留给票 07。
