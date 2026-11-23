# 解释层知识来源选型：RAG vs LLM Wiki vs SAG/SAG2

**状态：** 调研（不改变生产代码、`.scratch/rag-liver/spec.md` 或 `docs/adr/0018-rag-explanation-layer.md`）
**日期：** 2026-11-18
**范围：** 肝功解释层的知识来源与检索形态；不评估生成模型、UI 或医学内容本身。

## 0. 决策问题与结论

### 0.1 决策问题

在 10–20 份中英肝功指南、成人-only 硬过滤、每条 claim 必须带逐字 `quote + chunk_id`、无头 Go HTTP 服务、检索 ≤1s（本调研按 p95 口径使用）、端到端 p95 ≤30s、单人审核、C0 风险已接受的约束下，解释层的知识来源应选 RAG、LLM Wiki，还是 SAG/SAG2？

### 0.2 结论

**推荐维持 RAG 作为唯一生产证据层。** 不是因为它“更先进”，而是因为本需求的三条硬约束——逐字引文可校验、chunk 级硬 metadata 过滤、无头确定性服务——正好落在本 spec 这套 RAG 的现有证据契约里，而另外两个方案都需要在外部再补一层等价机制。

**SAG2 只作为 retrieval 备选实验，不进入 v1 生产路径。** 它的 BM25-first、查询时证据判断、词项扩展值得借来做 baseline；但现成形态没有 chunk 级 metadata 过滤，论文均值 1.47s/query 已超过检索 ≤1s（小语料可能更快，但未实测），且中文肝功单跳检索没有评测。只有在补齐过滤、延迟和中文证据后，才值得单独试点。

**LLM Wiki 不作为运行时层；最多作为离线人工审阅的派生视图。** Karpathy 的模式擅长跨文档综合，但页面是综合产物，不天然保留逐字来源；本研究引用的 preregistered 对比还显示 wiki 每次查询约 21× tokens、构建约两个多数量级 tokens，且引文支持优势在修正后不成立。它可以帮人“读厚”，但不能替代 `quote + chunk_id` 的证据契约。

**明确边界：** 如果将来采用“RAG 证据层 + Wiki 审阅视图”，Wiki 必须是只读派生品，每条陈述回链到 `chunk_id + quote`，不参与线上生成；这与 Karpathy 的“agent 直接读 wiki 回答”不是同一条路径。

### 0.3 改选条件

- **改选 SAG2 的条件：** 在 40 case 子集上，SAG2 加 chunk 级过滤后 Recall@10 ≥0.90、检索 p95 ≤1s、中文 case 不劣于 BGE-M3 混合检索，且 Jev 出站与成本可接受。四项同时成立才试点；缺一项都不换。
- **改选 LLM Wiki 的条件：** 产品从“12 条短 claim + 引文”转为“长篇跨指南综合报告”，且接受页面/claim 级人工复核、评估集重做、C0 改为编译期预审查。即使如此，也应保留 chunk 级证据层，Wiki 只是视图。
- **简化 RAG 的条件：** 如果 BM25-only 在 40 case 上 Recall@10 ≥0.90、p95 <300ms，可以删除 `rag-embed` 和 BGE-M3，退化为 Postgres BM25 + 规则校验；这是 RAG 内部的简化，不是改选 Wiki/SAG2。

## 1. 三个候选不是同一层级

先把名字拆开，否则对照表会失真。

| 名称 | 是什么 | 索引期做什么 | 查询期做什么 | 运行时形态 |
| --- | --- | --- | --- | --- |
| RAG（本仓库当前方案） | 一套具体的检索生成流水线 | 切 chunk、BGE-M3 dense+sparse、写 pgvector | metadata 过滤、dense/sparse RRF、top 8–12、LLM 写 claims、服务端校验 | Go HTTP + Postgres/pgvector + 独立 Python `rag-embed` |
| LLM Wiki | Karpathy 提出的 artifact/模式，不是产品 | LLM 读原始资料，抽取要点，维护互链 markdown 页面 | agent 搜页面、读页面、带引用综合回答 | 需要 agent 会话与文件工具；没有标准 HTTP 契约 |
| SAG | Zleap 的 event-entity 检索架构 | chunk、embedding、event/entity 抽取、关系存储 | 查询时 SQL join 共享实体，动态超边，返回原始 chunk | Python 引擎 + FastAPI/Web/MCP；SQLite+LanceDB 默认，Postgres/pgvector 可选 |
| SAG2 | SAG 之后的 embedding-free 检索方法 | 只建 SQLite FTS5 词法索引，不调用模型 | BM25 召回 + Jev 证据判断 + 词项扩展多跳 | Python benchmark/library；可被产品封装 |
| SAG-Wiki | Zleap 的桌面知识库产品，底层用 SAG2 | 本地文件索引，无 embedding/无 LLM 索引调用 | 快速 BM25 搜索；深度检索调用 Jev，再生成带引用回答 | Rust/Tauri 桌面 + CLI + 本地服务 + MCP；默认 Jev 云服务 |

### 1.1 名称与版本核对

- 任务描述中的“SAG v1.2.2”未在一手来源中找到。截至 2026-11-18，SAG README 徽标与 GitHub Releases 页均为 v1.9.2（2026-10-22）；本文以 v1.9.2 为准。
- “Python 3.11+”是 SAG 引擎 `zleap-sag` 的要求，不是 SAG-Wiki 的要求。SAG-Wiki 的 core 是 Rust；SAG2 benchmark 是 Python。
- SAG-Wiki 截至 2026-11-17 最新为 v0.3.11；v0.3.1 到 v0.3.11 在 2026-11-15 至 11-17 之间连续发布，属于非常年轻的快速迭代项目。
- SAG 与 SAG-Wiki 的代码是 MIT；Jev/Zev 是独立的商业模型服务，默认走 Zleap 云端点，不等于 MIT 覆盖的部分。

## 2. 三方案定义

### 2.1 RAG（当前 spec 方案）

