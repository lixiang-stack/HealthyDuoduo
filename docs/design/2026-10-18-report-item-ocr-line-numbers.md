# 2026-10-18 ReportItem 契约扩展:ocr_line_numbers（ocr_lines 精确化）

## 为什么做

票 07 的 `ocr_lines` 由服务端对 `raw_text` 做文本反推（重复碎片如 `%`/`↑ %` 靠几何消歧）——那是识别端契约冻结期（M0「识别端契约完全不动」）的临时妥协。精确行号本就在识别端：格与 OCR 输出逐行一一对应，`_Cell` 只是没有记下标。09 的核对闭环要「缩放至 OCR 行对应区域」，消费的必须是精确链接；继续反推等于把「布局巧合」当稳定性证据。维护者批准解除该冻结点（09 前置小票，`.scratch/recognizer-quality/issues/03`）。

## 做什么

- `schemas/report.schema.json` 的 `reportItem` 增 **required** `ocr_line_numbers: [int]`（0-based，与 `ocr_result.txts/boxes` 同下标；顺序与 `raw_text` 组成顺序一致）。
- HTTP 契约**零改动**：`components/ReportItem.ocr_lines: [{line, box}]` 形状不变，仅来源从反推变为「下标 → OCR 行框」直映；OpenAPI/fixtures/App codegen 均不动。
- `report_items` 增列 `ocr_line_numbers INTEGER[]`（迁移 0005，可空；与 `correction_pairs` 同型）。
- 服务端 `pipeline.LinkOCRLines`（反推）与其单测删除。

## 怎么实施

识别端（`postprocess._cells` 记行下标 → `run_postprocess` 按 raw_text 同序吐号）→ 契约三方言（JSON Schema / pydantic / Go `contract.ReportItem`）→ 存储（写随 items 全量；读 `COALESCE(…,'{}')`）→ serve `toAPIDetail` 下标直映。存量行回填口径 = **reparse**（`hdd reparse <sha…>` 或 e2e `refresh`）；未回填（NULL）行链接为空数组、raw_text 照常可见，不猜。

## 怎么验收

- 内环：`go test ./...`（含 contract/storage/serve 单测）；一次性 PG 真库冒烟（迁移 v5 + 下标写读 + NULL 读空）；`uv run pytest`（golden 全量重生成后逐文件回归）；本机 27 语料 `check` + VL `score` 与改前一致。
- 外环：`snapshot` → hdd 重建 → `refresh`（16 reparse 回填）→ `verify`：`ocr-lines: linked 198/198 items; join/box 一致` → `seed`/`verify --golden`/`clean`。
- 退化演练：SQL 置 NULL → 详情 200 + 空链接 + raw_text 保留 → refresh 复原。
