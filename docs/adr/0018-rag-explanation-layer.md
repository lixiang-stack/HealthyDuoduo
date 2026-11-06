# 0018 · 解释层改为 RAG：本地 BGE-M3 + pgvector，接受 C0 语料风险

**状态：** 采纳（2026-11-06）。

## 背景

- 当前解释层依赖 `internal/analysis/knowledge/*.yaml`：只有 2 条肝功条目，模型只写 `bridge_text`，四段医学正文逐字取自审核条目。覆盖窄、内容单薄，新增模式要手写 YAML + 人工 review。
- 真正瓶颈是审核过的医学内容少，不是检索不到。RAG 把工作单元从「按模式写条目」变成「按原文切知识块 + 检索组合」，降低新增覆盖的边际成本；代价是瓶颈转移到语料质量、检索质量和生成校验。
- 现有仓库没有 embedding、pgvector、语料表；Postgres 是 `postgres:17-alpine`，LLM 只有云端 DeepSeek。引入 RAG 是新的架构决策，需要独立记录。
- 详细需求、数据模型、评估 gate 见 `.scratch/rag-liver/spec.md`；UI 选型见 `.scratch/rag-liver/prototype/`。

## 决策

1. **RAG 取代 YAML 作为解释层知识来源。** v1 只做肝功、成人 18+、Panel + 单指标入口；生产环境不再加载 `knowledge/*.yaml`，旧 YAML 只留在 `internal/analysis/testdata/legacy_yaml/` 作为离线评估 baseline。
2. **模型写四段，服务端校验。** LLM 基于检索 chunk 生成 `claims`（`text + quote + chunk_id + source_id + evidence_ids`），服务端校验结构、引用存在、quote 逐字子串、数字/方向、内容边界；不再要求四段正文逐字来自 YAML。
3. **本地 embedding + pgvector。** 新增独立 Python 服务 `rag-embed`（BGE-M3，CPU，模型缓存 volume）；Postgres 改用 `pgvector/pgvector:pg17`，新增 `rag_documents` / `rag_chunks` / `rag_index_versions`。检索用 dense + sparse/BM25，RRF 融合，top 8–12。Embedding 不走出站请求。
4. **聊天模型仍走云端 DeepSeek，模型档位可配置。** 生成用 flash 档；评估 judge 用 pro/reasoner 档。沿用 ADR-0006 的局域网前提与既有 LLM 出站例外；本 ADR 不新增第三方出站，发送内容仍限于既有脱敏事实 + 公开指南片段。
5. **产品边界前移。** 允许展示指南中的病因、进一步检查、生活建议、跟踪、红黄绿灯、一般诊断标准和一般复查间隔；在证据完整时允许判断「符合/不符合某指南标准」。禁止个体疾病诊断、治疗/药物/剂量、预后、个体化复查医嘱；v1 儿科语料只入库、不检索、不输出。
6. **上线方式：RAG-only + kill switch，不做生产灰度，不兼容旧数据。** 项目未上线，可删除存量分析、旧缓存；`HDD_RAG_ENABLED=false` 时只输出事实层 + 框架层。YAML 不参与生产回退。
7. **语料按 C0 处理：整段保留，不按内容类型拆分、不丢弃治疗段。** 只按标题/段落切块；治疗、诊断标准、预后、儿科内容与允许内容可能在同一 chunk。

## 后果

- 覆盖和深度不再受 YAML 条目数量线性限制；新增来源只需入语料、标 metadata、重建索引。可溯源性从「条目 ID」变为「chunk_id + 逐字 quote」，更细，但依赖检索和校验质量。
- 旧的「模型不写医学断言」硬约束被替换为「模型写，但每条结论必须绑定逐字引文并经规则校验」。这是安全模型的实质变化，不能再用旧 spec 的 §4.12 条款解释新行为。
- 本地 BGE-M3 增加 NAS 的 CPU/内存占用与一个容器；pgvector 需要换 Postgres 镜像并做迁移。两件事都必须在部署前验证，不能假设 NAS 规格足够。
- 无生产灰度、无旧数据兼容：切换是直接的，出错时只能靠 kill switch 退回前两层。这是项目未上线阶段的取舍。
- 单人审核、无第二评审人；DeepSeek 同供应商 judge 有自评偏差。评估报告必须记录这两个限制。
- 云端 LLM 出站例外目前只记录在 `.scratch/llm-analysis/spec.md` §4.11，尚未升格为独立 ADR；本 ADR 不改变它，但后续合规/公网评审应把它补成正式 ADR。

## 已接受的残余风险：C0

C0 是本次最明确的风险接受，不是疏忽：

- 治疗段与允许段可能在同一 chunk；检索到该 chunk 后，模型能看到治疗内容。
- 运行时只有 prompt + 规则 validator；规则能抓显式药物名/剂量、治疗动词、预后词、个体诊断/个体化复查句式，但抓不住所有隐含治疗建议和语义反转。
- 因此「政策违规 0」只能写成「评估集 120 次运行中观察到 0」，不能写成「保证 0」。需求文档 §11 的 gate 已按此调整。
- 缓解手段：强 prompt（只引用允许内容）、运行时规则校验、对抗 case（治疗段高度相关、儿科 chunk 高度相关、证据不完整、伪造 quote）、G1/G2 人工复核。
- **触发重审的条件：** G1 或 G2 中任何一次用户可见输出出现治疗/药物/剂量/预后内容，或儿科 chunk 被成人 case 引用，立即停止上线并重新评估 C0；可选修复是按内容类型拆 chunk 或加句子级标签，需要新的 ADR。
- 产品负责人于 2026-11-06 明确接受该残余风险；本 ADR 是该接受的引用锚点。

## 备选方案

- **继续扩 YAML**：安全模型最简单，但新增覆盖的边际成本高、深度受条目粒度限制。否。
- **抽取式 RAG（四段正文逐字引用 chunk，模型只写连接词）**：可溯源性最强，但输出碎片化，且仍需人工按四段标 chunk。否——本需求要的是跨段综合。
- **云端 embedding**：不占 NAS、接入简单，但新增一个出站依赖，把语料/查询发给第三方。否——选本地 BGE-M3；聊天模型仍沿用既有云端例外。
- **文件索引 + 内存 cosine，不引入 pgvector**：10–20 份语料足够，运维更轻。否——选 pgvector 为后续增长和独立验证留空间；代价是镜像/迁移复杂度。
- **按内容类型拆 chunk 或句子级标签**：能显著降低 C0 泄漏风险，但人工 curation 和校验复杂度更高。v1 否；作为 C0 触发重审后的首选修复。
- **生产灰度 / YAML 兜底**：能降低切换风险，但项目未上线，灰度机制和维护冻结 YAML 都是无收益复杂度。否。
- **完全禁止诊断标准判断**：最保守，但用户明确要求允许「符合/不符合某指南标准」，并以证据完整性和 `standard_check` 约束。否。

## 关联

- 需求基线：`.scratch/rag-liver/spec.md`
- UI 原型：`.scratch/rag-liver/prototype/prototype.html`
- 既有分析设计：`.scratch/llm-analysis/spec.md`
- 局域网边界：`docs/adr/0006-lan-first-no-auth.md`
