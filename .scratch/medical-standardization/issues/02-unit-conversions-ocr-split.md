# 02: 单位换算层与 OCR 形分离

**What to build:** 把现有 `unit_families.yaml` 拆成两张职责单一的表：真实单位换算表（精确等价、线性系数、方程）与 OCR 字形修复表（只做印形修复）。loader 同时加载两张表；现有报告的单位输出保持不变，为后续按项换算和未验证输出做准备。

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] 换算表只含真实单位/等价关系/系数/方程；OCR 形只能进修复表。
- [ ] OCR 形与真实单位不得混放；同一字符串不得同时出现在两张表。
- [ ] 现有单位归一测试与 golden 的 unit 输出不变。
- [ ] 新增非法条目测试：OCR 形进换算表、真实单位进修复表都要失败。
