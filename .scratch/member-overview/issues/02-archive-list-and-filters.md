# 02: `/timeline` 改为「全部档案」，支持待核对 / 识别失败筛选

**What to build:** 家长打开当前成员的全部档案，可以用 `?filter=partial` 只看待核对报告、用 `?filter=failed` 只看识别失败报告；顶部说明当前筛选与份数，一键「清除筛选」回到全部档案；筛选后为空时给出「查看全部档案」出口。页面标题、上传页和核对页的回跳文案、改派成功文案统一从「时间线」改为「档案」。

**Blocked by:** 01（共享加载）。

**Status:** resolved

## 验收清单

- [x] 无 `filter` 或未知值：显示当前成员全部报告，沿用 `sortReports` 的日期降序 / 无日期沉底 / 同日期 `report_id` 升序；不出现筛选说明条。
- [x] `filter` 为数组时按首值解析；首值非法按无筛选处理。
- [x] `filter=partial`：只显示 `status === 'partial'` 且非影像单的报告；顶部 `只看待核对 · N 份` + 「清除筛选」；N 与概览「待核对 N 份」同口径，共用同一判定函数。
- [x] `filter=failed`：只显示 `status === 'failed'` 的报告（含影像单失败）；顶部 `只看识别失败 · N 份` + 「清除筛选」。
- [x] 筛选后为空：`没有符合筛选条件的报告。` + 「查看全部档案」，点击回到未筛选的全部档案；无筛选且无报告时沿用现有空态。
- [x] 成员切换后筛选参数保留，列表随当前成员刷新。
- [x] 页面标题为「全部档案」，页内可见文案不再出现「时间线」旧名（加载中、未选成员）。
- [x] 上传页、核对页的「去时间线查看」改为「去档案查看」，目标仍是 `/timeline`；改派成功文案改为「已移到该成员名下」。
- [x] 删除报告后仍回 `/timeline`，档案空态不崩（回归）。
- [x] 纯函数 `filterReports` 与 `parseReportFilter` 有单测：partial 排除影像单、failed、无筛选、非法值、数组首值。
- [x] E2E：原「时间线按成员过滤报告」改名「档案按成员过滤报告」，断言不变；新增直接打开 `/timeline?filter=partial` / `?filter=failed` 的用例（部分 / 清除 / 空态）；上传与同图重传、改派用例的文案断言同步；10 条全绿。
- [x] `docs/agents/app-e2e.md` 完成判据 9/9 → 10/10。
- [x] `npm run typecheck`、`npm test` 全绿。
- [ ] `npx expo lint` 全绿（阻塞于仓库基线：无 committed ESLint 配置；见 Comments）。

## 实现提示

- 纯逻辑按 spec 统一放 `reports/overview.ts`：本票先落 `filterReports` / `parseReportFilter` 与共享判定（`status === 'partial'` 且非影像单），03 再落 `pendingSummary` / `latestByCategory`；计数与筛选不许各写一套。
- `?filter=` 是 App 内部 URL 参数，不是 API 契约；服务端仍一次返回当前成员全量摘要，本票不做分页或服务端过滤。
- 筛选说明条、清除入口与筛选空态沿用现有页面的样式和错误/空态口径，不新增设计组件。
- 新增 E2E 用例放在上传用例之前（种子里小明仍是 3 份；上传会加第 4 份，核对会清零待核对），这样 spec 的「3 份 / 待核对 1 份」口径成立。
- 新增 locator：`archive-filter-banner`、`archive-filter-clear`、`archive-empty`；`report-card-${id}` 继续只用于档案页与详情头部。

## Comments

**实现（2026-10-21，票 02）**

- 纯逻辑：`reports/overview.ts` 新增 `filterReports` / `parseReportFilter` 与共享判定 `isPendingReviewReport` / `isFailedReport`；档案页用同一份 `visibleReports` 同时驱动列表与说明条计数。
- 界面：`/timeline` 标题改「全部档案」；筛选说明条 + 「清除筛选」；筛选空态 + 「查看全部档案」；加载中/未选成员文案去掉「时间线」；上传/核对回跳改「去档案查看」；改派成功文案去掉「时间线上」。
- 验证：`npm run typecheck` 通过；`npm test -- --runInBand` 14 suites / 88 tests 全绿；`npm run e2e` 10/10 通过；`docs/agents/app-e2e.md` 改为 10/10。

**数据口径修正（影响票 03）**

- spec 的 E2E 前提「小明 cbc_03 待核对 1 份」与实际数据不符：`samples/expected/report/cbc_03.json` 与专用栈均为 `status=success`（6 项中 1 项 low_confidence，未达 partial 阈值）；真实 `partial` 只有妈妈的 `cbc_04`。因此 02 的 `filter=partial` E2E 用妈妈/cbc_04 验证非空子集，小明只验证 `filter=failed` 空态与全量回归。
- 票 03 的概览 E2E 若仍按 spec 断言「小明 待核对 1 份」会红；需改为小明 0 份、妈妈 1 份，或另立票调整 seed / 状态派生。当前不改产品口径（仍按 `status === 'partial'` 且非影像单）。

**Lint 基线（同票 01）**

- `apps/mobile` 没有 committed ESLint 配置；`npx expo lint` 会自动安装并生成配置，随后报 3 个未改动文件的 pre-existing errors（`upload.tsx`、`LineZoom.tsx`、`review/components.tsx`）。本次改动文件无 lint 报错，自动生成的配置与依赖已还原，未新增依赖。

**残留命名**

- `reports/status.ts` 的慢上传提示仍写「时间线刷新后可见结果」；该文件不在 spec 的改动面（11 文件）内，本票未改。`app/index.tsx` 的「报告时间线」由票 03 替换调试页时消失。

**后续修复（2026-10-21，用户实测）**

- 核对页的「去档案查看」原来只在「没有需要核对的低置信项」时出现；有剩余项时只能返回两次。现改为页面底部常驻，确认后一次回到 `/timeline`；E2E 核对用例补了点击回档案断言。
