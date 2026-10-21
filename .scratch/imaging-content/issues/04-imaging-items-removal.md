# 04: 影像单停止指标项抽取并清理存量 items

**What to build:** 影像（超声）分类改为按报告正文段落锚（`findings` / `impression`）判定，不再依赖产科测量项；影像报告识别后不再产出或落库任何指标项，报告主体只有 `sections`（检查项目 / 检查所见 / 超声提示）。影像 status 由正文段落派生（有正文段落 = success，无正文段落 = failed），日期未印与测量值低置信不再触发 partial。存量影像 items 删除。非影像类别、核对流程与 App 现有呈现不变。

**Blocked by:** None（03 已 resolved）。

**Status:** resolved

## 背景与依据

- 03 已在 App 层收口：影像详情只呈现段落 + 原图，不呈现指标项、未知 / 待核对徽标。后端仍按检验单模型产出 18 个影像 items（us_01 6 / us_02 11 / us_03 1），全部 `ref_range=null`、`method=null`、`flag=unknown`；这是模型错配，不是识别失败。
- 当前影像 status 由 items 派生：无日期 → partial；低置信占比 ≥ 50% → partial（us_03 单指标项 `low_confidence=true`）。items 移除后必须同时改 status 派生，否则会落入「零 items ⇒ failed」不变量。
- `internal/pipeline/reports.go` 与 `internal/storage/store.go` 都有「空 items 必须 failed」的双保险；叙述型报告需要按 `sections` 开口子。
- 时间线 / trend / `flag_counts` / `unreviewed` 计数都读 `report_items`；只停止新产出、不清理存量会让旧影像单继续出现在趋势与徽标里。

## 已确认口径

1. **按报告段落分类，不按产检测量项**（2026-10-20 维护者提议）：影像词典删除全部 14 个测量项；`_select_category` 以正文锚（`findings` / `impression`：超声描述 / 检查所见 / 超声所见 / 超声提示 / 检查提示）作为影像的资格信号，`exam_name` 锚（检查项目 / 部位 / 内容）不参与分类。`LabDict` 增加 `min_section_hits`（影像 = 1）；达标 = 指标项命中 ≥ `min_matched_items` **或** 正文锚命中 ≥ `min_section_hits`；排序键 `(qualified, 正文锚命中数, 指标项命中数, 标题区命中)`；标题关键词保留兜底。

   证据：49 张样本 OCR 中只有 us_01 / us_02 / us_03 命中正文锚（三张均含 `findings` 或 `impression`，us_03 仅 `impression`），其余 46 张零命中；14 个测量项全是产科测值，无法覆盖腹部 / 甲状腺 / 乳腺等其他影像子类，不适合作为影像分类依据。

2. **影像 status 只反映叙述正文**：`findings` 或 `impression` 至少一段 → `success`；两者都没有 → `failed` + 固定 `fail_reason`。`report_date=null` 不再降级 partial（03 已有「检查单未印日期」文案）；影像不进入 partial，`unreviewed_low_confidence_count=0`。
3. **存量 items 删除而非迁移**：影像 items 是模型错配数据，没有可迁移的核对价值；删除 `report_items` 时 `correction_pairs` 级联删除。用迁移 0008 做一次性清理，reparse 全量替换保证幂等。
4. **边界**：不新增无真实样本支撑的正文锚（如「超声诊断 / 检查结论」）；正文锚缺失但标题命中 `超声 / 彩超` 时仍按影像归类，status 因无正文段落判 failed。

以上口径即按此实现。

## 验收清单

