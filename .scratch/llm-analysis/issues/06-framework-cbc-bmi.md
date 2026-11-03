# 06: 框架层：血常规与 BMI

**What to build:** 血常规分析出现贫血形态学分类和年龄段切点偏离；有生长记录时出现 BMI 与年龄性别百分位；手输身高体重只显示 BMI 数值不判超重。

**Blocked by:** 02 生长记录与 BMI, 04 分析事实层与结果页

**Status:** resolved

- [x] MCV / MCH / MCHC + RDW 组合输出贫血形态学分类
- [x] WHO 2024 Hb 切点按年龄段分档
- [x] 切点标签只在数值越过时输出，并同时注明单据参考范围
- [x] BMI 用 WS/T 586—2018 性别年龄别界值判定
- [x] 5–19 岁可用 WHO 2007 z-score
- [x] `source = manual` 不输出超重 / 肥胖标签
- [x] 缺 `birth_date` 或 `sex` 时不输出百分位
- [x] App 渲染形态学分类、切点标签与 BMI 百分位
- [x] 单测覆盖界值上下、缺数据、手输来源

## Comments

**实现（2026-11-03，分支 `llm-analysis-growth-facts`）**

- 形态学分类在 `internal/analysis/framework.go`：Hb 低（单据 flag 或 WHO 2024 切点）且 MCV 有方向时，组合 MCV / MCH / MCHC / RDW-CV 的单据 flag 输出小细胞 / 正细胞 / 大细胞、低色素与 RDW 升高；不另写固定 MCV 切点，避免覆盖实验室年龄别参考范围。
- WHO 2024 Hb 切点：6–23 月 105、24–59 月 110、5–11 岁 115、12–14 岁 120、15–65 岁女性 120 / 男性 130 g/L（65 岁按实足年龄到 66 岁生日前）；年龄按报告日期计算（不是分析日），只在低于切点时输出，并注明单据参考范围。
- BMI：`pipeline.BMI` 用测量时点值；6–18 岁按 WS/T 586—2018 半岁年龄别界值；5–19 岁输出 WHO 2007 z 值与百分位（LMS 表由 WHO 2007 扩展表生成）。缺 `birth_date` / `sex` 时不输出百分位；`source = manual` 只输出 BMI 数值（该来源已在票 02 体验修订中移除，领域层保留守卫）。
- App `FrameworkCard` 通用渲染框架层标签；RNTL 覆盖形态学、Hb 切点与 BMI 百分位文案。
- 单测覆盖 WS/T 界值上下（19.1 / 19.2 / 21.9）、缺生日/性别、manual 来源、WHO 5.5 岁 z 值、形态学缺 Hb/MCV。

**复核修订（2026-11-03）**

- 血常规与 BMI 规则拆到 `internal/analysis/framework_cbc.go` / `framework_bmi.go`。
- BMI 在 18 岁生日前用 WS/T 586，18–19 岁用 WHO 2007 z；WS/T 的 18.0 行不再用于 18 岁生日后的判定。
- 血常规与 BMI 也按识别端 `analyte` key 匹配；参考表 CSV 为唯一源，`go generate ./internal/analysis` 生成 Go 数据。
