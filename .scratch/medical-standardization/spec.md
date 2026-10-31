# Spec：医学检验词典标准化（medical-standardization）

Status: draft（2026-10-30，待医学复核人确认；`review_status=pending`）

**范围：** 9 个 `recognizer/recognizer/*_dict.yaml` 中指标项的 `name` / `code` / `unit` / `result_kind` / `ref_range`；含识别层的 `synonyms` / `ocr_aliases` / `ocr_unit_forms` 拆分。不含 `panel` / `report_type` 分类与 `method` 字段（它们是另一件事）。

**依据：** WS/T 886—2026《临床检验常用项目名称及代码》· WS/T 404 系列《临床常用生化检验项目参考区间》· WS/T 405—2012《血细胞分析参考区间》· WS/T 406—2024《临床血液检验常用项目分析质量标准》· WS/T 461—2024《糖化血红蛋白检测指南》· WS/T 477—2026《D-二聚体定量检测标准》· WS/T 478—2024《血清25-羟基维生素D2和D3的检测》· UCUM · LOINC（NLM Clinical Tables API 用于候选码核对）。

## 0. 决策摘要

| # | 决策 | 结论 |
|---|------|------|
| 1 | 标准源 | 名称/代码：WS/T 886—2026 + LOINC；单位：UCUM + WS/T 404/405/406/461/477/478；参考范围继续从单据抽取，不内置 |
| 2 | 数据分层 | 标准层：`synonyms` / `canonical_unit` / `accepted_units`；识别层：`ocr_aliases` / `ocr_unit_forms`。两层分文件、分证据规则 |
| 3 | 未验证单位 | `raw_unit` 保留；精确等价才归一；否则 `unit=null` + `unit_status=unverified` + `low_confidence=true` |
| 4 | 数值/参考范围 | 标准优先：`value`、`ref_range` 换算到 canonical 单位；定性/分级范围保留原文 + `ref_kind`；无权威换算不猜 |
| 5 | 非定量结果 | `result_kind` = quantitative / ratio / arbitrary / titer / qualitative / microscopy；不把 S/CO、AU、滴度、/HP 塞进 `unit` |
| 6 | 名称/代码 | `name` = WS/T 886 标准名；`code` = WS/T 886 码；`loinc` = LOINC 码；无 exact 标准码用 `local_code` + `source_status=local`，不伪造 |
| 7 | OCR 噪声 | 保留在识别层（`ocr_aliases` / `ocr_unit_forms`），带样本证据，不进入标准层 |
| 8 | 复核 | 每项 `source` + `review_status`；量化项必须有 canonical unit + source 才允许 `approved` |
| 9 | 边界 | 只覆盖指标项；`panel` / `report_type` / `method` 不在本期 |

**兼容性：** 产品未上线，允许破坏性重命名；不设双读窗口、不保留旧字段别名。

## 1. 审计结论（四问一因）

### 1.1 准确性/标准化：没有可追溯的系统性核对

