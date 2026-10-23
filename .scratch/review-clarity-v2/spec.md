# Spec：核对清晰度 v2（证据重做 + 确认显隐 + 待补录入口）

Status: resolved

日期：2026-10-21；分支：`review-clarity-v2`。

**取代：** 旧 `docs/design/2026-10-21-*.md` 与 `.scratch/review-clarity/` 已在本工作区移除；旧 03（未知项原图定位）的实现保留，旧 01/02/04 中仍成立的结论并入本 spec，证据部分由票 01 重做。

**依据：** `CONTEXT.md`（报告 / 指标项 / 家长 / 核对 / 未知）· `ADR-0005`（核对落库与修正对）· `ADR-0009`（影像报告零指标项）· `docs/design/2026-10-13-viz-and-client-spec.md` 5.2（核对闭环）· `docs/agents/issue-tracker.md` · 现有实现 `apps/mobile/src/review/{items.ts,components.tsx}`、`apps/mobile/src/reports/{overview.ts,refRange.ts}`、`internal/pipeline/{review.go,flag.go}`、`apps/mobile/e2e/{support.ts,tests/app.e2e.ts}`。

## 为什么做

上一轮已经得到三个稳定结论：`unknown` 不是低置信的同义词；`unknown` 不参与 `partial`；确认不改变 `flag`，只有修正才重算。但上一轮的证据与票据有三处硬伤：

1. **数据不独立**：`member=aaa` 与 `samples` 有重叠报告。审计自己写了 `cbc_01` 就是 aaa 的报告 `139fde0e0b0c`，而 `cbc_01` 又在 samples 语料里；`ana_04` 也极可能是 aaa 免疫单 `c9ce457c37ec` 的掩码副本。`22.9% vs 23.3%` 不能当作两次独立验证。
2. **根因写错**：旧审计把 `ana_04`/抗 Jo-1 的漏值归因于 `_extract_items` 配对，但 `samples/expected/ocr/ana_04.json` 的对应行根本没有这些值（拍屏摩尔纹，OCR 未输出）；把 `cbc_04` 的反向范围归因于 `_decompose` 并入相邻序号，但 `13.5--9.5`、`8210--1` 已经是 OCR 行文本，`_decompose` 只拆单格，不做跨格合并。
3. **票据成环**：旧 05 需要 06 提供“不显示确认”的卡片，旧 06 又需要 05 的 `backfillItems`；两边互相阻塞。

这三点会直接改变“先修识别端还是先做待补录”的排序。v2 先用票 01 重做证据，再用票 02 落地已确认的确认显隐规则，最后用票 03 在修正后的残量上决定是否做待补录入口。

## 结论（本轮已定）

1. **证据先重做（Q1）**：票 01 按 `report_id` 去重/标注重叠，把成因拆成 `OCR 漏识别 / OCR 行合并 / 提取器配对 / 纸面无参考范围 / 不可比较（定性·区间·单侧） / 参考范围不可解析 / 其他`，并对“有数值 + 缺范围”抽样核对原图。旧审计数字只作对照，不照抄。
2. **可操作未知一律不显示确认（Q2）**：只要修正能把 `unknown` 变成 `normal/high/low`（缺值 + 可解析范围，或范围上下界写反），即使它是低置信未核对项，也只给“补录缺失信息”/修正，不给确认。不可操作的低置信 unknown 仍保留确认——那是在完成“抽取可信度”任务。
3. **谓词归票 02（Q3）**：票 02 拥有 `isBackfillItem(item)` 与 `reviewActions(item)`；票 03 只消费，不再复制判定。`reviewActions` 不需要 `context` 参数；当前所有输入都在 `item` 上。这样旧 05↔06 的环消失。
4. **文档结构只做 Q4(b)(c)**：新增本 `spec.md` 作为 01–04 的单一入口；`CONTEXT.md` 把 UI 策略移出“核对”词条，并新增独立的“未知”词条。不做 Q4(a) 的“设计文档单一事实源”重构——旧设计文档已移除，新 spec 只保留决策与验收摘要，细节留给 issue。
5. **跟进项**：`参考范围未识别` 改为中性事实 `无参考范围`（票 04）；旧 04 矩阵把“查看原图定位”当状态动作的问题随旧文档移除，本 spec 明确它是所有有 `ocr_lines` 项的通用可见性，不进入动作矩阵。

## 做什么

### 1. 重做 unknown 审计（票 01）

