# 07: 生化-血脂/血糖/维生素标准输出端到端

**What to build:** 血脂、血糖与维生素/贫血端到端标准输出：TC/TG/HDL/LDL、空腹与餐后葡萄糖、HbA1c、C肽、胰岛素、25(OH)D、铁蛋白、叶酸、维生素B12。重点验证 HbA1c `%↔mmol/mol` 方程换算和血糖/血脂的 mg/dL 换算。

**Blocked by:** 03 (凝血标准输出端到端)

**Status:** resolved

- [x] 血脂/血糖/维生素项都有标准名/代码或 local_code、canonical unit、result_kind、source/review 状态。
- [x] HbA1c canonical `mmol/mol`，`%` 用 IFCC-NGSP 方程换算；保留 raw 值/单位。
- [x] 血糖/血脂 `mg/dL` 换算系数按复核结果生效；未复核系数不得标 verified。
- [x] C肽/胰岛素、25(OH)D/铁蛋白/叶酸/B12 的替代单位与换算有来源；不能换算的标 unverified。
- [x] golden/App/趋势验收；餐后时间点不混项。

## Comments

**实现（2026-10-31）**

- HbA1c：`%` → `mmol/mol` 用 `IFCC-NGSP` 方程（id/formula 存词典，代码按 id 实现）；glu_01 `6.50%` → `47.54`，glu_03 `11.0%` → `96.72`。
- 血糖/血脂 `mg/dL` 线性系数（pending 复核）；`mol/L` 等 1000× 错误被量级门拒绝（glu_02 餐后三小时输出 null + unverified）。
- C肽 `ng/mL×331`、胰岛素 `mIU/L/uIU/mL×6.945`、25(OH)D `ng/mL×2.5`、叶酸 `ng/mL×2.266`、B12 `pg/mL×0.7378`、铁蛋白 `ng/mL` 等值；value/ref 同步换算。
- 餐后半小时/1h/2h/3h 保持独立词条；标准名迁移后旧名在 synonyms，匹配不混项。