- Panel 归属有依据：`cbc_dict.yaml:28-30`、`chemistry_dict.yaml:6-15`、`endocrine_dict.yaml:6-11`、`coag_dict.yaml:7-8`、`immunology_dict.yaml:6-10`、`ua_dict.yaml:14-16` 的头部注释给出了分类依据。
- 但 name/alias/unit 没有逐项来源：没有一条单位/别名标注标准条款、LOINC 码、UCUM 码或复核人。
- `test_dict_coverage.py` 只测四件事：别名有样本证据（`:85-94`）、panel 合法（`:99-111`）、panel 依据注释（`:114-140`）、词条不是孤儿（`:143-153`）；**没有单位正确性测试**。
- `units` 并没有逐项生效：`_build_dict` 把整个词典的单位并集写入 `unit_literals`（`postprocess.py:256-263`），`_unit_match` 匹配词典级并集（`:826`），`_unit_or_none` 还会接受任意 ASCII 单位形态（`:846`）。所以 `ALT` 只登记 `U/L`，`lft_06` 能输出 `IU/L`；`RDW-SD` 只登记 `fL/fl`，`cbc_05` 能输出 `%`；`尿酸` 只登记 `mmol/L`，`lft_08` 能输出 `μmol/L`。
- golden 的 57 个非空 `unit` 中至少十几个是 OCR 碎片：`ag/L`、`rmol/L`、`10/a1`、`IU/al`、`T/6.01`、`1/00.91`、`*10`、`.9`、`0.`、`t`、`ng/al`、`ug/d1`、`pg/al`、`ng/d1`、`unol/1`、`mnol/L`、`mmo1/L` 等。
- 具体医学问题：`尿酸` 只有 `mmol/L`（标准 μmol/L）；`T3` 接受 `mmol/L`（标准 nmol/L，差 1000×）；`T4` 接受 `rmol/L`（OCR）；`eGFR` 只有 `ml/min`（标准 mL/min/1.73m²）；HbA1c 只有 `%`（WS/T 461—2024 同时给 IFCC mmol/mol）；D-二聚体只有 `ng/mL`（WS/T 477—2026 要求 FEU/DDU 口径，推荐 μg/L）；HCT 只有 `%`（WS/T 405 用 L/L）；`S/CO`、`AU` 被当作单位。

**结论：** 没有按权威标准系统核对过；现状是“OCR 语料证据”，不是“医学标准证据”。少数项碰巧与标准一致，不能代表整体。

### 1.2 aliases / units：部分是标准，整体不是标准集合

**aliases 混了四类东西：**

1. 标准缩写：`WBC`、`HGB`、`ALT`、`TSH`、`HbA1c`；
2. 标准中文同义名：`白细胞`、`血红蛋白`、`谷丙转氨酶`；
3. OCR 损坏形：`淤离三碘甲状腺原氨酸FT3`（`endocrine_dict.yaml:39`）、`问接胆红素[1-Bi1]`（`chemistry_dict.yaml:71`）、`谷草转氨酮`（`:31`）、`碱性硫酸酶`（`:43`）、`抗Sc1-70抗体`（`immunology_dict.yaml:42`）、`抗SS—A 60抗体`（`:30`）、`RH血型`（`bloodtype_dict.yaml:14`）；
4. 临床等价性错误：`尿素` 挂 `BUN`、`甲状腺素` 挂 `T74`、`二氧化碳` 挂 `Co2CP`。

第 3 类能通过 `test_dict_coverage`，因为测试只问“样本里有没有这个字符串”，不问“这个字符串是不是标准名称”。**“有语料证据”不等于“医学上正确”。**

**units 同样混了三类：**

- 规范单位：`10^9/L`、`g/L`、`μmol/L`；
- 真实替代单位：`mg/dL`、`ng/mL`、`%`；
- OCR 损耗形：`109/L`、`7/6.0`、`f1`、`ng/al`。

同一个物理单位在不同词典里还有多种 canonical 写法：`U/L` vs `IU/L`、`秒` vs `s`、`个/uL` vs `个/μL`、`ng/ml` vs `ng/mL`、`ug/L` vs `μg/L`、`ml/min` vs `mL/min`。这些不是“标准”，只是“历史上各自这么写”。

**结论：** 部分符合标准，但预定义集合整体不是标准集合；它没有标准来源字段，且把 OCR 损耗当成了单位值。

### 1.3 同一个指标真的没有这么多单位

**没有。真实单位很少，长列表绝大多数是 OCR 字形变体。**

例如白细胞：

- 真实单位只有 `10^9/L` 和 `10^3/μL`，二者是同一个物理单位；
- 词典里的 `10^9/L / *10^9/L / 10~9/L / *10~9/L` 是四种**印形**；
- `109/L / 7/6.0 / 1/6.0 / .09/L / ~9/L / f1` 是六种 **OCR 损耗**；
- 把它们叫“6 个单位”，是把“字形”当成了“量纲”。

