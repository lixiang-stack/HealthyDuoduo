# 03: 影像报告详情以段落 + 原图为主

**What to build:** 家长打开影像（超声）报告时，以报告段落（检查项目 / 检查所见 / 超声提示）和原图作为报告主体；不再把影像测量值作为「指标项」展示，也不显示每项的「未知」flag、待核对入口或时间线的「未知 N」徽标。影像单未印日期时显示「检查单未印日期」。检验单与既有核对流程不变。

**Blocked by:** None（02 已 resolved）。

**Status:** ready-for-agent

## 背景与依据

- 超声单是叙述文档，不印参考范围 / 检测方法；当前 `ReportItem` 复用了检验单模型，18 个影像指标项全部 `flag=unknown`，时间线会出现「未知 11 / 未知 6 / 未知 1」，详情每项都显示「未知」。
- 超声测量值是否正常取决于孕周，当前模型没有孕周；`ref_range` 全为 null，`flag=unknown` 是模型不匹配，不是识别失败。
- 词典命中数不等于报告里的测量值数量：`us_03` 原图有 5 个测值，OCR 把名称打乱成「观质 / 头图 / 披用 / 联骨长」，只匹配到股骨长 1 项；`us_02` 的 BPD/AC 还因 OCR 把 `mm` 读成 `m` 丢失单位。继续扩词典追乱码不可持续。
- 02 已交付的 `sections[]` 完整保留原文与 OCR 行链接；对影像报告，段落 + 原图已经覆盖家长需要的信息。

## 验收清单

- [ ] **影像详情页主体**：`report_type=影像` 时保留头部（检查项目 / 日期 / 失败原因）、`sections` 段落卡与 LineZoom、原图入口；隐藏「指标项(N)」标题、全部 `ReviewItemCard` 与「逐项核对 N 项」入口。
- [ ] **未知语义**：影像报告头部与时间线不显示 `未知 N` 徽标；影像项不再渲染，因此也不再出现逐项「未知」flag。检验单的 unknown 徽标与核对入口保持原样。
- [ ] **日期文案**：影像报告 `report_date=null` 显示「检查单未印日期」；有日期照常显示 `YYYY-MM-DD`。检验单仍显示「日期未知」。
- [ ] **状态文案**：影像报告不显示没有对应动作的「待核对」；`failed` 仍显示「识别失败」与 `fail_reason`，`success` 可显示「识别完成」。不修改后端 status 派生，只在 App 展示层按报告类别取文案。
- [ ] **failed 影像单**：段落卡、失败原因、原图、重新拍照入口并存，不被空态遮挡；隐藏指标项不影响失败出口。
- [ ] **时间线**：影像报告卡片不显示 `未知 N`；日期/类别/失败原因保持；检验单卡片不变。
- [ ] **边界**：`sections` 缺省或为 null 时，影像报告仍显示头部 + 原图，不崩、不空屏；非影像报告完全走原路径。
- [ ] **App E2E · us_01 / us_03**：详情页不出现「指标项」「未知」「逐项核对」；检查项目 / 段落 / 日期 / 原图入口可见；us_01 段落卡仍可展开 LineZoom。
- [ ] **App 回归**：既有 cbc 用例仍显示指标项、核对动作与 unknown 徽标；`npm run typecheck`、jest、`npm run e2e` 全绿。

## 实现提示

- 新增 `IMAGING_REPORT_TYPE = '影像'` 常量与 `isImagingReport(report)` 判定，避免裸字符串散落。
- `ReportCard` 的时间线/详情共用展示：日期文案、status 文案、`ReportBadges` 的 unknown 过滤都从报告类别派生；不要复制两套卡片。
- 详情页按 `isImagingReport(report)` 分支隐藏 items / pending 区块；段落与 LineZoom 继续复用 02 的 `ReportSectionCard`。
- 原图入口沿用现有折叠开关，不强制默认展开；failed 仍默认展开。
- 所有新展示判定放纯函数（如 `reportDateLabel` / `reportStatusLabel` / `flagBadges` 的类别参数）并补单元测试；E2E 用真实 us_01 / us_03。

## Out of scope（后续票）

- 后端停止影像 items 抽取、改分类 / status 派生 / golden / verify。
- 删除或迁移既有影像 items 数据。
- OCR 模型升级、低置信 `exam_name` 处理、段落人工修正。
- 生长趋势 / 孕周评估 / 测量值曲线。
