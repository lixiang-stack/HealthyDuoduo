# 研究：分析结果页 UX 复核

- 日期：2026-11-04
- 分支：`llm-analysis-framework-cloud`
- Feature spec：`.scratch/analysis-result-ux/spec.md`
- 范围：`apps/mobile/src/app/analysis/[analysis_id].tsx`、`apps/mobile/src/analysis/components.tsx`、`apps/mobile/src/analysis/facts.ts`、`internal/analysis/{facts,evidence,explain}.go`、`schemas/openapi.yaml`
- 本报告是研究与证据附录，不修改生产代码；实现工单见同目录 `issues/`。

## 0. 结论摘要

六项发现中，1 和 3 是同一个根因的两面：**解释层被当成页面末尾的补充信息，而它内部的“模型连接语”又被当成标题渲染**。2 是内部脱敏标记泄漏到用户界面。4 是“全局 12 个月窗口”和“按趋势键稀疏扩展”两个口径同时存在，但页面只展示了前一个。5 是纯文本没有交互能力。6 是两个计数口径不同，却被并排成两个相似标题。

优先级建议：

1. **P0：把“解读”移到首屏，并重做解释卡内部层级**（发现 1、3）。
2. **P0：把 `T-0` / `T-Nm` 在展示层转成用户语言**（发现 2）。
3. **P1：改计数标题与说明，修正“有效点”红色样式**（发现 6）。
4. **P1：改写窗口文案，按“窗口内 / 额外参考”分组报告，并改排除原因文案**（发现 4）。
5. **P1：来源链接可点、可复制、可读屏**（发现 5）。

不建议为此新增分析接口字段或改变事实层筛选规则；大部分问题可以用现有数据在前端解决。

第 3 节进一步从“家长用户”和“产品维护者”两个角色做了真机核对与扩展评审，并给出目标 IA 与 8 张候选工单；它不重复前六条，而是补首屏、内容覆盖、病历安全边界、状态、观测等更深层问题。

## 1. 当前页序：问题 1 的基线

当前页面从上到下是：

| 顺序 | 区块 | 代码 |
| --- | --- | --- |
| 1 | 标题、成员、范围、窗口、提示 | `apps/mobile/src/app/analysis/[analysis_id].tsx:153-177` |
| 2 | 本次范围卡（报告 / 病历列表） | `apps/mobile/src/app/analysis/[analysis_id].tsx:179` |
| 3 | 偏离参考范围 | `apps/mobile/src/app/analysis/[analysis_id].tsx:181-206` |
| 4 | 判读标签（框架层） | `apps/mobile/src/app/analysis/[analysis_id].tsx:208` |
| 5 | 与本人历史比较 | `apps/mobile/src/app/analysis/[analysis_id].tsx:210-233` |
| 6 | 相关病历 | `apps/mobile/src/app/analysis/[analysis_id].tsx:235-256` |
| 7 | 无法与参考范围比较 | `apps/mobile/src/app/analysis/[analysis_id].tsx:258-272` |
| 8 | 未纳入项及原因 | `apps/mobile/src/app/analysis/[analysis_id].tsx:274-293` |
| 9 | **解读** | `apps/mobile/src/app/analysis/[analysis_id].tsx:295` |
| 10 | 剩余次数、反馈、来源声明 | `apps/mobile/src/app/analysis/[analysis_id].tsx:301-325` |

用本次本地栈的实际分析 `GET /api/v1/analyses/15`（2026-11-04）做参照：在“解读”之前，页面要依次展示 **5 份报告、4 个偏离项、5 个框架标签、14 条历史趋势、1 份病历、2 个无法比较项、7 个未纳入项**，约 38 个条目/卡片。手机上这是数屏内容，结论被推到最后。

## 2. 六项发现

### 发现 1：解读应该先出现，证据放在后面

**现象**

- “解读”在页面第 295 行才渲染，前面是范围、偏离、框架、历史、病历、无法比较、未纳入项（`apps/mobile/src/app/analysis/[analysis_id].tsx:179-295`）。
- 解释层不可用时，占位块也在最后（`apps/mobile/src/app/analysis/[analysis_id].tsx:296-300`）。
- 现有 RNTL 测试只断言解释卡存在与内容，不检查它与事实层的先后顺序（`apps/mobile/src/test/screens/analysis.test.tsx:234-250`）。

**根因**

页面最初是按“事实层优先”写的：票 04 的注释明确说“只渲染事实层”，框架层/解释层是后来追加的（`apps/mobile/src/app/analysis/[analysis_id].tsx:40-42`）。追加时把解释卡放在了排除项之后，形成了“先证据、后结论”的结构。

Apple HIG 的 Layout 指南明确要求按相对重要性排序内容，把最重要的内容放在阅读顺序的前部 [S1]。分析结果页最重要的内容就是“这次结果意味着什么”，不是报告清单。

**用户为什么困惑**

- 用户点进“分析”，期望先看到结论；实际要先滚过 38 个条目才看到“解读”。
- 解释层不可用时，用户更晚才知道“这次没有解读”，会误以为页面只有一堆数据。
- 事实层再完整，也不能回答用户点“分析”的核心问题。

**建议**

方案 A（推荐，最小改动）：

- 把 `ExplanationCard` 从第 295 行移动到通知区之后、`ScopeCard` 之前。
- 解释层不可用时，在同一个位置渲染一个紧凑的“解读”状态块，而不是页面末尾的空段。
- `ScopeCard` 从首屏移到“相关病历”之后，作为“本次范围”折叠区。
- 事实层内部的相对顺序基本不动：偏离 → 历史 → 病历；只把判读标签提到偏离之前（它和解读一样属于解释，而不是原始数据）。

方案 B（如果不想改页序）：

- 在页首增加一个“结论摘要”卡，只显示解释层的结论首句 + “查看完整解读”锚点；完整四段式仍留在原位置。
- 缺点：同一结论出现两次，且需要滚动定位；不如方案 A 干净。

建议的目标页序见第 4 节。

**验收标准**

- 在 `analysis 15` 这类有解释层的结果上，首屏无需滚动即可看到“解读”标题和结论首句。
- RNTL 中存在顺序断言：`analysis-explanation` 出现在 `analysis-deviations`、`analysis-history`、`analysis-scope`、`analysis-excluded` 之前。
- 解释层不可用时，状态块出现在首屏区域，而不是页面底部。
- 现有事实层测试仍全绿（`npm test -- analysis`）。

### 发现 2：`T-0 时…` 是内部脱敏标记，不是用户语言

**现象**

本次分析的实际 `bridge_text` 是：

> `T-0 时丙氨酸氨基转移酶测定 83.5 U/L、天门冬氨酸氨基转移酶测定 45 U/L，碱性磷酸酶测定 80 U/L、总胆红素测定 15 umol/L。`

它被直接渲染在解释卡最上方，并且用的是卡片标题样式（`apps/mobile/src/analysis/components.tsx:102`、`:177`）。用户看到的是最醒目的“T-0 时…”。

**根因**

- 证据包把报告日期转成相对时间：`T-0` / `T-Nm`（`internal/analysis/evidence.go:236-247`），并写入每个证据点的 `relative_time`（`internal/analysis/evidence.go:95`）。
- 模型 system prompt 只要求“连接条目与证据、数字必须来自证据”（`internal/analysis/prompts/explanation_system.txt:1-3`），没有要求把 `T-0` 转成用户语言；模型自然照抄输入。
- `bridge_text` 校验只检查空值、长度、禁用推断词、数字是否属于证据（`internal/analysis/explain.go:227-252`），不检查 `T-` 标记。
- App 直接渲染 `section.bridge_text`，没有任何展示层转换（`apps/mobile/src/analysis/components.tsx:102`）。

`T-0` 的语义也不是“今天”，而是 `monthsBetween` 计算出的“不足 1 个月”（通常就是最近一次/本月）；`T-3m` 才是满 3 个月、不足 4 个月（`internal/analysis/framework.go:137-153`）。这层含义对用户完全不可见。

**用户为什么困惑**

- “T-0”像内部代号、实验编号或时间戳，用户不知道它指今天、本次报告还是基线。
- 它出现在最醒目的位置，进一步放大了“这段文字到底是什么”的困惑。
- 相对时间本来是为了脱敏（spec §4.11，`.scratch/llm-analysis/spec.md:299`），但脱敏标记不应该成为用户文案。

Apple HIG 的 Writing 指南要求使用简单、平实的语言，避免术语 [S3]。

**建议**

方案 A（推荐，前端展示层）：

- 在 `apps/mobile/src/analysis/facts.ts` 增加一个纯函数，例如 `humanizeBridgeText(text)`：
  - `T-0 时` / `T-0` → `最近一次`
  - `T-Nm 时` / `T-Nm` → `约 N 个月前`
- 在 `ExplanationCard` 渲染前调用；旧快照里的 `T-` 也会被修正，不需要重跑分析。
- 单测覆盖：`T-0 时`、`T-3m`、`T-12m`、无 token、连续多个 token。

方案 B（后端保存前转换，前端仍保留兼容）：

- 在 `validateSection` 通过后、写入 `BridgeText` 前，对 `bridge_text` 做同样的替换（`internal/analysis/explain.go:249-258`）。
- 这样新分析的 API / 快照里也不会有内部标记；旧分析仍靠前端函数兜底。
- **不要**直接把 `relativeTime` 改成中文“3 个月前”：`numbersBelongToEvidence` 会把 `3` 当成待校验数字；本次证据里没有 `3`，因此整个 section 会被拒绝（`internal/analysis/explain.go:284-308`）。如果要在证据包里改，必须同时改数字校验规则，成本更高。
- 若做后端转换，补 Go 单测，并保持与前端相同的 fixture，避免两套规则漂移。

**验收标准**

- 页面文本不出现 `T-0`、`T-3m`、`T-12m` 等标记。
- `T-0 时…` 渲染为“最近一次…”；`T-3m 时…` 渲染为“约 3 个月前…”。
- 旧分析（快照里已有 `T-`）无需重跑即可正常显示。
- `npm test -- analysis` 与 Go `internal/analysis` 单测全绿。

### 发现 3：解释卡层级混乱，`bridge_text` 被误当成标题

