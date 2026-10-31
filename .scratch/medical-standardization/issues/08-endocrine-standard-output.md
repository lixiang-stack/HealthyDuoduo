# 08: 激素标准输出端到端

**What to build:** 甲状腺与孕酮/hCG 端到端标准输出：TSH、Tg、T3、T4、FT3、FT4、TPOAb、TgAb、TRAb、孕酮、hCG。重点验证 T3/T4 `nmol/L`、FT3/FT4 `pmol/L`、T3 `mmol/L` 的量级门拒绝，以及 IU 与 U 不互换。

**Blocked by:** 03 (凝血标准输出端到端)

**Status:** resolved

- [x] 激素项都有标准名/代码或 local_code、canonical unit、result_kind、source/review 状态。
- [x] T3/T4 canonical `nmol/L`；FT3/FT4 `pmol/L`；TSH `mIU/L`；抗体按 IU/mL 或 IU/L。
- [x] T3 `2.04 mmol/L` 换算后 2040 nmol/L 超出量级门，标 unverified，不输出 canonical。
- [x] TPOAb/TgAb 的 `U/mL` 不默认等价 `IU/mL`；孕酮/hCG 的替代单位按复核系数换算。
- [x] golden/App/趋势验收；甲状腺报告不因单位误读产生 1000× 值。

## Comments

**实现（2026-10-31）**

- T3/T4 `nmol/L`、FT3/FT4 `pmol/L`、TSH `m[IU]/L`/display `mIU/L`、TPOAb/TgAb `[IU]/mL`、TRAb/hCG `[IU]/L`；旧名全部保留 synonym。
- T3 保留 `mmol/L` accepted（×1000）并设 `plausible_range 0.1–50 nmol/L`；tft_01 `2.04 mmol/L` 换算 2040 超界 → `unit=null`、raw 保留、low_confidence=true。
- T4 `rmol/L`、FT4 `pmo1/L`/`ng/d1`、FT3 `pg/al`、TPOAb `IU/al`、TgAb `10/a1` 等 OCR 形由 `ocr_forms.yaml` 修复；tft_03 四值完成换算。
- 孕酮 `ng/mL×3.18`、hCG `mIU/mL` 等值；TPOAb/TgAb `U/mL` 登记 `conversion:none`，不冒充 IU/mL。