离线把语料按标题/段落切成 200–500 token 的 chunk，标 `source_id`、`sections`、`panels`、`analytes`、`patterns`、`age_group`、`language`、`authority`、`year`；用 BGE-M3 生成 1024 维 dense 和 sparse 表示，写入 Postgres + pgvector。查询时强制 `age_group=adult`、`panels` 含 `liver`、`sections` 与 required sections 有交集，dense/sparse 各取候选后用 RRF 融合，取 top 8–12；LLM 基于 chunk 全文生成 claims，每条带 `text`、逐字 `quote`、`chunk_id`、`source_id`、`evidence_ids`；服务端做结构、引用存在、quote 子串、数字/方向、边界、`standard_check` 一致性校验，失败 claim 丢弃。见 [spec §5、§7、§8](.scratch/rag-liver/spec.md) 与 [ADR-0018](docs/adr/0018-rag-explanation-layer.md)。

关键点：**quote 校验不是“引用一个来源”，而是 quote 必须是本次检索到的 chunk 原文的逐字子串**。这使“模型编造来源”变成一个可被服务端确定性拒绝的错误。

C0 是本 spec 的语料风险：治疗段与允许段可能在同一 chunk；v1 不按内容类型拆分，只在运行时用 prompt + 规则 validator 控制，残余语义风险已接受。见 [spec §12.1](.scratch/rag-liver/spec.md) 与 [ADR-0018](docs/adr/0018-rag-explanation-layer.md)。

### 2.2 LLM Wiki（Karpathy 模式）

Karpathy 在 2026-04-04 的 gist 中提出：不要每次查询都从原始文档检索，而是让 LLM 增量维护一个持久 wiki。它有三层：

1. **Raw sources**：原始资料，只读，是事实来源。
2. **Wiki**：LLM 写的 markdown 页面，包括摘要、实体页、概念页、比较、综合；LLM 负责创建、更新、互链、记录矛盾。
3. **Schema**：例如 `CLAUDE.md` / `AGENTS.md`，告诉 LLM 如何 ingest、query、lint。

操作是 ingest / query / lint：ingest 时一个来源可能触及 10–15 个页面；query 时 LLM 先读 `index.md` 找相关页面，再读页面并带引用综合回答；lint 时让 LLM 检查页面间矛盾、过期 claim、孤儿页、缺失交叉引用、数据缺口。它声称在约 100 个来源、数百个页面的规模下，`index.md` 足够导航，不需要 embedding RAG 基础设施。[S1]

需要注意原文的边界：**它没有定义 claim 级 schema，没有要求每条 claim 回链到来源对象，也没有给出可验收的 lint 阈值或失败条件。**“每条 claim 链接回来源”是实现者可以加的约束，不是该 gist 的保证。原文还明确说自己是“intentionally abstract”，不是具体实现。[S1]

### 2.3 SAG / SAG2 / SAG-Wiki

**SAG（arXiv:2606.15971）** 是 event-entity 架构：离线把每个 chunk 抽成一个语义完整的 event 和多个 entity，形成 latent hyperedge；查询时用共享 entity 做 SQL join，动态生成 query-scoped 超边；输出仍是原始 chunk。它需要离线 LLM 抽取 event/entity，也需要 embedding；默认 SQLite+LanceDB，可选 Postgres/pgvector。[S4][S5]

**SAG2（arXiv:2610.22588）** 是 embedding-free 检索：只持久化 SQLite FTS5 词法索引；查询时 BM25 取 top 100，Jev 决策模型在共享上下文里给每个候选打“是否提供答案链事实”的分数；高分 seed 激活词项，词项扩展召回未访问 passage；最多 2 轮扩展加最终重排，最多 340 个候选判断。索引不调用模型，查询时调用 Jev。[S3]

**SAG-Wiki** 是把 SAG2 产品化的桌面知识库：Rust/Tauri + Svelte，CLI 与桌面共享同一个本地服务；SQLite FTS5 索引，chunk 约 512 字符、重叠约 80 字符；中文用 jieba 分词；默认 Jev 端点是 `https://api.zleap.com/v1`、模型 `Zev-1`，也支持自建 Jev-compatible 端点。它返回原始文档与 passage 位置，引用可回到原文。[S6][S7]

**容易混淆的三点：**

- SAG2 不是“不用模型”。它是“索引不用模型，查询用决策模型”；Jev 仍是每次语义查询的模型调用。
- SAG-Wiki 不是 Python 3.11+ 服务；它是 Rust 桌面/CLI。Python 3.11+ 是 SAG 引擎。
- SAG-Wiki 的 README 既说“no LLM calls for indexing or retrieval”，又说“uses Jev evidence scoring at query time”。合理解释是：快速 BM25 搜索不调模型，深度语义检索调 Jev；但这句话不能作为“查询零模型调用”的证据。[S6]

### 2.4 SAG2 四个特别核实点

- **Jev 是不是每次查询都调用 LLM？** Jev 不是生成式 LLM，而是输出概率化结构化判断的 decision model（论文默认 `jev-1.14`，经 Noul 接口调用）。语义检索每次查询都会调用它；默认配置下约 3 次 scoring 请求/问题，最多 340 个候选判断（候选级判断会被批处理，不是 340 次 HTTP 调用）。快速 BM25 搜索不调用模型。[S3][S6]
- **延迟与成本？** 论文在 MuSiQue、1,000 问题、10 并发下报告均值 1.47 s/query（检索阶段，不含答案生成）；p95 未报告。按论文 list price，检索加索引约 $4.33/1,000 问；20 次/天约 $2.6/月（按论文 Jev 单价估算，实际 Zev 价格未验证）。论文还提示稳定语料上查询成本会累积（MuSiQue 约 10,216 问后超过 SAG，按 20 次/天约 1.4 年），但量级仍小。成本不是主要障碍，延迟和过滤才是。[S3]
- **支持中文吗？** SAG-Wiki 的索引层用 jieba 分词，中文文本可以被索引；但 SAG2 论文只报告英文数据集，没有中文/跨语言实验，Jev 对中文医学 passage 的判断质量没有一手数据。因此“能索引中文”不等于“中文肝功检索达标”。[S3][S7]
- **支持 metadata 过滤吗？** 不支持 chunk 级 `age_group`/`panels`/`sections`/`patterns`。SAG-Wiki 的 scope 是目录/知识库根，SAG 原版是 source scope；`age_group=all` 的混合文档无法靠目录过滤。要硬过滤，必须改 SAG2 的 SQLite schema 与查询，或在我们的 chunk 层重建索引。[S3][S5][S6]

