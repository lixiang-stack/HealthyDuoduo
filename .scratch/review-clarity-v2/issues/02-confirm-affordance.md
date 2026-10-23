# 02: 确认动作按状态显隐 + 共享「可操作未知」谓词

**What to build:** 按本轮 Q2/Q3 的结论，把“确认只对未核对的低置信、且不可补录的项显示”落到客户端；把“可操作未知”的判定收敛成一份纯逻辑 `isBackfillItem(item)`，供本票的 `reviewActions` 与票 03 的待补录入口共用。可操作未知（缺值 + 可解析范围，或参考范围上下界写反）即使低置信未核对，也只给“补录缺失信息”/修正，不给确认。本票不改服务端、契约、数据库。

**Blocked by:** None（可立即开始；不依赖票 01 的审计数字）。

**Status:** resolved

> 2026-10-23 复核：核心行为已由后续 `unknown-reasons` 迭代落地；未补的验收缺口见 Comments，按本轮决定关闭本票。

## 验收清单

- [ ] `apps/mobile/src/reports/refRange.ts` 新增并导出 `isReversedInterval(text)`：剥性别前缀后，用现有区间正则匹配，比较原始上下界，返回 `low > high`；`parseRefRange` 会归一为 `min/max`，**不能**用它判断反向。
- [ ] `apps/mobile/src/review/items.ts` 新增并导出 `isBackfillItem(item)`：
  - `flag === 'unknown'`；
  - `value === null` 且 `parseRefRange(ref_range).kind` 是 `interval` 或 `bound`（不是 `scalar`/`none`）；
  - 或 `value` 可数值化（`parseNumericForm(value).kind === 'scalar'`）且 `isReversedInterval(ref_range)`。
  - 不在这里排除 `low_confidence && unreviewed`：那是票 03 的视图/入口问题；`reviewActions` 需要看到可操作项才能隐藏确认。
- [ ] `apps/mobile/src/review/items.ts` 新增并导出 `reviewActions(item)`，返回 `{ confirm, primary, correctLabel, helper? }`；`primary` 为 `'confirm' | 'backfill' | null`，`correctLabel` 为 `'修正' | '再修正' | '补录缺失信息'`。规则按优先级：
  1. `isBackfillItem(item)` → `confirm=false`，`primary='backfill'`，`correctLabel='补录缺失信息'`，`helper='照原图补齐后按值×范围重算。'`（不论 `low_confidence` / `review_status`，包括已确认/已修正的仍可操作 unknown）。
  2. 否则 `low_confidence && review_status==='unreviewed'` → `confirm=true`，`primary='confirm'`，`correctLabel='修正'`；若 `flag==='unknown'`，`helper='确认不会改变「未知」；定性结果、区间值或纸面无参考范围，无法比较。'`。
  3. 否则 → `confirm=false`，`primary=null`，`correctLabel = review_status==='corrected' ? '再修正' : '修正'`；若 `flag==='unknown'`，`helper='定性结果、区间值或纸面无参考范围，无法比较。'`。
- [ ] `ReviewItemCard`（`apps/mobile/src/review/components.tsx`）按 `reviewActions` 渲染：
  - `primary==='confirm'` 时渲染确认（filled）；`primary==='backfill'` 时渲染“补录缺失信息”（filled，点击进入现有四项编辑器）；`primary===null` 时无 filled。
  - `primary!=='backfill'` 时渲染 ghost 修正/再修正；`primary==='backfill'` 不再渲染第二个“修正”，避免同一编辑器两个入口。
  - 确认在无意义状态**隐藏**而非 disabled；每卡最多一个 filled primary。
  - 保留 `review-confirm-*` / `review-correct-*` testID；新增 `review-backfill-*`。隐藏时对应 testID 数量为 0。
