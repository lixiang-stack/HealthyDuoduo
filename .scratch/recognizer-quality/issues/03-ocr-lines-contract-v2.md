# 03: ocr_lines 契约 v2——识别端直吐行号,详情精确映射（09 前置小票）

**What to build:** 把 ocr_lines 的真理源从「服务端对 raw_text 的文本反推（重复碎片靠几何消歧）」改为「识别端直吐 `ocr_line_numbers`（与 txts/boxes 同下标的精确行号）」：识别端契约三方言（JSON Schema + pydantic + Go）同步扩展，`report_items` 迁移增列并随 reparse 回填，详情/上传/后续核对响应的行链接改为下标直映；文本反推启发式删除。规格铺垫：票 07 的反推是契约冻结期的临时妥协（当时识别端契约不动），09 的「缩放至 OCR 行对应区域」需要精确链接。

**Status:** resolved

- [x] `report.schema.json` / pydantic `ReportItem` / Go `contract.ReportItem` 增 required `ocr_line_numbers`（0-based,顺序与 raw_text 组成序一致）
- [x] 识别端 `_Cell.line` + 逐项吐出（含纵向回退吸收格）
- [x] 迁移 0005（`report_items.ocr_line_numbers INTEGER[]` 可空）+ storage 写读 + serve 下标直映;`LinkOCRLines` 反推删除
- [x] 存量语义:NULL → 空链接不炸页（raw_text 保留）;refresh/reparse 回填（真栈 0/198 NULL）
- [x] HTTP 契约/openapi/fixtures/App 零改动（`ocr_lines` 形状不变,来源变精确）
- [x] golden 全量重生成（samples 50 个 report 文件）+ 本机 27 语料刷新;pytest 165 通过;本机 score 与改前逐字节一致
- [x] 外环：snapshot → hdd 重建（goose 5）→ refresh（16 reparse）→ verify（**linked 198/198,join/box 一致**）→ seed → verify --golden → clean
- [x] 09 票面：引用本小票 + 「详情页显示短 ID」清单项

## Comments

**实现（分支 `recognizer-quality` 内单 commit；与 01/02/04 同 PR）**

- **识别端**（`contract.py` / `postprocess.py`）：`ocr_line_numbers: list[int]` 设为 **required**（识别端永远能给出:格与 txts/boxes 一一对应,`_cells` 补记下标）；`run_postprocess` 按 `raw_text` 的同一 `sorted(raw_cells, key=cx)` 顺序吐号,故「链接行文本按序 join == raw_text」由构造保证（e2e 度量继续守这条不变量）。纵向回退吸收的值格（如 `33.25岁`）同样在列。
- **契约互证**：schema `required` 同步扩到 8+1 字段；`schemas/testdata` 两个 report 示例补字段（Go `TestUnmarshalReportFixture` 锁读回）；HTTP 侧 `ReportItem` 的不透明句柄/核对状态/行链接形状本就独立,`ocr_lines: [{line, box}]` **形状与 fixtures 零改动**——本 v2 不触碰 OpenAPI/App codegen。
- **存储**（迁移 0005 + `store.go`）：`INTEGER[]` 可空（与既有 `correction_pairs.ocr_line_numbers` 同型同语义）；`WriteReport` 随 items 全量写；`ItemsWithReviewStatus` `COALESCE(…,'{}')` 读（存量 NULL → 空数组）。回填口径 = **reparse**（识别端重跑才有精确行号;`refresh` 即全量回填,真栈 0/198 遗留）。
- **serve**（`serve_reports.go`）：`toAPIDetail` 由「`LinkOCRLines(raw_text, ocr)` 反推」改为「下标 → `ocr.Boxes` 直映」；越界下标跳过不炸整页（数据损坏态;e2e 度量兜底）。`internal/pipeline/ocrlines.go` 及其 9 例反推单测随用途消失**删除**（反推是临时妥协，v2 后被死代码）。
- **验证证据**：Go `gofmt/vet/test` 全绿；一次性 PG 真库冒烟（goose 5 + 下标写读 + NULL 读空）通过；pytest 165 通过；本机 27 语料 `check` 27/27 且 VL `score` 与改前逐字节一致。外环（快照 `pre-ocr-lines-v2-20261018-104322.dump`）：hdd 重建（缺 `-- +goose` 标记的迁移在真库冒烟即被拦下,补齐后过）→ refresh 16 reparse → `verify` 输出 `ok: ocr-lines: linked 198/198 items; join/box 一致` → seed/`verify --golden` pass/clean 全过；另做存量 NULL 演练：SQL 置 NULL → 详情 200 + `ocr_lines: []` + raw_text 保留 → refresh 回填复原。
- **披露**：识别端「契约完全不动」的 M0 冻结点经维护者批准由本票解除（09 前置）；其它安装升级后需 `reparse`/`refresh` 回填存量行（未回填行链接为空,原始行文照常可见）；`ocr_line_numbers` 顺序即 raw_text 组成序,消费方勿按数值排序。