## 3. 十个硬约束对照

单元格格式为“判断 — 一句理由”。“部分”表示需要额外工程或只在特定形态下成立；“待验证”表示一手来源没有给出本仓库所需数据。

| 硬约束 | RAG（当前 spec） | LLM Wiki | SAG/SAG2 |
| --- | --- | --- | --- |
| 1. 逐字引用可校验 | 满足 — claim 原生带 `quote + chunk_id`，服务端做 chunk 原文子串校验。 | 不满足 — 页面是综合产物，原文不保证逐字保留；要做 claim 级 provenance 就得在页面下再存 chunk/quote，等于保留 RAG 证据层。 | 部分 — 返回原始 passage，具备逐字引文基础；但没有与我们的 `chunk_id` 对齐的稳定契约，SAG-Wiki 还会按约 512 字符重新切块。 |
| 2. 硬 metadata 过滤 | 满足 — Postgres `WHERE age_group='adult'` + `panels`/`sections` GIN 索引，语义是精确过滤；HNSW 过滤有召回 caveat，但年龄排除本身不会被绕过。 | 部分 — frontmatter 可以标 `age_group`/`panels`，但页面可能混合来源与年龄；硬过滤要求页面粒度与源 chunk 一致，需编译规范 + lint，不是原生保证。 | 不满足 — 论文没有 chunk 级 metadata 过滤；SAG-Wiki 只有目录/知识库根 scope，SAG 原版只有 source scope；`age_group=all` 的混合文档无法靠目录过滤。 |
| 3. 无头 Go HTTP 服务 | 满足 — Go HTTP + Postgres + `rag-embed` HTTP，没有 agent 会话依赖。 | 不满足 — 原文假设 agent 搜页面/读页面/综合；没有确定性 HTTP 契约；要变成服务得自建 page retrieval + prompt 流水线，本质是 RAG over pages。 | 部分 — SAG 原版有 FastAPI/OpenAPI/MCP，可无头；SAG-Wiki 有桌面/CLI + 默认 loopback 服务，v0.3.11 起有远程 HTTP/MCP 但 scope 仍是目录；SAG2 benchmark 是 Python 库，需要自己包 HTTP。 |
| 4. 延迟与成本 | 部分 — 成本可控且无新增出站；检索 ≤1s 在 NAS 上尚未实测，BGE-M3 CPU 查询 embedding 是主要未知。 | 不满足 — 按 Cochran 的 agent 浏览实现，约 127k tokens/query，超过 12k 输入上限；构建约 646× RAG tokens；除非改成单页读取，但那就不是 Wiki 模式。 | 不满足 — SAG2 论文检索均值 1.47s/query（10 并发，不含答案生成），约 3 次 scoring 请求/问题，最多 340 个候选判断；p95 未报告，已超过检索 ≤1s；小语料可能更快，但需实测。 |
| 5. 语料规模与更新 | 满足 — 10–20 份、低频更新正好适配 manifest + 重建索引；更新一份只需重切/重嵌入该文档，chunk_id 稳定。 | 部分 — 小稳定语料是 Wiki 的甜点区；但一个来源可能触及 10–15 个页面，更新一份指南要重审受影响页面与矛盾，单人审核是瓶颈。 | 部分 — SAG2 增量索引便宜、索引不调模型；SAG 原版每次更新要重跑 LLM event/entity 抽取和 embedding；两种形态差别很大。 |
| 6. 评估可操作性 | 满足 — 40 case、golden `chunk_id`、Recall@10、quote 100%、entailment ≥90% 都能直接复用现有设计。 | 不满足 — 评估单位变成页面/claim，40 个 chunk 级 golden 不能直接映射；需重做标注；Cochran 的自动 citation scorer 与人工只有 65–69% 一致。 | 部分 — 若写适配器返回 chunk，可复用 chunk 级 qrels；但 SAG2 的 benchmark 是英文多跳 QA，没有中文肝功/单跳/metadata 过滤评测。 |
| 7. 安全边界 C0 | 部分 — prompt + 规则 validator + 对抗 case；已接受残余风险，不是保证。 | 部分 — compile 阶段可以预审查治疗段，确实可能降低运行时泄漏；但代价是逐页人工复核、C0 决策变更、编译漂移，不是免费收益。 | 部分 — 没有 compile 预审查；多跳扩展可能拉入更多治疗段；运行时风险与 RAG 同级或更差。 |
| 8. 可审计/可复现 | 满足 — 快照已记录 corpus/index/embedding/retriever/prompt/model 与 chunk rank/score。 | 部分 — wiki 是 git markdown，页面历史与 log 可审计；claim 级 provenance、编译模型、页面版本与 quote 校验需自建，且重编译会漂移。 | 部分 — SAG2 索引确定性较好，但检索依赖 Jev 模型/版本/候选预算；SAG-Wiki 存文件 hash/位置，但没有我们这种快照契约。 |
| 9. 运维足迹 | 部分 — 新增一个 Python 容器 + pgvector 镜像；NAS CPU/RAM 是否够未实测。 | 部分 — 运行时不需要向量库，但需要编译流水线、lint、页面存储、审阅界面；人力运维大于机器运维。 | 部分 — SAG2 只需 SQLite + Jev 云调用，机器轻但新增出站依赖；SAG 原版还要 embedding + LLM 抽取 + LanceDB/pgvector。 |
| 10. 许可与成熟度 | 满足 — 自建 + BGE-M3 MIT + 开源 pgvector；无 vendor lock-in，维护成本自有但可控。 | 部分 — 是模式不是产品；gist 没有明确产品许可/支持，具体实现各异；锁定低但自建与维护成本高。 | 部分 — MIT，但 SAG-Wiki v0.3.11 很年轻、发布节奏快；默认 Jev/Zev 商业服务，模型与服务锁定需评估。 |

### 3.1 读表注意

