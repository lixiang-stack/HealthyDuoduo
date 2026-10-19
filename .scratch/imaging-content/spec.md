# Spec：影像单内容完整性 · 词典日期修补 + 报告级段落

Status: needs-triage

**依据文档：** 统一语言 `CONTEXT.md`（检查单 / 图像 / OCR 结果 / 报告 / 指标项）· 解析规则与扩展点约定 `recognizer/recognizer/postprocess.py` 头注释 · e2e 回路 `docs/agents/e2e.md` · ADR-0004（表格结构识别）/ ADR-0005（核对落库）/ ADR-0006（局域网无鉴权）· 契约四处同步先例（`schemas/report.schema.json` + `schemas/openapi.yaml` + `recognizer/contract.py` + `internal/contract/contract.go`，0002/0003/0005 三次迁移）。

**证据：** 三张真实影像单（库内 `b5cb…`/`bd39…`/`d2f7…`，与 `samples/us_01/02/03` 同源 sha）——三张均 `status=partial`、`report_date=null`，指标项入库 6/1/7 条，对应 OCR 全文 36/31/34 行。

## Problem Statement

影像（超声）单沿用检验单的指标项模型（name/value/unit/ref_range/method/flag）识别与落库，但超声单是「测量值 + 检查所见 + 超声提示」的三段式**叙述文档**，不是指标×值的表格。三重失配（反例即上述三张单）：

1. **主体内容不入库**：检查所见（大段结构观察描述）与超声提示（医师结论，临床价值最高的部分）整段丢弃——三张单合计约百行原文只有 14 条测量项入库。家长侧的后果：档案里这张「报告」既不是单据的完整内容、也不是单据最重要的内容。
2. **词典覆盖有限**：超声词典（当时 `imaging_dict.yaml`）仅 9 个 canonical 项，词典外测量值（头围 / 肱骨长 / NF / 侧脑室 / 后颅窝池 / 小脑径 / 子宫动脉 PI…）静默跳过；narrative 别名逐字匹配对 OCR 乱珠（观质=双顶径 / 头图=头围 / 披用=腹围）零鲁棒——bd39 单约 12 个测量只活 1 个。
3. **日期领域错配**：`postprocess.py` 的 `_DATE_LABELS` 只有检验单词汇（检验 / 检测 / 采样 / 核收 / 报告…），没有影像单标准印法「检查日期 / 检查时间」→ 三张单 `report_date` 全 null → 全部 partial（bd39 的 OCR 里明明有「检查日期：2024-09-1514:37:47」）。

字段死重：超声单不印参考范围（生物测量正常与否取决于孕周），14 条指标项 `ref_range` 全 null、`flag` 全 `unknown`、`method` 全 null——按检验单最复杂形态设计的字段集合，对影像单大半空转。

## Solution

两件事打包，让影像单的**完整内容**进档案、失配就地消解：

1. **数据级修补（识别端零结构变更）**：`_DATE_LABELS` 补「检查时间 / 检查日期」；`us_dict.yaml`（原 `imaging_dict.yaml`）按真实样本扩首批测量项与专指别名。
2. **报告级原文段落（契约新增可空字段）**：识别端按段落锚把叙述单切成三个报告级字段——检查项目（`exam_name`）/ 检查所见（`findings`）/ 超声提示（`impression`），**整段原文 + OCR 行号留痕，不做任何内容解析**；落库（reports 加列）、API 详情、App 详情页全链路可见，图像行级缩放复用既有 LineZoom。

指标项职责收窄为「数值测量」，叙述内容交给报告级段落——统一字段模型对影像单的失配由此消解，不需要分型契约（ReportItem 加 kind）与模型抽取（LLM/VLM）。全量 OCR 与原图本就留痕，本期把「结构化层」补齐到单据内容零丢失。

## User Stories

1. 作为家长，我想归档一张超声单后看到单上的完整内容——检查项目、检查所见、超声提示——而不是只有几个测量值，以便纸丢了这份档案就是可信原件。
2. 作为家长，我想超声单上的检查日期被正确识别，以便档案时间线不缺日期、不再整单因日期缺失而「待核对」。
3. 作为家长，我想常见测量项（头围 / 肱骨长 / 侧脑室…）也进指标项，以便生长追踪覆盖更多指标。
4. 作为家长，我想在详情页读所见 / 提示段落时能把图像缩放到对应区域，以便对照原文核对。
5. 作为开发者家长，我想段落切分是行级保底（不解析内容），以便乱码行照收原样、不产生错误的结构化值。
6. 作为开发者家长，我想检验单类别的契约语义、golden 行为、解析结果零变化，以便本期改动不背检验单回归的锅。

