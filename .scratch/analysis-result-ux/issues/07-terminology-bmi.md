# 07: 术语、单位与 BMI（P1/P2）

**What to build:** 消除页面内的术语、缩写和单位噪声，并把肝功页里同权重的三张 BMI 卡合并为一张“体格发育”卡。

**Blocked by:** 01（判读区块分组）

**Status:** ready-for-human

- [x] `ULN` 首次出现改为“正常上限”，如“2.09 倍正常上限”
- [x] 同一指标首次用“中文全称（缩写）”，后文统一缩写
- [x] `unit` 为 `1` 或空时不拼单位，避免“1.41 1”
- [x] 统一使用 `unit_display`，避免 `umol/L` 与 `μmol/L` 混用
- [x] `track_items` 映射到展示名，不只显示 ALT/AST 缩写
- [x] BMI 三张卡合并为一张“体格发育”卡（BMI、百分位、状态三行）
- [x] 体格卡默认折叠；是否从肝功页移出按产品决策确认
- [x] 新增展示名/单位单测，覆盖 ALT/AST/ALP/TBIL

**依据：** `.scratch/analysis-result-ux/spec.md` §2.3；`research.md` A5、B4、待确认决策 10。

## Comments

**2026-11-04 — implemented**

- `humanizeClinicalText` 把框架层固定文案与解释连接语里的 `T-0` / `T-Nm` 转为「最近一次 / 约 N 个月前」，`×ULN` 转为「倍正常上限」，页面不再出现内部时间标记或裸 `ULN`。
- 指标按页面出现顺序统一：同一缩写首次「中文全称（缩写）」，后文只显示缩写；前白蛋白（PA）与白蛋白（ALB）分开映射。
- `unit=1` 不拼单位；`unit_display` 优先，并补 `umol/L → μmol/L` 等常见别名。
- `track_items` 映射为展示名；方案 A 后结果页不再展示 BMI，成员页未做「体格发育」合并卡。

**2026-11-05 方案 A 复核**

2026-11-05 方案 A 复核：结果页不展示 BMI / 体格发育卡，BMI 回成员页；ULN、单位、指标展示名等术语改动保留。