- **RAG 的 metadata 过滤是“精确语义”，SAG 的 scope 是“文件/来源语义”。** 这两者不能互换：`sections`、`patterns` 是 chunk 级标签，目录 scope 无法表达；一份指南里成人段和儿科段混排时，目录过滤也切不开。
- **pgvector 的 HNSW 过滤是后置过滤。** 官方 README 明确说 approximate index 的 `WHERE` 过滤在 index scan 之后；过滤选择性高时可能返回不足。v1 语料 <10k chunk，建议过滤查询走 exact scan，或按 `age_group` 做 partial index/partition；若坚持 HNSW，需开 iterative scan（0.8.0+）并实测召回。[S8]
- **SAG2 的 1.47s 是均值，不是 p95。** 论文在 MuSiQue、1,000 问题、10 并发下报告 1.47 s/q；不能推出 p95 ≤1s，也不能推出 SAG-Wiki 云 Jev 的延迟。[S3]
- **Wiki 的 21× 是 token 比，不是美元比。** Cochran 明确说 token accounting 不含 embedding/reranker/存储，也不是 billable cost；21× 不能直接换算成账单倍数。[S2]

## 4. 关键事实与引用表

| 文档 | 它定义了什么 | 它不能推出什么 | URL / DOI / 访问日期 |
| --- | --- | --- | --- |
| [S1] Karpathy, “LLM Wiki” gist | LLM Wiki 模式：raw sources / wiki / schema 三层；ingest/query/lint；`index.md` 与 `log.md`；一个来源可能触及 10–15 页；声称约 100 来源、数百页面可不用 embedding 基础设施；批评 RAG 每次查询都重新发现知识、没有积累。 | 不能推出 claim 级 `quote + chunk_id` 契约（原文没有）；不能推出 lint 有可验收阈值；不能推出生产延迟/成本/中文支持；这是作者观点与模式，不是 peer-reviewed benchmark，也不是可安装产品。 | https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f （创建 2026-04-04；访问 2026-11-18） |
| [S2] Cochran, “Single-Round Vector RAG vs an LLM-Compiled Wiki” arXiv:2605.18490 | 预注册对比：24 篇论文、13 个问题、同一生成模型、两个盲评 judge；wiki 在跨论文综合上更好，但每次查询约 21× tokens（1.65M vs 78k/13 问），构建约 646× tokens（904.9M vs 1.4M）；修正截断错误后，wiki 没有 citation-support 优势；judge 在整体 groundedness 上 rank agreement 仅 rho=0.04。 | 不能推出我们的中文肝功语料/短 claim 任务也如此；不能推出 wiki 在所有综合任务都赢；不能推出 21× 可直接换算成美元；样本仅 13 问/24 篇/一个模型，非统计功效充分或代表性研究。注意：摘要有一句“wiki was cheaper to build”，与正文 H3a 表及 904.9M vs 1.4M 数字相反；本文按正文/表格，认 wiki 构建更贵。 | https://arxiv.org/abs/2605.18490 （v1 2026-05-18；v3 2026-10-02；访问 2026-11-18） |
| [S3] Wu et al., “SAG2: Embedding-Free Multi-Hop Retrieval with BM25 and Decision Models” arXiv:2610.22588 | SAG2 方法：SQLite FTS5 BM25 + Jev 决策模型 + 词项扩展；B=100、K=10、L=20、H=2；最多 340 个候选判断、约 3 次 scoring 请求/问题；MuSiQue/2Wiki/HotpotQA（各 1,000 问子集）R@10 86.62/97.43/99.00；检索均值 1.47 s/q、索引+检索 $4.33/1000 问；索引 2.36s/11,656 passages。 | 不能推出中文或肝功指南单跳检索更优；不能推出支持 chunk 级 metadata 过滤；不能推出 p95 ≤1s；不能推出 SAG-Wiki 云 Jev 的实际延迟/价格；benchmark 是英文多跳 QA，Jev 是专有模型，未复现。 | https://arxiv.org/abs/2610.22588 （v1 2026-10-15；v2 2026-11-10；访问 2026-11-18） |
| [S4] Wu et al., “SAG: SQL-Retrieval Augmented Generation with Query-Time Dynamic Hyperedges” arXiv:2606.15971 | SAG 原始架构：event-entity 索引、查询时 SQL 动态超边、证据始终是原始 chunk；离线需要 LLM event/entity 抽取与 embedding；MuSiQue R@5 80.36%。 | 不能推出 SAG 是 embedding-free 或轻量；不能推出 chunk 级 metadata 过滤；不能推出适合 NAS/无头服务；不能与 SAG2/SAG-Wiki 混为一谈。 | https://arxiv.org/abs/2606.15971 （v1 2026-06-14；v2 2026-08-25；访问 2026-11-18） |
| [S5] Zleap-AI/SAG GitHub README | SAG 产品形态：SQLite+LanceDB 默认，Postgres/pgvector 可选；cross-source/source-scoped 检索；REST/OpenAPI、MCP、Python 包；MIT；README 徽标 1.9.2；Releases 最新 1.9.2（2026-10-22）。 | 不能推出 chunk 级 `age_group`/`panels`/`sections` 过滤；不能推出索引零 LLM；不能推出 v1.9.2 的中文检索质量；不能推出 NAS 资源足够。 | https://github.com/Zleap-AI/SAG （访问 2026-11-18） |
| [S6] Zleap-AI/SAG-Wiki GitHub README | SAG-Wiki 产品：Rust/Tauri 桌面 + CLI + 本地服务 + MCP；底层 SAG2；原始文档与 passage 位置；默认 Jev 云端点 `https://api.zleap.com/v1`、模型 `Zev-1`，支持自建 Jev-compatible 端点；MIT；厂商对比表（SAG-Wiki / LLM Wiki / Typical vector RAG）。 | 不能推出厂商对比表中 RAG/LLM Wiki 的描述是独立事实（这是 SAG 方自述）；不能推出无头 LAN HTTP 即可用；不能推出 chunk 级 metadata 过滤；不能推出中文检索质量；不能推出成熟度。 | https://github.com/Zleap-AI/SAG-Wiki （访问 2026-11-18） |
| [S7] SAG-Wiki architecture docs 与 `core/src/text.rs` | 本地服务只监听 `127.0.0.1`、随机端口 + token；SQLite FTS5；chunk 约 512 字符、重叠约 80；中文用 jieba 分词；20 并发检索；文件 hash + byte/line/column 位置；query scope 与浏览分离。 | 不能推出 chunk 级 metadata 过滤；不能推出中文医学检索准确率；不能推出 p95；不能推出局域网多用户/无鉴权部署；代码存在不等于产品稳定。 | https://github.com/Zleap-AI/SAG-Wiki/blob/main/docs/en/architecture.md ；https://github.com/Zleap-AI/SAG-Wiki/blob/main/core/src/text.rs （访问 2026-11-18） |
| [S8] pgvector README | 支持 exact、HNSW、IVFFlat；`sparsevec` 自 0.7.0；iterative index scans 自 0.8.0；approximate index 的 `WHERE` 过滤在 index scan 之后；当前 README 版本 0.8.7。 | 不能推出 BGE-M3 在 NAS 上的吞吐；不能推出 <10k chunk 必须用 HNSW；不能推出带 `age_group` 过滤的 HNSW 召回一定够；不能推出医学 metadata 语义。 | https://github.com/pgvector/pgvector （访问 2026-11-18） |
| [S9] BGE-M3 论文与 Hugging Face model card | 1024 维、8192 token、100+ 工作语言；同时支持 dense、sparse、multi-vector；MIT。 | 不能推出中文医学/肝功领域的检索准确率；不能推出 NAS CPU 的查询延迟/吞吐；不能推出 hybrid 一定优于 BM25；通用 benchmark 不等于本语料。 | https://arxiv.org/abs/2402.03216 ；https://huggingface.co/BAAI/bge-m3 （访问 2026-11-18） |
| [S10] Lewis et al., “Retrieval-Augmented Generation for Knowledge-Intensive NLP Tasks” | RAG 的原始定义：参数化 seq2seq + 非参数化 dense Wikipedia index + neural retriever；RAG-Sequence / RAG-Token 两种形式。 | 不能推出 quote 子串校验、metadata 过滤、混合检索或医学事实性；不能把 2020 年 Wikipedia QA 的结果外推到本产品；RAG 是方法家族，不是质量保证。 | https://arxiv.org/abs/2005.11401 （NeurIPS 2020；访问 2026-11-18） |
| [S11] SQLite FTS5 官方文档 | FTS5 的 BM25、tokenizer（unicode61/ascii/porter/trigram）、列过滤、contentless 表；unicode61 按连续 token 字符切分，中文无空格时不会自动分词，需 trigram 或自定义 tokenizer。 | 不能推出 SAG2/SAG-Wiki 的中文效果；不能推出 jieba 分词后的医学术语召回；不能推出 FTS5 适合我们的 chunk 粒度。 | https://sqlite.org/fts5.html （访问 2026-11-18） |
| [S12] SAG-Wiki Releases | 截至 2026-11-17 最新 v0.3.11；v0.3.1–v0.3.11 在 2026-11-15 至 11-17 连续发布；v0.3.11 增加 HTTP/MCP key 的目录范围绑定。 | 不能推出稳定性/质量；不能推出 v0.3.11 的 API 会长期兼容；不能推出目录 scope 等于 chunk metadata 过滤。 | https://github.com/Zleap-AI/SAG-Wiki/releases （访问 2026-11-18） |