- 重跑 `member=aaa`（共享栈）与 `samples/expected/report/*.json`，先识别重叠报告，再按根因分层。
- 每个根因至少一个真实样本：报告 / 指标项 / `value` / `ref_range` / `raw_text` / `low_confidence` / OCR 行号或 `txts` 下标 / 原图裁切证据。
- 重点核对：
  - `ana_04` 的 7 个漏值：OCR 期望文件里没有值 → `OCR 漏识别`，不是 `_extract_items` 配对。
  - `cbc_04` 的反向范围：反向串已在 OCR 文本 → `OCR 行合并`；`淋巴细胞数`（`value=null`、`ref=6111.1--3.2`）计入同一缺陷家族，该家族至少 7 项。
  - “有数值 + 缺参考范围”（旧审计 aaa 8 / samples 24）：抽样核对原图，拆成“纸面无范围 / OCR 漏范围 / 未核实”，不得只凭 `raw_text` 判不可操作。
- 产出 `.scratch/review-clarity-v2/audit.md` 与可复现脚本；给出“可操作残量”表，作为票 03 的决策输入。
- 不改生产代码。

### 2. 确认动作按状态显隐（票 02）

- `isBackfillItem(item)`：`flag === 'unknown'` 且满足以下之一：
  - `value === null` 且参考范围是 `interval` 或 `bound`（`a-b` / `a~b` / `<v` / `≤v` / `>v` / `≥v`，含性别前缀）；
  - `value` 是数值，且参考范围是上下界写反的 `interval`。
- `reviewActions(item)`：返回 `{ confirm, primary, correctLabel, helper }`；确认只在 `low_confidence && review_status === 'unreviewed' && !isBackfillItem(item)` 时为真；可操作 unknown 的主操作是 `补录缺失信息`；每卡最多一个 filled primary。
- `ReviewItemCard` 按 `reviewActions` 渲染；隐藏无意义动作而非 disabled；保留 `review-confirm-*` / `review-correct-*` testID，新增 `review-backfill-*`。
- 不改服务端、契约、数据库；确认仍只改 `review_status`，修正仍重算 `flag`。

### 3. 待补录入口（票 03，先决策）

- 在票 01 的残量表出来后决定：**筛选 chip / 独立小节 / 不做**。
- 默认推荐**筛选 chip**：在现有指标项列表上方加 `全部 / 待补录 N` 切换，列表用 `backfillItems = items.filter(isBackfillItem)`，保持原序；N=0 不显示切换。这样没有分区、没有标题改写、没有“移出”第二套规则，`ReviewItemCard` 直接复用（可操作 unknown 已由 `reviewActions` 隐藏确认）。
- 如果残量太小或用户不需要，票 03 记录 defer 条件并附审计链接；只有维护者确定永久不做时才 `wontfix`。
- 只有维护者明确要求更高可见性时，才退回“独立小节 + 分区”；那需要另立实现票，不在票 03 里同时写两套验收。

### 4. 中性文案（票 04）

- `MISSING_REF_RANGE_LABEL` 改为 `无参考范围`；注释改成“只陈述数据事实，不推断是纸面未印还是抽取漏掉”。
- 这是旧 03 的跟进小修；不改 flag、核对动作或其他 UI 行为。

## 不做什么

- 不改 `flag` 语义（单侧比较、区间比较、定性值仍按现有 `ComputeFlag` 返回 `unknown`）。
- 不改服务端状态机、OpenAPI、数据库、`unreviewed_low_confidence_count`、`status` 派生、概览“待核对”口径。
- 不引入全局“待补录”队列（旧 C1）、不新增状态、不新增“忽略/稍后”。
- 不为影像单设计 unknown/待补录：`ADR-0009` 之后影像报告零指标项。
- 不把 `unknown` 直接等同于待核对；确认仍不改变 `unknown`。
- 不恢复旧的 2026-10-21 设计文档或旧 `.scratch/review-clarity` 工单。

## 票图

| 票 | 内容 | 依赖 | 状态 |
| --- | --- | --- | --- |
| 01 | 重做 unknown 审计（去重 + 根因分层 + 原图核验） | 无 | resolved |
| 02 | 确认动作按状态显隐 + 共享 `isBackfillItem` | 无 | resolved（核心行为已实现；按复核关闭） |
| 03 | 待补录入口：先决策（筛选 chip / 小节 / 不做） | 01、02 | resolved（选 C：暂缓入口） |
| 04 | `参考范围未识别` → `无参考范围` | 无 | resolved |
| 05（条件） | 待补录入口实现（仅当 03 选“做”） | 03 | 不建（03 选 C） |

> 2026-10-23 收口：01 已完成审计；02 复核为已实现（后续迭代）；03 选 C（暂缓入口，不建 05）；04 已改文案并补 E2E。按“只保留有代码修改的票据”删除 01-03 的 issue 文件，本目录仅保留 04；结论保留在上表与 git 历史中。

## 验收

