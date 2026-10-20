# 0008 · 报告段落作为可选值集合（JSONB），而非 Report 平铺必填字段

**状态：** 采纳（2026-10-20）。

影像（超声）单的检查项目 / 检查所见 / 超声提示是报告级叙述内容，需要进入档案详情；但不应以六个超声专用、必填、平铺字段的形式进入共享 `Report` 契约。本期采用**可选 `sections` 集合 + `reports.sections JSONB` 一列**：只有叙述型报告出现该字段，非影像报告的报告 JSON / golden / API 响应保持不变。

## 背景

`CONTEXT.md` 把「报告」定义为“由若干指标项组成”，指标项是 name/value/unit/ref_range/flag 的结构化数值。超声叙述单的主体是段落，不是指标项；把整段塞进 `ReportItem` 会破坏指标项语义。但把三个段落拆成 `exam_name` / `findings` / `impression` 六个 required 平铺字段，会让五类非影像报告恒为 null，并把 `report.schema.json` 的 required 从 4 个扩到 10 个——旧报告 JSON 失效、全量 golden/HTTP fixtures 重生成。这是破坏性契约变更，不是“新增可空字段”。

## 决策

- `Report.sections` 是**可选**数组（不在 JSON Schema `required` 中）；缺省或 null 表示无段落，空数组不合法。非叙述型报告不输出该字段。
- 每个 section 是值对象：`kind`（当前 `exam_name` / `findings` / `impression`，数组顺序即展示顺序）、`text`（原文按行拼接）、`ocr_line_numbers`（0-based，与 OCR txts/boxes 同下标）。
- 数据库：`reports.sections JSONB` 一列，nullable；随报告 upsert 全量覆盖；报告删除随行消失。
- 段落只读展示，不参与核对/修正/status/low_confidence；本期不建独立子表。

## Considered Options

- **六个平铺 required 字段**：与既有 `ReportItem.raw_text` + `ocr_line_numbers` 兄弟字段风格一致，但把超声专用形状带入共享契约；required 变化导致全量 golden 与消费者升级。否。
- **独立 `report_sections` 表**：约束/索引/局部更新更强，适合段落级核对、搜索、独立生命周期；但本期段落只读、整体重解析覆盖、无独立身份，建表会提前把值对象提升为实体，增加 CRUD/迁移/测试成本。暂不采用；出现段落级核对/搜索时再迁移。
- **读时从 OCR 重建**：不落文本，只存行号或完全按锚表实时切；存储最省，但 Go 侧需要复制锚表/切分规则，或增加识别服务调用；本期先存结构化结果，保持单一识别端规则源。
- **可选 `sections` 集合 + JSONB**：契约增量最小，非影像零漂移；未来迁移独立表时 API 形状不变。采纳。

## Consequences

- `schemas/report.schema.json` required 不变；旧 report golden/示例继续有效。
- 只有 `us_01/02/03` golden 新增 `sections`；非影像报告 JSON 逐字节不变（Pydantic 序列化省略 None，Go 用 `omitempty`）。
- 新增迁移 0007（一列 JSONB），而不是六列。
- 段落形状由 JSON Schema/Pydantic/Go 互证与 store roundtrip 测试约束；数据库只保证 JSONB 可空，不承担逐元素约束。
- 若后续出现段落级核对/修正、全文检索或独立状态，再迁移到 `report_sections` 表；对外 `sections` 数组形状可保持不变。
