# 07: 生化-血脂/血糖/维生素标准输出端到端

**What to build:** 血脂、血糖与维生素/贫血端到端标准输出：TC/TG/HDL/LDL、空腹与餐后葡萄糖、HbA1c、C肽、胰岛素、25(OH)D、铁蛋白、叶酸、维生素B12。重点验证 HbA1c `%↔mmol/mol` 方程换算和血糖/血脂的 mg/dL 换算。

**Blocked by:** 03 (凝血标准输出端到端)

**Status:** ready-for-agent

- [ ] 血脂/血糖/维生素项都有标准名/代码或 local_code、canonical unit、result_kind、source/review 状态。
- [ ] HbA1c canonical `mmol/mol`，`%` 用 IFCC-NGSP 方程换算；保留 raw 值/单位。
- [ ] 血糖/血脂 `mg/dL` 换算系数按复核结果生效；未复核系数不得标 verified。
- [ ] C肽/胰岛素、25(OH)D/铁蛋白/叶酸/B12 的替代单位与换算有来源；不能换算的标 unverified。
- [ ] golden/App/趋势验收；餐后时间点不混项。