- 票 01：重叠报告有清单；每个 `unknown` 有根因标签；`ana_04` 归 `OCR 漏识别`、`cbc_04` 归 `OCR 行合并` 有 OCR 证据；`cbc_04` 缺陷家族 ≥ 7 项；“有数值 + 缺范围”有原图抽核表；审计报告与脚本可复现；生产代码零改动。
- 票 02：`isBackfillItem` 与 `reviewActions` 单测覆盖矩阵（含 scalar 参考范围返回 false、上下界写反返回 true、低置信可操作 unknown 不显示确认）；`ReviewItemCard` 每卡最多一个 filled；`npm run typecheck`、`npm test`、`npx expo lint`、App E2E 全绿；服务端零改动。
- 票 03：决策记录含残量数字、形态选择与理由；选“做”则另立 05；选“不做”则记录 `wontfix`（永久）或 defer 条件，并附审计链接。
- 票 04：文案与注释更新；`npm test`、`npm run typecheck`、`npx expo lint` 全绿；手工确认详情页 `ref_range=null` 显示 `无参考范围`。
- 全局：不新增依赖；除本 spec、issues 与 `2026-10-13-viz-and-client-spec.md` 的修订注外，不改其他领域文档；所有新文档使用一致的路径引用，不引用已删除的 2026-10-21 文档。

## 已知事实与口径

- `pendingItems` = `review_status === 'unreviewed' && low_confidence`（`apps/mobile/src/review/items.ts:22-24`）。
- 概览 `pendingSummary` = `status === 'partial'` 且非影像单的份数 + 这些报告的 `unreviewed_low_confidence_count` 之和（`apps/mobile/src/reports/overview.ts:27-37`）。`pendingItems` 与概览口径的既有差异（success 报告里的未核对低置信项会进核对页但不进概览）不是本 feature 引入的，本次不顺手修。
- `ComputeFlag`：`value` 必须能数值化；参考范围支持 `a-b` / `a--b` / `a~b` 与 `<v / ≤v / >v / ≥v`；`value` 为文本、`<2.00`、`70-80`、参考范围为 `null`、不可解析或上下界写反 → `unknown`（`internal/pipeline/flag.go`）。**标量参考范围（如 `5`）不被 `ComputeFlag` 接受**，`isBackfillItem` 不能把 `parseRefRange` 的 `scalar` 当可操作。
- `ApplyReviewAction`：`Confirm` 只置 `review_status=confirmed`；`Correct` 覆盖 `value/unit/name/ref_range`、重算 `flag`、置 `corrected`、append 修正对（`internal/pipeline/review.go`）。
- `parseRefRange` 会剥性别前缀，并把区间归一为 `min/max`（`apps/mobile/src/reports/refRange.ts:44-49`）；它**丢失上下界写反的事实**。检测反向区间要另加纯函数（票 02 的 `isReversedInterval`）。
- 旧审计数字（待 01 复核，不作新结论）：`member=aaa` 18 份 / 188 项 / 43 unknown；samples 49 份 / 559 项 / 130 unknown；旧“可被修正消除” 11/43 与 17/130；旧 C2 估算 11 / 15。
- 数据重叠：`cbc_01` = aaa `139fde0e0b0c`；`ana_04` 需在 01 中确认是否为 aaa `c9ce457c37ec`。旧审计把两组当独立样本的百分比对比不成立。
- 根因证据：`samples/expected/ocr/ana_04.json` 的 `txts[62..98]` 只有名称/AU/方法/范围/序号，没有 1.53 等值；`.scratch/recognizer-quality/issues/01-column-assign.md` 的评论明确说这些值不在 OCR 输出（拍屏摩尔纹），重拍是诚实路径。`samples/expected/ocr/cbc_04.json` 的 `txts[46/50/98]` 已含 `8210--1`、`1/6.0 130--31`、`13.5--9.5 109/L`；`_decompose` 不合并相邻格。
- App E2E seed 只上传 `cbc_03.png`、`cbc_04.png`、`us_01.webp`、`us_03.jpeg`（`apps/mobile/e2e/support.ts:149-158`），没有 `cbc_01`；`cbc_04` 的 `血清淀粉样蛋白A`（`17.05`、无范围、高置信 unknown）可作为“数字 + 缺范围”的负例。

## 开放问题

- 01 重做后，可操作残量是否仍值得做入口？如果主要残量是 OCR 漏识别，人工补录的产品价值更高；如果是提取器配对，应先修数据链路。
- “有数值 + 缺范围”里有多少是 OCR 漏了范围？这决定票 03 的筛选谓词是否要覆盖这一类。
- 票 03 选筛选 chip 还是独立小节？默认 chip；只有可见性不足才升级。
- OCR 漏识别是否需要产品层的重拍提示？本轮不做，等 01 的残量数据。
