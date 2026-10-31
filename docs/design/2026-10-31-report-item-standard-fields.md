# ReportItem 标准层字段贯穿（2026-10-31）

## 为什么做

医学检验标准化（`.scratch/medical-standardization/`）的后续票需要把标准名/码、规范单位、原文快照与单位核验状态带到数据库、API 和 App。现有 `ReportItem` 只有 `name/value/unit/ref_range/method/flag/low_confidence/raw_text`，标准层字段没有载体；如果先在识别端逐 panel 输出，再补契约与存储，会造成多次破坏性迁移。

本票只做**契约与透传管道**，不引入单位换算或新的临床判定；识别端实际产出由票 03+ 逐 panel 启用。因此旧报告与旧 golden 必须逐字节不变。

## 做什么

`ReportItem` 新增 10 个可选字段：

| 字段 | 含义 |
|---|---|
| `code` / `loinc` | WS/T 886 码 / LOINC 码；未迁移词典为 null |
| `unit_display` | 人类可读展示单位；UI 优先，缺失回退 `unit` |
| `unit_status` | `verified` / `unverified`；未产出为 null |
| `raw_value` / `raw_unit` / `raw_ref_range` | 单据原文快照（换算/归一前） |
| `ref_kind` | 参考范围类型；本期未分类为 null |
| `result_kind` | 结果类型（quantitative / ratio / …）；未迁移词典为 null |
| `result_basis` | D-二聚体 FEU/DDU；其他为 null |

字段为 null 时序列化省略，确保旧报告 / 旧 golden 不变。`value` / `unit` / `ref_range` 行为不变。

## 怎么实施

1. **Python 契约**：`contract.py` 增加字段与 `Literal` 枚举；`_OPTIONAL_NULL_FIELDS` 统一省略 null。
2. **共享 schema**：`schemas/report.schema.json` 与 `schemas/openapi.yaml` 同步属性；Python/Go/TS 契约测试锁死字段集合。
3. **Go**：`contract.ReportItem`、HTTP `ReportItem`、`toAPIItem` 透传；字段用指针 + `omitempty`。
4. **存储**：迁移 `0012` 新增 10 个可空 TEXT 列；`WriteReport`、详情、核对读取与 scan 全链路覆盖。核对修正纸面单位时清空 `unit_display` / `unit_status`，避免继续展示旧单位。
5. **App**：`displayItemUnit()` 优先 `unit_display`、回退 `unit`（详情/核对卡/原图页）；核对卡展示原文单位与「单位未验证/已核验」。
6. **数据**：本期识别端不产出新字段，旧行全为 NULL；`review_status=pending` 与 `unit_status` 的翻转由票 03–15 完成。

## 怎么验收

- Python `ReportItem` 字段集合 == `report.schema.json` properties；null 字段省略、有值字段往返。
- Go struct JSON 字段集合 == schema properties；HTTP fixture 与 Go 镜像双向回转。
- 迁移 `0001..0012` 冒烟：10 个新列均为可空 text，旧行读取不报错。
- 真库 storage 冒烟：`WriteReport` 写入并读回，核对修正后 `unit_display` / `unit_status` 清空。
- App RNTL：`unit_display` 优先、缺失回退 `unit`；原文单位与单位状态可见。
- `pytest`、`go test ./...`、mobile `npm test` / `tsc --noEmit`、`check-docs.py` 全绿；golden 文件逐字节不变。
