# 02: App 报告段落呈现与全链路验收

**What to build:** 家长打开影像报告详情，能按单据原样读到检查项目、检查所见、超声提示；点段落可把原图缩放到对应 OCR 行。三张真实影像单在共享栈上 reparse 后，段落、日期、测量项与 golden / 原图一致；检验单与既有 App 流程不回归。

**Blocked by:** 01（影像单报告级段落后端数据链路）。

**Status:** resolved

- [x] **详情页段落卡**：影像报告的检查项目并入报告头部；检查所见 / 超声提示在指标项列表上方渲染为段落卡，按 `sections[].kind` 取标签；`sections` 缺省或对应 kind 不存在时不渲染。
- [x] **LineZoom 行对照**：点段落卡使用该 section 的 `ocr_lines` 行链接与图像地址，复用既有 LineZoom 组件缩放到对应区域；必要时把 LineZoom 抽到共享位置，不复制实现。
- [x] **failed 影像单**：段落卡与失败原因 / 原图并存，不被「没有识别出可核对的指标项」空态遮挡；时间线卡片保持不动。
- [x] **App E2E · us_03**：seed us_03 → 上传 → 详情页显示检查项目 + 超声提示 + 检查日期 2024-09-15 + 股骨长指标项；findings 缺省时不渲染检查所见卡片，符合契约。
- [x] **App E2E · us_01**：seed us_01 → 详情页显示检查项目 + 检查所见 + 检查提示三段完整内容，覆盖「有检查所见锚」的完整三段渲染路径。
- [x] **App 回归**：既有 cbc 用例全绿；`npm run typecheck`、jest、`npm run e2e` 全绿；`schema.d.ts` 与 OpenAPI 保持同步。
- [x] **verify --golden 扩段落**：报告级逐字段比对 `sections`（kind / text / 行号）与 golden 一致；非影像报告 sections 缺省一致。
- [x] **共享栈全链路**：按 `snapshot → refresh → verify → verify --golden` 跑通；refresh 后存量影像单段落回填；段落行号与 OCR box 的 join / box 不变量通过。
- [x] **三张真实影像单验收**：`b5cb…` / `bd39…` / `d2f7…`（与 us_01 / 02 / 03 同源）reparse 后，检查所见 / 超声提示段落完整且与原文一致；bd39 的 `report_date=2024-09-15`；us_02 的肱骨长 36mm 等首批命中项入库；us_01 / 02 日期 null + partial 属诚实行为。
- [x] **无回归**：既有类别 score / 字段零变化；时间线 / trend / 核对端点行为不变；本期不扩段落核对 / 修正，段落文本只存不回流。
