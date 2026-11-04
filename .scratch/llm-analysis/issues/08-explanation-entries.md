# 08: 解释层与首个条目

**What to build:** 肝功某个组合命中知识条目时，结果页出现四段式解释；医学原文逐字来自条目，模型只写连接语；模型改写原文、引用不存在的数字或使用禁用推断词时，该段被拒绝。

**Blocked by:** 05 框架层：肝功, 07 云端脱敏、LLM 客户端与配额

**Status:** resolved

- [x] 条目 YAML 从 `internal/analysis/knowledge/` 加载，只有 `review.decision = accepted` 可引用
- [x] 精确匹配优先；多命中按 `priority` 排序取前 2；部分匹配不输出
- [x] 模型输出 `sections`，每段带 `entry_id`、`evidence_ids`、`bridge_text`
- [x] `bridge_text` 中出现的数字必须能在 `evidence_ids` 找到
- [x] `bridge_text` 不含禁用推断词：可能 / 提示 / 考虑 / 建议 / 应该
- [x] `bridge_text` 长度 ≤ 200 字
- [x] `entry_id` 不存在或未 accepted 时拒绝该段
- [x] 医学原文由服务端从条目直接拼接，模型无法修改
- [x] 无命中时不显示空状态；必要时附「本次没有可引用的解读条目，可以带着上面的数值问医生」
- [x] 首个肝功条目入库并能命中
- [x] App 渲染四段式与「查看依据」
- [x] 单测覆盖校验失败的各条路径：数字不在证据、禁用词、超长、条目未审核

## Comments

**实现（2026-11-04，分支 `llm-analysis-framework-cloud`）**

- `internal/analysis/knowledge/` 内嵌 YAML，`knowledge.go` 负责加载与精确匹配：`trigger.panel` + `any_flag_high` + `all_normal` 全部满足才算命中，只有 `accepted` 条目参与；按 `priority` 升序取前 2，部分匹配不输出。启动时 YAML 不合法直接报错，不静默跳过条目。
- 首个条目 `liver-hepatocellular-mild`：ALT / AST 任一高、ALP 与总胆红素正常；为此在识别端肝功词典与 `internal/contract` 增加 `bilirubin_total` 分析物，并重生成 8 份含总胆红素的报告 golden。存量栈需 `reparse` 后旧报告才会带上该 analyte。
- `AnalysisExplanationSection` 新增 `explanation` / `check_directions` / `general_advice` / `track_items`；这些字段由 `validateSection` 从命中条目逐字填入，模型只提供 `entry_id` / `evidence_ids` / `bridge_text`。OpenAPI、HTTP fixture、App codegen 与 RNTL 同步。
- App `ExplanationCard` 渲染四段式；「查看依据」展开显示条目 `source_ids` 的标准与说明。
- 生产 matcher 接线后启用 `entry_miss` 事件；命中条目 id → version 写入快照 `entry_versions`，用于后续「依据已更新」判断。
- 单测覆盖：精确命中、部分匹配（ALP 高 / 缺总胆红素 / 无转氨酶升高）、Panel 不符、rejected 条目、priority 前 2、YAML 非法、服务端原文拼接与 `entry_versions`；HTTP 用例验证真实 matcher 命中后解释层与快照一起返回。

**命名复核（2026-11-04）**

- `analyte` 统一为 LOINC component 风格全称的小写 snake_case；缩写只用于 UI 与框架文案，不再进入 key。映射：`alt → alanine_aminotransferase`、`ast → aspartate_aminotransferase`、`alp → alkaline_phosphatase`、`mcv → mean_corpuscular_volume`、`mch → mean_corpuscular_hemoglobin`、`mchc → mean_corpuscular_hemoglobin_concentration`、`rdw_cv → erythrocyte_distribution_width_cv`；`hemoglobin` / `bilirubin_total` 不变。
- 迁移 `0018_analyte_full_names.sql` 将存量 `report_items.analyte` 就地改名，并补回填总胆红素；识别端词典、goldens 与评估 fixture 同步重生成。
