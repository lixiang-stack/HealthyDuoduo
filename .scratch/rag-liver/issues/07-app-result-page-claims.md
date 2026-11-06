# 07: App 结果页方案 B（内联引用编号 + 来源列表）

**What to build:** 结果页按已选方案 B 展示四段 claims + 内联引用编号；点编号滚动/高亮到来源；来源项显示标题、机构、年份、权威等级、quote、打开原文；「指南标准」卡片显示缺少证据；删除硬编码来源文案，改用 API 元数据；RNTL 覆盖关键状态。验证截图 + RNTL；原型见 `.scratch/rag-liver/prototype/`。

**Blocked by:** 06 (HTTP 分析接入 RAG).

**Status:** ready-for-agent

- [ ] 四段 claims 渲染；每条 claim 有引用编号，来源列表按 source_id 去重。
- [ ] 点击编号定位/高亮来源；来源多时能打开来源弹层；44px 触控目标、可访问标签。
- [ ] 「指南标准」卡片显示 complete / missing；缺证据时文案明确「无法判断」。
- [ ] 解释不可用原因（rag_disabled / no_results / unavailable / validation_rejected）有对应文案；儿童固定提示。
- [ ] 移除硬编码来源文案；未知来源有回退显示；可见来源不再隐藏 RAG 来源。
- [ ] OpenAPI codegen 更新；App typecheck / test / lint 通过。
- [ ] 与原型 B 的视觉/交互一致；真机 smoke 按 App 验收清单执行。
