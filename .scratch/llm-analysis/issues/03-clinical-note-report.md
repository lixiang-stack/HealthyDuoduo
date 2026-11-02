# 03: 病历作为叙述型报告

**What to build:** 家长上传病历照片，系统把它归档为病历报告，按标题词切出主诉 / 现病史 / 诊断 / 处理段落；切不出来时整页作为一个段落；家长能在病历页看到段落原文和对应 OCR 行。

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] `report_type` 新增 `clinical_note`，上传链路可接收
- [ ] `sections` 的 `kind` 接受 `chief_complaint` / `present_illness` / `diagnosis` / `treatment` / `note`
- [ ] 段落切分按标题词自动执行
- [ ] 切不出段落时整页作为单个 `note` section
- [ ] 每个 section 带 `text` 与 `ocr_line_numbers`
- [ ] 空病历或识别失败走既有 `failed` 规则
- [ ] App 病历页展示段落原文
- [ ] 识别端测试覆盖切分与兜底；HTTP 测试覆盖上传与详情
