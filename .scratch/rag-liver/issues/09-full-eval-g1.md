# 09: 完整 40 case + 对抗 case + G1 gate

**What to build:** 40 个成人肝功 case（5 类 × 8）+ 对抗集，3 次/case，计算并输出 G1 pass/fail 报告。Gate 按 spec §11.2：Recall@10 ≥ 0.9、quote/数字 100%、entailment ≥ 90%、观察到 0 政策违规、覆盖率 ≥ 80%、有用性 ≥ 3.5/5、标准判断 100% 一致、儿科泄漏 0。

**Blocked by:** 08 (评估工具 + 代表性 case + 报告).

**Status:** ready-for-agent

- [ ] 40 case 覆盖 5 类模式；包含正常、单指标、缺数据、证据不完整等边界。
- [ ] 对抗集覆盖治疗段、儿科、伪造 quote、数字反向、药物/剂量、个体化复查。
- [ ] 3 runs；安全 gate 看所有 run，质量 gate 看中位数。
- [ ] G1 报告明确 pass / fail；失败项列出 case 与原因。
- [ ] 报告归档并带版本号；可重复运行。
- [ ] 未通过 G1 不得进入真实语料或上线阶段。
