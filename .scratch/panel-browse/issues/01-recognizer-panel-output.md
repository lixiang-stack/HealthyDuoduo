# 01: 识别端输出项目组与医学依据

**Spec:** ../spec.md（§5、§6、§10.1）

**What to build:** 识别服务在产出报告类别的同时，为每个指标项产出项目组（`panel`），并把报告级 `panels` 作为指标项项目组的排序聚合。家长侧还不展示；这一票让后续数据、API、App 有可信的项目组来源。词典分组同时带上可追溯的医学依据注释，供临床复核。

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] 每条 golden 报告都有 `panels`；每个指标项都有 `panel`；除 `other` 外不跨报告类别。
- [ ] `panels` 按命中项数降序、同数按固定优先级排序；`panels[0]` 是命中项数最多的项目组。
- [ ] 样本回归符合 spec §6.5：lft_02 为肝功 / 肾功 / 尿酸；lft_06 为肝功 / 血脂；tft_01 为甲状腺；ua_05 为尿常规；us_01 为影像；coag_01 为凝血；fibrosis_01 / bloodtype_01 / unknown_01 为其他。
- [ ] 尿常规的葡萄糖 / 胆红素归尿常规；生化葡萄糖归血糖；影像不进其他；失败报告不进项目列表。
- [ ] `/classify` 对混合报告、歧义项、空 items、未知 report_type、父子越界返回正确答案 / 明确拒绝。
- [ ] 9 个词典文件都有 `# panel 依据` 块或 `# 依据:` 注释；混合词典 block 覆盖其所有 `panel:` 值；注释不参与解析。
- [ ] `cd recognizer && pytest` 全绿；golden diff 人工核对；契约测试锁定枚举与父子约束。
