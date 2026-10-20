# 01: 影像单报告级段落后端数据链路（日期 / 测量项 / sections）

**What to build:** 归档一张影像单后，检查日期进入时间线，词典内的常见测量项进入指标项，检查项目 / 检查所见 / 超声提示作为**可选 `sections` 集合**连同 OCR 行号进入报告契约、以 JSONB 落库并从详情 API 返回。既有报告契约 required、非影像 golden 与 API 响应保持不变；孕检保留。

**Blocked by:** None（可立即开始）。

**Status:** resolved

- [x] **日期**：`_DATE_LABELS` 在「检测日期」后补「检查时间 / 检查日期」；含「检查日期：2024-09-15…」的影像单归档后 `report_date=2024-09-15`（bd39 / us_03 同源单）。检验单不印该标签，golden 逐字段零变化；us_01 / us_02 单面无日期，维持 `report_date=null` + `partial` 属诚实行为，不强行补日期。
- [x] **词典首批（孕检保留）**：`imaging_dict.yaml` 保持文件名与既有 9 项；新增 canonical 测量项：肱骨长（HL）、鼻骨长、颈项皮肤褶皱厚度（NF）、侧脑室宽度（侧脑室）、后颅窝池宽度（后颅窝池）。只收印刷 clean 印形；乱珠形（观质 / 头图 / 披用类）不收；无样本证据的项（头围 / 胫骨长 / 小脑横径 / 脐动脉PI / 子宫动脉PI）不入词典。`title_keywords` 收 `超声` / `彩超`。
- [x] **别名专指性**：narrative 别名不得使用裸后缀词（厚度 / 宽度 / 长度）与单字方向词（左 / 右）；用测试锁定，防止「厚度」类泛别名把 NT 行误配给胎盘厚度。us_02 归档后可见肱骨长 36mm 等首批命中项。
- [x] **段落锚注册**：`imaging_dict.yaml` 的 `category` 新增 `section_anchors`（机制与 `title_keywords` / `mode` 同构）：exam_name = 检查项目 / 检查部位 / 检查内容；findings = 超声描述 / 检查所见 / 超声所见；impression = 超声提示 / 检查提示。锚 = 行首去空白后命中锚词的行。
- [x] **切分规则**：段落 = 锚词之后的同行尾缀（若有）+ 后续全部行，至下一锚行或页尾；按 OCR txts 下标序；整段行下标写入该 section 的 `ocr_line_numbers`；「注：」尾段并入其前段，不设终止锚；同类锚多次出现按 OCR 行序保留多个 section。
- [x] **exam_name 最小终止规则（已确认，对统一规则的有意偏离）**：exam_name 只取锚行尾缀；尾缀为空时取紧随其后的一行。us_01 的 exam_name =「筛查(自费)、11-14周NT筛查(生育)」；us_03 的 exam_name =「产科胎儿彩超检查（中孕）」，不得吞入姓名 / 日期 / 图像 / 测值等后续行。us_02 的「检查内容」锚后是乱码检查内容行（含档案号），按本规则只取紧随其后的一行，需在 golden 人工核对中确认该行原文。findings / impression 仍按完整段落规则切分；`超声图像` 等非文本段标签不入锚。
- [x] **段落与指标项独立**：类别判定选中带 `section_anchors` 的词典即切段；零指标项的 failed 影像单同样切段，段落与 `fail_reason` 并存，空 items 不变量不破；段落不参与 status 派生，也不参与 low_confidence 标记。
- [x] **报告契约（可选集合）**：`Report.sections` 可选（不在 required 中）：`[{kind, text, ocr_line_numbers}]`；`kind` 非空、`text` 非空、`ocr_line_numbers` 非空；非叙述型报告不输出该字段。`schemas/report.schema.json`、pydantic `Report`、Go `contract.Report` 同步；required 保持 `report_type/report_date/status/items` 不变。
- [x] **序列化零漂移**：非影像报告 `model_dump_json()` 不出现 `sections`（而非 `sections:null`），Go HTTP/JSON 用 `omitempty`；全量 lab report golden 与 main 逐字节一致，只有 us_01/02/03 新增 sections。
- [x] **存储与详情 API**：迁移 0007 给 `reports` 加一列 `sections JSONB`（可空，存量 NULL）；`WriteReport` 全量写入，`SectionsByImage` 读取（NULL/缺失 → nil）；OpenAPI `ReportDetail` 新增可选 `sections`（`OcrLineLink[]` 行链接，与 items 的 `ocr_lines` 同源同构）；HTTP fixtures 与 App `schema.d.ts` 更新；摘要 / 时间线 / trend / 核对端点零变化；refresh 全量 reparse 自动回填存量影像单。
- [x] **内环验证**：pytest 覆盖 optional sections 契约、切段边界（锚行独立 / 锚词同行尾缀 / 锚到锚 / 锚到页尾 / 无锚单 / 注尾段 / 同类多次）、别名专指性、空 items + failed 兼容；golden 全量重生成后确认只有 us_01/02/03 变化；`local_golden check/score` 影像单不退化、其他类别逐字段零变化。
- [x] **Go / HTTP 验证**：`go test ./...` 全绿；契约双语互证测试扩 sections；store 迁移 0007 升降版通过；store 写入/读回 sections、非影像 NULL、缺失报告 nil；详情 fixture 带 sections，旧 fixture 不带 sections 仍通过。
- [x] **无回归**：孕检测量项与既有测试保留；词典不改名；时间线 / trend / 核对端点行为不变；本期不扩段落核对 / 修正。

## Comments

**实现（分支 `imaging-sections`，worktree `.worktree/imaging-sections`，单 commit，push/PR 待用户）**

- 方案：`Report.sections` 可选数组 + `reports.sections JSONB`（ADR-0008）；required 仍为 `report_type/report_date/status/items`。
- 迁移：0007 加 `sections JSONB`（0006 为旧分支占位，保持版本链连续）；up/down 与 store roundtrip 真库通过。
- 内环：pytest 213 passed / 28 skipped；go test ./... 全绿；golden 只有 us_01/02/03 变化，其余 report golden 逐字节不变。
- 共享栈：`refresh` 19 reparses ok（0007 自动升版）；`verify` 全 ok；三张影像单 DB sections 与 golden 逐字段一致；非影像报告 `sections` 为 NULL。
- API 冒烟：新 `hdd serve` 详情对 us_01 返回 3 个 sections（行链接齐全）；cbc_01 详情不输出 `sections`。
- 已知边界：`22f705…` 是共享库里一张半截真实图（有 OCR、无 reports 行，文件名 `11005edc-….jpeg`），不是 samples/golden；`refresh` 已按 reports 取身份并跳过它。`verify --golden` 按标准序列需先 `seed`；无报告样本现在会清晰报 `FAIL: <sid> not in db`（不再 `hdr[0]` TypeError）。App 详情页/段落卡归票 02。

