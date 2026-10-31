# 10: 免疫标准输出端到端

**What to build:** 免疫 38 项端到端标准输出：自身抗体、乙肝/丙肝/HIV/梅毒、肝纤维化四项。重点验证 `AU` 作为 arbitrary unit、`S/CO` 作为 ratio、定性结果无 unit，以及 dsDNA IU/mL 与 C1q U/mL 不互换。

**Blocked by:** 03 (凝血标准输出端到端)

**Status:** resolved

- [x] 免疫项都有标准名/代码或 local_code、result_kind、source/review 状态。
- [x] `AU` 用 `[arb'U]` 或等价 arbitrary 表达；`S/CO` 用 ratio/`1`，不再作为 unit 字符串。
- [x] 定性项无 unit；`ANA(IIF)`、核型、抗线粒体/平滑肌等按 qualitative/informational 处理。
- [x] dsDNA `IU/mL`、C1q `U/mL`、TPOAb/TgAb `IU/mL` 不互相换算；无权威换算标 unverified。
- [x] golden/App/趋势验收；感染性指标不产生数值趋势。

## Comments

**实现（2026-10-31）**

- 38 项迁移；自身抗体 `AU` → `[arb'U]`/display `AU`；`S/CO` → ratio `1`；定性项 unit null、`result_kind=qualitative`。
- dsDNA `[IU]/mL`、C1q `U/mL`、HBsAg `[IU]/mL`、HBsAb `m[IU]/mL` 各自独立；`U/mL` 与 `IU/mL` 不互换。
- 肝纤维化四项 `ug/L`/display `μg/L`；现有 fibrosis golden 值/flag 不变。
- `flag_policy.yaml` 增加标准名键（HBsAg/HBeAg/HBsAb/HBeAb/HBcAb/ANA），infectious_02 定性 flag 保持原值。
- ANA 核型为 informational；感染性指标仍不按数值趋势消费（趋势过滤在识别/查询层）。
