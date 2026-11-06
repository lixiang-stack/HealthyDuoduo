# 02: 本地 BGE-M3 embedding 服务

**What to build:** compose 能启动独立的 Python `rag-embed` 服务，加载 BGE-M3（CPU、模型缓存），`/health` 在模型就绪后返回 ready，`/embed` 对中文和英文输入返回 1024 维 dense + sparse。超时和加载失败有明确错误，且不影响未接入时的现有产品行为。

**Blocked by:** None (can start immediately).

**Status:** ready-for-agent

- [ ] compose 服务、Dockerfile、healthcheck、模型缓存 volume 就绪；冷启动与加载中有区分。
- [ ] `/health` 在模型加载完成后返回 ready；加载失败返回非 ready 并带原因。
- [ ] `/embed` 返回 1024 维 dense + sparse；中英文本各一条集成测试。
- [ ] 请求超时 10s；失败错误可诊断，不阻塞 `hdd serve`。
- [ ] 记录首次加载时间与常驻内存，供 NAS 容量评估（spec §12.3）。
- [ ] 不提交模型权重；镜像/volume 说明清楚。
