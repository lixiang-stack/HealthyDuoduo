# 02: `/timeline` 改为「全部档案」，支持待核对 / 识别失败筛选

**What to build:** 家长打开当前成员的全部档案，可以用 `?filter=partial` 只看待核对报告、用 `?filter=failed` 只看识别失败报告；顶部说明当前筛选与份数，一键「清除筛选」回到全部档案；筛选后为空时给出「查看全部档案」出口。页面标题、上传页和核对页的回跳文案、改派成功文案统一从「时间线」改为「档案」。

**Blocked by:** 01（共享加载）。

**Status:** ready-for-agent

## 验收清单

- [ ] 无 `filter` 或未知值：显示当前成员全部报告，沿用 `sortReports` 的日期降序 / 无日期沉底 / 同日期 `report_id` 升序；不出现筛选说明条。
- [ ] `filter` 为数组时按首值解析；首值非法按无筛选处理。
- [ ] `filter=partial`：只显示 `status === 'partial'` 且非影像单的报告；顶部 `只看待核对 · N 份` + 「清除筛选」；N 与概览「待核对 N 份」同口径，共用同一判定函数。
- [ ] `filter=failed`：只显示 `status === 'failed'` 的报告（含影像单失败）；顶部 `只看识别失败 · N 份` + 「清除筛选」。
- [ ] 筛选后为空：`没有符合筛选条件的报告。` + 「查看全部档案」，点击回到未筛选的全部档案；无筛选且无报告时沿用现有空态。
- [ ] 成员切换后筛选参数保留，列表随当前成员刷新。
- [ ] 页面标题为「全部档案」，页内可见文案不再出现「时间线」旧名（加载中、未选成员）。
- [ ] 上传页、核对页的「去时间线查看」改为「去档案查看」，目标仍是 `/timeline`；改派成功文案改为「已移到该成员名下」。
- [ ] 删除报告后仍回 `/timeline`，档案空态不崩（回归）。
- [ ] 纯函数 `filterReports` 与 `parseReportFilter` 有单测：partial 排除影像单、failed、无筛选、非法值、数组首值。
- [ ] E2E：原「时间线按成员过滤报告」改名「档案按成员过滤报告」，断言不变；新增直接打开 `/timeline?filter=partial` / `?filter=failed` 的用例（部分 / 清除 / 空态）；上传与同图重传、改派用例的文案断言同步；10 条全绿。
- [ ] `docs/agents/app-e2e.md` 完成判据 9/9 → 10/10。
- [ ] `npm run typecheck`、`npx expo lint`、`npm test` 全绿。

## 实现提示

- 纯逻辑按 spec 统一放 `reports/overview.ts`：本票先落 `filterReports` / `parseReportFilter` 与共享判定（`status === 'partial'` 且非影像单），03 再落 `pendingSummary` / `latestByCategory`；计数与筛选不许各写一套。
- `?filter=` 是 App 内部 URL 参数，不是 API 契约；服务端仍一次返回当前成员全量摘要，本票不做分页或服务端过滤。
- 筛选说明条、清除入口与筛选空态沿用现有页面的样式和错误/空态口径，不新增设计组件。
- 新增 E2E 用例放在上传用例之前（种子里小明仍是 3 份；上传会加第 4 份，核对会清零待核对），这样 spec 的「3 份 / 待核对 1 份」口径成立。
- 新增 locator：`archive-filter-banner`、`archive-filter-clear`、`archive-empty`；`report-card-${id}` 继续只用于档案页与详情头部。
