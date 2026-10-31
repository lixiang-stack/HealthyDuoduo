# 11: 专项/血型标准输出端到端

**What to build:** 专项与血型 8 项端到端标准输出：产前筛查风险项、母龄风险、hCGβ亚单位、ABO/Rh/抗体筛查。重点验证风险比值/滴度与定性结果的 `result_kind`，以及无标准码项目的 local_code。

**Blocked by:** 03 (凝血标准输出端到端)

**Status:** resolved

- [x] 专项/血型项都有标准名或 local_code、result_kind、source/review 状态。
- [x] 风险比值/滴度按 ratio/titer 建模，不塞进普通 unit；定性血型无 unit。
- [x] `1:19879` 等结果保留原文，不误当数值浓度；hCGβ 的 ng/mL 与 U/mL 按复核口径处理。
- [x] 已复核项 verified；未复核项 raw+unverified。
- [x] golden/App 验收；专项报告不被误判为生化/免疫。

## Comments

**实现（2026-10-31）**

- 产前筛查四项 `result_kind=ratio`、canonical `1`；`1:1913`/`1:19879`/`<1:1000` 等原文保留在 `value`/`raw_value`，不按数值浓度解析。
- hCGβ 定量 canonical `ug/L`/display `μg/L`，`ng/mL` 等值、`U/mL` 登记 `conversion:none`；`LOCAL-SPECIAL-*` local_code。
- 血型/抗体筛查 `result_kind=qualitative`、unit null；ABO/RH 继续走 informational flag 策略。
- 专项 narrative 分类测试锁定 report_type=其他专项；bloodtype golden 字段/flag 不变。
