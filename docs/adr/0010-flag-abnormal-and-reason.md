# 0010 · unknown 可解释化：abnormal flag + flag_reason + 共享比较策略

**状态：** 采纳（2026-10-22，review-clarity-v2 后续）。

## 背景

`flag=unknown` 同时承载抽取失败、语义不可比（定性/单侧/区间）、纸面无参考范围、未核实四类事实。App 只显示“未知”，家长既不知道原因，也不知道下一步。审计（`.scratch/review-clarity-v2/audit.md`）显示非 OCR 残量可以分层修复，但需要先决定 flag 枚举与比较策略的归属。

## 决策

1. **flag 增加 `abnormal`**：`normal/high/low/abnormal/unknown`。
   - `high/low` 只用于有方向的数值异常；
   - `abnormal` 只用于无方向的定性异常；
   - `normal` 用于数值正常或定性正常；
   - `unknown` 只表示本应能比较但失败/暂不支持。
2. **item 增加 `flag_reason`**（仅 unknown 输出）：
   `ocr_miss / ocr_merge / extractor_pairing / no_reference / qualitative_unsupported / unverified / narrative / informational / indeterminate / scalar_ref / other`。
   - 时间线 `flag_counts` 仍按 outcome 统计，不按 reason 展示多类徽标；`unknown` 只统计可操作/抽取失败原因，`no_reference`/信息项/叙述体不计入「未知」徽标。
   - 纸面确实无参考范围时，家长可发 `action=no_reference` 确认；`flag` 仍 unknown，`flag_reason=no_reference`，详情显示「无参考范围」而非「未知」。
3. **共享 comparison policy**：`recognizer/recognizer/flag_policy.yaml` 由 Python 识别端与 Go 数据服务共读。抽取词典（`*_dict.yaml`）仍只在识别端；Go 只感知比较策略，用于 ADR-0005 的修正后 flag 重算。
4. **比较语义**：
   - 标量参考 `0`/`0.0` → 精确零点；其他标量 → `unknown/scalar_ref`。
   - 单侧值 `<x`/`>x` 视为可能区间，与参考范围做区间包含；跨边界 → `unknown/indeterminate`。
   - 区间值 `a-b` 同理。
   - 定性值按项目 profile：negative/weak_positive/positive → normal/abnormal/informational；未覆盖 → `unknown/qualitative_unsupported`。
   - `value` 与 `ref` 都是阴性（含 `-`/`-`、`阴性`/`-`）→ `normal`。
5. **不做**：参考范围知识库（F）、OCR 升级（G）、`indeterminate`/`not_applicable` 一等 flag、通用表格解析器重写。提取器只做审计样例的低复杂度修复。

## 备选与否决

- **另加 `qualitative_result` 字段**：类型更干净，但卡片/计数/趋势/筛选要同时看两个字段；当前规模不值得。
- **定性阳性映射成 high**：医学语义错误（尿潜血 3+ 不是“偏高”）。
- **保持 4 值 + 只加 reason**：无法表达定性异常，方案 D 需要 `abnormal`。
- **Go 完全不感知比较策略**：与 ADR-0005 冲突；定性项修正后无法重算，只能保留旧 flag 或要求重新上传。

## 后果

- schema/OpenAPI/DB CHECK/`FlagCounts`/移动端文案与徽标均增加 abnormal；
- `report_items` 增加 `flag_reason`（0009 迁移）；
- 识别端 golden 重新生成，unknown 数量按审计预期下降；
- 定性词典是项目级策略数据，新增项目未覆盖时安全落 `unknown/qualitative_unsupported`，不猜。