## 5. 对本仓库的具体影响

### 5.1 如果维持 RAG（推荐）

**不变的条款：**

- [ADR-0018](docs/adr/0018-rag-explanation-layer.md) 的决策 1、2、4、5、6、7 不变：RAG 取代 YAML、claim+quote 校验、云端 DeepSeek 生成、产品边界、RAG-only + kill switch、C0 整段保留。
- [spec](.scratch/rag-liver/spec.md) §5 总体架构、§8 生成与校验、§9 契约与快照、§11 gate 的主体不变。
- 40 case × 3、Recall@10 ≥0.90、quote 100%、entailment ≥90%、儿科泄漏 0 的验收口径不变。

**建议补充（仍需用户确认后才改 spec）：**

1. **§7.3 增加 BM25 baseline 实验臂。** 在 dense/sparse RRF 之外，离线比较 BM25-only、dense-only、dense+sparse RRF 三组；如果 BM25-only 已达到 Recall@10 ≥0.90，v1 可以删除 `rag-embed`，降低 NAS 运维；如果未达到，保留 BGE-M3 混合检索。这个实验不会改变 RAG 证据层，只决定检索实现。
2. **§7.4 补充 pgvector 过滤 caveat。** v1 语料 <10k chunk 时，带 `age_group`/`sections` 过滤的查询优先 exact scan，或按 `age_group` 建 partial index/partition；若使用 HNSW，启用 `hnsw.iterative_scan` 并实测过滤后的召回。当前 spec 说“<10k chunk 时 exact scan 也可”，建议把它从“也可”提升为“过滤查询的默认”，避免 HNSW 后置过滤导致召回不足。[S8]
3. **§11.2 增加按语言/模式的检索分层报告。** 中文指南和英文指南的 Recall@10 分开报；肝细胞型、胆汁淤积型、混合型、单纯胆红素、低白蛋白分开报。否则一个总体 0.90 可能掩盖中文或某一模式的失败。
4. **§12.3 开放问题增加三项。** 中文 BM25/jieba 与 BGE-M3 的对比；NAS CPU 上 BGE-M3 查询 embedding 的 p95；pgvector filtered HNSW 的召回。三项都已有最小实验可回答。
5. **§14 决策记录增加一条“SAG2 仅作 retrieval 备选，不采用”。** 记录理由：无 chunk metadata、1.47s 均值超 1s、中文/单跳未评测。这样未来有人再提 SAG2 时，有可追溯的拒绝理由，而不是重新讨论。

**不建议做的：**

