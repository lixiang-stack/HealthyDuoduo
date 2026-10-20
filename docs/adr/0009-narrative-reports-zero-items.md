# 0009 · 叙述型报告以段落为主体：零指标项不变量与 status 派生

**状态：** 采纳（2026-10-20）。

影像（超声）单的主体是检查所见 / 超声提示等叙述段落，不是检验单式的指标项。本期把影像从指标项模型彻底移出：识别端不再抽取影像指标项，`Report.items` 为空、`Report.sections` 为正文；status 只由正文段落派生。这是对 ADR-0008（段落作为可选值集合）的语义收紧。

## 背景

ADR-0008 让影像段落进入报告详情，但识别端仍按检验单模型产出 14 个产科测量项（本期前 18 条落库 items，全部 `ref_range=null`、`flag=unknown`）；这是模型错配，不是识别失败。继续保留会污染趋势、flag 聚合与「待核对」徽标，也会把影像分类绑死在产科测量项上，无法覆盖腹部 / 甲状腺 / 乳腺等其他影像子类。

## 决策

- **分类按正文锚**：影像词典删除全部测量项；`findings` / `impression` 锚（超声描述 / 检查所见 / 超声所见 / 超声提示 / 检查提示）是影像的资格信号，`exam_name` 锚（检查项目 / 部位 / 内容）只用于段落抽取，不参与分类。达标 = 指标项命中 ≥ `min_matched_items` 或正文锚命中 ≥ `min_section_hits`；排序键为 `(达标, 正文锚命中数, 指标项命中数, 标题区命中)`。标题关键词保留兜底。
- **识别端输出**：影像报告 `items=[]`，只保留 `sections`；`findings` 或 `impression` 至少一段 → `success`，两者都没有 → `failed`。日期未印、低置信不再触发 partial，影像不进入核对流程。
- **契约不变量**：零指标项 + 非空 `sections` 合法；零指标项 + 空 `sections` 仍必须 `failed`。Pydantic、Go pipeline 与存储层三处一致，绕过识别端直灌也不能写入违规形状。
- **失败原因**：影像零指标项且无正文段落时固定为 `no report body recognized`；其他类别继续 `no metric items recognized`。
- **存量清理**：迁移 0008 直接删除 `reports.report_type='影像'` 的 `report_items`（`correction_pairs` 级联删除）。影像 items 是错配数据，无迁移价值；reparse 全量替换保证幂等。
- **验证门禁**：默认 `verify` 对 `reports.sections IS NOT NULL` 的报告硬性要求零 items，且 status 与 findings/impression 是否存在一致；golden 在零 items 下逐字段比对。

## Considered Options

- **保留产科测量项**：能继续提供测量值，但只覆盖产科，且与影像单的叙述语义冲突；趋势与徽标持续被错配数据污染。否。
- **把测量项迁移为段落内结构化字段**：需要新的子模型、核对与展示口径，且当前无腹部 / 甲状腺等其他影像子类的真实样本驱动。本期不做。
- **只停新产出、不清理存量**：旧影像单会继续出现在趋势与 flag 聚合里；一次性删除 + reparse 幂等更彻底。采纳。
- **status 继续由日期 / 低置信派生**：零 items 后必然落入 failed 不变量；直接按正文段落派生，语义与展示一致。采纳。

## Consequences

- 影像报告的 `flag_counts` 全为 0，不再产生趋势点或「未知 / 待核对」徽标；App 仍按报告类别隐藏指标项区块。
- 非影像类别、核对 / 修正流程与既有 golden / API 形状不变；只有 `us_01/02/03` 的报告 golden 与影像 HTTP fixture 变为零 items + 新 status。
- 新增迁移 0008（一次性删除）。Down 不恢复数据：reparse 可重建报告，但新规则下影像 items 仍为空。
- 未来新增叙述型报告类别时复用 `min_section_hits` + `sections` + 零 items 不变量；verify 以 `sections IS NOT NULL` 判定，不硬编码类别。
- 与 ADR-0008 的关系：段落仍是可选值集合；本期只是把「影像以段落为主体」从展示约定升级为契约与数据规则。