同时，**真实替代单位反而缺**：

| 指标 | 真实可换算单位 | 词典现状 |
|---|---|---|
| WBC/PLT | `10^9/L` ≡ `10^3/μL` | 4 个印形 + 11 个 family forms |
| 葡萄糖 | `mmol/L` ↔ `mg/dL` | 5 个 entries，含 `mol/L`、`unol/1`、`mnol/L`、`mmo1/L` |
| 血红蛋白 | `g/L` ↔ `g/dL` | 只有 `g/L` |
| 尿酸 | `μmol/L` ↔ `mg/dL` | 只有 `mmol/L` |
| T3/T4 | `nmol/L` ↔ `ng/dL`/`ng/mL` | 含 `mmol/L`、`rmol/L`、`ng/d1` |
| HbA1c | `%` ↔ `mmol/mol` | 只有 `%` |
| D-二聚体 | FEU/DDU；推荐 `μg/L` | 只有 `ng/mL` |
| 红细胞压积 | `L/L` ↔ `%` | 只有 `%` |

所以准确说法是：**不是“单位太多”，而是“噪声太多、标准单位不全、真假混在一个字段里”。**

### 1.4 unit_families.yaml 确实被噪声影响，但它的定位本身就有问题

你说得对：`.09/L`、`)12/L` 不是单位。文件头也明确写了它是 **OCR 损耗形**（`unit_families.yaml:9-16`），`.09/L` 来自 `cbc_05` 的熔断尾片（`:28`），`)12/L` 来自 `cbc_05` 的尾片（`:36`）。作为“单位族标准表”，它被噪声污染；作为“OCR 修复表”，它是故意的。

真正的问题是它**把两种完全不同的关系混在一张表里**：

- **精确等价**：`uIU/ml` → `mIU/L`（`:63`），这是真实的单位换算；
- **字形修复**：`7/6.0` → `10^9/L`、`.09/L` → `10^9/L`、`)12/L` → `10^12/L`，这是根据样本猜 OCR 印形。

这两种关系的证据要求、置信度、使用方式都不同。再加上：

- `f1` 同时属于 `fL` 和 `10^9/L` 两族（`:30`、`:57`），靠“该项注册集”裁决；
- 每条 form 通常只有一个样本证据，没有置信度字段；
- 没有覆盖真实替代单位，也不解决换算。

所以它不是“单位标准表”，而是“OCR 损耗修复表”。**不应该删掉它；应该拆开、改名、加证据等级。**

### 1.5 根因：一个字段承担了两个互相冲突的职责

`units` 同时在回答两个问题：

1. **“OCR 可能把单位印成什么？”** —— 开放世界，见过就收，样本驱动；
2. **“这个指标在医学上允许什么单位？”** —— 封闭世界，有标准才收，标准驱动。

前者要求“尽量全”，后者要求“尽量准”。目标冲突，噪声必然进入。继续加 alias/unit，只会让识别更全、标准更差。

第一性原理的解法是分层：

- **标准层**：名称/代码、规范单位、真实替代单位、换算方式、来源、复核状态；
- **识别层**：同义名、缩写、OCR 别名、OCR 单位印形；
- **输出层**：只暴露经过验证的 canonical unit，同时保留 `raw_unit` 溯源。

这不是“再加一层架构”，而是把原本混在一个字段里的两种数据分开。

### 1.6 支撑数据

- 9 个词典共 181 项、341 条别名、66 个单位字面量。
- golden 的 57 个非空 `unit` 中至少十几个是 OCR 碎片（清单见 §1.1）。
- `test_dict_coverage.py` 只覆盖别名/panel/孤儿，不覆盖单位正确性。
- `postprocess.py` 的词典级单位并集（`:256-263`、`:826`）与任意 ASCII 兜底（`:846`）是“units 不按项生效”的直接原因。

