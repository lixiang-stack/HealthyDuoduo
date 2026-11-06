# 05: 解释层引擎 seam + RAG 元数据落点

**What to build:** 分析 HTTP 路径可以通过同一个接口切换旧 YAML 引擎和新 RAG 引擎；引擎返回 explanation + 可选检索元数据；handler 把元数据写入快照；`entry_versions` 从生产路径移除（旧数据可删）。验证现有 YAML 分析测试仍绿、fake 引擎注入能跑通、快照能存 RAG 元数据。

**Blocked by:** None (can start immediately).

**Status:** ready-for-agent

- [ ] 定义 explanation engine 接口（Explain + 可选 metadata），handler 不再直接依赖具体 YAML 类型。
- [ ] 旧 YAML 引擎仍可通过接口运行；现有分析测试与契约 fixture 通过（移除 entry_versions 后同步更新）。
- [ ] fake 引擎注入测试：返回 explanation + rag metadata，handler 能落库并回读。
- [ ] 快照增加 rag 元数据位置（corpus / index / embedding / retriever / prompt / model / retrieved_chunks），未生成时省略。
- [ ] 不改变现有 API 的 facts / framework 形状。
- [ ] 回读已保存分析时，rag 元数据与 explanation 的解码路径有测试。
