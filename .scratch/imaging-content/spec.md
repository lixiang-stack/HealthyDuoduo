# Spec：影像单内容完整性 · 报告段落（可选集合 + JSONB）

Status: ready-for-agent

**依据文档：** 统一语言 `CONTEXT.md`（报告 / 报告段落 / 指标项）· ADR-0008（报告段落作为可选值集合）· 解析规则与扩展点约定 `recognizer/recognizer/postprocess.py` 头注释 · e2e 回路 `docs/agents/e2e.md` · 契约同步先例（`schemas/report.schema.json` + `schemas/openapi.yaml` + `recognizer/contract.py` + `internal/contract/contract.go`）。

**证据：** 三张真实影像单（库内 `b5cb…`/`bd39…`/`d2f7…`，与 `samples/us_01/02/03` 同源 sha）——三张均 `status=partial`、`report_date=null`，指标项入库 6/1/7 条，对应 OCR 全文 36/31/34 行。OCR 全文与原图已经留痕；缺的是报告级叙述内容的**结构化暴露**，不是原始数据丢失。

## Problem Statement

影像（超声）单沿用检验单的指标项模型（name/value/unit/ref_range/method/flag）识别与落库，但超声单是「测量值 + 检查所见 + 超声提示」的三段式**叙述文档**，不是指标×值的表格。三个现实缺口：

1. **叙述内容没有报告级出口**：检查项目 / 检查所见 / 超声提示只存在于 `ocr_results.engine_output` 的整页 OCR 中，详情 API 只返回指标项，App 无法按“检查项目 / 所见 / 提示”呈现。原始数据没有丢，但家长看到的“报告”不完整。
2. **词典覆盖有限**：`imaging_dict.yaml` 仅 9 个 canonical 项；词典外测量值（肱骨长 / 鼻骨长 / NF / 侧脑室 / 后颅窝池）静默跳过。narrative 别名逐字匹配对 OCR 乱珠零鲁棒。
3. **日期领域错配**：`postprocess.py` 的 `_DATE_LABELS` 只有检验单词汇，没有影像单标准印法「检查日期 / 检查时间」→ 三张单 `report_date` 全 null → 全部 partial（bd39 的 OCR 里明明有「检查日期：2024-09-1514:37:47」）。

字段死重：超声单不印参考范围（生物测量正常与否取决于孕周），指标项 `ref_range` 全 null、`flag` 全 `unknown`、`method` 全 null。指标项职责应保持“数值测量”；叙述内容不应塞进指标项。

## Solution

两件事，分切片交付：

1. **数据级修补（识别端规则/词典，不改报告契约）**：`_DATE_LABELS` 补「检查时间 / 检查日期」；`imaging_dict.yaml` 按真实样本扩首批测量项与专指别名；`_vertical_value` 允许相邻 OCR 行框小幅重叠（us_02 侧脑室）。
2. **报告级段落（可选集合 + JSONB，不破坏既有契约）**：识别端按段落锚从 OCR 行切出报告段落，**整段原文 + OCR 行号留痕，不解析内容**；`Report.sections` 是**可选**数组，只有叙述型报告出现；落库为 `reports.sections JSONB` 一列；详情 API 返回 `sections`（文本 + 行链接）；App 详情页后续票 02 实现。

**关键约束：**

- 既有 `Report` 的 required 字段不变（仍为 `report_type/report_date/status/items`）。
- 非影像报告的 report golden / JSON / API 响应保持原样；`sections` 缺省即不出现。
- 不采用六个平铺字段（`exam_name` / `findings` / `impression` + 三个行号数组）；不再改 `reports` 六列。
- 孕检保留：现有 9 个超声/孕检测量项不删；本期只新增有真实样本证据的 5 项。`imaging_dict.yaml` 文件名保持不变，不做 `us_dict.yaml` 改名。
- 报告段落不参与核对/修正，不参与 status 派生，不参与 low_confidence 标记；只读展示。

## Contract

`Report` 新增一个**可选**属性：

```json
"sections": [
  {"kind": "exam_name",  "text": "产科胎儿彩超检查（中孕）", "ocr_line_numbers": [5]},
  {"kind": "findings",   "text": "宫内单活胎。\n胎心率159次/分。", "ocr_line_numbers": [10, 11]},
  {"kind": "impression", "text": "宫内单活胎。\n注：仅供临床参考。", "ocr_line_numbers": [20, 21]}
]
```

- `sections` 不在 `required` 中；缺省或 null 视为“无段落”，空数组不合法。非叙述型报告不输出该字段。
- `ReportSection`：`kind` 非空字符串（当前已知值 `exam_name` / `findings` / `impression`，数组顺序即展示顺序）；`text` 非空；`ocr_line_numbers` 非空整数数组（0-based，与 `ocr_result` 的 `txts/boxes` 同下标，顺序与文本组成顺序一致）。
- 数组可包含同类段落多次出现（按 OCR 行序保留），不强行合并。
- Pydantic/JSON Schema/Go 三处同步；JSON Schema 保持 `additionalProperties: false`，`sections` 可选。

## Implementation Decisions

**段落切分（识别端）**

- 锚表放词典 `category.section_anchors`（机制与 `title_keywords` / `mode` 同构；`imaging_dict.yaml` 首个注册者）：

