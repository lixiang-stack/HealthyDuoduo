# 05: 详情页「待补录」小节（可操作未知）

**What to build:** 按 `docs/design/2026-10-21-unknown-in-review.md` 的 C2 方案，在报告详情页指标项列表上方增加一个「待补录 N 项（未知）」小节，只列可由现有修正消除/补齐数据的未知项；不进入核对队列、不进入概览计数、不新增状态。修正后该项从列表移出。

**Blocked by:** 02（队列模型，已 resolved）、03（原图定位，已 resolved）、06（`ReviewItemCard` 的确认显隐，尚未实现）。

**Status:** ready-for-human

## 验收清单

- [ ] 纯逻辑 `backfillItems(items)` 返回可操作未知项并保持报告原序；判定只含两类：①`value === null` 且参考范围可解析（`a-b` / `a~b` / 单侧比较 / 性别前缀）；②参考范围可解析但上下界写反。同时排除 `low_confidence && review_status === 'unreviewed'`（已在核对队列，避免重复入口）。定性值、单侧比较值、区间值、数字+缺范围、参考范围不可解析、非 unknown 项都不入列。
- [ ] 详情页非影像单、非 failed 时渲染 `待补录 N 项（未知）`；N=0 不渲染；列表项复用 `ReviewItemCard` 的编辑器与「查看原图定位」（票 03），**不显示确认**。
- [ ] C2 与普通指标项列表**分区不重复**：C2 项从下方列表移出；C2 存在时普通列表标题为 `指标项（M）`（M = 总项数 − N）；N=0 时保持现有 `指标项（总数）`。每项只渲染一次。
- [ ] 修正后：能消除 unknown 的项按服务端响应重算 flag 并移出；补了定性值/仍 unknown 但 value 已非空的项也移出（列表衡量数据补齐，不衡量 unknown 是否消失）。
- [ ] `pendingItems`、`needsReview`、核对页、上传分岔、概览「待核对 N 份 / M 项」、`status` 派生、`unreviewed_low_confidence_count` 全部不变。
- [ ] 单测覆盖判定矩阵（含边界：空 items、无 `ocr_lines`、已确认/已修正）；`npm run typecheck`、`npm test` 全绿。
- [ ] App E2E：用 `cbc_04`（含缺值与上下界写反）验证小节出现、补录后移出、C2 项不再重复出现在 `指标项（M）`；用 `cbc_01` 的中性细胞数（数字+缺范围）作为反例不出现在小节；现有 12 条用例保持全绿。
- [ ] 不改 OpenAPI、数据库、服务端；不新增依赖。

## 实现提示

- 判定所需参考范围解析可复用 `apps/mobile/src/reports/refRange.ts` 的 `parseRefRange`，但「上下界写反」需要新增纯函数（现有解析会把区间归一为 min/max，丢失反向事实），放在 `review/items.ts` 或 `reports/refRange.ts` 并在单测中锚定。
- 小节标题与帮助文案在 `ReviewItemCard` 之外的详情页容器中定义；动作仍走现有 `reviewItem(client, reportId, itemId, {action:'correct', ...})`。复用 `ReviewItemCard` 需要 06 提供「不显示确认」的入口（prop 或 context）；若 06 未先落地，本票不得自行复制一套核对卡。
- 列表不新增服务端字段；`flag_counts.unknown` 仍是报告级总未知数，不等于本小节 N。
- 若维护者认为本票不应本期做，保持 `Status: ready-for-human` 并附决策文档链接即可。
