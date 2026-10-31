# 12: 趋势按 code + canonical unit 连接

**What to build:** 把趋势连接从“指标名 + 原单位字符串”改为“标准 code（LOINC 兜底）+ canonical unit”。同一分析物跨报告、跨面板可连接；未验证单位点默认排除；跨单位换算只在 verified 时参与。

**Blocked by:** 03–11（各 panel 标准输出）

**Status:** ready-for-agent

- [ ] 趋势键优先 `code`，缺失用 `loinc`，再回退 `name`；不再只靠名称字符串。
- [ ] 同一 canonical unit 的 verified 点可连接；不同单位只有在换算 verified 后才连接。
- [ ] `unit_status=unverified` 的点默认排除，`--all` 可见并标注原因。
- [ ] App 趋势显示 `unit_display`，不显示 raw OCR 单位。
- [ ] Go/API/App 测试覆盖跨面板、跨单位、未验证排除三类场景。
