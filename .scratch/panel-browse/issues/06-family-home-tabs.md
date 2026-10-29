# 06: 家庭首页 + 双 Tab + 设置 header + 上传 FAB

**Spec:** ../spec.md（§8.1、§8.2、§8.7）

**What to build:** `/` 变成家庭卡片首页：每张卡回答「谁现在有什么要处理」；家庭 / 档案双 Tab；设置进 header；上传用档案页 FAB。点成员卡 = 选中该成员并进档案；当前成员上下文、深链与现有回跳保持可用。

**Blocked by:** 02（报告项目组纵切）、03（按项目浏览）、04（异常指标按项目分组）

**Status:** resolved

- [x] 家庭卡展示待核对、最近项目 + 日期 / 距今、异常项目、最近报告偏高 / 偏低、报告总数；孩子优先；无报告成员只显示空文案。
- [x] 点卡选中该成员并进档案；档案 chips 切换成员；上传 FAB 只在档案页且当前成员已选时出现。
- [x] 双 Tab「家庭 / 档案」；URL 保持 `/` 与 `/timeline`；设置只在 header；内容区不再有设置链接。
- [x] 家庭页空态：无成员 / 未配置 / 错误 / 成员无报告；档案未选成员提示回家庭。
- [x] 现有深链、报告详情 / 核对回跳、上传归属、改派 / 删除不回归。
- [x] 家庭卡、双 Tab、设置 header、FAB 有 E2E；`docs/agents/app-e2e.md` 更新。
- [x] `npm test`、`npm run typecheck`、`npx expo lint`、`npm run e2e` 全绿；LAN 2–5 成员首屏 <2s 留给票 07 手工验收。

## Comments

**实现（2026-10-28，分支 `family-tabs-impl`，叠在票 04 `panel-trends-impl` 上）**

- 路由：`src/app/index.tsx` / `timeline.tsx` 移入 `src/app/(tabs)/`，新增 `(tabs)/_layout.tsx`（家庭 / 档案双 Tab，每个 Tab 独立 header「设置」入口）；根 `_layout.tsx` 对 `(tabs)` 关闭 Stack header。URL 仍为 `/` 与 `/timeline`；报告 / 核对 / 上传 / 设置 / 成员 / 项目 / 异常指标仍是根 Stack。
- 家庭数据：新增 `useFamilyOverview`，按 `sortMembers`（孩子优先）后并发取每个成员的 `listReports` + `listMemberPanels`；首版 N+1 按 spec §11 保留，聚焦重载，未配置 / 错误 / 重试同现有口径。
- 家庭卡：新增 `familyCardSummary` 纯逻辑，输出待核对（份数 + 项数 + 识别失败份数）、最近项目 / 日期 / 相对日期、异常项目、最近报告偏高偏低、报告总数；无报告只返回「这位成员还没有检查单」，页面隐藏其余行。新增 `relativeDateLabel`（今天 / 昨天 / N 天前 / 周 / 月 / 年前，未来与非法日期不给文案）与 `familyAbnormalLabel` / `totalAbnormalCount`。
- 档案：上传 FAB 只在当前成员已选时出现，位置按 `useBottomTabBarHeight` 上移；未选成员提示「选择一个成员查看其档案。」+「返回家庭」；按时间视图恢复待核对 / 识别失败入口（`pendingSummary` 口径），按项目顶部新增「全部异常 N 项 ›」→ `/trends`（票 04 入口迁移）；内容区移除上传链接与设置链接；回到档案（含从家庭点卡）时 `useFocusEffect` 重置为按时间。
- 清理：旧首页「各类最近一次」删除后，`latestByCategory` 与其测试、`ReportCard.testIDPrefix` 一并删除；首页不再使用 `overview-*` testID。
- E2E：`app.e2e.ts` 重写家庭首页用例（成员卡字段用 fixture 字面量断言、以无报告成员为当前成员再点小明卡验证切换、无报告卡、双 Tab、header 设置、FAB 仅在档案且有当前成员、未选成员回家庭、档案待核对入口）；`trends.e2e.ts` 改从档案「全部异常」入口进入并复用 `totalAbnormalCount`；`app-e2e.md` 同步 22 条用例与 testID。
- 验证：`npm test` 22 suites / 197 tests；`npm run typecheck`；`npx expo lint`（0 error，仅既有 `review/items.test.ts:212` warning）；`npm run e2e` 7 files / 22 tests 全绿；`python3 scripts/check-docs.py` 86 files OK。

**已知边界：** 家庭首页 N+1 请求在 E2E 专用栈上通过；真实 LAN 2–5 成员 / 20–50 份的首屏 <2s 需按票 07 手工验收在目标设备实测。家庭页「无成员 / 未配置」态已实现但未进 E2E：共享栈始终有成员，Web 开发态会从 dev host 推导默认后端地址，无法稳定复现未配置；错误态与无报告态已由 E2E 覆盖。
