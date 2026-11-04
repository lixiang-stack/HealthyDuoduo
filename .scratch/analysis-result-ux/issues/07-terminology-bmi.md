# 07: 术语、单位与 BMI（P1/P2）

**What to build:** 消除页面内的术语、缩写和单位噪声，并把肝功页里同权重的三张 BMI 卡合并为一张“体格发育”卡。

**Blocked by:** 01（判读区块分组）

**Status:** needs-triage

- [ ] `ULN` 首次出现改为“正常上限”，如“2.09 倍正常上限”
- [ ] 同一指标首次用“中文全称（缩写）”，后文统一缩写
- [ ] `unit` 为 `1` 或空时不拼单位，避免“1.41 1”
- [ ] 统一使用 `unit_display`，避免 `umol/L` 与 `μmol/L` 混用
- [ ] `track_items` 映射到展示名，不只显示 ALT/AST 缩写
- [ ] BMI 三张卡合并为一张“体格发育”卡（BMI、百分位、状态三行）
- [ ] 体格卡默认折叠；是否从肝功页移出按产品决策确认
- [ ] 新增展示名/单位单测，覆盖 ALT/AST/ALP/TBIL

**依据：** `.scratch/analysis-result-ux/spec.md` §2.3；`research.md` A5、B4、待确认决策 10。
