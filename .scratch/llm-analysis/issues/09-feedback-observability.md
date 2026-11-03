# 09: 反馈与观测

**What to build:** 每个分析结果提供「这不对」按钮，记录 `analysis_id`、条目、理由和时间；服务端记录所有降级事件，供每周查看。

**Blocked by:** 04 分析事实层与结果页

**Status:** resolved

- [x] 结果页提供「这不对」按钮
- [x] 提交记录 `analysis_id`、`entry_id`（可选）、理由、时间
- [x] 只记录，不自动降级条目
- [x] 服务端日志记录：LLM 超时、JSON 不合法、`bridge_text` 校验失败、配额用尽、信任门控全未通过
- [ ] 条目未命中：08 接真实 matcher 后启用（`Explainer.RecordEntryMiss`）
- [x] 记录可查询，用于每周计算反馈率与降级率
- [x] HTTP 测试覆盖记录写入；RNTL 覆盖按钮与提交

## Comments

**实现（2026-11-03，分支 `llm-analysis-growth-facts`）**

- 结果页「这不对」按钮 + 理由弹层（`apps/mobile/src/analysis/FeedbackModal.tsx`）；`POST /analyses/{id}/feedback` 记录 `analysis_id`、可选 `entry_id`、理由与时间，只写入，不自动降级条目。
- `analysis_events` 记录 `llm_timeout` / `llm_invalid_json` / `bridge_text_rejected` / `quota_exhausted` / `trust_gate_failed` / `entry_miss` / `llm_error` / `input_too_large`；迁移 `0016_analysis_observability.sql` 的 CHECK 与 Go 常量同步。
- `GET /analyses/metrics?from&to` 返回分析数、反馈数、反馈率、降级事件数、降级率与按 kind 分布，供每周查看。
- HTTP 测试覆盖反馈写入 / 校验 / 404 与指标区间；RNTL 覆盖按钮、理由输入与提交。
- `entry_miss` 事件的生产侧发射由票 08 打开（`Explainer.RecordEntryMiss`）；07 的空匹配器不把每次分析都记成条目未命中。

**真机验收（2026-11-03）**

- 设备：Android 16 真机（USB + Expo Go SDK 58）。
- 隔离栈：`bash scripts/android-acceptance.sh --seed`；另上传 `samples/cbc_04.png` 并修正 MCV 参考范围 / 单位、血红蛋白值，构造 Hb 108 g/L + MCV 79.7 fL 的形态学用例。
- 已验证：肝功 R 值 / 分级 / AST/ALT；CBC 形态学与 WHO 2024 Hb 切点；BMI 与 WHO 2007 z / 百分位；今日剩余解读次数；配额用尽提示且反馈按钮仍可点；「这不对」提交后 `analysis_feedback` 入库；`GET /analyses/metrics` 返回 5 次分析 / 1 条反馈 / 2 条 `trust_gate_failed`。
- 真机发现并修复：框架标签的长 `value` 会挤压标签标题，改为标题与值纵向排列（`apps/mobile/src/analysis/components.tsx`）。