**现象**

- `bridge_text` 在卡片最上方，使用 `labelTitle`：15px、600 字重（`apps/mobile/src/analysis/components.tsx:102`、`:177`）。
- “可能原因”标题使用 `partTitle`：13px、600 字重（`apps/mobile/src/analysis/components.tsx:105`、`:179`）。
- 正文也是 13px、400 字重（`apps/mobile/src/analysis/components.tsx:106`、`:180`）。

结果是：**模型连接语比“可能原因”标题更大、更醒目**，但它并不是结论，也不是异常列表。本次 `bridge_text` 除了 ALT/AST 两个异常值，还包含正常的 ALP 80 和总胆红素 15；这两个正常值是命中条目 `liver-hepatocellular-mild` 的条件（`internal/analysis/knowledge/liver-hepatocellular-mild.yaml:3-7`）。所以它既不是“异常值列表”，也不是“结论”，而是**本次判读用到的证据值串**。

**根因**

- 规格把 `bridge_text` 定义为模型唯一的自由文本，只负责“跨条目的顺序与过渡”和“病历与数据对齐”，医学原文由服务端从条目拼接（`.scratch/llm-analysis/spec.md:320-358`）。
- 因为模型被禁止推断、只能引用证据数字，`bridge_text` 天然会写成“数值 + 数值 + 数值”的流水账。它不是设计成给用户当标题的。
- UI 没有区分“模型连接语”和“结论正文”的语义，直接给了标题样式。
- “可能原因 / 进一步检查 / 生活建议 / 长期跟踪”四个标题都是 13px/600，正文 13px/400，层级只靠 200 字重差，视觉上确实容易糊成一片。

Apple HIG 的 Typography 指南建议通过字号、字重和颜色共同建立层级，而不是只靠一种差异 [S2]。Material 3 的 type scale 也把 display / headline / title / body / label 分成不同角色 [S13]。

**用户为什么困惑**

- 最上方那段像“标题”，但读起来像一串数值，用户不知道它是结论、摘要还是证据。
- 它和下方“偏离参考范围”里的数值重复，用户会怀疑页面重复渲染。
- “可能原因”是真正的解释，却比上面的数值串更小，视觉权重倒置。
- 四个小标题与正文只差字重，长文里很难扫读。

**建议**

方案 A（推荐）：把 `bridge_text` 从首屏主路径移走，放进“查看依据”。

解释卡改成：

```text
解读
┌──────────────────────────────────────┐
│ 可能原因                              │ 12px / 500 / muted
│ 转氨酶升高而碱性磷酸酶和胆红素正常，   │ 15px / 400 / 23
│ 通常提示肝细胞受累，而不是胆道淤积。   │
│                                      │
│ 进一步检查                            │ 15px / 600
│ · 复查肝功能，观察是否持续升高         │ 14px / 400 / 21
│                                      │
│ 生活建议                              │ 15px / 600
│ · …                                  │
│                                      │
│ 长期跟踪                              │ 15px / 600
│ 建议持续记录：ALT、AST                │ 14px / 400 / 21
│                                      │
│ 本次判据（4 项）                 展开 ⌄ │ 12px / muted
│ 依据来源  AASLD · R 值分型 …           │ 12px / muted
│           [打开] [复制]                │
└──────────────────────────────────────┘
```

- 新分析：默认不显示 `bridge_text`；点“本次判据”后在“查看依据”里显示，并套用发现 2 的相对时间转换。
- 旧分析：如果 `explanation` 为空、只有 `bridge_text`，仍把 `bridge_text` 作为正文兜底（现有兼容逻辑保留，`apps/mobile/src/analysis/components.tsx:95-108`）。
- 四个小标题统一 15px/600，正文 14px/21；主结论正文 15px/23；组与组之间至少 12px 间距。
- 不再用 13px 标题 + 13px 正文这种只差字重的组合。

方案 B（如果产品坚持让 `bridge_text` 可见）：

- 把它放在结论正文之后，标签改为“本次判读依据”，字号 12px、颜色 `colors.muted`，不加粗。
- 明确说明它“包含用于判断的正常值”，避免用户把它当成异常列表。
- 仍然建议默认折叠，因为它与“偏离参考范围”重复。

**验收标准**

- 有 `explanation` 的新分析，卡片第一段可见文字是结论正文，不是 `bridge_text`。
- `bridge_text` 只出现在“本次判据 / 查看依据”展开区，且已做相对时间转换。
- 四个小标题与正文在字号/字重/间距上有明显差异；390×844 截图人工确认可扫读。
- 旧分析只有 `bridge_text` 时仍能显示，不崩溃。
- “查看依据”展开/收起、`accessibilityState.expanded` 仍正常（`apps/mobile/src/analysis/components.tsx:117-126`）。

### 发现 4：同一份报告既“纳入”又“早于分析窗口”，窗口口径没有讲清

**现象**

本次分析 `analysis 15` 的实际数据：

- 窗口：`{ months: 12, from: "2025-11-04", to: "2026-11-04", sparse: true }`
- 本次范围卡列出 5 份报告，包含 **2025-09-28** 和 **2025-09-24**：
  - `2026-11-04`、`2026-09-04`、`2026-05-16`、`2025-09-28`、`2025-09-24`
- 未纳入项 7 条，全部是 `outside_window`，且全部来自 **2025-09-28**：
  - 总胆红素测定、直接胆红素测定、间接胆红素、丙氨酸氨基转移酶测定、天门冬氨酸氨基转移酶测定、白蛋白测定、碱性磷酸酶测定
- 但同一份 2025-09-28 报告的 GGT、总蛋白、球蛋白、白蛋白/球蛋白，又以历史趋势点的形式出现在页面里。

页面文案：

- 窗口行只写 `2025-11-04 至 2026-11-04（部分指标历史数据较少）`（`apps/mobile/src/analysis/facts.ts:73-76`）。
- 范围卡写“纳入 5 份报告”（`apps/mobile/src/analysis/facts.ts:78-83`），并逐份列出报告日期（`apps/mobile/src/analysis/components.tsx:18-22`）。
- 未纳入项原因写“早于分析窗口”（`apps/mobile/src/analysis/facts.ts:29`）。

用户看到的就是：**2025-09-28 的报告在“本次范围”里，但它的胆红素又说“早于分析窗口”**。

**根因**

1. **窗口锚点是分析时刻，不是最近报告日期。** `from = now - months`（`internal/analysis/facts.go:23-24`）。本次 `2026-11-04 - 12 个月 = 2025-11-04`。2025-09-28 比窗口起点早 **37 天**，距锚点约 **13 个月 7 天**。它不是“最近 12 个月内”的报告。
2. **稀疏扩展是按趋势键单独生效的，不是全局窗口。** 某趋势键在 12 个月内有效点少于 2 个时，该趋势键的 `effectiveFrom` 扩展到 24 个月，最多保留 4 个点（`internal/analysis/facts.go:200-208`）。但 `Window.From` 仍然是 12 个月的起点。于是：
   - GGT、总蛋白、球蛋白、白蛋白/球蛋白在 12 个月内只有 1 个点 → 扩展到 24 个月 → 2025-09-28 的点被纳入。
   - 总胆红素在 12 个月内已有 3 个点（2026-05-16、2026-09-04、2026-11-04）→ 不扩展 → 2025-09-28 的点被排除。
   - 直接胆红素、间接胆红素、ALT、AST、白蛋白、ALP 同理，都已有足够的窗口内点，所以 2025-09-28 的对应点被排除。
3. **`facts.reports` 是“贡献了任意一个证据点的报告集合”，不是“整份报告都被完整纳入”。** `facts.Reports` 从所有 `evidencePoints` 收集报告引用（`internal/analysis/facts.go:89`、`:392-411`）。所以 2025-09-28 因为 GGT 等趋势点进入列表，但它的其他指标仍可能被排除。页面把它展示成“纳入 5 份报告”，语义过强。
4. **契约描述与实现不一致。** OpenAPI 说 `window.from` 是“实际使用的窗口起点；稀疏扩展时向前到 24 个月”（`schemas/openapi.yaml:1445-1448`），但实现和测试都保持 12 个月的 `from`，只把 `sparse` 置为 true（`internal/analysis/facts.go:200-208`；`internal/analysis/facts_test.go:86-88`）。这会让任何客户端误以为全局窗口已经变成 24 个月。

**用户为什么困惑**

- 用户按“最近一年”理解“12 个月”，把 2025-09 当成“最近”；实际窗口起点是 2025-11-04，2025-09-28 已经出界 37 天。
- 同一份报告在“本次范围”里，但它的指标又在“未纳入项”里，看起来像自相矛盾。
- “肝功项目包含胆红素”与“胆红素被排除”并不矛盾：项目范围决定“看哪些指标”，时间窗口决定“看哪些日期”。但页面没有把这句关键区别说出来。
- “早于分析窗口”没有说明是“早于 12 个月常规窗口”，还是“早于该指标扩展后的 24 个月窗口”。用户无法判断到底是哪一个。

**建议**

方案 A（推荐，先改文案与分组，不改接口）：

1. 顶部窗口行改为：

   ```text
   分析窗口：2025-11-04 至 2026-11-04（12 个月）
   部分指标历史数据较少，最多向前参考至 24 个月
   ```

   如果不想写“24 个月”这个常量，就写“部分指标因历史数据较少，已向前扩展参考范围；具体见各指标标注”。不要只写“部分指标历史数据较少”这种不解释后果的尾巴。

2. 范围卡标题从“本次范围”改为“本次使用的数据”，摘要从“纳入 5 份报告”改为“涉及 5 份报告中的部分指标、1 份病历”。把报告分成两组：

   ```text
   本次使用的数据
   涉及 5 份报告、1 份病历

   窗口内（3 份）
     2026-11-04 · 生化
     2026-09-04 · 生化
     2026-05-16 · 生化

   因历史数据较少额外参考（2 份）
     2025-09-28 · 生化
     2025-09-24 · 生化
   ```

   这两组可以直接用现有数据推导：`facts.reports` 中日期早于 `facts.window.from` 的报告就是“额外参考”，不需要新接口。

