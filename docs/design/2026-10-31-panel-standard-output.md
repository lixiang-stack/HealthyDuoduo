# 各 panel 标准输出端到端（2026-10-31，票 03–11）

## 为什么做

票 01 已把 `code/loinc/unit_display/unit_status/raw_*/ref_kind/result_kind/result_basis` 打通到契约、数据库、API 与 App，但识别端还没有产出这些字段；票 02 已把真实单位换算与 OCR 印形修复拆成两张表。本批（票 03–11）把 9 个词典的全部 181 项迁移到标准 schema，并让后处理按项输出 canonical 单位、换算后的数值/参考范围与原文快照。

不做的后果：`unit` 继续携带 `109/L`、`7/6.0`、`ag/L` 等 OCR 碎片；HCT、HbA1c、尿酸、D-二聚体等单位不统一，趋势不可比；`unit_status` 无法区分“已确认”与“仅 OCR 匹配”。

## 做什么

### 输出模型（全部 panel 共用）

| 字段 | 规则 |
|---|---|
| `name` | 迁移为 WS/T 886 标准名；旧规范名保留在 `synonyms` 供匹配 |
| `code` | WS/T 886 码；无码时回退 `local_code`；`loinc` 单独透传 |
| `value` / `ref_range` | 有可信换算时换算到 canonical；否则保留印形 |
| `unit` | canonical UCUM/约定单位；无法可靠解析时为 `null` |
| `unit_display` | 人类可读展示单位（如 `10^9/L`、`μg/L`、`个/μL`、`秒`） |
| `unit_status` | `verified` 仅当词典项 `review_status=approved`；当前全部 `pending` → `unverified` |
| `raw_value` / `raw_unit` / `raw_ref_range` | 单据原文快照，始终保留 |
| `ref_kind` | `numeric` / `qualitative` / `ordinal` / `tiered` / `none` |
| `result_kind` | `quantitative` / `ratio` / `arbitrary` / `titer` / `qualitative` / `microscopy` |
| `result_basis` | 仅 D-二聚体 `FEU`（DDU 不换算） |

**待复核项的口径（用户确认）**：`review_status=pending` 时仍输出可可靠归一/换算的 canonical 单位，但 `unit_status=unverified`；只有单位无法解析、量级门拒绝或换算不可用时才 `unit=null`，并置 `low_confidence=true`。票 14/15 复核后翻 `approved`，`unit_status` 才可能变 `verified`。

### 各 panel 要点

| 票 | panel | 关键决定 |
|---|---|---|
| 03 | 凝血 | PT/APTT/TT → `s`/`秒`；PTR/INR → ratio `1`；D-二聚体 → `ug/L` + `FEU`，`ng/mL`（×1）、`mg/L`（×1000）；FIB `g/L`，FDP `ug/mL` |
| 04 | CBC | HCT `%` ×0.01 → `L/L`（value + ref 同步）；WBC/RBC/PLT 等值替身归一；RDW-CV `%` 与 RDW-SD `fL` 分开；PDW 按单据 `fL` 标 local（SD 定义），不把 `%` 当换算 |
| 05 | 肝功 | 胆红素 `mg/dL` ×17.1 → `μmol/L`；蛋白 `g/dL` ×10 → `g/L`；AST/ALT、A/G 保持 ratio；酶类 `IU/L` 与 `U/L` 不默认等价 → `unit=null` + unverified |
| 06 | 肾功/尿酸/电解质 | 尿酸 `mmol/L` ×1000 → `μmol/L`；eGFR canonical `mL/min/1.73m2`，`mL/min` 缺 BSA 归一 → unverified；CO2CP 标准名改 `二氧化碳结合力` |
| 07 | 血脂/血糖/维生素 | HbA1c `%` → `mmol/mol` 用 IFCC-NGSP 方程；血糖/血脂 `mg/dL` 线性换算；C 肽、胰岛素、25(OH)D、铁蛋白、叶酸、B12 的替代单位按 pending 系数换算；`mol/L` 等 1000× 错误被量级门拒绝 |
| 08 | 激素 | T3/T4 → `nmol/L`，FT3/FT4 → `pmol/L`，TSH → `m[IU]/L`；T3 `mmol/L` 换算后 2040 nmol/L 超量级门 → `unit=null`；TPOAb/TgAb 的 `U/mL` 与 `IU/mL` 不互换 |
| 09 | 尿常规 | 干化学定性无 unit、`ref_kind=qualitative`；沉渣计数 `个/uL` → canonical `/uL` + display `个/μL`；镜检项 `result_kind=microscopy`、unit 为 null，原文 `/HP`、`/LP` 留在 `raw_unit` |
| 10 | 免疫 | 自身抗体 `AU` → `[arb'U]` + display `AU`；`S/CO` → ratio `1`；定性项无 unit；dsDNA `[IU]/mL`、C1q `U/mL`、TPOAb/TgAb `[IU]/mL` 不互相换算 |
| 11 | 专项/血型 | 产前风险比值按 ratio 建模，原文 `1:19879` 保留在 `raw_value`；血型/抗体筛查 qualitative 无 unit；hCGβ `ng/mL` → `ug/L`，`U/mL` 不可换算 |

