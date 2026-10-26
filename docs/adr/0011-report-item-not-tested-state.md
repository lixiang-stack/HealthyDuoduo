# 0011 · 未检测作为一等指标状态（not_tested + not_tested_count）

**状态：** 采纳（2026-10-25，需求拷问会定稿）。

结果与参考区间都是**裸短横 `-`** 的指标，表示**本次检查没有做该项**。它不是正常、不是未知、也不是识别失败。采用新增一等 `flag = "not_tested"` + 报告级派生 `not_tested_count`：报告仍为 `success`，逐项灰色「未检测」、报告卡「未检测 N 项」信息徽标；不进入核对、待处理、趋势与异常计数。

## 背景

- RapidOCR 对孤立短横线经常不输出；上一版靠原图像素补 `-` 后把两侧裸短横判成 `normal`。但 `normal` 的语义是“可比较且落在参考范围内”，未检测没有结果，语义错误。
- 现有 `unknown` 表示“本应能比较但抽取/比较失败”，并用 `flag_reason` 区分原因；未检测是已知的“没有做”，不应借用 unknown。
- App 的核对页只列低置信项；未检测项通常高置信，家长在核对页看不到，只能在详情逐项修正。产品需要明确表达“本次没做”，而不是制造无法完成的待办。
- 部分定性报告里结果 `-` + 参考 `阴性(-)`/`阴性` 是已做且阴性；只有**两侧都是裸短横**才是未检测，不能按单侧 `-` 一刀切。

## Considered Options

- **复用 `unknown + flag_reason = "not_tested"`**：不改 flag CHECK、改动小；但 flag 字段仍说谎（已知未检测却叫 unknown），报告卡/核对/趋势都要按 reason 特判，`CONTEXT.md` 的「未知」定义被撑大。否。
- **新增独立 `result_status = performed / not_tested` 字段**：概念上把“是否做了”和“比较结果”分开；但 flag 仍必须填 normal/unknown 之类的谎值，字段组合更复杂，查询/计数/展示仍需两个字段。否。
- **隐藏未检测项**：结构化报告项数变少，家长看不到“这次没做哪几项”。否。
- **把未检测并入 `partial` / 待核对**：产生家长无法完成的待办，且把“识别完成”和“检查完整”混为一谈。否。
- **新增 `flag = not_tested` + 独立 `not_tested_count`**：语义清晰；`FlagCounts` 保持异常/留意口径，报告卡信息徽标独立；代价是 schema/DB/Go/Python/App 全链路和迁移。采纳。

## Consequences

- `ReportItem.flag` 枚举新增 `not_tested`；`flag_reason` 为空。
- `ReportHeader`（Summary/Detail）新增派生 `not_tested_count`；`FlagCounts` 不变；DB 无新列。
- 迁移 0010 扩展 `report_items.flag` CHECK；旧行不受影响。
- 识别规则：`value` 与 `ref_range` 经 `_policy_text` 后都为 `-` → 未检测；单侧短横仍按阴性/阳性规则。
- 报告状态：未检测不触发 partial、不计入低置信多数与 `unreviewed_low_confidence_count`；报告为 success + 未检测徽标。
- 核对：不进核对页；详情保留次级「修正」逃生口（改回真实值后 flag 重算）。
- 趋势：SQL 排除 not_tested，不产生点。
- 只对新上传/重新识别生效；不做历史回填。09-04 尿常规需单独重识别。
- `/reparse` 只有 OCR、无原图，不能恢复短横线；旧 App 需重新生成类型/重新构建才能显示未检测。