## 2. 目标与成功标准

**目标：**

1. 每个量化项都有：标准名、WS/T 886 码或 `local_code`、LOINC 码或 `missing`、canonical unit + UCUM、真实替代单位 + 换算、`result_kind`、来源、复核状态。
2. `unit` 字段永远不携带 OCR 碎片；未验证单位进 `raw_unit` + `unit_status=unverified`。
3. 已验证的换算同时作用于 `value` 与 `ref_range`，保证 `flag` 同量纲。
4. 识别层保留现有召回；OCR 形只出现在 `ocr_aliases` / `ocr_unit_forms`。

**成功标准：**

- 181/181 项有 `source_status`（`primary` / `secondary` / `local` / `pending`）。
- 量化项 100% 有 canonical unit + source；0 项 OCR 形进入 `canonical_unit` / `accepted_units`。
- `unit` 字段中 OCR 碎片为 0；未验证项均为 `null` + `unverified`。
- `pytest` 新增单位/来源门禁全绿；golden 变更逐项有说明。

## 3. 目标模型

### 3.1 词典 YAML

```yaml
report_type: 临检
category:
  title_keywords: [...]
  min_matched_items: 10
  panel: cbc

items:
  - name: 白细胞计数                 # WS/T 886 标准名
    code: "0100101A"                # WS/T 886 码；无则 null
    loinc: "6690-2"                 # LOINC 码；无则 null
    local_code: null                # 仅当无 WS/T/LOINC exact 码
    source_status: primary          # primary | secondary | local | pending
    review_status: pending          # approved | pending
    synonyms:                       # 标准同义名/缩写，必须有来源
      - term: 白细胞
        source: "WS/T 886 0100101A"
      - term: WBC
        source: "LOINC 6690-2"
    ocr_aliases:                    # OCR 变体，只认样本证据
      - term: 白细胞数目
        evidence: ["cbc_01"]
    result_kind: quantitative       # quantitative | ratio | arbitrary | titer | qualitative | microscopy
    canonical_unit: "10*9/L"        # UCUM 码
    unit_display: "10^9/L"          # 人类可读展示
    accepted_units:                 # 真实替代单位 + 换算；不含 canonical 与 OCR 形
      - unit: "10^3/uL"
        ucum: "10*3/uL"
        conversion: { type: linear, factor: 1.0, source: "UCUM" }
    ocr_unit_forms:                 # OCR 损耗形 → canonical；只做字形修复
      - form: "109/L"
        evidence: ["cbc_01"]
```

字段规则：

- `name`、`result_kind`、`source_status` 必填；`review_status` 默认 `pending`；`result_kind=quantitative` 时 `canonical_unit` 必填。
- 代码优先级：`code`（WS/T 886）→ `loinc` → `local_code`。三者都没有时 `source_status` 必须是 `pending` 或 `local`；不允许伪造标准码。
- `accepted_units` 只放**真实替代单位**；canonical 本身永远接受，不在 `accepted_units` 重复。`ocr_unit_forms` 只放**OCR 损耗形**。同一字符串不得同时出现在两处。
- `conversion` 支持 `linear`、`equation`、`none`：
  - `linear`：`factor` + `source`，仅用于有权威换算的项；
  - `equation`：`id` / `formula` + `source`（如 IFCC-NGSP HbA1c）；
  - `none`：不可换算（如 D-二聚体 FEU vs DDU、U/L vs IU/L），只能保留 `result_basis` 或标 `unverified`。
- `result_kind != quantitative` 时，`canonical_unit` 可为 null；`ratio` 仍用 `1`，`arbitrary` 仍用 `[arb'U]`，`titer` 用 `titer` 字段，`qualitative` 无 unit，`microscopy` 用 `field: HPF|LPF`。
- `result_basis` 只用于结果基准不是单位换算的项（D-二聚体 `FEU|DDU`）；缺失时不得把数值当 `verified`。

