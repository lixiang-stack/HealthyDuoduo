# 04: 确认与修正并存的交互决策

**What to build:** 明确「确认」与「修正」的语义、适用场景和界面主次，解决同一指标项同时出现两个动作时的选择困惑；产出交互决策（必要时 prototype），覆盖低置信、高置信 unknown、已确认、已修正、failed 等状态。本票不改生产代码。

**Blocked by:** 02（unknown 是否进入核对队列会影响动作模型）。

**Status:** resolved

## 验收清单

- [x] 定义：确认 = 认可当前解析结果（只改 `review_status`，不改 flag）；修正 = 按纸面事实改写 value/unit/name/ref_range（服务端重算 flag）；明确 unknown 项点确认不会消除 unknown。
- [x] 列出用户困惑场景：低置信项（该确认还是修正？）、高置信 unknown（确认无意义？）、已确认项、已修正项、failed 项；逐场景给出应显示的动作、主次与文案。
- [x] 提出具体交互方案（如「确认无误」vs「修正数值/范围」、确认仅对低置信项显示、unknown 缺数据时把修正设为主操作、帮助文案），并说明是否隐藏/禁用某个动作。
- [x] 与 02 的队列模型一致：可操作 unknown 的入口、不可操作 unknown 的展示；不重新发明第三套状态。
- [x] 产出决策文档 +（如需）可点击 prototype；给出实现票的验收标准与 E2E/单测边界。
- [x] 不直接改生产代码。

## 实现提示

- 现状：`ReviewItemCard` 对每个指标项都显示「确认」（primary，已确认时禁用）与「修正」（ghost）；`buildCorrection` 无差异时不发请求；`ApplyReviewAction` 的 confirm 只改 `review_status`，correct 才重算 flag。
- `pendingItems` 只收未核对低置信项；高置信 unknown 不在核对页，但详情页仍显示两个动作。
- 可先用 `prototype` 验证按钮层级/文案，再落实现票；不要在本票里改生产代码。

## Comments

**决策完成（2026-10-21，分支 member-overview）**

- 决策文档：`docs/design/2026-10-21-confirm-vs-correct.md`。
- 确认只对未核对低置信项显示；高置信/已确认/已修正不显示确认；可操作未知用「补录缺失信息」（修正的文案）；每卡最多一个 filled primary；unknown 保留帮助文案说明确认不改变 flag。
- 与 02 的 C2 模型一致：待补录列表只用修正，不引入第三状态。
- 后续实现票：`.scratch/review-clarity/issues/06-confirm-affordance.md`（ready-for-human）。
- 未改生产代码；未做 prototype（仅动作显隐/文案，矩阵足以评审）。
