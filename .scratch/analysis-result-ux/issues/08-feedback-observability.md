# 08: 反馈与观测升级（P2）

**What to build:** 把反馈从“结果级一段自由文本”升级为可归因的信号，并补上解释覆盖率、部分拒绝和 trust gate 的可观测性。

**Blocked by:** 03、05（来源/反馈入口与状态语义）

**Status:** ready-for-human

- [ ] 结果级 / 合并卡反馈保留；合并解读卡后条目级 `entry_id` 反馈入口暂未保留，待补
- [x] 反馈原因改为固定分类：数值不对 / 解释不对 / 病历冲突 / 看不懂 / 其他
- [ ] 增加正向反馈“有帮助 / 看懂了”
- [ ] 增加解释覆盖率 `analyses_with_explanation / analyses`，按 panel 分组
- [ ] 增加部分拒绝指标：`rejected_sections` 数量 + `entry_id`
- [x] 补 `trust_gate_failed` 的生产者；确认触发条件与 spec §4.13 一致
- [ ] 客户端事件（不含健康内容）：来源打开、报告打开、分享、重跑、范围切换
- [ ] 周检看四个数：反馈率、降级率、解释覆盖率、条目级负反馈 Top N
- [ ] `/analyses/metrics` 或等价查询能区分无条目 / LLM 失败 / 部分拒绝

**依据：** `.scratch/analysis-result-ux/spec.md` §2.6；`research.md` B6、B3。

## Comments

**2026-11-04 — implemented frontend-only, backend gaps recorded**

- 已实现：结果级反馈 + 合并卡反馈；原因改为固定五分类，不再发送自由文本。wire 值使用稳定 key（`value_wrong` / `explanation_wrong` / `note_conflict` / `unclear` / `other`），中文只作展示文案，避免改文案导致聚合断裂。
- `trust_gate_failed` 生产者已在 `cmd/hdd/serve_analysis.go` 存在，并由 `TestServeAnalysisTrustGateFailure` 覆盖；本轮确认无需再补。
- 2026-11-05 合并解读卡后，条目级 `entry_id` 反馈入口被移除；当前卡片反馈与结果级反馈都不带 `entry_id`。若保留按条目归因，需要补回入口。
- 未做：
  - 正向反馈需要新契约区分正/负向，避免污染反馈率；当前 `AnalysisFeedbackRequest` 只有 `reason`。
  - 解释覆盖率、部分拒绝指标、按 panel 分组、条目级负反馈 Top N 需要新的 metrics 查询/字段。
  - 客户端事件需要新的上报端点或本地分析管道，当前没有可复用的匿名事件通道。
  - 现有 `/analyses/metrics.degradation_kinds` 已能区分 `entry_miss` / LLM 失败 / `bridge_text_rejected`，但没有覆盖率与部分拒绝段数。

**2026-11-05 方案 A 复核**

2026-11-05 方案 A 复核：反馈文案改为「反馈这段内容」，结果级为「反馈问题」；固定分类保留；合并卡后 `entry_id` 归因入口暂未保留。
