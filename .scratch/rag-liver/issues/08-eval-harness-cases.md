# 08: 评估工具 + 代表性 case + 报告

**What to build:** 一个可重复的 eval 命令，对 5 类模式各 1 个 + 3 个对抗 case（治疗段同 chunk、儿科 chunk、证据不完整）跑 3 次，调用 pro judge，输出 Markdown / JSON 报告和版本信息（corpus / index / prompt / model / commit）。验证命令能跑、报告有指标和失败 case。

**Blocked by:** 04 (RAG 生成 + claim 校验 CLI).

**Status:** ready-for-agent

- [ ] eval 命令复用生产检索/生成/校验路径，不另写一套。
- [ ] 8 个 case：5 类模式 + 3 对抗；每个 case 有期望 chunk、关键点、禁止结论。
- [ ] 3 runs；计算 Recall@10、quote、数字/方向、entailment、政策违规、覆盖率、有用性、标准一致性、儿科泄漏。
- [ ] judge 用 pro 模型；报告记录同供应商偏差；提供 20% 人工抽检入口。
- [ ] 报告带版本号与失败 case 明细；可重复运行。
- [ ] 不改变生产 API / App。
