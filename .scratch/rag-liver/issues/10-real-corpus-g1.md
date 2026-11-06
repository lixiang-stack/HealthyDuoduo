# 10: 真实语料 ingestion + 真实语料 G1

**What to build:** 用用户提供的 10–20 份语料建真实 manifest 和 versioned index；用同一批 40 case 在真实语料上重跑 G1；报告语料缺口（哪些模式没有足够 chunk）。验证真实 index + G1 报告；不再依赖 fixture。

**Blocked by:** 09 (完整评估集 + G1 gate)；用户提供语料清单（外部依赖）。

**Status:** needs-info

- [ ] manifest 包含用户提供的文档名/链接/年份/语言/age_group/authority；全文只在本地私有目录，不入 git。
- [ ] 真实 index 可重复构建；chunk metadata 完整；pediatric 内容标 `pediatric` 且不进入 v1 检索。
- [ ] 40 case 在真实语料上跑 G1；报告覆盖每类模式的 chunk 数与召回。
- [ ] 语料缺口有明确清单（哪类模式缺来源、缺哪些 analyte/section）。
- [ ] 未通过 G1 不得进入 G2 或上线。