3. 未纳入项原因从“早于分析窗口”改为“超出该指标的时间窗口”。在该组下补一行日期：

   ```text
   超出该指标的时间窗口   7 项
   总胆红素测定、直接胆红素测定、间接胆红素 等
   来自 2025-09-28 的报告
   ```

   底部说明改为：

   > 同一份报告里的不同指标可能使用不同窗口：历史数据充足的指标只看最近 12 个月；历史数据不足的指标才向前扩展。未纳入项不作为本次结论依据。

4. 在“本次使用的数据”卡底部加一句：

   > 项目范围决定看哪些指标，时间窗口决定看哪些日期；两者独立。

方案 B（如果产品要求精确到每个指标的窗口）：

- 在 API 增加 `window.extended_from`，或在 `History` / `Exclusion` 上增加 `effective_from` / `window_kind`，让前端显示“该指标使用了 12 个月 / 24 个月窗口”。
- 这是契约变更，要同步 OpenAPI、Go 类型、App codegen、HTTP fixture、RNTL fixture（见 `docs/agents/contracts.md`）。
- 只有在产品明确要展示“每个指标的窗口徽标”时才值得做；当前用报告日期分组已经能消除主要歧义。

**验收标准**

- 用 `analysis 15` 的 fixture 渲染时，窗口行显示 `2025-11-04 至 2026-11-04`，并说明部分指标最多参考 24 个月。
- 范围卡显示“窗口内 3 份 / 额外参考 2 份”，2025-09-28 不再被笼统写成“纳入”。
- 未纳入项显示“超出该指标的时间窗口”，并带 `2025-09-28` 日期。
- 页面同时出现“2025-09-28 在额外参考组”和“其胆红素在未纳入组”时，文案能解释为什么，不产生直接矛盾。
- 若修正 OpenAPI 的 `window.from` 描述，`npm run codegen` 后类型不变，Go / App 测试全绿。

### 发现 5：来源链接是纯文本，不能打开也不能复制

**现象**

- “查看依据”展开后，每个来源被拼成一个 `Text`：
  - `standard · note · url`（`apps/mobile/src/analysis/components.tsx:129-134`）。
- 该 `Text` 没有 `onPress`、没有 `selectable`、没有 `Linking` 调用；字号 11px、颜色 muted（`apps/mobile/src/analysis/components.tsx:182`）。
- 当前代码库没有任何 `Linking` / `Clipboard` 使用（`apps/mobile/src` 内检索无结果）；`expo-linking` 已在依赖中（`apps/mobile/package.json:12`），`expo-clipboard` 未安装（`apps/mobile/package.json:6-23`）。

React Native 的 `Text` 默认 `selectable=false`，不开启原生选择/复制 [S7]；没有 `onPress` 也不会打开浏览器。所以用户既点不开，也长按复制不了。

**根因**

- 来源最初只是“展示标准名”，实现成了脚注式纯文本；后续没有补交互。
- `sourceToggle` 本身是可访问的按钮（`Pressable` + `accessibilityRole="button"` + `accessibilityState.expanded`，`apps/mobile/src/analysis/components.tsx:117-126`），但展开后的 URL 不是链接。
- 没有把“打开来源”和“复制链接”当成两个明确的用户任务。

**用户为什么困惑**

- URL 以 11px 灰色文字出现在卡片底部，看起来像装饰性脚注，不像可操作元素。
- 用户想查证标准时，只能手动抄写或长按（但默认不可选），在手机上几乎不可行。
- 读屏用户听到的是一整串“标准 · 说明 · https…”，没有“链接”角色，也没有“打开”提示。

**建议**

方案 A（推荐）：用 `expo-linking` 打开，用 `expo-clipboard` 复制。

- 打开：`import * as Linking from 'expo-linking'`，调用 `Linking.openURL(url)`。`expo-linking` 已经安装（`apps/mobile/package.json:12`），Expo 官方文档说明 `openURL` 会在系统默认浏览器中打开 URL [S10][S11]。
- 复制：`npx expo install expo-clipboard`，调用 `Clipboard.setStringAsync(url)`；Expo 官方文档提供 `setStringAsync` 示例 [S12]。
- 只允许 `http://` / `https://`；其他 scheme 不打开，避免来源配置错误跳到未知应用。
- 打开失败时给一句可执行的反馈：“无法打开链接，请复制后到浏览器打开。”
- 复制成功后显示“已复制”，并调用 `AccessibilityInfo.announceForAccessibility('链接已复制')` [S9]；Android 可同时给确认文本 `accessibilityLiveRegion="polite"` [S8]。

建议的来源行布局：

```text
AASLD
R 值分型与肝细胞型定义
https://www.aasld.org/...            打开 ↗   复制
```

交互与可访问性：

- URL 用 `Pressable` 包裹，`accessibilityRole="link"`。RN 文档明确说 `link` 角色用于“打开外部 URL 或网页”的交互元素 [S8]。
- `accessibilityLabel` 不要只读 URL，读“AASLD，R 值分型与肝细胞型定义，打开来源”；`accessibilityHint="在浏览器中打开"`。
- “复制”用独立 `Pressable`，`accessibilityRole="button"`，`accessibilityLabel` 为“复制 AASLD 来源链接”。
- 触控区域至少 44×44pt [S6]；必要时加 `hitSlop`。
- 链接不要只靠颜色区分；保留下划线或“打开”文字，满足“不能只靠颜色传达信息”的无障碍原则 [S5]。
- Apple HIG 的 VoiceOver 指南提到用户会用转子按“链接”导航；把外部 URL 标成 link 角色能让它进入转子的链接列表 [S4]。

方案 B（不新增原生依赖的兜底）：

- 打开：仍用 `expo-linking`。
- 复制：给 URL `Text` 加 `selectable`，让用户使用系统选择/复制 [S7]；同时显示“长按选择复制”的提示。
- 缺点：发现性差、长按选择在长 URL 上体验差，且不能一键复制。只建议作为临时方案。

**验收标准**

- 点击来源 URL 会调用 `Linking.openURL`，参数与 `source.url` 完全一致；测试中 mock `expo-linking` 断言调用。
- 点击“复制”会调用 `Clipboard.setStringAsync`，参数为完整 URL；显示“已复制”并触发读屏播报。
- URL 元素有 `accessibilityRole="link"`，复制按钮有 `accessibilityRole="button"`，两者都有可读的 `accessibilityLabel`。
- 触控区域不小于 44×44pt；Android 真机可点、可复制。
- 打开失败有可见反馈，不静默失败。
- `npm test -- analysis`、`npm run lint -- --max-warnings=0`、`npx tsc --noEmit` 全绿。

### 发现 6：“偏离 4 项”和“历史比较 14 项”不是同一口径

**现象**

本次 `analysis 15`：

- `deviations.length = 4`：
  - 丙氨酸氨基转移酶测定 83.5 high（2026-11-04）
  - 天门冬氨酸氨基转移酶测定 45 high（2026-11-04）
  - 总蛋白测定 60.70 low（2026-09-04）
  - 白蛋白测定 35.50 low（2026-09-04）
- `history.length = 14`：所有有有效点的趋势键，包含正常、`unknown`、以及只有 1 个点的系列。
- 14 条历史中：
  - 3 条只有 1 个点（胆汁酸、谷草/谷丙比值、铜蓝蛋白），无法比较变化。
  - 2 条最近点 `flag=unknown`（谷草/谷丙比值、间接胆红素），被列入“无法与参考范围比较”。
  - 4 条最近点异常，就是上面的 4 个偏离项。
- 页面标题分别写“偏离参考范围 4 项”和“与本人历史比较 14 项”（`apps/mobile/src/app/analysis/[analysis_id].tsx:183`、`:210`）。
- 历史行右侧的“`N 个有效点`”使用的是 `styles.flag`，颜色是 `colors.fail` 红色（`apps/mobile/src/app/analysis/[analysis_id].tsx:218`；样式定义 `apps/mobile/src/app/analysis/[analysis_id].tsx:358`）。

**根因**

- `facts.History` 对每个有有效点的趋势键都追加一条，不按“有变化/持续异常”筛选（`internal/analysis/facts.go:74`）。这是规格有意的，避免选择偏差（`.scratch/llm-analysis/spec.md:267`）。
- `facts.Deviations` 只在该趋势键的**最近点** flag 为 `high` / `low` / `abnormal` 时追加（`internal/analysis/facts.go:75-77`；`isDeviation` 定义 `:499-506`）。
- `History.Comparable` 为 `latest.Flag != unknown`（`internal/analysis/facts.go:251`），所以 `unknown` 也会进入 History，只是被单独列到 `Uncomparable`。
- 因此两个数字是“子集 vs 全集”的关系：**偏离 4 项是历史 14 项中最近点异常的 4 项**，不是两个并列的异常计数。

**用户为什么困惑**

- “偏离参考范围 4 项”和“与本人历史比较 14 项”并排出现，用户会以为后者是“14 项和历史比过”，甚至以为有 14 项异常。
- 14 项里包含正常项、无法比较项、只有 1 个点的项；标题没有说明这是“全部趋势”，不是“异常趋势”。
- “本次”也有歧义：4 个偏离项里，ALT/AST 来自 2026-11-04，总蛋白/白蛋白来自 2026-09-04，并不是同一份报告。
- “3 个有效点”用红色显示，看起来像异常标记，进一步加深“14 项都不正常”的错觉。

**建议**

方案 A（推荐，先改文案与样式）：

- 偏离标题改为：

  ```text
  各指标最近一次结果异常 4 项
  按每项最近一次有效结果统计；既往异常、最近已恢复的不在此列。
  ```

  比“偏离参考范围”更准确，也解释了为什么不是“最新报告 4 项”。

- 历史标题改为：

  ```text
  历史趋势 14 项
  包含所有有有效数据的指标；其中 3 项只有 1 个点，暂无法比较。
  其中 4 项最近一次结果异常，已在上方列出。
  ```

- 历史行的“`N 个有效点`”改为“`共 N 个点`”，使用 `styles.mutedSmall` 或中性色，不再用红色的 `styles.flag`。
- “无法与参考范围比较”可以保留独立小节，但在历史标题下先用一句话说明“2 项最近结果为 unknown，无法与参考范围比较”（本次数据）。