### 3.2 报告指标项（contract）

```json
{
  "name": "白细胞计数",
  "code": "0100101A",
  "loinc": "6690-2",
  "result_kind": "quantitative",
  "value": "6.50",
  "unit": "10*9/L",
  "unit_display": "10^9/L",
  "unit_status": "verified",
  "raw_value": "6.50",
  "raw_unit": "10^9/L",
  "ref_range": { "low": "4.00", "high": "10.00", "unit": "10*9/L" },
  "raw_ref_range": "4.00~10.00",
  "ref_kind": "numeric",
  "flag": "normal",
  "low_confidence": false
}
```

- `value` / `ref_range` 是 canonical；`raw_*` 是单据原文。
- `unit` 是 UCUM 码；`unit_display` 是人类可读展示。UI 默认显示 `unit_display`。
- `ref_kind`：`numeric` / `qualitative` / `ordinal` / `tiered` / `none`。
- 未验证：`unit=null`、`unit_status="unverified"`、`low_confidence=true`；`raw_unit` 保留。
- 趋势按 `code`（优先）/ `loinc` + canonical unit 连接；不再按 `name` 字符串连接。

## 4. 单位与换算规则

### 4.1 canonical 选择

canonical 采用 **SI / WS/T 报告单位**，不是单据惯用单位：

| 指标 | canonical | 真实替代（accepted） |
|---|---|---|
| WBC/PLT | `10^9/L` | `10^3/μL`（等值） |
| RBC | `10^12/L` | `10^6/μL`（等值） |
| HGB/MCHC | `g/L` | `g/dL`（×10） |
| HCT | `L/L` | `%`（×0.01） |
| 葡萄糖 | `mmol/L` | `mg/dL`（×0.0555） |
| 尿酸 | `μmol/L` | `mg/dL`（×59.48） |
| 胆红素 | `μmol/L` | `mg/dL`（×17.1） |
| T3/T4 | `nmol/L` | `ng/dL` / `μg/dL`（需复核） |
| FT3/FT4 | `pmol/L` | `pg/mL` / `ng/dL`（需复核） |
| HbA1c | `mmol/mol` (IFCC) | `%` NGSP（方程） |
| D-二聚体 | `μg/L` + `result_basis: FEU` | `ng/mL FEU`（等值）、`mg/L FEU`（×1000）；DDU 不换算 |
| eGFR | `mL/min/1.73m²` | `mL/min`（不换算，标 unverified） |

### 4.2 换算严格度

- **精确等值**（同一物理单位的不同印法）：直接归一，`source="UCUM"`。
- **有权威系数的线性换算**：`factor` + `source`（WS/T、IFCC、LOINC）；必须医学复核。
- **方程换算**：HbA1c `%` ↔ `mmol/mol` 用 IFCC-NGSP 方程，存 `conversion.id` + `formula`。
- **不可换算**：D-二聚体 FEU/DDU（结果基准不同）、U/L vs IU/L（酶单位体系不同）、AU vs IU/mL（任意单位 vs 国际单位）→ `conversion: none`，保留 `result_basis` 或 `unit_status=unverified`。
- 没有权威来源的系数，**宁可不换算**；不能猜。

### 4.3 量级合理性门（防“单位正确、量级错误”）

换算前必须做一次**量级合理性检查**：转换后的 `value` / `ref_range` 要落在该分析物的宽泛报告量级内；超出则 `unit_status=unverified`，不输出 canonical。

| 例子 | 单据 | 换算后 | 判定 |
|---|---|---|---|
| 尿酸 | `0.246 mmol/L` | `246 μmol/L` | 合理，转换 |
| HCT | `42.6%` | `0.426 L/L` | 合理，转换 |
| T3 | `2.04 mmol/L` | `2040 nmol/L` | 超出量级（正常约 0.5–10 nmol/L），不转换，标 unverified |
| D-二聚体 | `440 ng/mL` | `440 μg/L` | 合理，转换（需 FEU 基准） |