```yaml
section_anchors:
  exam_name:  [检查项目, 检查部位, 检查内容]
  findings:   [超声描述, 检查所见, 超声所见]
  impression: [超声提示, 检查提示]
```

- 锚 = 行首（去空白）命中锚词的行；段落 = 锚行尾缀（若有）+ 后续全部行，至下一锚行或页尾；按 `txts` 下标序；「注：」尾段自然并入其前段（不设终止锚）。
- `exam_name` 例外（与维护者确认）：只取锚行尾缀；尾缀为空时取紧随其后的一行，不吞患者信息 / 日期 / 图像 / 测值。
- 类别判定选中带 `section_anchors` 的词典即切段；**零指标项的 failed 影像单同样切段**（段落与 `fail_reason` 并存，空 items 不变量不破）。
- 段落切分独立于 items 抽取；段落不参与 status / low_confidence。

**日期与词典**

- `_DATE_LABELS` 在「检测日期」后追加「检查时间 / 检查日期」；检验单不印该标签，零影响。
- `imaging_dict.yaml` 新增：肱骨长（HL）、鼻骨长、颈项皮肤褶皱厚度（NF）、侧脑室宽度（侧脑室）、后颅窝池宽度（后颅窝池）；只收印刷 clean 印形，乱珠形不收。
- `title_keywords` 收 `超声` / `彩超`（us_03 印法）。
- 别名专指性硬约束：narrative 别名不得用裸后缀词（厚度 / 宽度 / 长度）与单字方向词（左 / 右）。
- `_vertical_value` 允许相邻行框 ≤1/4 格高的上下重叠（us_02 侧脑室 9.1mm）；以 golden 回归确认其他类别零字段漂移。

**存储与 API**

- 迁移 0007：`reports` 加一列 `sections JSONB`（可空；存量行 NULL）。Down 删除该列。
- `WriteReport` 随报告 upsert 全量写 `sections`；reparse 全量覆盖；非影像报告写 NULL。
- 详情读取：`SectionsByImage(reportSha) ([]contract.ReportSection, error)`；报告缺失/列为 NULL 返回 nil。
- OpenAPI `ReportDetail` 新增可选 `sections` 数组（`ReportSection`：`kind` + `text` + `ocr_lines: OcrLineLink[]`）；`ReportSummary` 不变；现有 HTTP fixtures 不含 `sections`，保持有效。
- App `schema.d.ts` 同步生成/更新可选 `sections`；详情页渲染属票 02。

## Testing Decisions

- **识别端**：pytest 契约测试（optional sections、空 items + failed 兼容、section 字段约束）+ 切段边界单测（锚行独立 / 锚词同行尾缀 / 锚到锚 / 锚到页尾 / 无锚单 / 注尾段 / 同类多次 / exam_name 最小终止）+ 别名专指性 + 词典覆盖门禁 + golden：**只有 us_01/02/03 新增 sections**，其余 report golden 逐字节不变。
- **Go / HTTP**：`go test ./...`；契约双语互证测试扩 sections；store 迁移 0007 up/down；store 读写 roundtrip（非影像 NULL / 缺失报告 nil）；详情 fixture 带 sections；无 sections 的旧 fixture 仍通过。
- **全链路**：`refresh → verify → verify --golden`；sections 纳入 DB vs golden 比对（票 02 或后端收口时）。
- **App**：本期只更新 `schema.d.ts`；详情页/E2E 归票 02。

## Out of Scope

- 段落级核对 / 修正 / 搜索 / 统计（本期只读展示）。
- 分型契约（`ReportItem` 加 kind）与 LLM/VLM 抽取。
- 所见内容结构化（发现列表 / NER）、孕周评估与生长曲线消费。
- 乱珠别名词典化（观质 / 头图 / 披用类不收）；侧脑室 / 子宫动脉的左右侧建模（取首个命中）。
- 删除或重命名影像词典；删除孕检测量项（明确保留）。
- 时间线卡片展示段落摘要；报告类别细分。

## Success Criteria

- 非影像报告的 `samples/expected/report/*.json` 与 `main` 逐字节一致；`schemas/report.schema.json` 的 required 不变。
- `samples/expected/report/us_01/02/03.json` 新增 `sections`；三张单的检查项目 / 有锚的检查所见 / 超声提示文本与 OCR 行号可人工核对。
- us_03（bd39）`report_date=2024-09-15`；us_02 新增肱骨长 36mm 等首批命中项；us_01/02 单面无日期维持 null + partial。
- `pytest`、`go test ./...` 全绿；`npm run typecheck` 通过（schema 类型同步）。
- 共享栈 `refresh → verify → verify --golden` 通过；sections 行号与 OCR box join 一致。

## Further Notes

- **交付切片**：
  1. 后端（本票 01）：日期 + 词典 + 切段 + 可选 sections 契约 + JSONB 迁移/存储/详情 API + Go/pytest。
  2. App（票 02）：详情页段落卡 + LineZoom + App E2E + `verify --golden` 扩段落。
- **决策记录**：ADR-0008；`CONTEXT.md` 新增「报告段落」并更新「报告」定义。
- **词典/规则改动**按 `postprocess.py` 头注释扩展点优先级执行：先数据文件，再规则；golden 样本已就位（us_* 先于规则改动入库）。