方案 B（进一步分组，仍然只改前端）：

- 在历史标题下加一行摘要 chips：

  ```text
  有变化 11 · 仅 1 个点 3
  ```

  这 14 条里，11 条有 up/down 变化，3 条只有 1 个点、变化为 unknown；两组不重叠。另有 2 条最近点 `flag=unknown`（与上面两组可交叉），单独用一句话说明即可，不要把它们做成第三个互斥分组。

- 把历史行按两组展示：
  - “有变化（11）”
  - “仅 1 个点，暂无法比较（3）”（默认折叠）
- 不改变后端 `history` 的全量语义，也不按“有变化”过滤；只是展示分组，避免 14 条平铺。
- 如果不想做分组，至少保留方案 A 的标题、说明和中性色，已经能消除大部分误解。

**验收标准**

- 偏离标题为“各指标最近一次结果异常 4 项”，并有“按最近一次结果统计”的说明。
- 历史标题为“历史趋势 14 项”，说明中包含“3 项只有 1 个点”。
- 历史行不再用红色显示点数；`共 N 个点` 与异常 flag 在视觉上可区分。
- 在 `analysis 15` fixture 上，页面数字与 API 的 `deviations.length` / `history.length` 一致，且文案能解释两者关系。
- RNTL 覆盖：4/14 的 fixture、3 个单点项的说明、点数颜色/样式不依赖 `styles.flag`。

## 3. 用户 / 产品角色扩展评审

### 3.1 真机核对：首屏实际看到什么

核对条件：Android 真机，1080×2400、density 440（约 393×873dp），Expo Go，2026-11-04 19:55–19:57。截图右上角的灰色齿轮是 Expo Go 开发入口，不属于产品 UI，以下判断不考虑它。

实际页面顺序与首屏占位：

| 位置 | 真机实际内容 | 代码 |
| --- | --- | --- |
| App bar | “肝功分析” + 返回 | `apps/mobile/src/app/analysis/[analysis_id].tsx:119` |
| 正文标题 | 又一次“肝功分析”，与 App bar 重复 | `apps/mobile/src/app/analysis/[analysis_id].tsx:153-155` |
| 元信息 | “小虎 · 纳入 5 份报告、1 份病历” | `apps/mobile/src/app/analysis/[analysis_id].tsx:156-158` |
| 窗口 | “2025-11-04 至 2026-11-04（部分指标历史数据较少）” | `apps/mobile/src/app/analysis/[analysis_id].tsx:159-161` |
| 本次范围 | 5 份报告（含 2025-09-28、2025-09-24）+ 病历 2026-02-14，占约 176dp | `apps/mobile/src/analysis/components.tsx:15-28` |
| 偏离参考范围 | 4 张卡；首屏只显示到第 4 张（白蛋白）的上半部分 | `apps/mobile/src/app/analysis/[analysis_id].tsx:181-206` |
| 判读标签 | 5 张卡：转氨酶分级、AST/ALT、BMI、BMI 状态、BMI z 值 | `apps/mobile/src/app/analysis/[analysis_id].tsx:208` |
| 与本人历史比较 | 14 张卡；每张右侧“N 个有效点”均为红色 | `apps/mobile/src/app/analysis/[analysis_id].tsx:210-233` |
| 相关病历 | “诊断 肝功能不全”“处理 服药治疗…甘草酸二铵 150mg…”无“医生当时”限定 | `apps/mobile/src/app/analysis/[analysis_id].tsx:235-256` |
| 无法与参考范围比较 | 2 项，只有名称/数值/日期，没有原因 | `apps/mobile/src/app/analysis/[analysis_id].tsx:258-272` |
| 未纳入项 | “早于分析窗口 7 项”，只显示 3 个名称 + “等”，无报告日期 | `apps/mobile/src/app/analysis/[analysis_id].tsx:274-293` |
| 解读 | `T-0 时…` 以 15px/600 显示在最上方；“可能原因/进一步检查/长期跟踪”均 13px/600，正文 13px/400 | `apps/mobile/src/analysis/components.tsx:102-116`、`:177-180` |
| 依据 | “收起依据”是蓝色按钮；展开后 URL 是灰色 11px 纯文本 | `apps/mobile/src/analysis/components.tsx:117-137` |
| 底部 | “今日剩余解读次数 17 次”“这不对”“内容来自…不是诊断” | `apps/mobile/src/app/analysis/[analysis_id].tsx:301-325` |

`uiautomator dump` 的 clickable 节点只有：返回、`收起依据`、`这不对`。报告日期、指标数值、病历、URL 都不是可点击节点；所以“查看原始报告”“打开来源”“分享/重跑/改时间范围”目前都没有入口。

首屏结论很直接：**用户在 393×873dp 的第一屏里看不到任何“这意味着什么”**。第一屏被重复标题、重复范围摘要、5 份报告日期和 4 张红色异常卡（第 4 张只露一半）占满；判读标签、历史趋势、病历和解读都在下一屏之后。Apple HIG 的 Layout 指南要求按相对重要性排序内容 [S1]；当前页面把“资料清单”放在了“答案”前面。

### 3.2 A. 用户角色（家长，非医学专业）

#### A1. 首屏是“资料页”，不是“答案页”

**现象**

- 正文标题与 App bar 重复（`apps/mobile/src/app/analysis/[analysis_id].tsx:153-155`）；元信息和“本次范围”卡又重复“纳入 5 份报告、1 份病历”（`:156-158`、`apps/mobile/src/analysis/components.tsx:15-17`）。
- 首屏只到第 4 张偏离卡的上半部分；解读在页面第 295 行，判读标签在第 208 行。
- 先出现红色“偏高/偏低”，再出现“转氨酶升高：轻度”的判读，最后才出现“可能原因”。用户先被“异常”吓到，再得到解释。

**用户影响**

- 家长点“分析”想要的是“这严重吗、下一步做什么”，第一眼却看到报告日期和红字。
- 没有结论锚点，红色 flag 会被放大成焦虑；用户可能截图发给医生，但截图里没有解读。
- 重复标题/范围让首屏显得冗长，实际信息密度低。

**建议**

- 删除正文里的重复标题，App bar 已足够；把成员/窗口压成一行。
- “本次范围”默认折叠成一行摘要（“涉及 5 份报告、1 份病历”），展开才看日期。
- 首屏先给“结论 + 判读”，再给“异常数值”；至少在“偏离参考范围”上方加一句一句话总结，例如“本次主要是转氨酶轻度升高；总蛋白、白蛋白偏低尚未被本条解读覆盖”。
- 参考 Apple HIG Layout 的“按相对重要性排序” [S1]。

**优先级：P0**

**验收标准**

- 393×873dp 首屏无需滚动即可看到“解读”标题、结论首句和至少一个下一步/判读标签。
- App bar 与正文不再出现两次“肝功分析”；“纳入 5 份报告、1 份病历”只出现一次。
- 本次范围默认折叠，展开后仍能看到全部报告/病历。

#### A2. 4 个异常里只有 2 个被解读，另外 2 个没有“这意味着什么”

**现象**

- `deviations` 有 4 项：ALT 83.5 high、AST 45 high、总蛋白 60.70 low、白蛋白 35.50 low。
- 解释层的 `evidence_ids` 只覆盖 ALT、AST、ALP、总胆红素；总蛋白、白蛋白不在其中（本次 API `evidence_ids` = `ev-ffffffffffff-252..255`，对应 ALT/AST/ALP/TBIL）。
- 框架层也只覆盖 ALT/AST（转氨酶分级、AST/ALT 比值），没有总蛋白/白蛋白标签。
- 页面没有“本条解读覆盖了哪些异常”的说明；用户看到“解读”会以为 4 项都被解释了。
- 按 spec 用户故事 19/26/28（`.scratch/llm-analysis/spec.md:64`、`:71`、`:73`），每个异常都应有可核对的说明，而不是只给 flag。

**用户影响**

- 家长最需要“这个低值要不要紧”的两项，恰好没有任何系统解释；可能以为不重要，或以为页面漏了。
- “解读”看起来完整，实际只覆盖一半异常，降低对整个结果的信任。
- 也容易把“肝功能不全”的病历诊断与 4 项异常混在一起，自行建立因果。

**建议**

- 在解读卡顶部加“本次解读覆盖 2/4 项异常”的覆盖提示；在每个偏离卡上标“已解读 / 未覆盖”。
- 对未覆盖项给固定、非诊断的说明：“本条解读针对转氨酶与胆红素；总蛋白、白蛋白偏低未纳入本条解读，请结合病历或医生意见。”
- 中期增加针对低蛋白/白蛋白的审核条目（需要医学审核），不要用模型自由补写。
- 覆盖关系直接由 `ExplanationSection.evidence_ids` 与 `Facts.Deviations` 求交集，不需要新接口。

**优先级：P0**

**验收标准**

- `analysis 15` fixture 上显示“覆盖 2/4 项”；总蛋白、白蛋白卡片明确标“未覆盖”。
- 每个偏离项都有“已解读/未覆盖”状态，不存在无说明的孤儿异常。
- 新增文案经过医学审核；RNTL 覆盖 4 项异常、2 项覆盖的用例。

#### A3. 病历原文没有“医生当时”的归属，药物剂量可能被当成建议

**现象**

- “相关病历”卡片直接显示“诊断 肝功能不全”“处理 服药治疗不适随诊 / 甘草酸二铵（天晴甘平）/ 每次量：150mg，口服，日三次”；标签只有“诊断/处理”，没有“医生当时”。
- 页面底部才有“不是诊断”的声明（`apps/mobile/src/app/analysis/[analysis_id].tsx:323-325`），离病历卡很远。
- 规格要求不改写医生诊断（用户故事 32，`.scratch/llm-analysis/spec.md:80`），系统不推荐药物/剂量（用户故事 35，`:83`）；但“原文展示”与“像建议”之间缺少一层归属说明。

**用户影响**

- “肝功能不全”可能被读成系统给孩子的诊断，而不是 2026-02-14 那次医生的记录。
- “150mg，日三次”可能被当成当前用药建议；家长照着旧处方用药是真实风险。
- 用户无法判断这段话是“医生说的”还是“系统说的”，违反“可信、不过度诊断”的产品边界。

