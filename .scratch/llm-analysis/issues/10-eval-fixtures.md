# 10: 评估集冻结

**What to build:** 事实层与框架层的期望输出被冻结为 fixture 并进 CI；代码改动若改变输出会导致测试失败。

**Blocked by:** 04 分析事实层与结果页, 05 框架层：肝功, 06 框架层：血常规与 BMI

**Status:** resolved

- [x] 事实层约 100 个 case：复用现有 golden 报告，补成员年龄性别，生成期望输出
- [x] 框架层约 20 个边界用例：R 值 1.9 / 2.0 / 4.9 / 5.0，BMI 百分位在界值上下，切点跨年龄段
- [x] 期望输出由代码生成一次、人工确认后冻结为 fixture
- [x] CI 运行这些 fixture；输出变化时失败
- [x] 解释层评估不在本单范围（季度人工盲评）

## Comments

**实现（2026-11-04，分支 `llm-analysis-framework-cloud`）**

- 冻结文件：`internal/analysis/testdata/eval/fixtures.json`；CI 由 `TestEvalFixtures` 读取，逐 case 对 `BuildFacts` / `BuildFramework` 输出做 JSON 全等比较。测试从不重新生成 fixture。
- 事实层 110 case：现有 55 个 golden 报告 × 项目组组合，每个组合生成 Panel 入口与单指标入口两个 case；每个 case 同时纳入目标报告前 24 个月内同项目组的点，冻结窗口 / 稀疏历史。成员年龄按报告日期倒推 10 年、性别固定男，分析时刻锚定在报告日期次日。缺失日期 / 未核实单位 / 低置信等真实形态原样保留并冻结。
- 框架层 29 case：R 值 1.9 / 2.0 / 4.9 / 5.0；转氨酶分级 1.9 / 2.0 / 4.9 / 5.0 / 15.0 / 15.1；WS/T BMI 超重 / 肥胖界值 19.1 / 19.2 / 21.9；WHO z 界值上下（16.6 / 16.8 / 18.3 / 18.4 kg/m²）；BMI 17y11m / 18y0m / 19y2m 年龄段切换；WHO Hb 切点 6 / 23 / 24 月、11y11m / 12y0m、14y11m / 15y0m、65y11m / 66y0m。
- 生成器：`go run ./internal/analysis/gen/eval`（仓库根目录）。仅在人工作出输出变更决定后运行，并审查 diff；`fixtures.json` 的 `rule_version` 与当前 `RuleVersion` 不一致时测试会要求重新确认。
- 说明见 `internal/analysis/testdata/eval/README.md`。解释层评估保持季度人工盲评，不在本测试内。