- 不要因为 SAG2 的 multi-hop 结果好看就替换检索层。我们的查询是“panel + analyte + pattern + section”的单跳定向检索，不是 MuSiQue 式多跳；SAG2 的收益来源（词项扩展、共享上下文）与我们的瓶颈（语料质量、claim 校验、C0）不匹配。
- 不要引入 Jev 出站。当前出站例外只覆盖 DeepSeek；新增 Zleap/Typesafe 需要新的合规/隐私评估，且会把指南片段与查询发给第三方。

### 5.2 如果改选 SAG2（retrieval-only）

**可以保留的：**

- `Claim` / `quote` / `chunk_id` / `source_id` / `standard_check` 契约可以保留，前提是写一个适配器：SAG2 返回的 passage 必须映射回我们的稳定 `chunk_id`，并把原文传给生成器。这样 quote 子串校验仍然成立。
- 生成、validator、UI、快照的大部分逻辑可以复用；SAG2 只替换“检索”这一层。

**必须重做的：**

- **metadata 过滤。** 需要 fork SAG2 的 SQLite schema/查询，在 BM25 候选、扩展候选、最终重排三处都加 `age_group`/`panels`/`sections` 过滤；不能只在最后过滤，否则候选池会被儿科/非肝功内容污染。还要为 `age_group=all` 的混合文档设计 chunk 级拆分或标记。
- **chunk_id 映射。** SAG-Wiki 自己按约 512 字符切块，和我们的 200–500 token chunk 不一致；要么改 SAG2 的 chunker 直接吃我们的 chunk，要么维护映射表。前者更干净，但等于把 SAG2 当库用，不是用 SAG-Wiki 产品。
- **延迟与成本。** 先测 p95；如果 >1s，需要减少 scoring 轮次或候选预算，但这会牺牲 Recall@10（论文 H=0 的 MuSiQue R@10 只有 67.93，H=2 是 86.62）。要在我们的 40 case 上重新找 operating point。
- **出站与许可。** Jev 是商业模型服务，默认云端点；需要新的出站 ADR/隐私评估。自建 Jev-compatible 端点可以规避出站，但模型质量未验证。
- **评估。** 40 case 的 golden `chunk_id` 可以复用（如果映射稳定），但要重跑 Recall@10、延迟、中文分层；SAG2 的英文多跳 benchmark 不能作为验收证据。

**结论：** SAG2 是一个“检索层替换”项目，不是一个“换个库”项目；工作量接近 spec §13 的阶段 3–4，且会新增一个出站依赖。除非 2 天实验证明它在中文肝功单跳上显著优于 BM25/BGE-M3，否则不值得。

### 5.3 如果改选 LLM Wiki

**契约与 gate 会变：**

- `Claim.quote + chunk_id` 不再是主契约；至少需要 `page_id + claim_id + source_quote + source_chunk_id`，否则无法做服务端逐字校验。也就是说，**即使改选 Wiki，RAG 的 chunk 证据层仍要保留**，否则 quote 100% gate 无法成立。
- 40 case 的 golden `chunk_id` 不能直接映射到页面；需要重标“期望页面/期望 claim/期望 quote”。这是一次评估集重做，不是改一个字段。
- `Recall@10` 要改成页面级/claim 级召回；`quote 100%` 要改成“页面引文可回链到源 chunk 且逐字匹配”；`entailment` 仍可用，但 judge 的输入从 chunk 变成页面，评审难度更高。
- 快照要新增 `wiki_version`、`compile_model`、`compile_prompt_version`、`page_source_map`、`page_content_hash`；否则无法复现“这次回答读的是哪一版页面”。
- UI 的来源列表要从 `source_id` 变成“页面 + 源 chunk + quote”；现有方案 B 的内联引用编号可以复用，但数据模型要改。

**C0 的变化：**

- Wiki 的 compile 阶段确实可以成为预审查机会：让 LLM 在写页面时只保留允许内容，把治疗/药物/剂量段排除或标记。但这会把 C0 从“运行时接受残余风险”改成“编译期人工复核”，需要新的 ADR 和新的审核流程。
- 代价：每个页面/claim 都要人工复核；来源更新会触发页面重编译，可能产生“这次改对了、下次改错了”的漂移；如果 compile 把治疗段删掉，页面可能失去上下文，反而影响医学准确性。
- 如果 C0 触发重审，更简单的修复仍是 spec §12 已列出的“按内容类型拆 chunk 或句子级标签”，不是引入 Wiki。

**工作量：** 至少新增编译流水线、lint、页面存储、审阅界面、评估重标；单人审核下，这会把瓶颈从“写 YAML”转移到“审页面”，与本需求“降低边际成本”的目标相反。

### 5.4 不改什么

- 不修改 [ADR-0018](docs/adr/0018-rag-explanation-layer.md)。如果未来改选 SAG2 或 Wiki，应新增 ADR，而不是改写已采纳的 0018。
- 不修改 [spec](.scratch/rag-liver/spec.md)。本文只给影响分析与实验建议；是否把 BM25 baseline、pgvector caveat 写进 spec，由用户决定。
- 不修改 `internal/analysis`、`internal/rag`、`rag/ingest.py`、`rag-embed` 或任何迁移。
- 不引入 SAG2/Jev/Wiki 依赖；本文不产生新依赖。

## 6. 分层结论

### 6.1 一手事实

- Karpathy gist 确实提出三层结构、ingest/query/lint、`index.md`/`log.md`，并声称约 100 来源/数百页面可不用 embedding 基础设施；它没有 claim 级 schema，也没有可验收的失败阈值。[S1]
- Cochran 的预注册研究确实报告 wiki 在跨论文综合上更好，但 query tokens 约 21×、ingest tokens 约 646×；修正 citation scorer 的截断错误后，wiki 没有 citation-support 优势；两个 judge 在整体 groundedness 上的 rank agreement 仅 0.04。[S2]
- SAG2 确实是 embedding-free 索引 + 查询时 Jev 决策模型 + 词项扩展；论文报告 1.47 s/q、3 次 scoring 请求/问题、$4.33/1000 问；benchmark 是英文多跳 QA，没有中文/metadata 过滤评测。[S3]
- SAG 原始架构确实需要离线 LLM event/entity 抽取与 embedding，不是 embedding-free；SAG2 与 SAG 不是同一个东西。[S4][S5]
- SAG-Wiki 确实是 Rust/Tauri 桌面 + CLI + 本地服务，底层 SAG2；默认 Jev 云端点；中文用 jieba 分词；本地服务默认 loopback + token；最新 v0.3.11 非常年轻。[S6][S7][S12]
- pgvector 确实支持 HNSW/IVFFlat、`sparsevec`（0.7.0+）、iterative scan（0.8.0+），且 approximate index 的过滤在 index scan 之后。[S8]
- BGE-M3 确实是 1024 维、8192 token、100+ 语言、dense/sparse/multi-vector、MIT。[S9]
- SQLite FTS5 确实提供 BM25 与多种 tokenizer；unicode61 不会自动切分无空格中文，需 trigram 或自定义 tokenizer。[S11]

