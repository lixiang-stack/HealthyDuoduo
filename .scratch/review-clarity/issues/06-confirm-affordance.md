# 06: 确认动作按状态显隐

**What to build:** 按 `docs/design/2026-10-21-confirm-vs-correct.md` 的场景矩阵，让「确认」只在未核对的低置信项出现；高置信项、已确认项、已修正项不再显示确认；可操作未知显示「补录缺失信息」为主操作；每张卡最多一个 filled primary；unknown 项补帮助文案说明确认不会改变 flag。

**Blocked by:** 02（队列模型，已 resolved）、04（交互决策，本票依据）。

**Status:** ready-for-human

## 验收清单

- [ ] 纯逻辑 `reviewActions(item, context)`：返回 `{ confirm: boolean; correctLabel: string; helper?: string }`（形状可随实现微调），测试矩阵覆盖：低置信未核对 normal/high/low/unknown、高置信 unknown（可操作/不可操作）、已确认、已修正、failed 无项。
- [ ] `ReviewItemCard` 按 `reviewActions` 渲染：确认只在 `low_confidence && review_status === 'unreviewed'` 出现；高置信/已确认/已修正无确认；已修正的修正文案为「再修正」；可操作未知的 primary 为「补录缺失信息」（打开现有编辑器）；不可操作未知显示原因说明与 ghost 修正。
- [ ] 低置信 unknown 保留确认，但显示帮助文案：`确认不会改变「未知」；缺数值/范围请用修正补录。`
- [ ] 每卡最多一个 filled primary；无意义动作用隐藏而非 disabled；现有 `review-confirm-*` / `review-correct-*` testID 保持可定位（隐藏时数量为 0）。
- [ ] 服务端/契约/数据库零改动：确认仍只改 `review_status`，修正仍重算 flag（`internal/pipeline/review.go` 不变）。
- [ ] 单测覆盖矩阵；`npm run typecheck`、`npm test` 全绿。
- [ ] App E2E：`cbc_03` 低置信项确认后显示「已确认」且确认按钮消失；`cbc_04` 高置信 unknown 详情不出现确认；修正后显示「已修正」且无确认；现有 12 条用例保持全绿。
- [ ] 不新增依赖；不引入「忽略/稍后」第三状态。

## 实现提示

- 现有 E2E 已覆盖确认/修正主链路（`e2e/tests/app.e2e.ts` 的「核对低置信项并修正指标项」与「改归属、删除与同图重传」）；新增断言即可，不必改 seed。
- `reviewActions` 依赖票 02 的 `backfillItems` 判定；若 02 的 C2 未落地，可先按 `flag === 'unknown'` + 缺值/上下界写反的本地判定实现，避免与 02 的入口重复。
- 给 `ReviewItemCard` 提供「不显示确认」的入口（prop 或 context），供 05 的待补录列表复用；不要复制第二套卡片。
- 不建议为本次做 prototype：只改动作显隐与文案，不改布局/手势；场景矩阵即评审依据。
- 改动集中在 `apps/mobile/src/review/components.tsx`、`apps/mobile/src/review/items.ts` 与对应单测。