量级范围只用于**单位/量级 sanity check**，不用于临床 flag；具体上下限由检验复核人确认，写入每项的 `plausible_range`（canonical unit）。没有 `plausible_range` 的量化项不得标记 `approved`。

## 5. 映射表与分批

### 5.1 全量

- `mapping.csv`：181 项全量骨架；列含 `batch` / `dict` / `current_name` / `standard_name` / `wst886_code` / `loinc` / `loinc_status` / `unit_display` / `canonical_unit`（UCUM） / `accepted_units` / `result_kind` / `source_status` / `sources` / `review_status` / `notes`。
- 第一批（batch 1）：CBC 31 + 生化 44 + 凝血 9 + 激素 11 = 95 项；标准名/码、canonical unit、accepted units、result_kind 已填，待复核。
- 第二批（batch 2）：尿常规 40 + 免疫 38 + 专项 5 + 血型 3 = 86 项；骨架已列，标准名/码/单位在第二批映射时填。
- LOINC 码用 NLM Clinical Tables API 逐码回查 `LONG_COMMON_NAME`：`confirmed` = 码与名称匹配；`candidate` = 名称/方法学需复核；`missing` = 无 exact 码。`confirmed` 不等于医学复核通过，最终仍以 `review_status` 为准。

### 5.2 已知待核项

| 项 | 问题 | 处理 |
|---|---|---|
| `PDW` | fL 与 % 是两种定义（SD vs CV），不是单位换算 | 拆成 `血小板分布宽度-SD` / `-CV`，或只保留一种并标 `local` |
| `D-二聚体` | FEU/DDU 是结果基准，不是单位 | `result_basis` 必填；缺基准 → `unverified` |
| `HbA1c` | `%` 与 `mmol/mol` 是方程换算 | `conversion: equation`，IFCC-NGSP |
| `eGFR` | `mL/min` 与 `mL/min/1.73m²` 不是同一量 | canonical 用后者；前者 `unverified` |
| `U/L` vs `IU/L` | 酶单位体系不同，不默认等价 | 无权威换算 → `unverified` 或保留 `result_basis` |
| `S/CO` / `AU` / `1:80` / `/HP` | 伪单位 | `result_kind` 拆分，见 §3.1 |
| `二氧化碳` | 实际测 CO2CP | 标准名改 `二氧化碳结合力`（local）；或标 `local` |
| `尿素` 别名 `BUN` | BUN 是尿素氮，不等同尿素 | 删除 `BUN` 同义名；`BUN` 另立 local 项 |

## 6. 实施顺序

1. **标准层数据**：在现有 `*_dict.yaml` 内新增字段（不新增目录）；先落 batch 1 的 95 项；再落 batch 2。
2. **识别层拆分**：`aliases` → `synonyms` + `ocr_aliases`；`units` → `accepted_units` + `ocr_unit_forms`；`unit_families.yaml` 拆成 `unit_conversions.yaml` + `ocr_forms.yaml`。
3. **后处理**：`_unit_match` / `_normalize_unit` 改为逐项 `accepted_units` 校验；`_unit_or_none` 不再接受任意 ASCII；未验证输出 `unit=null`。
4. **换算**：新增 `convert(value, ref_range, from_unit, to_unit)`；只对 `conversion != none` 的项生效；同时换算 `value` 与 `ref_range`。
5. **契约/Go/App**：`ReportItem` 新增 `code` / `loinc` / `unit_display` / `unit_status` / `raw_*` / `ref_kind`；趋势按 `code` + canonical unit 连接。
6. **golden / reparse**：重新生成全部 golden；逐项说明变更；`hdd reparse` 全量重跑。

## 7. CI 门禁

