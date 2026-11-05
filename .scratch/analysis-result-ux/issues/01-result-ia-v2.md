# 01: 结果页 IA v2（P0）

**What to build:** 分析结果页从“证据优先”改为“结论优先”：解读出现在页头提示之后，范围卡默认折叠并后移，删除重复标题，事实层按用户六层重新分组。

**Blocked by:** 无（本单先做布局，不改数据契约）

**Status:** ready-for-human

- [x] 删除正文中与 App bar 重复的“肝功分析”标题
- [x] 解读卡 / 无解读状态块移到页头提示之后、“本次使用的数据”之前
- [x] “本次范围”改为“本次使用的数据”，默认折叠成一行摘要
- [x] 判读标签移到偏离参考范围之前
- [x] 页面按六层组织：结论 → 判读 → 异常与变化 → 医生记录 → 范围与局限 → 操作
- [x] 保持事实层内部相对顺序，不改变后端筛选
- [x] RNTL 增加页序断言：`analysis-explanation` 在 `analysis-deviations`、`analysis-history`、`analysis-scope`、`analysis-excluded` 之前
- [x] 旧分析缺少解释层时，状态块出现在首屏位置

**依据：** `.scratch/analysis-result-ux/spec.md` §2.1–2.2；`.scratch/analysis-result-ux/research.md` 发现 1、A1、B1。

## Comments

**2026-11-04 — implemented**

- 结果页改为「结论 → 判读 → 异常与变化 → 医生记录 → 范围与局限 → 操作」；正文重复标题已删除，页头压成成员 · Panel · 窗口一行。
- 解读卡/无解读状态块位于页头提示之后；范围卡默认折叠并后移。
- `apps/mobile/src/test/screens/analysis.test.tsx` 增加区块顺序断言。
- 真机 smoke 已完成；方案 A 后历史趋势、范围分组、未参与清单移出结果页，页序以 09 为准。

**2026-11-05 方案 A 复核**

2026-11-05 方案 A 复核：结果页不再包含历史趋势、报告范围分组、未参与清单；改为“数据说明 + 项目页链接”。页序与验收以 09 为准。