**建议**

- 标签改为“医生当时诊断（2026-02-14 病历原文）”“医生当时处理（原文，不是本次建议）”。
- 在病历卡内加固定一句：“以下是当时病历原文，不是本次分析的建议；请勿按旧处方自行用药。”
- 药物/剂量细节可以默认折叠在“查看处理原文”里，但不要删除原文（保留可追溯）。
- 在解读卡附近加一行简短“这是解读，不是诊断”，不要只依赖页面底部的长声明。

**优先级：P0**

**验收标准**

- 病历卡出现“医生当时诊断/处理”归属和“历史处方，勿自行用药”提示。
- 页面同时保留医生原文，不改写、不摘要。
- 医学/合规 review 通过；截图人工确认“诊断/处理”不再像系统结论。

#### A4. 正常值也会被写成“升高/窗口内最高”，制造不必要焦虑

**现象**

- 总胆红素 15 μmol/L（参考 0–26，normal）在历史卡里显示“较上次升高 · 本次为窗口内最高”。
- GGT 7 U/L（normal）在历史卡里显示“较上次降低 · 本次为窗口内最低 · 历史数据较少”。
- 每张历史卡右侧的“N 个有效点”都是红色（`apps/mobile/src/app/analysis/[analysis_id].tsx:218`、`:358`）。
- 用户故事 20 要求“看到升高/降低、是否为最高/最低，以便判断变化”（`.scratch/llm-analysis/spec.md:65`）；当前只给了方向和极值，没有给“是否仍正常”的判断语境。

**用户影响**

- “升高”“最高”在没有“仍正常”限定时，会被读成恶化；正常值也可能引发焦虑。
- 红色“N 个有效点”让 14 张卡看起来都像异常，强化“14 项都不正常”的误解。
- 家长无法快速区分“数值在变但正常”和“数值异常且需要关注”。

**建议**

- 每张历史卡都显示最近一次 flag 的人话状态：“本次在参考范围内”/“本次偏高”/“本次偏低”/“无法比较”。
- 改变措辞：“较上次升高，仍在参考范围内”“为本窗口最高值（仍正常）”；只有异常时才用警示色。
- 点数改成中性色“共 N 个点”；红色只留给真正的异常 flag。
- 对“历史数据较少”的卡片，用中性说明，不要和异常红混在一起。

**优先级：P1**

**验收标准**

- 每张历史卡都有“本次是否在参考范围内”的状态。
- 正常值不再出现红色或“最高/升高”单独构成的警示文案。
- `analysis 15` 的 TBIL/GGT 两张卡人工确认不再引发“恶化”误读。

#### A5. 术语与单位噪声：ULN、ALT/AST、`1.41 1`、`umol/L`

**现象**

- 框架卡写“ALT 2.09×ULN”“AST 1.29×ULN”，其说明写“转氨酶升高 2–5×ULN”。ULN 是“正常上限”的缩写，页面没有解释。
- 长期跟踪写“建议持续记录：ALT、AST”，但上面卡片用全称“丙氨酸氨基转移酶测定/天门冬氨酸氨基转移酶测定”。
- 历史卡出现“1.41 1”“1.20 1”：无量纲单位 `1` 被原样拼在数值后（`apps/mobile/src/analysis/facts.ts:94-100`）。
- 解释卡桥接文本用 `umol/L`，卡片用 `μmol/L`；同一页出现两种写法。

**用户影响**

- 非医学家长不知道 ULN；看到“2.09×ULN”只能跳过，判读标签失去意义。
- “1.41 1”看起来像排版错误，降低对数据准确性的信任。
- 同一指标全称/缩写混用，增加认知负担。

**建议**

- 首次出现用“丙氨酸氨基转移酶（ALT）”“天门冬氨酸氨基转移酶（AST）”，后文再用缩写。
- “ULN”改成“正常上限”或“参考上限”；“2.09×ULN”写成“2.09 倍正常上限”。
- `unit` 为 `1` 或空时不要拼单位；统一使用 `unit_display`（μmol/L）。
- 长期跟踪的 `track_items` 映射到展示名，不只显示 ALT/AST 缩写。

**优先级：P1（文案/单位）／P2（全局术语表）**

**验收标准**

- 页面不出现未解释的裸“ULN”；不出现“ 1”这种无量纲单位。
- 同一指标的缩写/全称/单位在页面内一致。
- 新增一个展示名映射单测，覆盖 ALT/AST/ALP/TBIL。

#### A6. 操作不顺手：不能核对原文、不能分享、不能重跑、不能改范围

**现象**

- `uiautomator` 可点击节点只有返回、`收起依据`、`这不对`；报告日期、数值、病历、URL 都不可点。
- “本次范围”列出报告日期，但不能打开报告详情；偏离卡有日期，但不能定位到原始报告；病历卡也不能打开病历详情。
- 页面没有分享、导出、重新分析、调整 6/12/24 个月窗口的入口；“这不对”也没有 `accessibilityRole="button"`（`apps/mobile/src/app/analysis/[analysis_id].tsx:307-316`）。
- 结果页没有“分析生成时间”，用户无法判断这次分析有多旧。

**用户影响**

- 用户想核对“83.5 是不是报告上的值”时，只能返回项目页再找报告，路径断裂。
- 想发给医生/家人时没有分享入口；想更新数据时只能重新走入口流程。
- 看到结论后想改 6/24 个月窗口，必须返回；不符合“看完再调整”的自然路径。
- “这不对”含义模糊：是数据不对、解读不对，还是整页不对？

**建议**

- P1：报告日期、偏离卡日期、病历卡都链到 `/report/[report_id]`；使用现有详情页（`apps/mobile/src/app/panel/[panel].tsx:159` 已是同样模式）。
- P1：把“这不对”改成“解读不对？”或“反馈问题”，加 `accessibilityRole="button"`；在解释卡内提供“这段不对”并按 `entry_id` 反馈。
- P1：`stale` 时显示“基于最新数据重新分析”；复用 `POST /analyses`，保留当前 panel/window。
- P2：结果页加时间范围切换（复用 `apps/mobile/src/analysis/TimeRangeModal.tsx:31-64`），创建新分析。
- P2：分享摘要（文字/图片）并自动带“不是诊断”的声明；导出 PDF 更后。

**优先级：P1（报告链接、反馈语义、stale 重跑）／P2（范围切换、分享/导出）**

**验收标准**

- 点击报告日期/病历卡能打开对应报告详情；返回后仍停留在原分析页。
- `stale` 结果有“重新分析”入口，点击后带着原 panel/window 创建新分析。
- “这不对”有按钮角色和明确范围；解释卡可按 entry_id 反馈。
- 结果页可以切换 6/12/24 个月并创建新分析（不覆盖旧快照）。

### 3.3 B. 产品角色（产品维护者 / 开发者）

先给结论：当前页面**基本守住了“不过度诊断”**——解释正文来自审核条目，底部有固定声明；但**“可信”和“可追溯”只完成了一半**：来源存在但不可操作，报告存在但不可达，医生记录与系统解读没有“说话人”区分，快照版本信息没有露出。按 spec 的用户故事 31/32/35（不诊断、不改写医生、不推荐药物，`.scratch/llm-analysis/spec.md:79-83`）和 40/41（快照可复现，`:91-92`），这三条边界需要分别落到页面上，而不是只存在于后端。

#### B1. 三层输出在代码里分开，在页面上没有形成“结论→判读→证据→局限”的层级

**现象**

- 事实层（范围、偏离、历史、病历、未纳入）先占满页面；框架层夹在偏离和历史之间；解释层在最后（`apps/mobile/src/app/analysis/[analysis_id].tsx:179-295`）。
- 页面没有“结论/判读/数据/局限”这样的用户语言分组；三层只体现在区块标题上。
- “判读标签”里的 BMI 三张卡与肝功两张卡同权重，用户看不出哪些是本次肝功的核心判读。

**产品影响**

- 三层模型在工程上是清晰的，但用户看到的是“报告清单 + 异常清单 + 最后一段解释”，不是“结论 → 依据”的推理链。
- 解释层被事实层淹没，产品的核心差异（LLM 解读）不可见。
- 框架层和解释层的边界不清：用户不知道“判读标签”是代码规则，“解读”是模型组织 + 审核条目。

**建议**

- 用用户语言重组为六层：结论 → 判读 → 异常与变化 → 医生记录 → 范围与局限 → 操作。
- 不把“事实层/框架层/解释层”这些内部词显示给用户；只在代码/文档里保留。
- 判读标签按领域分组：肝功判读（2 项）与体格发育（3 项，默认折叠或合并为 1 张 BMI 卡）。
- 解释卡内标注“系统解读（依据审核条目）”，病历卡标注“医生记录（原文）”。

**优先级：P0**

**验收标准**

- 页面区块顺序与目标 IA 一致；首屏是结论/判读，不是范围卡。
- 判读标签按领域分组；BMI 不再以 3 张同权重卡片出现。
- 页面不出现“事实层/框架层/解释层”等内部术语。

#### B2. 可信/可追溯：来源、报告、快照都没有形成可操作的证据链

**现象**

- 框架卡的来源只显示标准名（`apps/mobile/src/analysis/framework.ts:15-17`、`apps/mobile/src/analysis/components.tsx:63-65`），没有 note/URL，也不能点。
- 解释卡的来源藏在“查看依据”后面，展开后是纯文本 URL（`apps/mobile/src/analysis/components.tsx:117-137`）。
- 顶层 `AnalysisResult.sources` 没有任何 App 使用；来源分散在框架卡和解释卡里。
- 报告日期/指标日期/病历都没有 `report_id` 链接；`facts.reports` 有 `report_id` 但只渲染日期/类型（`apps/mobile/src/analysis/components.tsx:18-22`）。
- 结果页没有 `created_at`；`Record` 有 `CreatedAt`（`internal/analysis/store.go:17-31`），HTTP `AnalysisResult` 没有（`internal/httpapi/analysis.go:35-42`）。

**产品影响**

