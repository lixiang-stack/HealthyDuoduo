# 12: 生产切换 + YAML 清理 + 文档同步

**What to build:** 生产 RAG-only；kill switch 验证；删除存量分析与缓存；生产 YAML 加载路径移除，legacy YAML 只留在评估目录；README / CONTEXT / OpenAPI / 契约镜像同步；切换后监控 2 周。验证生产分析、kill switch、文档检查、测试全绿。

**Blocked by:** 07 (App 结果页方案 B), 10 (真实语料 G1), 11 (G2 离线影子).

**Status:** ready-for-human

- [ ] 切换前删除存量 analyses / analysis_events / analysis_llm_calls / analysis_explanation_cache；不写回填或兼容。
- [ ] 生产代码不再加载 YAML；legacy YAML 只在评估目录；相关生产测试与类型清理。
- [ ] `HDD_RAG_ENABLED=false` 验证：只输出事实层 + 框架层，App 显示对应文案。
- [ ] 生产 smoke：成人肝功分析返回 claims + 来源；儿童/非肝功保持降级；配额/缓存正常。
- [ ] README / CONTEXT / OpenAPI / fixtures / 契约镜像文档同步；文档检查通过。
- [ ] 全量测试：`go test ./...`、`go vet ./...`、App typecheck/test/lint、真库 storage 测试通过。
- [ ] 切换后 2 周监控反馈率/降级率/延迟；异常时用 kill switch 回退前两层。
