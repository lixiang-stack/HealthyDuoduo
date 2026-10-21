# 2026-10-21 确认与修正的交互决策

## 为什么做

同一个指标项上现在同时出现「确认」和「修正」：确认是 primary、修正是 ghost，两种状态都显示。对高置信项，确认没有实际作用；对未知项，确认不会消除未知；对已修正项，再点确认会把「已修正」标签覆盖成「已确认」。家长无法从按钮判断「我到底该点哪个」。票 02 已决定未知不进入核对队列；本票在此基础上定义两个动作的语义、适用场景与主次。

## 结论

1. **确认 = 「解析结果与纸面一致」**，只对**未核对的低置信项**显示；它只改 `review_status`，不改任何纸面事实、不重算 flag、不消除 unknown。
2. **修正 = 「纸面事实不是这样」**，对所有非 failed 指标项可用；它改写 value/unit/name/ref_range 四项之一并按新值 × 范围重算 flag。**「补录」不是第三种动作**，只是修正的一种文案（缺数值/范围时打开同一个编辑器）。
3. **不在无意义处显示确认**：高置信项、已确认项、已修正项都不显示确认。服务端保留重复确认能力（ADR-0005），但 UI 不暴露会覆盖「已修正」标签的路径。
4. **每张卡最多一个 filled primary**：低置信未核对 → `确认无误`；高置信可操作未知 → `补录缺失信息`；其余状态只有 ghost `修正` / `再修正`。
5. **unknown 的卡片必须有解释**：`确认` 对未知项不会改变 flag；可操作未知给出补录入口，不可操作未知说明「定性结果/无参考范围，无法比较」，而不是假装确认就能解决。

## 现状事实

- **服务端**：`ApplyReviewAction`（`internal/pipeline/review.go`）——`Confirm` 只置 `review_status=confirmed` 并原样返回；correct 覆盖传入字段、`ComputeFlag` 重算 flag、置 `corrected`、append 修正对。确认不产生修正对（ADR-0005）。
- **客户端**：`ReviewItemCard`（`apps/mobile/src/review/components.tsx`）对每个项都渲染确认（primary，`review_status === 'confirmed'` 时禁用）与修正（ghost）。
- **出现位置**：核对页只列 `pendingItems`（未核对低置信项）；详情页列全部指标项，因此高置信 unknown 也能看到两个动作。
- **计数**：确认后低置信项移出核对队列，`unreviewed_low_confidence_count` 减少，`partial` 可能翻为 `success`；但确认不改 flag，所以 `flag_counts.unknown` 不变。
- **审计事实**：`member=aaa` 的 43 个 unknown 中只有 2 个是低置信项；其余 41 个高置信 unknown 根本不在核对页，却仍在详情页显示确认。对它们显示确认没有意义。

## 语义定义

| 动作 | 用户意图 | 服务端效果 | 何时显示 |
| --- | --- | --- | --- |
| 确认 | 这次解析和纸面一致，我认可 | 只改 `review_status=confirmed`；不产生修正对；flag 不变 | 仅 `low_confidence && review_status === 'unreviewed'` |
| 修正 | 纸面事实不是这样，按纸面改写 | 覆盖四项事实；重算 flag；append 修正对；置 `corrected` | 所有非 failed 指标项；按状态调整文案 |
| 补录（修正的文案） | 缺值/缺范围，照原图补上 | 同修正 | 高置信 unknown 且满足票 02 的「可操作」判定；低置信项也可用修正补 |

**关键事实**：确认不会把 unknown 变成 normal/high/low。唯一能改变 flag 的是修正（改 value/ref_range）。因此高置信未知项的主操作不应该是确认；低置信 unknown 的确认仍服务于「抽取可信度」，但同样不会消除 unknown。

## 场景矩阵

