# 02: M0 契约冻结

**What to build:** 需求文档 v1.3 定稿的 HTTP API 面（12 端点）与其全部数据形状一次冻结：OpenAPI 契约、数据库迁移、Go 契约镜像、HTTP fixtures 四者一致——此后所有实现票的 API 测试都有锚，识别端契约与存量识别 golden 零触碰。

**Blocked by:** 01（`hdd serve` 骨架落真栈）。

**Status:** ready-for-agent

- [ ] OpenAPI 契约含全部 12 端点与形状：成员（显示名 + 是否孩子）；报告（归属成员可空、fail_reason）；指标项（核对状态 + 稳定外部句柄 + ocr_lines 行号与框坐标）；修正对表形状（before→after 全集 + 原始行文 + OCR 行号）；trend 单位门控参数
- [ ] 修正字段集契约化：核对修正只收 value / unit / name / ref_range，`flag` 由服务端按值×参考范围重算，契约不提供手改入口
- [ ] 新迁移落库：成员表；报告表加成员归属（可空，存量 CLI 时代报告无主）与失败原因列；指标项表加核对状态；append-only 修正对表
- [ ] 状态派生语义进契约：未核对低置信计数为响应派生值（不落列）且只统计 partial 报告；报告状态派生单调（partial→success 单向，success/failed 恒定）
- [ ] 对已含核对决策的报告重跑识别 → 409（显式覆盖才放行）写入契约与 fixtures
- [ ] Go 契约镜像与 OpenAPI 一致性测试绿（沿用识别端双向互证模式）；HTTP fixtures golden 建立并对账测试绿
- [ ] 识别端契约文件与存量识别 golden 逐字节不动（本票不改识别行为）
