# Spec：未检测指标状态（结果与参考区间均为 “-”）

Status: ready-for-agent

维护者 2026-10-25 定稿：结果与参考区间都是**裸短横 `-`** = 本次检查没有做该项，产品术语「未检测」。它不是 normal、不是 unknown；报告仍 success，逐项灰色展示 + 报告卡「未检测 N 项」信息徽标，不进入核对/待处理/趋势/异常计数。只对未检测项做原图像素确认，不推断缺失；单表单组表头才恢复，多表跳过。

**依据文档：** `CONTEXT.md`（指标项 / 未知 / 未检测 / 核对 / 趋势）· `docs/adr/0011-report-item-not-tested-state.md` · `schemas/report.schema.json` / `schemas/openapi.yaml` · 现有实现 `recognizer/recognizer/{postprocess,dash_recovery,contract}.py`、`internal/{pipeline,httpapi,storage}`、`apps/mobile/src/{reports,review}`。

## 为什么做

尿常规等定性表里，结果列与参考区间列常同时印孤立的 `-`。它表示**本次没有检测该项**，不是阴性、不是正常，也不是 OCR/比较失败。

现状问题：

- RapidOCR 对孤立短横线经常不输出；后处理得到 `value=null、ref_range=null、flag=unknown、reason=ocr_miss`，报告降为 `partial`；
- 这些项通常不是低置信项，核对页只列低置信项，家长在核对页看不到，只能在详情逐项修正；
- 上一版把两侧裸短横当成 normal，语义错误：正常表示“可比较且落在参考范围内”，未检测没有结果；
- 直接把“缺失”推断成 `-` 不安全：OCR 漏掉的 `+` 也会被误判成正常/未检测。

## 做什么

### 产品表达

- **术语**：未检测（`not_tested`）；详情文案「本次未检测该项」；报告卡「未检测 N 项」。避免「未检查」——App 里「核对」是家长动作，易混淆。
- **逐项展示**：报告详情保留该行，灰色「未检测」，结果/参考显示 `-`，不画正常/异常条带，不染异常色；保留原文与原图定位。
- **报告级**：`not_tested_count` 派生字段，报告卡/详情显示信息徽标；`FlagCounts`（偏高/偏低/异常/可操作未知）不变。
- **报告状态**：仍为 `success`；未检测项不触发 `partial`、不计入低置信多数、不计入 `unreviewed_low_confidence_count`。
- **核对**：未检测项不是核对任务；核对页不列；详情保留次级「修正」作为 OCR 误判逃生口（改回真实值/参考后 flag 重算）。
- **趋势**：不产生趋势点；相邻已检测点正常连接；不在图上加“未检测”标记。
- **判定边界**：只有结果、参考两格都是裸短横（`-`/`－`/`—`/`−`/`–`）才是未检测。
  - 结果 `-` + 参考 `阴性(-)`/`阴性` → 已做，按阴性/正常；
  - 结果 `阴性` + 参考 `-` → 已做，按阴性/正常；
  - 结果 `+` + 参考 `-` → 已做，按阳性/异常；
  - 结果 `-` + 数值参考 → 不猜未检测，保留现有比较/未知；
  - 空白、`未做`、`/` 等其他印形不在本期自动推断。
- **恢复范围**：RapidOCR 漏掉短横线时，仅用原图像素确认“单条水平短横线”后补 `-`；有文字、加号、纵向笔画或表格线一律不补；多表/重复表头跳过。
- **历史**：只对新上传/重新识别生效；不做全局回填。当前 09-04 尿常规单独重新识别一次；先做只读扫描确认影响范围。

### 数据模型

- `ReportItem.flag` 新增一等状态 `not_tested`；`flag_reason` 为空（不是 unknown）。
- `ReportHeader`（Summary/Detail）新增派生 `not_tested_count`，不落列。
- DB：迁移 `0010_report_item_not_tested.sql` 扩展 `report_items.flag` CHECK。
- `/reparse` 只有存库 OCR，没有原图像素，不恢复短横线；需走 `/report` 重新识别。
- App 重新生成 OpenAPI 类型；未检测标签/灰色/徽标/核对排除/趋势排除。

### 明确不做

- 不把“缺失/空白”推断为未检测。
- 不在多表/重复表头布局上猜列归属。
- 不新增报告级 `incomplete` 状态；不把未检测放进 `待处理`。
- 不做历史批量重识别；不自动回溯改库。
- 不改 App 现有「核对只列低置信项」的口径。

## 怎么实施

1. **识别端**
   - `dash_recovery.py`：单表单组表头时，按表头行、名称行与原图像素，分别确认结果/参考格是否为唯一水平短横线；只补缺失字段 `-`。
   - `postprocess._compare_qualitative`：`_policy_text(value) == "-" and _policy_text(ref) == "-"` → `Flag.NOT_TESTED`，先于阴性/阳性规则；单侧短横仍按现有 negative 规则。
   - `postprocess.run_postprocess`：`not_tested` 不触发 partial；低置信多数仅统计已检测项。
   - `/report` 传原图字节；`/reparse` 不传。
   - `flag_policy.yaml` 注释更新；共享向量 `flag.vectors.json` 双短横 → not_tested。
2. **数据服务**
   - `pipeline.FlagNotTested`；`Compare` 同规则。
   - `pipeline.ReportSummary.NotTestedCount` / `httpapi.ReportHeader.NotTestedCount`；聚合 SQL `COUNT(*) FILTER (WHERE flag='not_tested')`。
   - `unreviewed_low` 派生与 `sqlReportDeriveSuccess` / `sqlReportReviewState` 排除 not_tested。
   - 趋势 SQL 排除 not_tested。
   - 迁移 0010；HTTP fixtures/契约测试。
3. **App**
   - `FLAG_LABELS` / `FLAG_COLORS` 增 `not_tested`（未检测 / 灰色）。
   - `flagBadges` 增「未检测 N」信息徽标；`FlagBadgeKind` 扩展。
   - `needsReview` / `pendingItems` 排除 not_tested（即使 low_confidence）。
   - `reviewActions` 给 not_tested 无主操作、保留「修正」逃生口；`applyReviewResult` 重算 `not_tested_count`。
   - App 测试夹具补 `not_tested_count`；状态/核对/趋势单测覆盖。
4. **验收**
   - `pytest`（recognizer）、`go test ./...`、App `npm test` / `typecheck` / `lint` 全绿。
   - App E2E 15/15（现有样本无 not_tested；不改既有用例）。
   - 只读扫描共享库 15 份报告：列出将出现 not_tested 的报告；对 09-04 尿常规单独 `override=true` 重识别，确认 8 项 `未检测`、报告 success、`not_tested_count=8`。
   - 现有 samples 有原图重跑无 golden diff（无误报）。
5. **已知边界**
   - 多表/重复表头布局跳过；`/reparse` 不恢复；历史报告需重新识别。
   - 旧 App 未重新生成类型时，`not_tested` 会显示为无标签/无徽标；需重新构建 App。
   - 多图上传分支合并本分支后，其 `ReportDetail` 测试夹具需补 `not_tested_count: 0`。
