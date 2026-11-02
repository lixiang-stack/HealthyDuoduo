# 08: 解释层与首个条目

**What to build:** 肝功某个组合命中知识条目时，结果页出现四段式解释；医学原文逐字来自条目，模型只写连接语；模型改写原文、引用不存在的数字或使用禁用推断词时，该段被拒绝。

**Blocked by:** 05 框架层：肝功, 07 云端脱敏、LLM 客户端与配额

**Status:** ready-for-agent

- [ ] 条目 YAML 从 `internal/analysis/knowledge/` 加载，只有 `review.decision = accepted` 可引用
- [ ] 精确匹配优先；多命中按 `priority` 排序取前 2；部分匹配不输出
- [ ] 模型输出 `sections`，每段带 `entry_id`、`evidence_ids`、`bridge_text`
- [ ] `bridge_text` 中出现的数字必须能在 `evidence_ids` 找到
- [ ] `bridge_text` 不含禁用推断词：可能 / 提示 / 考虑 / 建议 / 应该
- [ ] `bridge_text` 长度 ≤ 200 字
- [ ] `entry_id` 不存在或未 accepted 时拒绝该段
- [ ] 医学原文由服务端从条目直接拼接，模型无法修改
- [ ] 无命中时不显示空状态；必要时附「本次没有可引用的解读条目，可以带着上面的数值问医生」
- [ ] 首个肝功条目入库并能命中
- [ ] App 渲染四段式与「查看依据」
- [ ] 单测覆盖校验失败的各条路径：数字不在证据、禁用词、超长、条目未审核
