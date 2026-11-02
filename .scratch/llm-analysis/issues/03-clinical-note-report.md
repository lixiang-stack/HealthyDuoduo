# 03: 病历作为叙述型报告

**What to build:** 家长上传病历照片，系统把它归档为病历报告，按标题词切出主诉 / 现病史 / 诊断 / 处理段落；切不出来时整页作为一个段落；家长能在病历页看到段落原文和对应 OCR 行。

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] `report_type` 新增 `clinical_note`，上传链路可接收
- [x] `sections` 的 `kind` 接受 `chief_complaint` / `present_illness` / `diagnosis` / `treatment` / `note`
- [x] 段落切分按标题词自动执行
- [x] 切不出段落时整页作为单个 `note` section
- [x] 每个 section 带 `text` 与 `ocr_line_numbers`
- [x] 空病历或识别失败走既有 `failed` 规则
- [x] App 病历页展示段落原文
- [x] 识别端测试覆盖切分与兜底；HTTP 测试覆盖上传与详情

## Comments

**实现（2026-11-02，分支 `llm-analysis-spec`）**

- 识别端新增 `clinical_note_dict.yaml`：`report_type = clinical_note`，标题词（病历 / 门诊记录 / 诊断证明等，排除「病历号」）兜底分类；裁剪件靠 ≥2 个段落锚归类。按主诉 / 现病史 / 诊断 / 处理锚词切段，切不出时整页作为单个 `note` 段落。
- 病历复用叙述型报告链路：零指标项 + sections 合法；空白页仍走既有 `failed`（`unknown` 类别，`no metric items recognized`）；若识别端显式给出 `clinical_note` + failed 且无正文，pipeline 固定原因 `no report body recognized`。
- 新增 panel `clinical_note`，三端枚举 / 优先级 / 标签同步；App 报告详情按叙述报告展示段落原文，不进入核对。
- 识别端测试覆盖切分、整页兜底、空页失败与「诊断」行不抢检验单分类；HTTP 测试覆盖上传与详情。

**真机反馈修订（2026-11-03）**

- 病历日期优先取记录日期，其次初诊时间；没有日期标签时不抓出生日期等独立日期行。
- 处理段落遇到「请妥善保管门诊病历…」、页码、记录日期等页脚 / 元数据即停止；底部印章或水印产生的杂字（如 LUUCKY）不再并入处理内容。