## Implementation Decisions

**契约（四处同步，沿用 0002/0003/0005 先例）**
- `Report` 新增六个可空字段，平铺镜像 `ReportItem` 的 `raw_text` + `ocr_line_numbers` 兄弟字段风格：

```text
exam_name                    string | null   检查项目/部位行原文
exam_name_ocr_line_numbers   int[]  | null
findings                     string | null   检查所见段落原文（行文本按序拼接）
findings_ocr_line_numbers    int[]  | null
impression                   string | null   超声/检查提示段落原文
impression_ocr_line_numbers  int[]  | null
```

- 字段成对约束（pydantic `model_validator` + JSON Schema 描述）：文本非 null ⇒ 行号数组非 null 且非空；文本 null ⇒ 行号数组 null。序列化保留 null（与 `value: null` 同风格）→ 全量 report golden 机械性新增六个 null 字段。
- 非 narrative 类别（全部检验单）三字段恒 null——契约零影响。

**段落切分（识别端）**
- 锚表放词典 `category` 注册（机制与 `title_keywords` / `mode` 同构，`us_dict.yaml`（原 `imaging_dict.yaml`）首个注册者）：

```yaml
section_anchors:
  exam_name:  [检查项目, 检查部位, 检查内容]
  findings:   [超声描述, 检查所见, 超声所见]
  impression: [超声提示, 检查提示]
```

- 切分规则：锚 = 行首（去空白）命中锚词的行；段落 = 锚行锚词之后的同行尾缀（若有，如「超声提示：宫内单活胎」）+ 后续全部行，至下一锚行或页尾；按 txts 下标序；整段行下标进 `*_ocr_line_numbers`。「注：」尾段并入其前段（不做终止锚，YAGNI）。`检查内容` 按 us_02 真实印法纳入 exam_name。
- exam_name 例外（2026-10 与维护者确认）：exam_name 只取锚行尾缀；尾缀为空时取紧随其后的一行（us_01 取检查部位下一行、us_03 取检查项目同行尾缀、us_02 取检查内容下一行），不吞患者信息 / 日期 / 图像；findings / impression 仍按完整段落规则。`超声图像` 等非文本段标签不入锚。
- 独立于 items 抽取：类别判定选中超声词典（标题或达标命中）即切段；**零指标项的 failed 影像单同样切段**——bd39 式乱码单的保底价值正在于此（`fail_reason` 与段落并存，不冲突空 items 不变量）。
- 段落不参与 status 派生、不参与 low_confidence 标记（段落是原文留痕语义，不是结构化值）。

**日期（方向 1）**
- `_DATE_LABELS` 在「检测日期」后追加「检查时间 / 检查日期」。检验单不印该标签，零影响；影像单单内唯一日期标签即检查日期。

**词典首批与分组（方向 1）**
- **分组（2026-10 修订，对齐临检组）**：影像大类改走 logical merge：`us_dict.yaml` 为超声子词典，`report_type: 影像`；其他影像模态（CT/MRI 等）按真实样本新增文件、共用 `report_type`。类别标题词收 `超声` / `彩超`（us_03 印法）。当前 items 仅覆盖产检超声测量项，非产科超声按样本续录。
- 新增 canonical 项（别名只收印刷 clean 印形，乱珠形不收；golden 回归裁定命中，未命中零伤害）：肱骨长（HL）、鼻骨长、颈项皮肤褶皱厚度（NF）、侧脑室宽度（侧脑室）、后颅窝池宽度（后颅窝池）。无样本证据的项（头围 / 胫骨长 / 小脑横径 / 脐动脉PI / 子宫动脉PI）不入词典，真实样本出现后按覆盖门禁补录。
- 侧脑室取首个命中（us_02 左侧 9.1mm）；相邻 OCR 行框允许 ≤1/4 格高的上下重叠（us_02 名称行 / 值行实测 4px/17px），否则正下方值行被跳过、远行门牌号会被误当值。该规则改动以 us_02 golden 为证据，golden 全量回归其他类别零字段漂移。
- **别名专指性硬约束**：narrative 别名必须专指——禁止「厚度 / 宽度 / 长度」等裸后缀词与单字「左 / 右」（防「厚度」类泛别名把 NT 行误配给胎盘厚度）。

