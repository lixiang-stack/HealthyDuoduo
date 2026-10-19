# 01: 影像单完整内容数据链路（日期 / 测量项 / 报告段落）

**What to build:** 归档一张影像单后，报告里不再只有零星测量项：检查日期进入时间线，词典内的常见测量项进入指标项，检查项目 / 检查所见 / 超声提示三段原文连同 OCR 行号进入报告契约、落库并从详情 API 返回。检验单的报告形状与解析行为保持零变化；零指标项的 failed 影像单也保留段落原文。

**Blocked by:** None（可立即开始）。

**Status:** resolved

- [x] **日期**：日期标签表在「检测日期」后补「检查时间 / 检查日期」；含「检查日期：2024-09-15…」的影像单归档后 `report_date=2024-09-15`（bd39 / us_03 同源单）。检验单不印该标签，golden 逐字段零变化；us_01 / us_02 单面无日期，维持 `report_date=null` + `partial` 属诚实行为，不强行补日期。
- [x] **词典首批**：超声词典新增 canonical 测量项：肱骨长（HL）、鼻骨长、颈项皮肤褶皱厚度（NF）、侧脑室宽度（侧脑室）、后颅窝池宽度（后颅窝池）。只收印刷 clean 印形；乱珠形（观质 / 头图 / 披用类）不收；侧脑室左右侧不建模，取首个命中，合并为已知限制。无样本证据的项（头围 / 胫骨长 / 小脑横径 / 脐动脉PI / 子宫动脉PI）不入词典，待真实样本出现后按覆盖门禁补录。侧脑室取 us_02 左侧 9.1mm：相邻行框允许 ≤1/4 格高重叠（实测 4px/17px），golden 全量回归无其他类别漂移。
- [x] **词典分组（2026-10 修订）**：影像大类改走 logical merge（对齐临检组）：`imaging_dict.yaml` 改名 `us_dict.yaml`，仍 `report_type: 影像`；其他影像模态按真实样本新增文件、共用 `report_type`；`title_keywords` 收 `超声` / `彩超`（us_03 印法）；当前 items 仅覆盖产检超声测量项。
- [x] **别名专指性**：narrative 别名不得使用裸后缀词（厚度 / 宽度 / 长度）与单字方向词（左 / 右）；用测试锁定，防止「厚度」类泛别名把 NT 行误配给胎盘厚度。us_02 归档后可见肱骨长 36mm 等首批命中项。
- [x] **段落锚注册**：超声词典的类别注册新增 `section_anchors`（机制与 `title_keywords` / `mode` 同构）：exam_name = 检查项目 / 检查部位 / 检查内容；findings = 超声描述 / 检查所见 / 超声所见；impression = 超声提示 / 检查提示。锚 = 行首去空白后命中锚词的行。
- [x] **切分规则**：段落 = 锚词之后的同行尾缀（若有）+ 后续全部行，至下一锚行或页尾；按 OCR txts 下标序；整段行下标写入对应 `*_ocr_line_numbers`；「注：」尾段并入其前段，不设终止锚。
- [x] **exam_name 最小终止规则（已确认，对 spec 统一规则的有意偏离）**：exam_name 只取锚行尾缀；尾缀为空时取紧随其后的一行。us_01 的 exam_name =「筛查(自费)、11-14周NT筛查(生育)」；us_03 的 exam_name =「产科胎儿彩超检查（中孕）」，不得吞入姓名 / 日期 / 图像 / 测值等后续行。us_02 的「检查内容」锚后是乱码检查内容行（含档案号），按本规则只取紧随其后的一行，需在 golden 人工核对中确认该行原文。findings / impression 仍按完整段落规则切分；`超声图像` 等非文本段标签不入锚，「注：」并入前段。
- [x] **段落与指标项独立**：类别判定选中超声词典（标题或达标命中）即切段；零指标项的 failed 影像单同样切段，段落与 `fail_reason` 并存，空 items 不变量不破；段落不参与 status 派生，也不参与 low_confidence 标记。
- [x] **报告契约**：Report 新增六个可空但必现字段：`exam_name` / `findings` / `impression`（string | null）与三个 `*_ocr_line_numbers`（int[] | null）。成对约束：文本非 null ⇒ 行号非 null 且非空；文本 null ⇒ 行号 null。`schemas/report.schema.json`、pydantic `Report`、Go `contract.Report` 同步；非影像报告三字段恒 null；全量 report golden 机械性新增六个 null 字段。
- [x] **存储与详情 API**：迁移 0006 给 reports 加六列（三个 TEXT + 三个 INTEGER[]，与契约字段 1:1，存量 NULL）；`WriteReport` 全量写入，详情读取投影六字段，非影像报告为 null；OpenAPI `ReportDetail` 增三个文本 + 三个 `*_ocr_lines`（`OcrLineLink[]`，行号→box 映射与 items 的 `ocr_lines` 同源同构）；HTTP fixtures 与 App `schema.d.ts` 重新生成；摘要 / 时间线 / trend / 核对端点零变化；refresh 全量 reparse 自动回填存量影像单。`refresh` 只对 reports 行批量 reparse；images 无报告行（半截 ingest）跳过，避免 `no report for image` 假失败。
- [x] **内环验证**：pytest 覆盖契约成对约束、切段六种边界（锚行独立 / 锚词同行尾缀 / 锚到锚 / 锚到页尾 / 无锚单 / 注尾段）、别名专指性、空 items + failed 兼容；golden 全量重生成并人工核对 us_01 / us_02 / us_03 的段落原文与行号；`local_golden check/score` 影像单不退化、其他类别逐字段零变化。
- [x] **Go / HTTP 验证**：`go test ./...` 全绿；契约双语互证测试扩六字段；store 迁移测试 0006 升降版通过；真库冒烟写入后读回六字段；详情 fixture 六字段齐全，时间线与核对 fixture 不受影响。

## Comments

**实现（分支 `imaging-content`，worktree `.worktree/imaging-content`，单 commit，push/PR 待用户）**

- 内环：pytest 212 passed（词典裁剪后移除 2 个豁免门禁用例）；`go test ./...` 全绿；迁移 0006 up/down 与 store 真库冒烟通过；local score 230/236 与 main 一致，27 张本地报告仅新增六个 null 字段。
- **词典重组（2026-10 与维护者确认）**：影像大类对齐临检组 logical merge——`imaging_dict.yaml` 恢复为子类型文件 `us_dict.yaml`（`report_type: 影像`，其他影像模态按样本新增文件）；5 条无样本证据测量项（头围 / 胫骨长 / 小脑横径 / 脐动脉PI / 子宫动脉PI）与 4 个对应别名从词典及覆盖门禁豁免表删除；golden 全量重生成零漂移。
- 共享栈：`snapshot` → `refresh` 19 reparses → `verify` 全 ok（ocr-lines 223/223 join/box 一致）；三张影像单六字段回填，bd39 `report_date=2024-09-15`。词典重组项在三张影像单均无命中，结论沿用。
- 快照：`/Users/haoli/github/HealthyDuoduo/data/e2e-snapshots/healthyduoduo-20261019-201243.dump`（103099B）。
- 已知边界：us_03 检查所见标题 OCR 为「话：」无锚，`findings=null`；App E2E 用 us_01 覆盖完整三段（票 02）。
- 待办：票 02（App 段落卡 + `verify --golden` 扩段落 + App E2E）。