| 场景 | 识别方式 | 主操作 | 次操作 | 文案 / 说明 |
| --- | --- | --- | --- | --- |
| 低置信、未核对、非 unknown | `low_confidence && unreviewed && flag !== unknown` | `确认无误`（filled） | `修正`（ghost） | 常规核对；确认后移出队列 |
| 低置信、未核对、unknown | 同上且 `flag === unknown` | `确认无误`（filled） | `修正`（ghost） | 帮助文案：`确认不会改变「未知」；缺数值/范围请用修正补录。` |
| 高置信、unknown、可操作 | 票 02 的 `backfillItems` 命中 | `补录缺失信息`（filled，打开编辑器） | `查看原图定位` | 说明：`照原图补齐后按值×范围重算。` |
| 高置信、unknown、不可操作 | unknown 但不满足可操作判定 | 无 | `修正`（ghost）+ `查看原图定位` | 说明：`定性结果或纸面无参考范围，无法比较。` |
| 高置信、normal/high/low | 非 unknown 且非低置信 | 无 | `修正`（ghost） | 不显示确认；没有待确信的抽取 |
| 已确认 | `review_status === 'confirmed'` | 无 | `修正`（ghost） | 保留「已确认」标签；不再显示会空转的确认 |
| 已修正 | `review_status === 'corrected'` | 无 | `再修正`（ghost） | **不显示确认**，避免把「已修正」覆盖成「已确认」并丢失状态语义 |
| failed 报告 | `status === 'failed'` | 无 | 无 | 无指标项；展示图像与失败原因（现有形态） |

### 主次与按钮形态

- **filled primary**：只给「下一步最该做的事」——低置信项的确认、可操作未知的补录。每张卡最多一个。
- **ghost**：修正/再修正、查看原图定位。
- **不用的做法**：
  - 不用「disabled 的确认」表达「不需要确认」——禁用但不解释比隐藏更困惑；确认在无意义状态直接隐藏。
  - 不新增「忽略/稍后」按钮——票 02 明确不引入第三套状态。
  - 不把「补录」做成第三个服务端动作——它是修正的文案，服务端状态机不变。

## 与票 02 队列模型的一致性

- 核对页仍只收低置信项；低置信 unknown 的确认仍会移出核对队列（它完成的是「抽取可信度」任务），但帮助文案说明 unknown 不会被消除。
- 详情页的「待补录 N 项（未知）」只列可操作 unknown；这些项只显示补录/修正，不显示确认。
- **低置信且可操作的 unknown 不重复进入 C2**：它已经在核对队列/核对页里，保留「确认无误 / 修正」两个动作（确认完成的是抽取可信度任务，修正完成数据补齐）；C2 只收高置信、或低置信但已不在核对队列的项，并与普通指标项列表**分区不重复**。
- 不可操作 unknown 只在详情页作为普通指标项展示（带原图定位与原因说明），不进入任何待办计数。
- 不发明第三套状态：`review_status` 仍是 `unreviewed / confirmed / corrected`，`flag` 仍是 `normal/high/low/unknown`。

## 需要更新的文档

- `CONTEXT.md`：在「核对」词条下补一句「确认只对未核对的低置信项显示；未知是结果状态，确认不会改变它」。
- `docs/design/2026-10-21-unknown-in-review.md`：C2 列表的动作与本矩阵一致。
- 实现落点：`apps/mobile/src/review/components.tsx` 与 `apps/mobile/src/review/items.ts`；不改 OpenAPI、服务端、数据库。

## 实现票验收标准（另立票）

另立 `.scratch/review-clarity/issues/06-confirm-affordance.md`（`Status: ready-for-human`）。实现票至少覆盖：

- **纯逻辑** `reviewActions(item, context)`：返回是否显示确认、修正文案、帮助文案；测试矩阵覆盖低置信/高置信、normal/high/low/unknown、unreviewed/confirmed/corrected、failed 无项。
- **组件**：确认只在低置信未核对项出现；高置信项无确认；已确认/已修正无确认；可操作未知显示补录 primary；不可操作未知显示说明与 ghost 修正；每卡最多一个 filled。
- **服务端不变**：确认仍只改状态、修正仍重算 flag；不新增动作/状态/字段。
- **单测**：矩阵每行至少一例；断言不出现「确认」的 case 用组件可见性测试（可测 `reviewActions` 纯逻辑 + E2E 兜底）。
- **App E2E**：
  - `cbc_03`（低置信）确认后「已确认」，且不再出现确认按钮；
  - `cbc_04`（高置信 unknown）详情项不出现确认；可操作未知显示补录入口；
  - 修正后出现「已修正」且无确认；
  - `failed` 影像/检验单不回归（沿用现有用例）。
- **边界**：不建议用 prototype 评审——本次只改动作显隐与文案，不改布局/手势；场景矩阵足以评审。若维护者要求视觉确认，可在实现票里加一张静态 wireframe，不进入生产代码。

## 不做什么

- 不重新定义 `flag` 语义（单侧比较、区间比较属于识别/契约层，另票）。
- 不改「确认后是否允许再修正」（服务端已允许；UI 保留修正出口）。
- 不删除已确认/已修正项的历史；修正对仍 append-only（ADR-0005）。