**存储与 API**
- 迁移 0006：`reports` 表加六列（三个 `TEXT` + 三个 `INTEGER[]`，与契约字段 1:1）。存量行 NULL；e2e `refresh` 全量 reparse 自动回填存量影像单。
- `WriteReport` / `ReportDetail` 查询扩六字段；摘要 / 时间线 / trend / 核对端点零变化。
- OpenAPI `ReportDetail` 增六个字段（三个文本 + 三个 `ocr_lines`：`OcrLineLink[]`，行号→box 映射与 items 的 `ocr_lines` 同源同构）；App `schema.d.ts` 重新生成。
- App 详情页：影像单在指标项列表上方渲染「检查所见 / 超声提示」两张段落卡片（检查项目并入报告头部），点段落可 LineZoom 行对照（复用既有组件与既有段落样式）；时间线卡片不动。
- 核对 / 修正域不扩：段落文本不参与四纸面事实修正（乱码段落的出口 = 看原图，本期明确接受）。

## Testing Decisions

- **内环（识别端）**：pytest 契约测试（字段成对约束、空 items + failed 不变量兼容）+ 切段单测（锚行独立 / 锚词同行尾缀 / 锚到锚 / 锚到页尾 / 无锚单 / 注尾段）+ 别名专指性测试（泛别名禁入）+ 词典覆盖门禁（无样本证据条目不入典，无豁免）+ golden **全量重生成**（全部类别新增六 null 字段；us_01/02/03 三张人工核对段落原文与行号）+ `local_golden check/score`（影像单 score 不退化、其他类别逐字段零变化）。
- **Go / HTTP**：契约双语互证测试扩六字段；`schemas/testdata/http/report.detail.json` 等 fixtures 扩；store 迁移测试（0006 升降版）。
- **全链路**：`refresh` → `verify` → `verify --golden`，段落字段纳入 DB vs golden 逐字段比对。
- **App E2E**：新增影像单用例（seed `us_03`：上传 → 详情页三段渲染 → 检查日期 2024-09-15 呈现 → 指标项仍在）；既有 cbc 用例全绿不回归。

## Out of Scope

- 分型契约（`ReportItem` 加 kind）与 LLM/VLM 抽取——本期方向 3/4 明确不做。
- 段落文本的核对 / 修正（只存不回流，与修正对同款挂起）。
- 所见内容结构化（发现列表 / NER）、孕周评估与生长曲线消费。
- 乱珠别名词典化（观质 / 头图 / 披用类不收）；侧脑室 / 子宫动脉的左右侧建模（取首个命中，左侧值合并为已知限制）。
- 检验单类别的解析行为 / 契约语义变化（新字段 null 即零影响）。
- 时间线卡片展示段落摘要；报告类别细分（NT 筛查 / 中孕 / 系统筛查仍统一为「影像」）。

## Further Notes

- **交付切片与门禁**：
  1. 识别端（日期标签 + 词典首批 + 切段 + golden 重生成）→ pytest 全绿、us 三张段落人工核对、score 零退化；
  2. 契约与存储（schemas ×2 + Go + 迁移 0006 + store/fixtures）→ `go test ./...` 全绿、goose 升降版通过；
  3. serve ReportDetail + App 详情页 + App E2E 用例 → `npm run e2e` 全绿；
  4. 全链路收口（snapshot → refresh → verify → verify --golden）。
- **成功标准**（以三张单 reparse 后核验）：三张单中有锚的检查所见 / 超声提示段落完整呈现并与原文一致；us_03（bd39）的检查所见标题被 OCR 成「话：」、无锚，`findings=null` 属诚实行为（App E2E 用 us_01 覆盖完整三段）；bd39 的 `report_date=2024-09-15`（us_01/02 单面无日期，维持 null + partial 属诚实行为）；us_02 新增肱骨长 36mm 等首批命中项。
- 词典 / 规则改动按 `postprocess.py` 头注释扩展点优先级执行：先数据文件、golden 样本已就位（us_* 先于规则改动入库）、切段规则属「新版式」级改动，本 spec 即其依据。
