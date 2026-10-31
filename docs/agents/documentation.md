# Documentation conventions

## When to write

- 详细设计、契约变更、架构决策、跨团队方案：写文档。
- 若 `.scratch/<feature>/spec.md` 与 `issues/` 已构成该需求的权威设计记录：更新它们，不在 `docs/design/` 另写复述。`docs/design/` 只用于跨 feature、长期契约或需要独立存档的决策。
- 单文件 bug fix：commit message + 代码注释即可，不单独生成文档。
- 总结 / 摘要：只讲结论与重点，不展开细节。

## Where and naming

- 统一归档在 `docs/` 下，按类型分目录：ADR 在 `docs/adr/`，agent 约定在 `docs/agents/`。
- 英文命名，格式 `{YYYY-MM-DD}-{feature-name}.md`。
- 设计 / 方案类文档必答四问：**为什么做、做什么、怎么实施、怎么验收**。
- 链接使用相对路径；提交前运行 `python3 scripts/check-docs.py`。