## 怎么实施

1. **词典数据**：7 个非凝血词典按 `mapping.csv` 迁移到标准 schema；标准名、码/local_code、`result_kind`、canonical/display、`accepted_units`（类型化 exact/linear/equation/none）、`plausible_range`、`review_status=pending`。旧 `aliases` 保留用于召回；旧规范名移入 `synonyms`，避免进入 OCR 语料门。
2. **输出层**：`postprocess._standardize_item()` 按项解析印形 → canonical；`exact` 原值、`linear` 用 Decimal 换算并保留原文、`equation` 按 `id` 实现（不 eval 词典字符串）；value 与 numeric ref 同步换算；分级/定性 ref 不换算。
3. **量级门**：`plausible_range` 只做单位/量级 sanity check；换算后 value/ref 越界 → 不输出 canonical、`low_confidence=true`。T3 `mmol/L`、血糖 `mol/L` 由此被拦截。
4. **单位表**：`unit_conversions.yaml` 增加 canonical 真实单位；`ocr_forms.yaml` 增加有语料证据的损耗形（`rmol/L`、`ag/L`、`IU/al`、`ng/d1`、`ug/d1`、`t`、`1/00.91`、`T/6.01` 等）。同一 form 可属多族，由项注册集裁决。
5. **定性 flag**：`flag_policy.yaml` 增加标准名键，保留旧名键供存量行；否则定性项的 normal/abnormal 会退化为 unknown。
6. **测试**：`test_standard_schema.py` 增加 181 项身份/单位/换算完整性门禁；`test_postprocess.py` 增加 03–11 的 golden 与合成样本断言。
7. **golden**：全部 49 份重生成；status/item 数与 flag 零变化，value/unit 变更逐类见下。

### golden 变更分类

- **单位换算（value + ref 同步）**：HCT `%→L/L`（9 份 CBC）；HbA1c `%→mmol/mol`（2 份）；尿酸 `mmol/L→μmol/L`；C 肽/胰岛素/25(OH)D/叶酸/B12、T3/T4/FT3/FT4 的替代单位。
- **仅单位归一（值不变）**：凝血 `秒→s`；D-二聚体 `ng/mL→ug/L`；CBC 计数 `109/L`、`7/6.0`、`1/00.91` 等 → `10*9/L`；尿沉渣 `个/uL→/uL`；免疫 `AU→[arb'U]`、`S/CO→1`。
- **置 null + unverified**：RDW-CV/SD 的错误印形、PDW `%`、eGFR `ml/min`、`IU/L` 酶类、T3 `mmol/L`（量级门）、血糖 `mol/L`（量级门）、TSH `.9`、C1q `0.`、镜检 `/HP`、`/LP` 等。
- **状态/计数/flag**：全部 49 份 status、item 数、flag 与迁移前一致（逐份脚本比对）。

## 怎么验收

- `cd recognizer && uv run pytest -q`：370 passed / 28 skipped。
- `cd recognizer && uv run ruff check .` 全绿。
- `go test ./...` 全绿（契约/存储/API 未改字段）。
- `python3 scripts/check-docs.py` 全绿。
- 数据门禁：181/181 有 `result_kind` + 码/local_code + source/review；quantitative 全部有 canonical；`accepted_units` 换算方向一致；无 OCR 碎片进入 canonical；有线性/方程换算的项都有 `plausible_range`。
- golden 门禁：`unit` 仅 canonical；pending 项无 `verified`；`value` 非空时 `raw_value` 存在。
- 未做：真库 `hdd reparse` / e2e verify（worktree 无本地数据，留票 13）；趋势身份与未验证排除留票 12（本票保证 `unit` 已是 canonical，未验证点不会冒充 verified）。

## 风险与待复核

- 全部 `review_status=pending`：标准名、换算系数、量级范围需票 14/15 逐项确认；`unit_status=verified` 尚未出现。
- 部分系数来自 `mapping.csv` 的“待核”标注（血脂、C 肽、胰岛素、维生素、T3/T4 等）；当前换算结果只用于展示与趋势准备，不声称已复核。
- PDW 暂按单据 `fL` 标 local（SD 定义）；若复核要求拆 SD/CV，票 14 再调整。
- eGFR 的 UCUM 表达 `mL/min/1.73m2` 待复核；当前 `mL/min` 一律 unverified。
