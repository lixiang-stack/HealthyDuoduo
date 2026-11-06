# 06: HTTP 分析接入 RAG（contract + snapshot + cache + kill switch）

**What to build:** `POST /analyses` 对成人肝功使用 RAG 引擎，返回新的解释结构（`sections[].claims` + `sources`）；快照带 rag 元数据；缓存 key 含 corpus / index / embedding / retriever / prompt / model；`HDD_RAG_ENABLED=false` 时只输出事实层 + 框架层；`rag_no_results` / `rag_unavailable` / `rag_validation_rejected` 等原因可见；OpenAPI、Go 类型、App 类型、fixtures、契约测试同步。验证 curl 成功、kill switch、无结果三种 case。

**Blocked by:** 04 (RAG 生成 + claim 校验 CLI), 05 (解释层引擎 seam + RAG 元数据落点).

**Status:** ready-for-agent

- [ ] 成人肝功分析走 RAG；儿童、非肝功、clinical_note 不走 RAG（儿童固定提示，其他保持现状）。
- [ ] 新解释结构与来源元数据（source_id / title / org / year / authority / language / url / quote）通过 OpenAPI 与 Go 类型暴露。
- [ ] 快照写入 rag 元数据；缓存 key 包含版本；缓存命中不消耗配额、不重新 embedding。
- [ ] kill switch：关闭时 explanation 省略，reason=`rag_disabled`，facts / framework 正常。
- [ ] 失败原因 `rag_index_unavailable` / `rag_unavailable` / `rag_no_results` / `rag_validation_rejected` 映射到响应与事件；analysis_events CHECK 与 metrics 镜像同步。
- [ ] 现有 `/analyses` 契约测试、fixtures、契约镜像文档更新；`go test ./...` 通过。
- [ ] 不改变事实层 / 框架层结果。