新增测试（建议 `recognizer/tests/test_dict_standard.py`）：

1. 每个量化项必须有 `name` + (`code` 或 `local_code`) + `canonical_unit` + `source_status` + `review_status`。
2. `accepted_units` / `ocr_unit_forms` 不得含 OCR 碎片；同一字符串不得同时出现在两处。
3. `ocr_aliases` 必须有样本证据（沿用现有 corpus gate）；`synonyms` 必须有 `source`。
4. `conversion: linear` 必须有 `factor` + `source`；`equation` 必须有 `id` + `formula` + `source`；`none` 不得参与换算。
5. `unit_status=verified` 的项必须 `review_status=approved`；`pending` 不得进入 golden 的 verified 集合。
6. 每条 golden 的 `unit` 必须是 UCUM 码或 `null`；不得是 OCR 碎片。
7. `result_kind != quantitative` 的项不得有 `canonical_unit`（`ratio`/`arbitrary` 除外，按 §3.1）。
8. 每个 `result_kind=quantitative` 且 `review_status=approved` 的项必须有 `plausible_range`（canonical unit）；换算结果超出范围时不得标 `verified`。

## 8. 验收

- 内环：`pytest` 全绿；新增标准门禁全绿；`test_dict_coverage` 仍绿。
- 数据：181/181 有 `source_status`；batch 1 的 95 项全部 `review_status=approved` 或有明确 `pending` 理由。
- golden：`unit` 中 OCR 碎片为 0；`unit=null` + `unverified` 的项逐项列在验收报告。
- 趋势：同一 `code` + canonical unit 的历史点可连接；未验证点不连接。
- 回归：CBC/生化/甲功/凝血四组的 matched/value 不下降；unit 变更全部有说明。

### 8.1 初步 golden 影响（batch 1，实施前基线）

对现有 golden 报告按 `mapping.csv` 做一次静态分类（不含 OCR 形修复）：

- **canonical 不变**：267 项。
- **需换算**：29 项。主要是 HCT `%→L/L`（9 份 CBC）、HbA1c `%→mmol/mol`（2 份）、25(OH)D/叶酸/维生素B12/铁蛋白（vitanemia 各 2 份）、尿酸 `mmol/L→μmol/L`（lft_02）、D-二聚体 `ng/mL→μg/L`（coag_01）、孕酮/hCG/C肽/胰岛素（各 1 份）、T3 `ng/mL→nmol/L`（tft_03）、PT/APTT/TT 的 `秒→s`（3 份）。
- **需 OCR 修复或置空**：约 40 项。包括 `IU/L` vs `U/L`（酶类）、`*10~12/L`/`109/L` 等 OCR 形、RDW-SD 的 `%`、PDW `fL`、eGFR `ml/min`、TSH `.9`、TPOAb `IU/al` 等。OCR 形将在 `ocr_forms.yaml` 修复；无法修复的置 `null` + `unverified`。
- **batch 2**：88 项 unit 暂按 `unverified` 计，待第二批映射后重算。

逐项 golden 变更清单在实施票生成，不在本 spec 复制。

## 9. 开放项（需人确认）

1. **复核人**：具名临床检验人员；未定前 `review_status=pending`。
2. **标准全文**：WS/T 886—2026、WS/T 404 系列正式 PDF；拿不到全文的项 `source_status=secondary`。
3. **LOINC 候选**：用 NLM Clinical Tables API 生成候选，逐项复核；不在本表伪造。
4. **PDW 定义**：拆 SD/CV 还是只保留一种。
5. **D-二聚体基准**：FEU/DDU 的默认与缺失处理。
6. **eGFR UCUM**：`mL/min/1.73m²` 的 UCUM 表达待核。
7. **U/L vs IU/L**：是否接受等值（需检验人员确认）。
8. **`plausible_range`**：每个量化项的宽泛量级上下限，需检验复核人确认；未确认前不得 `approved`。
