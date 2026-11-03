# 04: 分析事实层与结果页

**What to build:** 家长从项目详情页点「分析这个项目」，看到本次纳入的报告与病历范围、偏离参考范围的指标、与本人历史比较、相关病历诊断原文、未纳入项及原因。首次点击时展示数据去向确认；结果页底部固定展示来源声明。

**Blocked by:** 01 成员模型扩展, 03 病历作为叙述型报告

**Status:** resolved

- [x] `POST /analyses` 接受 `member_id`、`panel`、`entry`、`indicator_name?`、`report_sha?`、`window_months`
- [x] 窗口默认最近 12 个月、每趋势键最多 6 个有效点，两个边界同时生效
- [x] 有效点判定五项条件全覆盖：同成员、同趋势键 + 同 canonical unit、`unit_status = verified`、`flag ≠ not_tested` 且有日期、通过信任门控
- [x] 默认排除项不被引用为结论依据
- [x] 稀疏时扩展至 24 个月或最多 4 个点并标注「历史数据较少」
- [x] 项目详情页与单指标入口都能进入；单指标自动扩展至所属 Panel 并提示范围
- [x] 事实层输出纳入范围、偏离项、历史比较、病历诊断与处理原文、未纳入原因
- [x] 首次点击展示数据去向确认，之后不再展示
- [x] 结果页底部展示来源声明
- [x] 快照记录成员、Panel、窗口、有效点集合、病历引用与规则版本
- [x] App 结果页只渲染事实层时不报错
- [x] HTTP 测试覆盖窗口、门控、排除、稀疏扩展；RNTL 覆盖结果页与首次确认

## Comments

**实现（2026-11-03，分支 `llm-analysis-growth-facts`）**

- 纯领域层 `internal/analysis`：窗口锚点为分析时刻当日；默认 12 个月 / 每趋势键 6 点，稀疏时（默认 12 个月窗口内有效点 < 2）扩展到 24 个月 / 最多 4 点并标记 `sparse`；用户选 6 / 24 个月时尊重用户边界，不再自动扩展。
- 有效点门控：同成员、同趋势键（code > loinc > name）+ 同 canonical unit、`unit_status = verified`、`flag ≠ not_tested` 且有日期、信任门控（`confirmed` / `corrected` 或非低置信）；`unknown` 保留为历史点但不进入偏离项，单列「无法与参考范围比较」。
- 未纳入项带原因码（跨成员 / 项目不符 / failed / 叙述型 / 无日期 / 未检测 / 未核实单位 / 未核对低置信 / 缺值 / 跨单位 / 超窗口 / 超点数 / 无指标名），不参与任何结论依据。
- 病历：窗口内全部 + 最近一次窗口外病历（标「早于分析窗口」）；事实层只引用诊断 / 处理原文，切不出段落时用 `note` 整页兜底；主诉 / 现病史不进入事实层。
- 迁移 `0015_analyses.sql`：事实层与快照 JSONB 永久保留；成员删除级联；快照记录成员数据指纹 `data_version`，`GET /analyses/{id}` 与当前指纹不同即标 `stale=true`。`POST /analyses` 返回 `analysis_id` / `status` / `layers.facts` / `snapshot` / `sources` / `quota` / `stale`；`GET /analyses/{id}` 回读同一份快照。
- App：项目详情页「分析这个项目」与每项「分析这个指标」入口；点击后先弹时间范围选择（6 / 12 / 24，默认 12），首次再弹数据去向确认（AsyncStorage 只记一次）；趋势层与病历详情页也分别提供单指标 / 病历分析入口；结果页只渲染事实层，底部固定来源声明；单指标入口展示「已从指标 X 扩展至项目 Y」。
- 配额当前为占位（`used_today=0` / `limit_today=20` / `explanation_available=false`）；云端脱敏、LLM 客户端与真实配额属工单 07，框架层属 05/06，解释层属 08。
- 存储 / 迁移冒烟走 `bash scripts/test-db.sh`。
