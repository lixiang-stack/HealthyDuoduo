# 01: 结果页 IA v2（P0）

**What to build:** 分析结果页从“证据优先”改为“结论优先”：解读出现在页头提示之后，范围卡默认折叠并后移，删除重复标题，事实层按用户六层重新分组。

**Blocked by:** 无（本单先做布局，不改数据契约）

**Status:** needs-triage

- [ ] 删除正文中与 App bar 重复的“肝功分析”标题
- [ ] 解读卡 / 无解读状态块移到页头提示之后、“本次使用的数据”之前
- [ ] “本次范围”改为“本次使用的数据”，默认折叠成一行摘要
- [ ] 判读标签移到偏离参考范围之前
- [ ] 页面按六层组织：结论 → 判读 → 异常与变化 → 医生记录 → 范围与局限 → 操作
- [ ] 保持事实层内部相对顺序，不改变后端筛选
- [ ] RNTL 增加页序断言：`analysis-explanation` 在 `analysis-deviations`、`analysis-history`、`analysis-scope`、`analysis-excluded` 之前
- [ ] 旧分析缺少解释层时，状态块出现在首屏位置

**依据：** `.scratch/analysis-result-ux/spec.md` §2.1–2.2；`.scratch/analysis-result-ux/research.md` 发现 1、A1、B1。
