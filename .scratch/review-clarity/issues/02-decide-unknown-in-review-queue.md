# 02: 分析「未知」是否进入核对队列

**What to build:** 基于 01 的成因分类，判断哪些 unknown 指标项应该进入「核对」流程，哪些不该；给出队列模型（现状 / 全部 unknown / 仅可操作 unknown 的独立「待补录」队列）、计数口径、动作与对概览的影响，产出决策建议。本票不改生产代码。

**Blocked by:** 01（需要成因分类）。

**Status:** resolved

## 验收清单

- [x] 逐类判断 01 的 unknown 成因是否可操作：缺参考范围但纸面有值/范围 → 可补录；OCR 误读 → 可修正；定性/无参考范围/影像测量 → 不可判定；提取缺陷 → 修数据链路而非凑核对。
- [x] 对比至少三个选项并给出取舍：保持现状（unknown 不进队列）、全部 unknown 进队列、只把可操作 unknown 放进独立「待补录/待修正」队列；说明对队列长度、概览「待核对 N 份 / M 项」、用户认知和确认/修正动作的影响。
- [x] 明确与 `status=partial`、`unreviewed_low_confidence_count`、`pendingItems` 的关系；不得让「待核对」同时表示低置信和不可判定的 unknown 而无法解释。
- [x] 给出推荐方案、验收标准，以及需要更新的 spec/ADR/概览文案；若采纳，另立实现票。
- [x] 不改生产代码；产出决策文档（如 `docs/design/2026-10-21-unknown-in-review.md`）。

## 实现提示

- 现状：`pendingItems` 只收 `low_confidence && review_status === 'unreviewed'`；`reportBadgeLabels` 的「待核对 N 项」用 `unreviewed_low_confidence_count`；概览的「待核对 N 份」按 `status === 'partial'` 且非影像单计数。
- 关键反例：`中性细胞数 4.19` 高置信、unknown、无参考范围；这正是「进队列会没有可执行动作」的典型。
- 不要把「unknown」直接等同「待核对」；先决定它属于哪类用户任务（补录、修正、仅展示）。

## Comments

**决策完成（2026-10-21，分支 member-overview）**

- 决策文档：`docs/design/2026-10-21-unknown-in-review.md`。
- 推荐：核对队列保持只收未核对低置信项；不把全部 unknown 塞进队列；可操作 unknown 只在报告详情以「待补录 N 项（未知）」小节出现（C2），不进概览计数、不改状态、不引入第三状态。
- 依据：unknown 占 22.9%/23.3%，但可被修正消除仅 11/43 与 17/130；aaa 43 个 unknown 全部在 success 报告；samples 16 份 partial 中 6 份含低置信项。
- 后续实现票：`.scratch/review-clarity/issues/05-pending-backfill-section.md`（ready-for-human）。
- 未改生产代码。
