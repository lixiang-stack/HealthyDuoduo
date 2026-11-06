# 03: 语料 manifest + 离线 ingestion + 检索 CLI

**What to build:** 开发者用一份小 fixture 语料（3–5 份公开短摘录或合成文本，不提交受版权保护全文）和 manifest，跑通「抽取 → 切块 → 标 metadata → embed → 写 pgvector → 生成 index_version」，再用检索 CLI 对肝功 case 打印 top chunks、分数和过滤结果。验证 fixture 查询有结果、pediatric fixture 被 adult 过滤、重复构建幂等。

**Blocked by:** 01 (pgvector 基础与 RAG schema), 02 (本地 BGE-M3 embedding 服务).

**Status:** ready-for-agent

- [ ] manifest 校验：必填字段、source_id 唯一、age_group / sections / panels 合法；错误信息可定位。
- [ ] 抽取与切块保留标题路径；chunk 目标 200–500 token；chunk_id 稳定；content_hash 可识别内容变化。
- [ ] 每个 chunk 写入 sections / panels / analytes / patterns / age_group / language / authority / year。
- [ ] 构建生成新的 index_version，可重复执行且不产生重复 chunk；失败不激活新版本。
- [ ] 检索 CLI 支持 panel / analyte / framework label / section 过滤，打印 rank / score / chunk id / 来源。
- [ ] `age_group=adult` 硬过滤排除 pediatric fixture；有测试。
- [ ] fixture 语料不含受版权保护全文；manifest 模板与 spec §6.5 一致。