### 6.2 厂商声明

- SAG-Wiki README 的“fast indexing / precise retrieval”“index a million characters in ~200 ms”“no LLM calls for indexing or retrieval”是厂商自述；其中“retrieval 不调模型”与同一 README 的“Jev evidence scoring at query time”存在措辞冲突，不能当作独立事实。[S6]
- SAG-Wiki 的三方对比表（SAG-Wiki / LLM Wiki / Typical vector RAG）是厂商自述；它对 RAG/LLM Wiki 的描述只能作为线索，必须回到各自一手来源核实。[S6]
- SAG README 的“new SOTA for RAG”“best on every benchmark”是厂商/作者声明；论文本身只支持其英文 benchmark 与特定配置，不外推到中文肝功。[S5]
- SAG-Wiki 的“original documents, with passage locations”与引用回原文是产品声明，和其架构文档/代码一致；但“能回原文”不等于“能保证 claim 级逐字 quote + 稳定 chunk_id”，后者仍需适配。[S6][S7]

### 6.3 本仓库产品决定

- 选 RAG、选本地 BGE-M3 + pgvector、选 C0 整段保留、选成人-only、选 40 case × 3、选 12k input / 2.5k output / 20 次/天、选检索 ≤1s / 端到端 p95 ≤30s，都是本仓库的产品决定，不是外部文献的结论。[spec](.scratch/rag-liver/spec.md) [ADR-0018](docs/adr/0018-rag-explanation-layer.md)
- “允许符合某指南标准，但证据完整才允许”是产品边界前移，不是 RAG 或 Wiki 自带的医学能力；无论选哪个方案，都要靠评估集和人工复核。
- “C0 残余风险接受”是产品负责人 2026-11-06 的明确决定；本文不重新裁决 C0，只分析 Wiki 是否能降低它。
- “不做生产灰度、不兼容旧数据”是本项目未上线阶段的取舍；与知识来源选型无关。

### 6.4 待验证假设

- 中文肝功指南上，BM25（jieba/trigram）是否已经足够；BGE-M3 的 dense/sparse 是否真的带来 Recall@10 增益。
- BGE-M3 在 NAS CPU 上的查询 embedding p95；`rag-embed` 冷启动/常驻内存；20 次/天是否会造成排队。
- pgvector 带 `age_group`/`sections` 过滤的 exact scan/HNSW 召回与延迟。
- SAG2 在中文肝功单跳查询上的 Recall@10；Jev 对中文医学 passage 的判断质量；SAG-Wiki 云 Jev 的 p95 与真实价格。
- SAG2 能否在不 fork 的情况下实现 `age_group`/`panels`/`sections` 硬过滤；如果不能，fork 成本是否值得。
- Wiki 在本语料上的 query tokens 是否也接近 21×；如果改用确定性单页读取，是否还能保留综合优势。
- Wiki 的 claim 级引文能否做到 100% 逐字回链；Cochran 的自动 scorer 只有 65–69% 与人工一致，说明“自动判定引文支持”本身不可靠。[S2]

## 7. 最小验证实验（1–2 天）

**前置：** 需要 10–20 份语料的 manifest + 至少 20 个 case 的 golden `chunk_id`（40 case 的前一半即可）。如果语料尚未就绪，先用 3–5 份公开短摘录 + 10 个合成 case 验证管线；结论只能算方向性，不能作为 gate。

| 时间 | 实验 | 样本 | 指标 | 能推翻什么 |
| --- | --- | --- | --- | --- |
| Day 1 上午 | 建 chunk 集与 qrels：按 spec 的切块规则生成 chunk，复用/补齐 20 个 case 的 required-section golden。 | 10–20 份文档，约 1–3k chunk；20 case（5 模式 × 4） | chunk 数、每 section 的 golden 覆盖、儿科 chunk 数 | 如果 golden 无法稳定对齐 chunk_id，先修 chunker/metadata，不进入检索比较。 |
| Day 1 下午 | 检索 bake-off：A=BM25（中文 jieba，英文空白/小写）；B=BGE-M3 dense；C=BGE-M3 dense+sparse RRF；D=BM25 + LLM/Jev rerank top 20（可选）。 | 同一 20 case；固定 `age_group=adult`、`panels=liver`、section 过滤 | Recall@10（按 required section）、MRR@10、零结果率、p50/p95 检索延迟、儿科泄漏 | 如果 A ≥0.90 且 p95 <300ms，当前 BGE-M3/rag-embed 不必要；如果 C <0.90，当前检索设计不达标，需先修语料/切块/metadata，而不是换 Wiki/SAG2。 |
| Day 2 上午 | SAG2/Jev 延迟与过滤：用 SAG-Wiki CLI 或 SAG-Benchmark 代码跑 20 个中文/英文查询；记录请求数、p50/p95、token/费用；尝试给一个 `age_group` 过滤的最小改造。 | 20 query；2–3 份文档；1 个中文 doc | p50/p95、scoring 请求数、每次查询费用、过滤后 Recall@10 | 如果 p95 >1s，或过滤必须 fork 且 fork 后 Recall@10 下降，SAG2 不能作为主检索；如果 p95 <1s 且中文 Recall@10 ≥0.90，SAG2 进入试点候选。 |
| Day 2 下午 | Wiki 成本/引文 spot check：用 3 份文档编译一个小 wiki，问 10 个 case；记录 query tokens、页面读取数、引文能否逐字回链源 chunk。 | 3 份文档、10 query | tokens/query、引文逐字率、人工复核分钟数 | 如果 tokens/query >12k 或引文逐字率 <100%，Wiki 不适合 v1 运行时；如果都通过，仍需评估页面维护与 C0 预审查成本。 |

