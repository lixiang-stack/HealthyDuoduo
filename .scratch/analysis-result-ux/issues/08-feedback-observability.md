# 08: 反馈与观测升级（P2）

**What to build:** 把反馈从“结果级一段自由文本”升级为可归因的信号，并补上解释覆盖率、部分拒绝和 trust gate 的可观测性。

**Blocked by:** 03、05（来源/反馈入口与状态语义）

**Status:** needs-triage

- [ ] 结果级“这不对”保留；解释卡支持按 `entry_id` 反馈
- [ ] 反馈原因改为固定分类：数值不对 / 解释不对 / 病历冲突 / 看不懂 / 其他
- [ ] 增加正向反馈“有帮助 / 看懂了”
- [ ] 增加解释覆盖率 `analyses_with_explanation / analyses`，按 panel 分组
- [ ] 增加部分拒绝指标：`rejected_sections` 数量 + `entry_id`
- [ ] 补 `trust_gate_failed` 的生产者；确认触发条件与 spec §4.13 一致
- [ ] 客户端事件（不含健康内容）：来源打开、报告打开、分享、重跑、范围切换
- [ ] 周检看四个数：反馈率、降级率、解释覆盖率、条目级负反馈 Top N
- [ ] `/analyses/metrics` 或等价查询能区分无条目 / LLM 失败 / 部分拒绝

**依据：** `.scratch/analysis-result-ux/spec.md` §2.6；`research.md` B6、B3。