- 用户无法完成“这个数从哪来 → 哪份报告 → 哪条标准”的核对链；这直接削弱“可信”和“可追溯”。
- 框架卡的来源看起来比解释卡更正式（有“依据 ACG 2016”），但两者都不可点；解释卡的 URL 反而像脚注。
- 分析快照虽然后端永久保存，页面只读 `snapshot.member_name`（`apps/mobile/src/app/analysis/[analysis_id].tsx:157`），用户看不到“生成时间/规则版本/条目版本”，无法判断新旧。
- ADR-0012 明确“报告级只做聚合，分类落在指标项”（`docs/adr/0012-item-level-panels.md:5`）；页面却用报告级列表表达“纳入”，与数据模型不一致。

**建议**

- 统一 `SourceRow`：标准名 + 说明 + URL + 打开/复制；框架层和解释层共用。
- 解释卡即使折叠，也显示“依据 AASLD”这样的可见 chip；展开再显示完整来源。
- 报告日期、偏离卡日期、病历卡都链接到报告详情；数值卡可加“查看原报告”次级动作。
- 顶栏或“本次使用的数据”里加“分析生成于 …”，`stale` 时加“数据已更新”。
- 中期在快照里展示规则版本/条目版本（可折叠），支撑用户故事 40/41。

**优先级：P1**

**验收标准**

- 每个框架标签和解释段都有可打开/可复制的来源；折叠时至少能看到标准名。
- 每个报告日期/病历卡都能打开对应报告详情。
- 页面显示分析生成时间；`stale` 有明确提示。
- 来源/报告链接有 `accessibilityRole` 和可读 label；RNTL 覆盖。

#### B3. 状态没有设计完整：无条目、额度、旧分析、数据更新、条目更新、云端失败、部分拒绝

**现象**

当前状态处理如下：

| 状态 | 当前行为 | 缺口 |
| --- | --- | --- |
| 无条目 | `explanationUnavailableText` 返回“本次没有可引用的解读条目…”；只在解释层缺失时显示，且在页面底部（`apps/mobile/src/app/analysis/[analysis_id].tsx:296-300`） | 没有“为什么没有/下一步是什么”的结构；首屏看不到 |
| 额度不足 | “今日额度已用完，事实与判读仍可用”；显示剩余 0 次（`apps/mobile/src/app/analysis/[analysis_id].tsx:301-305`） | 没有恢复时间、没有“明天再试”或重试入口 |
| 旧分析 | 没有生成时间；只有 `stale` 时提示“数据已更新” | 用户不知道分析是今天的还是上个月的 |
| 数据更新 | `stale` 提示存在（`apps/mobile/src/app/analysis/[analysis_id].tsx:167-171`） | 没有“重新分析”按钮；只能返回重走入口 |
| 条目更新 | spec 承诺 `entry_updated=true` 显示“依据已更新”（`.scratch/llm-analysis/spec.md:271`；用户故事 47，`:98`） | API/App 都没有这个字段/状态 |
| 云端失败 | `llm_timeout` / `llm_error` / `llm_invalid_json` 等统一显示“本次解释暂不可用…”（`apps/mobile/src/analysis/framework.ts:22-45`） | 没有重试；用户不知道是临时网络还是配置问题 |
| 部分条目被拒 | `validateExplanation` 记录 `bridge_text_rejected` 事件，但响应只返回通过的 sections（`internal/analysis/explain.go:144-152`） | 用户不知道“有一段被省略”，会把不完整当完整 |
| 截断 | 显示“解释内容达到长度上限，已截断”（`apps/mobile/src/analysis/components.tsx:81-83`） | 没有说明缺了哪部分，但至少有提示 |

**产品影响**

- 用户无法区分“没有内容”“系统失败”“额度用完”“数据过期”，容易把系统问题当成医学结论缺失。
- 旧分析和条目更新没有时间/版本线索，违背用户故事 42/47 的“知道结论可能过时”。
- 部分拒绝静默，直接损害“可追溯”和“可信”。

**建议**

- 做一张状态矩阵：每个状态有标题、原因、影响范围（只影响解读/整页）、下一步动作。
- API 增加 `created_at`、`entry_updated`、`rejected_sections`/`partial`（或等价字段）；如果不想改契约，至少前端对已知 reason 给不同文案和重试入口。
- 增加“只重试解读”的端点或在现有分析上补写解释层；不要每次重试都创建新分析、消耗新配额。
- 无条目时给“可以带着这些数值问医生”的固定引导，并链接到“如何问医生”说明（如果产品有）。
- 部分拒绝时显示“有 1 段解读未通过校验，已省略”，不要静默。

**优先级：P1**

**验收标准**

- 上述 8 个状态各有不同文案/动作；RNTL 覆盖每个状态。
- 旧分析显示生成时间；条目更新显示“依据已更新”；部分拒绝显示提示。
- `stale` 有重新分析入口；云端失败有重试入口（额度允许时）。

#### B4. 默认折叠/分组/可视化：5 个判读、14 条历史、5 份报告、7 个排除项都在平铺

**现象**

- 判读标签 5 张卡，其中 3 张是 BMI：BMI 23.1、在参考范围内、BMI 年龄别 z 值与百分位。BMI 是成员级规则，除 `clinical_note` / `other` 外都会追加（`internal/analysis/framework.go:31-47`），但在肝功页与肝功判读同权重。
- 历史 14 张卡全部平铺；3 张只有 1 个点，2 张 unknown，仍然占同样高度。
- 本次范围 5 份报告全部展开；未纳入项只显示 3 个名称 + “等”（`apps/mobile/src/analysis/facts.ts:64-70`）；无法比较 2 项没有原因。
- 页面没有任何趋势可视化；14 张卡的文字“较上次升高/降低”需要逐行读。

**产品影响**

- 肝功页被 BMI 和全量历史稀释，用户找不到“肝功到底怎么样”。
- 14 条历史 + 7 个排除项造成长滚动；关键异常和关键变化被淹没。
- 只显示 3 个排除名称，用户无法核对“到底哪 7 项被排除”，可追溯性不足。
- 没有图，用户难以感知“升高/降低”的幅度和趋势。

**建议**

- 判读：按“肝功判读（2）/ 体格发育（3）”分组；BMI 三张合并为一张“体格发育”卡（BMI、百分位、状态三行），默认折叠。
- 历史：默认分“有变化 11 / 仅 1 个点 3”两组；“仅 1 个点”默认折叠；每行加“本次在参考范围内/异常”状态。
- 范围：默认折叠成一行；展开后按“窗口内 3 份 / 额外参考 2 份”分组。
- 未参与判断：把“未纳入 7 + 无法比较 2”合并为一个折叠区“未参与本次判断（7+2）”，按原因分组；每组显示全部名称/日期或“查看全部”。
- 可视化：对 ALT/AST/GGT/TBIL 等有 ≥2 个点的核心指标，加一个小 sparkline 或参考范围条；复用现有 `apps/mobile/src/reports/TrendChart.tsx` 的 SVG 能力，不要新引图表库。

**优先级：P1（分组/折叠/点数）／P2（sparkline、范围条）**

**验收标准**

- 默认页面只展示结论、2 个肝功判读、4 个偏离、11 条有变化趋势；BMI/单点趋势/范围/未参与项折叠。
- BMI 只出现一张卡（或一个分组标题），不再 3 张同权重卡。
- 未参与项可展开看到全部 7+2 项及原因/日期。
- 至少 ALT、AST、GGT、TBIL 有趋势可视化或明确的“查看趋势”入口。

#### B5. 功能缺口：报告原文、重跑、对比上次、导出/分享、时间范围

按“对可信/可追溯的贡献”排序，建议进入后续工单的缺口：

| 缺口 | 用户价值 | 现状 | 建议优先级 |
| --- | --- | --- | --- |
| 报告原文定位 | 核对数值、看 OCR 原文、确认日期 | 报告日期/数值/病历都不可点 | P1 |
| stale 重跑 | 新报告/核对后更新结论 | 只有提示，无动作 | P1 |
| 条目级反馈 | 指出“哪一段不对” | 只有结果级“这不对”，不传 entry_id | P1 |
| 时间范围切换 | 看完结果后改 6/12/24 个月 | 只能回入口重选 | P2 |
| 分享/导出 | 带给医生/家人 | 无分享/导出 | P2 |
| 对比上次分析 | 看两次分析之间变化 | 历史趋势已有部分替代 | P2（暂缓） |

其中“报告原文定位”和“stale 重跑”最值得先做：它们直接支撑“可信、可追溯”，而且现有路由/接口已经具备（报告详情页 `apps/mobile/src/app/report/[report_id].tsx`；`POST /analyses` 可复用）。

**优先级：P1（前两项、条目级反馈）／P2（范围、分享、对比）**

**验收标准**

- 报告日期/病历卡能打开详情；返回后保持分析页。
- `stale` 能一键重新分析；新分析有独立 analysis_id，不覆盖旧快照。
- 条目级反馈携带 `entry_id`，后台可按条目统计。
- 分享内容自动带“不是诊断”声明。

#### B6. 产品指标/观测：只有“反馈率/降级率”，缺“理解度/覆盖率/条目归因”

**现象**

- `AnalysisMetrics` 只有 `analyses`、`feedback`、`feedback_rate`、`degradations`、`degradation_rate`、`degradation_kinds`（`internal/httpapi/analysis.go:60-70`）；spec 4.13 的周检也只要求反馈率/降级率两个比率（`.scratch/llm-analysis/spec.md:362-366`）。
- 反馈只有结果级 `reason` 文本；API 支持可选 `entry_id`，但 App 当前不传（`schemas/openapi.yaml:1644-1656`；`apps/mobile/src/app/analysis/[analysis_id].tsx:93-102`）。
- `analysis_events` 有 `entry_miss`、`bridge_text_rejected` 等，但没有“解释覆盖率”“部分拒绝段数/entry_id”“用户是否看懂”的字段。
- `EventTrustGateFailed` 只有常量定义（`internal/analysis/explain.go:35`），全仓没有对应的 `record` 调用；spec 4.13 要求记录“信任门控全未通过”（`.scratch/llm-analysis/spec.md:364`），这条降级原因目前不会出现在 `analysis_events`。
- `analysis_llm_calls` 只有 `analysis_id` 和时间，没有耗时/状态/token（`internal/storage/migrations/0016_analysis_observability.sql:32-36`）。