### 7.1 决策规则

- **RAG 维持：** C（dense+sparse RRF）Recall@10 ≥0.90，且中文分层不低于英文 5 个百分点；检索 p95 在 NAS 上 ≤1s。满足则按 spec 推进。
- **RAG 简化：** A（BM25-only）Recall@10 ≥0.90，p95 <300ms；可以删除 `rag-embed`，改为 Postgres BM25 + 规则校验。这需要新 ADR，因为改变了 ADR-0018 决策 3。
- **RAG 需要修：** B/C 都 <0.90；先查语料覆盖、chunk 切分、metadata 标注、query 构造；不要用 SAG2/Wiki 掩盖检索设计问题。
- **SAG2 试点：** p95 ≤1s、中文 Recall@10 ≥0.90、metadata 过滤不需要 fork 或 fork 成本 <3 天、Jev 出站可接受；四项同时成立。
- **Wiki 试点：** 只有在“长篇综合报告”产品形态下，且 tokens/query ≤12k、引文逐字率 100%、人工复核可承受；否则不做。

### 7.2 实验的证伪边界

- 这个实验只能回答“检索/成本/延迟”是否达标，不能回答“医学内容是否正确”；后者仍靠 40 case 的 entailment + 人工抽检。
- 20 case 的 Recall@10 置信区间很宽；如果结果在 0.85–0.92 之间，不能直接判 gate 通过/失败，需要扩到 40 case。
- SAG2 的 20 query 只能看延迟量级，不能估计 p95；要 p95 至少需要 100+ 次查询或明确的延迟分布假设。
- Wiki 的 3 文档 spot check 只能看 token 量级；不能代表 10–20 份语料的页面维护成本。

## 8. 证据边界

- **没有本地 benchmark。** 本文没有运行 BM25/BGE-M3/SAG2/Wiki 的任何实验；所有性能数字来自论文/文档，不能当作本仓库的验收结果。
- **语料尚未就绪。** spec §6.5 的 manifest 还是模板；40 case 尚未构建；中文肝功指南的 BM25/BGE-M3 表现没有一手数据。
- **Karpathy gist 是模式/观点，不是 peer-reviewed 研究。** 它没有明确的 license/产品/支持承诺，也没有 claim 级 schema；“约 100 来源”是作者经验，不是可复现 benchmark。[S1]
- **Cochran 是单一预注册研究。** 13 问、24 篇论文、一个生成模型（Claude Opus 4.7）、两个 judge；作者自己说 preregistration 不提供统计功效或代表性；wiki 的 query cost 依赖 agent harness（最多 30 turns），不能直接外推到确定性服务。[S2]
- **Cochran 摘要与正文有数字矛盾。** 摘要说“wiki was cheaper to build”，正文 H3a 表与 904.9M vs 1.4M tokens 显示 wiki 构建更贵；本文采用正文/表格，并把该矛盾作为“不要只读摘要”的证据。[S2]
- **SAG2 是英文多跳 benchmark。** MuSiQue/2Wiki/HotpotQA 的 1,000 问子集 + EnterpriseRAG-Bench；没有中文、没有肝功、没有 chunk metadata 过滤、没有 p95；Jev 是专有模型，未独立复现。[S3]
- **SAG/SAG-Wiki 是厂商文档。** 对比表、SOTA、`~200ms` 索引、`no LLM calls for retrieval` 都是厂商自述；SAG-Wiki v0.3.11 非常年轻，API 可能变化；不能把厂商声明当独立验证。[S5][S6][S12]
- **pgvector/BGE-M3/FTS5 文档只定义能力。** 它们不证明本语料的检索质量；BGE-M3 的通用多语言 benchmark 不等于中文医学指南；FTS5 的 tokenizer 文档不等于 SAG2 的中文效果。[S8][S9][S11]
- **产品决定不是证据。** 12k/2.5k、20 次/天、1s/30s、成人-only、C0 接受，都是本仓库的选择；外部文献不能替这些选择背书。
- **访问日期：** 所有 URL 访问于 2026-11-18；版本/价格/模型可能已变化。SAG-Wiki 最新版本以 Releases 页为准（v0.3.11，2026-11-17），不是 README 徽标或任务描述中的版本。

## 9. 参考与复现

### 9.1 本仓库上下文

- 需求与 gate：[`.scratch/rag-liver/spec.md`](.scratch/rag-liver/spec.md) §0、§1、§5–§8、§11、§12、§14
- 已采纳决策：[`docs/adr/0018-rag-explanation-layer.md`](docs/adr/0018-rag-explanation-layer.md)
- 调研规范：[`docs/agents/research.md`](docs/agents/research.md)
- 云端 LLM 出站例外：[`.scratch/llm-analysis/spec.md`](.scratch/llm-analysis/spec.md) §4.11
- 实施约束：`.scratch/rag-liver/issues/` 下 01/02/03/04/08/11/12

### 9.2 外部来源

见 §4 的 [S1]–[S12]。所有外部结论在正文用 [S#] 标注；没有标注的句子是产品决定或本文分析。

### 9.3 复现建议

- 检索 bake-off 的脚本应复用生产 chunker/metadata，不要另写一套切块；否则 Recall@10 不可比。
- BM25 中文分词固定一个版本（jieba 词典/版本），并记录在实验报告；否则中文结果不可复现。
- SAG2 实验固定 Jev 模型版本、B/M/K/L/H 参数、并发数；否则 1.47s 之类的数字不可比。
- Wiki 实验固定 compile 模型、prompt、页面 schema、lint 规则；否则 query tokens 与引文质量不可比。

---

**最终建议一句话：** 维持 RAG 作为生产证据层；用 2 天实验决定“BM25-only 还是 BGE-M3 混合”；把 SAG2 作为 retrieval 备选、把 LLM Wiki 作为离线审阅视图，两者都不进入 v1 运行时。