- [x] **识别端输出**：us_01 / us_02 / us_03 的 `report_type=影像`、`items=[]`、`sections` 逐段与现 golden 一致；非影像报告输出逐字节不变。
- [x] **分类按正文锚**：us_01 / us_02 / us_03 均识别为 `影像`；其余 46 张样本零误分类；`exam_name` 锚（检查项目 / 部位 / 内容）不参与分类，避免「检查项目」类表头误判。正文锚缺失但标题命中 `超声 / 彩超` 时仍保留标题兜底。
- [x] **status 派生**：有 `findings` / `impression` → success（即使 `report_date=null`）；无正文段落 → failed，`fail_reason` 固定为 `no report body recognized`；影像不再产生 partial。
- [x] **空 items 不变量**：`pipeline.applyFailInvariant` 与 `storage.WriteReport` 允许「零 items + 非空 sections」；仍拒绝「零 items + 空 sections + 非 failed」。相关 Go 单测覆盖两条分支。
- [x] **golden / HTTP fixtures**：`samples/expected/report/us_01/02/03.json` 改为 `items=[]` + 新 status；`schemas/testdata/http/report.detail.imaging.json` 与 `report.sections.example.json` 同步（items 空、`flag_counts` 全 0）；其余 report golden 逐字节不变。
- [x] **verify**：默认 `verify` 对「有 sections 的报告」硬门禁 `items=0`，并校验 status 与正文段落一致；`verify --golden` 的 items / sections 比对在零 items 下通过；非影像检查不变。
- [x] **存量数据**：迁移 0008 删除 `reports.report_type='影像'` 的 `report_items`（`correction_pairs` 级联）；共享库 18 条影像 items 清零，19 reports 数量不变；reparse 后仍为 0。
- [x] **App 回归**：App E2E seed 的 us_01 / us_03 现在 `items=[]`；更新「后端仍有指标项」断言与逐项隐藏循环；详情仍显示检查项目 / 段落 / 日期 / 原图，cbc 核对闭环不变；`npm run typecheck`、jest、`npm run e2e` 全绿。
- [x] **文档**：更新 `CONTEXT.md` 的报告定义（叙述型报告以段落为主体、不含指标项）；新增 ADR 记录「叙述型报告零 items 不变量 + status 派生」，并在 ADR-0008 加交叉引用。
- [x] **全链路**：pytest、`go test ./...`、App 三条回路全绿；共享栈按 `snapshot → refresh → verify → seed → verify --golden → clean` 跑通。

## 实现提示

- `imaging_dict.yaml` 删除 `items` 与 `min_matched_items`，新增 `category.min_section_hits: 1`；保留 `title_keywords` 与 `section_anchors`。
- `LabDict` 增加 `min_section_hits`，`_build_dict` 允许叙述型词典 `items: []`；`_select_category` 计算每个词典的正文锚命中数，按 `(qualified, 正文锚命中数, 指标项命中数, 标题区命中)` 排序。`exam_name` 锚只用于段落抽取，不参与分类。
- `run_postprocess`：影像词典没有 items，items 循环自然为空；sections 抽取保持现状，并按正文段落派生 status。
- `fail_reason`：零 items + 空 sections 时，影像报告用 `no report body recognized`，非影像继续 `no metric items recognized`；在 contract 增加报告类别常量，避免 Go 层裸字符串。
- 迁移 0008：`DELETE FROM report_items USING reports WHERE report_items.report_sha256 = reports.image_sha256 AND reports.report_type = '影像';`（`correction_pairs` 有 `ON DELETE CASCADE`）。Down 不做数据恢复，注释说明可经 reparse 重新生成（但新规则下仍为空）。
- `verify` 的叙述型判定用 `reports.sections IS NOT NULL`，不要硬编码 `report_type`，便于未来其他叙述型类别复用。
- App 端 03 已按类别隐藏 items，预计只需改 E2E seed 断言与受影响的 fixture，不改页面逻辑。

## Out of scope

- OCR / 词典识别精度、乱码、低置信 `exam_name` 处理。
- 段落级核对 / 修正 / 编辑 / 搜索。
- 为腹部 / 甲状腺 / 乳腺等影像子类新增专用锚或词典（本票只做通用正文锚分类与 items 移除，新增锚需真实样本驱动）。
- 孕周评估、生长趋势、测量值曲线。
- App 详情布局调整（03 已交付）。
