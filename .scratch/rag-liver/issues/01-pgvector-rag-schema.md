# 01: pgvector 基础与 RAG schema

**What to build:** 本地 Postgres 能启用 pgvector，并具备可重复迁移的 RAG 存储：文档、知识块、索引版本三张表；知识块上带 dense 向量、sparse 表示和 metadata（sections / panels / analytes / patterns / age_group / authority / year / content_hash / corpus_version）。现有真库 storage 测试在新镜像上通过，迁移可重复执行，未改变现有分析 HTTP 行为。

**Blocked by:** None (can start immediately).

**Status:** ready-for-agent

- [ ] 部署镜像支持 `CREATE EXTENSION vector`；本地 compose 与真库测试使用同一 pgvector 镜像。
- [ ] 迁移创建三张表、向量索引和 metadata 索引；空库与已有库都能升级，重复执行不报错。
- [ ] 真库 storage 测试通过；表约束、级联、索引有测试覆盖。
- [ ] 索引版本的生成/激活方式有文档说明（见 spec §7.4），后续 ingestion 可直接使用。
- [ ] 不改变现有分析 API 的 facts / framework / explanation 形状。