**产品影响**

- 只能看到“有没有反馈/有没有降级”，看不到“解读覆盖了多少分析”“哪条条目最常被拒/被反馈”“用户是否看懂”。
- 反馈是自由文本，无法聚合；也没有正向信号（“有帮助/看懂了”），只能看到负面。
- 无法评估本次 IA 改版是否真的提升理解；也无法判断“无条目”是内容缺口还是匹配失败。
- `trust_gate_failed` 缺失会让降级率低估。

**建议**

- 增加解释覆盖率：`analyses_with_explanation / analyses`，按 panel 分组；`entry_miss` 已有，可同时展示。
- 增加部分拒绝指标：`rejected_sections` 数量 + `entry_id`，不要只写自由文本 detail。
- 反馈升级：结果级 + 条目级（`entry_id`）；原因用固定分类（数值不对/解释不对/病历冲突/看不懂/其他）+ 可选文本；增加正向“有帮助/看懂了”。
- 客户端事件（不含健康内容）：`source_opened`、`report_opened`、`share_started`、`rerun_started`、`range_changed`。
- 补 `trust_gate_failed` 的生产者；确认 `analysis_llm_calls` 是否需要延迟/状态字段。
- 周检看四个数：反馈率、降级率、解释覆盖率、条目级负反馈 Top N。

**优先级：P2（条目级反馈与 trust gate 可提前到 P1）**

**验收标准**

- `/analyses/metrics` 返回解释覆盖率、部分拒绝数、按 entry 的反馈统计（或等价查询）。
- 条目级反馈能写入 `entry_id`；原因分类可聚合。
- `trust_gate_failed` 在符合条件的分析上产生事件。
- 至少有一个看板/查询能区分“无条目”“LLM 失败”“部分拒绝”。

### 3.4 目标结果页信息架构

把“工程三层”翻译成“用户六层”：

| 层级 | 回答的用户问题 | 内容 | 默认状态 |
| --- | --- | --- | --- |
| 0. 页头 | 这是谁、什么时候、看多长时间 | 成员、Panel、分析生成时间、6/12/24 个月窗口 | 一行；窗口可改 |
| 1. 结论 | 这意味着什么、严重吗、下一步 | 解读四段式 + 覆盖 2/4 项 + “不是诊断” + 依据 chip | 展开；无解读时显示状态卡 |
| 2. 判读 | 规则怎么判 | 肝功判读 2 项；体格发育 1 张合并卡 | 肝功展开；体格折叠 |
| 3. 异常与变化 | 哪些数不对、在变好还是变差 | 4 个偏离（已解读/未覆盖）+ 有变化 11 / 单点 3 | 偏离展开；单点折叠 |
| 4. 医生记录 | 医生当时怎么说 | 医生诊断、处理原文（标注“当时/原文/不是建议”） | 展开；药物细节可折叠 |
| 5. 范围与局限 | 数据从哪来、没用什么 | 5 份报告（窗口内 3 / 额外 2）、未纳入 7 项 + 无法比较 2 项、来源 | 折叠；显示计数 |
| 6. 操作 | 我能做什么 | 反馈、分享、重新分析、改时间范围、查看原报告 | 底部；主操作不超过 3 个 |

一句话版本：

```text
[肝功分析] 小虎 · 12 个月 · 生成于 2026-11-04
[结论] 本次转氨酶轻度升高；总蛋白/白蛋白偏低未被本条解读覆盖。  [不是诊断]
[判读] 肝功 2 项 | 体格 1 项（展开）
[异常] 4 项（2 项已解读 / 2 项待医生） | 历史：有变化 11 / 单点 3
[医生记录] 2026-02-14 当时诊断：肝功能不全；处理原文（非建议）
[范围] 涉及 5 份报告（窗口内 3 / 额外 2）| 未纳入 7 + 无法比较 2
[操作] 反馈 · 分享 · 重新分析 · 时间范围
```

这个 IA 与第 4 节的页序表一致；区别是这里按“用户问题”分层，而不是按代码里的三层。

### 3.5 最值得做的 8 张后续工单

| # | 优先级 | 工单 | 范围 | 验收 |
| --- | --- | --- | --- | --- |
| 1 | P0 | 结果页 IA v2：结论优先、去重、分组折叠 | 删除正文重复标题；范围卡折叠；解读/判读前置；六层分组 | 首屏见结论；无重复标题/范围；RNTL 顺序断言 |
| 2 | P0 | 异常覆盖与病历归因 | 4 项异常标注已解读/未覆盖；低蛋白/白蛋白补审核条目或明确未覆盖；病历“医生当时诊断/处理” + 历史处方提示 | 覆盖 2/4 可见；每个异常有归属；病历卡安全提示；医学 review |
| 3 | P1 | 可追溯链接层：报告原文 + 指南来源 | 统一 `SourceRow`（打开/复制/读屏）；报告日期/数值/病历链到 report detail；框架来源也可见 | 所有来源可操作；报告可打开；accessibility label/role；RNTL |
| 4 | P1 | 窗口与排除透明度 | 报告按“窗口内/额外参考”分组；未纳入 7 项可展开看全部+日期；无法比较 2 项显示原因 | `analysis 15` fixture 无矛盾；全部名称/日期可查；文案 review |
| 5 | P1 | 状态与操作补齐 | 分析生成时间、`entry_updated`、部分拒绝提示、云端失败重试、stale 重跑、时间范围切换、分享摘要（P2） | 8 个状态各有文案/动作；分享带“不是诊断”声明；API/RNTL；旧快照兼容 |
| 6 | P1 | 历史趋势可读性 | 正常/异常状态词；点数中性色；`unit=1` 隐藏；有变化/单点分组；ALT/AST/GGT/TBIL sparkline | 正常值不再红色/“最高”单独警示；分组默认折叠；可视化至少 4 项 |
| 7 | P1/P2 | 术语、单位与 BMI 合并 | ULN→正常上限；ALT/AST 全称；μmol/L 统一；BMI 三卡合并为“体格发育”卡；track items 展示名 | 无裸 ULN、无“ 1”单位；术语一致；BMI 只 1 卡 |
| 8 | P2 | 反馈与观测升级 | 条目级反馈（entry_id）、原因分类、正向“看懂了”、覆盖率/部分拒绝/trust gate 指标、客户端 source/report 打开事件 | metrics 可查；entry_id 可聚合；trust gate 有事件；周检四率 |

这 8 张工单中，1–2 是“可信/不过度诊断”的底线，3–5 是“可追溯/可操作”的主干，6–8 是“长期可维护/可观测”的增益。建议不要一次全做：先做 1–2，再做 3–4，最后 5–8。

## 4. 建议的目标页序

下表把第 3 节的用户六层落到具体区块。

| 顺序 | 区块 | 默认状态 | 说明 |
| --- | --- | --- | --- |
| 1 | 标题 + 成员 + 范围摘要 | 展开 | 只保留一行元信息，不列报告清单 |
| 2 | 窗口行 | 展开 | `2025-11-04 至 2026-11-04（12 个月）` + 稀疏扩展说明 |
| 3 | 提示（stale / 证据不足 / 单指标扩展） | 条件显示 | 保持在结论之前，避免用户误读 |
| 4 | **解读** | 展开 | 结论、四段式；无解读时显示紧凑状态块 |
| 5 | 判读标签 | 展开 | 框架层，属于“解释”而非“原始数据” |
| 6 | 各指标最近一次结果异常 | 展开 | 原“偏离参考范围”，标题说明口径 |
| 7 | 历史趋势 | 展开 | 原“与本人历史比较”，说明包含全部趋势 |
| 8 | 相关病历 | 展开 | 医生诊断/处理原文 |
| 9 | 本次使用的数据 | 可折叠 | 报告按“窗口内 / 额外参考”分组 |
| 10 | 未参与本次判断 | 可折叠 | 未纳入项 + 无法比较；原因与日期 |
| 11 | 剩余次数 / 反馈 / 来源声明 | 展开 | 次级信息，放页面末尾 |

这个顺序只做两处大移动：**解读从第 9 位到第 4 位；范围卡从第 2 位到第 9 位**。另外把判读标签上移到偏离之前一位（它属于“解释”，不属于原始数据）。其余事实层保持原有相对顺序，diff 小、回归风险低。

如果产品希望医生病历更早出现，可以把“相关病历”移到“历史趋势”之前；但本次没有证据表明必须这样做，建议先按上表实施。

## 5. 实施计划（按优先级）

| 优先级 | 改动 | 主要文件 | 验证 |
| --- | --- | --- | --- |
| P0-1 | 解读卡移到通知区之后；无解读状态块同位置 | `apps/mobile/src/app/analysis/[analysis_id].tsx` | RNTL 顺序断言；Android 首屏 smoke |
| P0-2 | 解释卡重排：结论正文优先、四段式标题 15/600、正文 14/21；`bridge_text` 移入“本次判据/查看依据” | `apps/mobile/src/analysis/components.tsx` | 截图对比；旧快照兜底测试 |
| P0-3 | `T-0` / `T-Nm` 展示层转换 | `apps/mobile/src/analysis/facts.ts`、`components.tsx` | 单测覆盖 `T-0` / `T-3m` / `T-12m`；旧快照测试 |
| P1-1 | 计数标题、说明、点数中性色 | `apps/mobile/src/app/analysis/[analysis_id].tsx`、`facts.ts` | RNTL 4/14 fixture；视觉检查 |
| P1-2 | 窗口文案、报告分组、排除原因文案 | `apps/mobile/src/analysis/facts.ts`、`components.tsx`、`app/analysis/[analysis_id].tsx` | `analysis 15` fixture；文案 review |
| P1-3 | 来源行打开/复制/读屏 | `apps/mobile/src/analysis/components.tsx`、`apps/mobile/package.json` | mock `expo-linking` / `expo-clipboard`；Android 真机 |
| P2 | 修正 OpenAPI `window.from` 描述；如产品需要再增加 `extended_from` | `schemas/openapi.yaml`、Go 类型 / fixtures | `npm run codegen`；Go / App 全绿 |

实施注意：