- [ ] 低置信可操作 unknown 不再有确认出口：它留在 `pendingItems`，直到补录/修正；修正置 `corrected` 后自然移出核对队列。不要在实现里把它从 `pendingItems` 提前移除。
- [ ] 服务端/契约/数据库零改动：确认仍只改 `review_status`，修正仍按 `value × ref_range` 重算 `flag`（`internal/pipeline/review.go` 不变）。
- [ ] 单测覆盖 `isReversedInterval`、`isBackfillItem`、`reviewActions` 矩阵：
  - `isBackfillItem`：`value=null + 0-20` → true；`男:0-20` → true；`<5` → true；`value=null + 5`（标量范围）→ false；`value=null + null` → false；`value=null + 13.5--9.5` → true；`value=17.3 + 13.5--9.5` → true；`value=17.3 + 3.5--9.5` → false（且 flag 非 unknown）；`value=17.3 + null` → false；`value=阴性 + 0-20` → false；`value=70-80 + 0-3` → false；`value=<2.00 + 0-30` → false；`flag=normal + 反向范围` → false。
  - `reviewActions`：低置信未核对 normal/high/low（确认）、低置信未核对 unknown 可操作（补录、无确认）、低置信未核对 unknown 不可操作（确认 + 帮助）、高置信 unknown 可操作（补录）、高置信 unknown 不可操作（无 filled + 修正 + 帮助）、高置信 normal/high/low（无 filled + 修正）、已确认可操作 unknown（补录、无确认）、已确认不可操作 unknown（无 filled + 修正 + 帮助）、已修正可操作 unknown（补录，不是“再修正”）、已修正不可操作（“再修正”）、failed 无项。
- [ ] `npm run typecheck`、`npm test`、`npx expo lint` 全绿；App E2E 现有 12 条保持全绿，并新增/调整：
  - `cbc_03` 的低置信 normal 项：确认后显示“已确认”，确认按钮数量为 0。
  - `cbc_04` 的 PLT（低置信、unknown、`ref_range=130--31`）：详情不出现确认，出现 `review-backfill-*` 主操作。
  - `cbc_04` 的白细胞计数（高置信、unknown、`13.5--9.5`）：可直接打开补录/修正；把范围改成原图的 `3.5--9.5` 后，flag 按服务端响应重算，项从可操作集合移出（它本来不在 `pendingItems`，高置信）。
  - `cbc_04` 的血清淀粉样蛋白A（高置信、unknown、`value=17.05`、无范围）：不出现确认，也不出现补录主操作（不可操作），ghost 修正可见，帮助文案可见。
- [ ] 不新增依赖；不引入“忽略/稍后”第三状态；不改 `pendingItems`、`needsReview`、核对页、上传分岔、概览计数、`status` 派生、`unreviewed_low_confidence_count`。

## 实现提示

- `ComputeFlag`（`internal/pipeline/flag.go`）只接受区间与单侧比较；**标量参考范围（如 `5`）仍然返回 unknown**。所以 `isBackfillItem` 不能写成 `parseRefRange(...).kind !== 'none'`，否则会把标量范围误判为可补录。
- `parseRefRange` 已剥性别前缀并归一区间；`isReversedInterval` 直接复用 `refRange.ts` 的 `INTERVAL_RE` / `SEX_PREFIX_RE`，不要复制正则。
- `ReviewItemCard` 现有 `editing` 状态与 `review-save-*` / `review-cancel-*` 编辑器可原样复用；`primary==='backfill'` 只是把“进入编辑器”的按钮换成 filled 文案，保存逻辑不变。
- E2E 用 `cbc_04` 的白细胞计数做“改范围后 flag 重算”的确定样例（原图 `3.5--9.5`）；PLT 只验证“低置信可操作 unknown 无确认 + 补录主操作”，不强行给它编一个正确范围。
- 票 03 会消费 `isBackfillItem`；本票不定义 `backfillItems`，避免又出现旧 05/06 的循环依赖。

## Comments

**复核（2026-10-23，分支 `review-clarity-v2-followup`）**

- 核心行为已由后续 `unknown-reasons` 迭代落地（`80d48d1`）：`isReversedInterval`（`apps/mobile/src/reports/refRange.ts:70`）、`isBackfillItem` / `reviewActions`（`apps/mobile/src/review/rules.ts:51`、`apps/mobile/src/review/items.ts:90`）、`ReviewItemCard` 按 primary 显隐并新增 `review-backfill-*`（`apps/mobile/src/review/components.tsx:178`）。低置信可操作 unknown 不再出现确认；E2E 已有 cbc_04 可补录项无确认断言。
- 按本轮决定不补票面剩余验收缺口：不可操作 unknown 的解释由更细的 `flagReasonText` 承担，不叠加票面 helper；单测矩阵与 cbc_03/cbc_04 的其余 E2E 场景保留现状。
- `unknown-reasons` 迭代为 `isBackfillItem` 增加了 `flag_reason` 抽取类限定（`ocr_miss` / `ocr_merge` / `extractor_pairing`）；票面结构矩阵仍成立，本票不改回。
- 服务端/契约/数据库零改动。
