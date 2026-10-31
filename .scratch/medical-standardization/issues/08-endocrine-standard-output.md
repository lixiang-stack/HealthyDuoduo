# 08: 激素标准输出端到端

**What to build:** 甲状腺与孕酮/hCG 端到端标准输出：TSH、Tg、T3、T4、FT3、FT4、TPOAb、TgAb、TRAb、孕酮、hCG。重点验证 T3/T4 `nmol/L`、FT3/FT4 `pmol/L`、T3 `mmol/L` 的量级门拒绝，以及 IU 与 U 不互换。

**Blocked by:** 03 (凝血标准输出端到端)

**Status:** ready-for-agent

- [ ] 激素项都有标准名/代码或 local_code、canonical unit、result_kind、source/review 状态。
- [ ] T3/T4 canonical `nmol/L`；FT3/FT4 `pmol/L`；TSH `mIU/L`；抗体按 IU/mL 或 IU/L。
- [ ] T3 `2.04 mmol/L` 换算后 2040 nmol/L 超出量级门，标 unverified，不输出 canonical。
- [ ] TPOAb/TgAb 的 `U/mL` 不默认等价 `IU/mL`；孕酮/hCG 的替代单位按复核系数换算。
- [ ] golden/App/趋势验收；甲状腺报告不因单位误读产生 1000× 值。