- **不要**为了窗口分组新增分析接口字段；`facts.reports` + `facts.window.from` 已能区分“窗口内 / 额外参考”。
- **不要**改变事实层的筛选与排序规则（尤其是 History 的全量语义），本次只改展示口径。
- `expo-clipboard` 是原生模块；`npx expo install expo-clipboard` 后，现有 development build 需要重建（Expo Go 已内置该模块，但项目若用 dev client 需重新构建）。`expo-linking` 已在依赖中，打开链接不需要新增原生依赖。
- 按 repo 规则，合并 UI 前跑 `npm test`、`npm run lint -- --max-warnings=0`、`npx tsc --noEmit`，并按 `apps/mobile/ANDROID.md` 做真机 smoke。

## 6. 附录 A：`analysis 15` 的实际数据

来源：本地验证栈 `GET http://localhost:8080/api/v1/analyses/15`（2026-11-04）。以下为整理后的关键数据（数组与计数做了压缩）。

```json
{
  "window": { "months": 12, "from": "2025-11-04", "to": "2026-11-04", "sparse": true },
  "reports": [
    "2026-11-04", "2026-09-04", "2026-05-16", "2025-09-28", "2025-09-24"
  ],
  "deviations": [
    { "name": "丙氨酸氨基转移酶测定", "value": "83.5", "flag": "high", "report_date": "2026-11-04" },
    { "name": "天门冬氨酸氨基转移酶测定", "value": "45", "flag": "high", "report_date": "2026-11-04" },
    { "name": "总蛋白测定", "value": "60.70", "flag": "low", "report_date": "2026-09-04" },
    { "name": "白蛋白测定", "value": "35.50", "flag": "low", "report_date": "2026-09-04" }
  ],
  "history_count": 14,
  "uncomparable_count": 2,
  "excluded": [
    { "reason": "outside_window", "report_date": "2025-09-28", "name": "总胆红素测定" },
    { "reason": "outside_window", "report_date": "2025-09-28", "name": "直接胆红素测定" },
    { "reason": "outside_window", "report_date": "2025-09-28", "name": "间接胆红素" },
    { "reason": "outside_window", "report_date": "2025-09-28", "name": "丙氨酸氨基转移酶测定" },
    { "reason": "outside_window", "report_date": "2025-09-28", "name": "天门冬氨酸氨基转移酶测定" },
    { "reason": "outside_window", "report_date": "2025-09-28", "name": "白蛋白测定" },
    { "reason": "outside_window", "report_date": "2025-09-28", "name": "碱性磷酸酶测定" }
  ],
  "bridge_text": "T-0 时丙氨酸氨基转移酶测定 83.5 U/L、天门冬氨酸氨基转移酶测定 45 U/L，碱性磷酸酶测定 80 U/L、总胆红素测定 15 umol/L。"
}
```

14 条历史趋势（名称 / 点数 / 最近日期 / 最近 flag）：

| 指标 | 点数 | 最近日期 | 最近 flag |
| --- | ---: | --- | --- |
| 丙氨酸氨基转移酶测定 | 3 | 2026-11-04 | high |
| 天门冬氨酸氨基转移酶测定 | 3 | 2026-11-04 | high |
| 总胆红素测定 | 3 | 2026-11-04 | normal |
| 碱性磷酸酶测定 | 2 | 2026-11-04 | normal |
| γ-谷氨酰基转移酶测定 | 2 | 2026-09-04 | normal |
| 总蛋白测定 | 2 | 2026-09-04 | low |
| 球蛋白 | 2 | 2026-09-04 | normal |
| 白蛋白/球蛋白 | 2 | 2026-09-04 | normal |
| 白蛋白测定 | 2 | 2026-09-04 | low |
| 直接胆红素测定 | 2 | 2026-09-04 | normal |
| 胆汁酸测定 | 1 | 2026-09-04 | normal |
| 谷草/谷丙比值 | 1 | 2026-09-04 | unknown |
| 间接胆红素 | 2 | 2026-09-04 | unknown |
| 铜蓝蛋白 | 1 | 2025-09-24 | normal |

这组数据同时解释了发现 4 和发现 6：

- `2025-09-28` 的 GGT/总蛋白/球蛋白/白蛋白/球蛋白因为稀疏扩展进入历史；同报告的总胆红素/直接胆红素/间接胆红素/ALT/AST/白蛋白/ALP 因为窗口内点足够而被排除。
- 14 条历史是全集；4 个偏离是其中“最近点异常”的子集；3 条只有 1 个点，2 条最近点 `unknown`。

## 7. 附录 B：外部依据

| 编号 | 来源 | 用途 | URL |
| --- | --- | --- | --- |
| S1 | Apple Human Interface Guidelines — Layout | “按相对重要性排序内容，把重要内容放在前部” | https://developer.apple.com/design/human-interface-guidelines/layout |
| S2 | Apple Human Interface Guidelines — Typography | 用字号、字重、颜色建立信息层级 | https://developer.apple.com/design/human-interface-guidelines/typography |
| S3 | Apple Human Interface Guidelines — Writing | 使用简单平实语言，避免术语 | https://developer.apple.com/design/human-interface-guidelines/writing |
| S4 | Apple Human Interface Guidelines — VoiceOver | 用户会用转子按标题、链接导航；链接应被识别 | https://developer.apple.com/design/human-interface-guidelines/voiceover |
| S5 | Apple Human Interface Guidelines — Accessibility | 不能只靠颜色传达信息；对比度要求 | https://developer.apple.com/design/human-interface-guidelines/accessibility |
| S6 | Apple Human Interface Guidelines — Buttons | 按钮触控区域至少 44×44pt | https://developer.apple.com/design/human-interface-guidelines/buttons |
| S7 | React Native — Text | `selectable` 默认 false；开启后才可使用原生复制/粘贴 | https://reactnative.dev/docs/text#selectable |
| S8 | React Native — Accessibility | `accessibilityRole="link"` 用于打开外部 URL/网页的元素；`accessibilityLiveRegion` | https://reactnative.dev/docs/accessibility#accessibilityrole |
| S9 | React Native — AccessibilityInfo | `announceForAccessibility` 让读屏播报复制成功 | https://reactnative.dev/docs/accessibilityinfo#announceforaccessibility |
| S10 | Expo — Linking (SDK 58) | `Linking.openURL` 打开 URL | https://docs.expo.dev/versions/v58.0.0/sdk/linking/ |
| S11 | Expo — Linking into other apps | `Linking.openURL` 在系统默认浏览器打开 https URL | https://docs.expo.dev/linking/into-other-apps/ |
| S12 | Expo — Clipboard (SDK 58) | `Clipboard.setStringAsync` 写入剪贴板；平台含 Expo Go | https://docs.expo.dev/versions/v58.0.0/sdk/clipboard/ |
| S13 | Material Design 3 — Typography | type scale 用 display/headline/title/body/label 建立层级 | https://m3.material.io/styles/typography/overview |

## 8. 待确认的产品决策

1. **无解读时的首屏呈现。** spec §4.4 说“解释层缺条目时整层省略，不显示空状态”（`.scratch/llm-analysis/spec.md:167`），但发现 1 要求结论优先。建议：配额用尽/调用失败时显示紧凑状态块；`no_entries` 时显示一行固定引导（“本次没有可引用的解读条目，可以带着上面的数值问医生”），不要显示完整空卡片。是否接受？
2. **`bridge_text` 是否默认隐藏。** 建议默认放进“本次判据 / 查看依据”，避免与偏离项重复；如果产品要求透明展示模型连接语，则保留为 12px muted 的“本次判读依据”，但不能作为标题。选哪个？
3. **“可能原因”这个标题是否保留。** 它是 UI 标签，不是医学原文；如果医学审核认为“可能原因”容易被读成病因推断，可改为“这可能意味着”或“本次解读”。需要产品/医学确认。
4. **窗口契约怎么收口。** 当前 OpenAPI 描述与实现不一致（`schemas/openapi.yaml:1445-1448` vs `internal/analysis/facts.go:200-208`）。建议先修正描述为“用户选择的窗口起点；稀疏扩展按趋势键单独生效，不改变该字段”；只有在 UI 必须显示“该指标扩展到 2024-11-04”时才增加 `extended_from`。是否要现在改契约？
5. **历史趋势是否分组/折叠。** 建议先做标题、说明、中性色（方案 A）；如果 14 条仍然太长，再把“仅 1 个点”组默认折叠（方案 B）。是否愿意改变“全部平铺”的现状？
6. **来源链接打开方式。** 建议用系统浏览器（`Linking.openURL`），保持来源可分享、可回到浏览器；如果产品希望不离开 App，再评估 `expo-web-browser`。选哪个？
7. **是否接受新增 `expo-clipboard`。** 它提供一键复制和读屏反馈，但属于原生模块，现有 dev build 需要重建；如果不能加依赖，只能退化为 `selectable` 文本。是否接受？
8. **总蛋白/白蛋白偏低怎么处理。** 是新增一条经医学审核的“低蛋白/白蛋白”解释条目，还是先只显示“本条解读未覆盖，请结合医生意见”？前者内容更完整，但需要医学校对；后者更快，但会留下两个未解释异常。建议先做覆盖提示，再排期条目。
9. **病历里的药物/剂量是否默认折叠。** 建议保留原文但默认折叠在“查看处理原文”，并在卡片内固定“历史处方，勿自行用药”。如果产品希望一眼看到医生处理全貌，则保持展开但加安全提示。选哪个？
10. **BMI 是否留在肝功结果页。** 当前 BMI 三张卡与肝功判读同权重。建议合并为一张“体格发育”卡并默认折叠；是否进一步从肝功页移出，只在成员首页/体格页展示？需要产品确认。
11. **结果页操作的首批范围。** 报告原文链接、stale 重跑、条目级反馈是 P1；分享/导出、结果页时间范围切换是 P2。是否同意这个取舍？如果分享是家长的高频需求，可把“分享摘要”提前到 P1。
12. **是否为 `created_at` / `entry_updated` / `partial_rejected` 改契约。** 三者都需要 OpenAPI、Go、App codegen 与 fixtures 同步；如果短期不改，至少前端对已知 `explanation_reason` 给不同文案和重试入口。是否现在改？
