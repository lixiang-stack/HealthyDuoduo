# 13: 全量 golden / reparse / e2e 验收

**What to build:** 全量收口：重新生成所有 golden，执行 `hdd reparse`，跑 e2e verify，确认 `unit` 字段没有 OCR 碎片、未验证项按约定为 null + unverified、趋势/展示/契约一致。允许 `review_status=pending` 的项以 raw+unverified 形态存在，verified 翻转由 14/15 负责。

**Blocked by:** 12 (趋势按 code + canonical unit)

**Status:** ready-for-agent

- [ ] 全量 golden 重冻结，逐项变更说明；不再有 `.09/L`、`7/6.0`、`ag/L` 等 `unit`。
- [ ] `hdd reparse` 全量重跑成功；API/App/趋势读到的字段一致。
- [ ] e2e verify 通过；未验证项数量与原因可审计。
- [ ] CBC/生化/甲功/凝血 matched/value 不下降；unit 变更全部有说明。
- [ ] 本地无数据时，CI + 样本子集验收，并在票内记录缺口。
