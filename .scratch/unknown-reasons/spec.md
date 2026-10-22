# Spec：unknown 可解释化与可比较化

Status: resolved

日期：2026-10-22；分支：`unknown-reasons`（基于 `review-clarity-v2`）。

**依据：** `.scratch/review-clarity-v2/audit.md`（unknown 根因审计）· `CONTEXT.md` · `ADR-0005`（flag 服务端重算）· `schemas/testdata/flag.vectors.json` · 现有 `internal/pipeline/{flag.go,review.go}`、`recognizer/recognizer/{contract.py,postprocess.py}`、`apps/mobile/src/{reports/status.ts,review/items.ts,review/components.tsx}`。

## 为什么做

当前 `flag=unknown` 把四类事实混在一起：抽取失败、定性/单侧/区间等语义不可比、纸面无参考范围、未核实。App 卡片只显示“未知”，家长既不知道发生了什么，也不知道该做什么。审计显示非 OCR 残量可以分层修复：提取器配对、标量范围、单侧/区间、定性结果、无参考范围/未核实。

## 结论（已定）

1. **flag 只加 `abnormal`**：`normal/high/low/abnormal/unknown`。high/low 只用于有方向的数值异常；abnormal 只用于定性无方向异常；normal 可用于定性正常；unknown 只保留“本应能比较但失败/暂不支持”。
2. **新增 item 级 `flag_reason`**：解释 unknown/异常原因，服务端计算；时间线 `flag_counts` 仍按 outcome 统计，不按 reason 统计。
3. **共享 comparison policy**：Go/Python 共读一份 `flag_policy.yaml`；抽取词典仍只在识别端，Go 不感知 OCR 别名/单位。
4. **比较语义**：
   - 标量参考 `0`/`0.0` → `<=0`；其它标量按 unsupported。
   - 单侧值 `<x` / `>x` 按可能区间与参考范围比较；跨边界 → unknown + reason=indeterminate。
   - 区间值 `a-b` 按区间包含比较；跨边界 → unknown + reason=indeterminate。
   - 定性值按 policy：normal/abnormal/informational；`value` 与 `ref` 都是 negative（含 `-`/`-`）→ normal。
5. **卡片**：显示 reason 文案 + 一个主操作；`isBackfillItem` 只对可补录 reason 为真。
6. **不做**：参考范围知识库（F）、OCR 升级（G）、`indeterminate`/`not_applicable` 一等 flag、通用表格解析器重写。

## 做什么

### 1. Comparison policy

- 新增 `flag_policy.yaml`：normalizers（negative/weak_positive/positive）、profiles、items（仅例外/无 ref 项）。
- Python `postprocess.py` 与 Go `internal/pipeline` 共读；Go 通过 embed 或启动加载，需有 parity test。
- 定性词典由现有真实数据生成，条目覆盖审计里的 27 个定性 unknown；血型/RhD/ANA 核型/ HBeAb/HBcAb 归 informational，HBsAb 阳性 normal/阴性 informational，维C 阳性 informational。
- `-`/`-`、`阴性`/`-` → normal；`+`/`-` → abnormal。

### 2. Flag/reason 语义

- `internal/pipeline/flag.go` + `recognizer/postprocess.py` 实现同一 `Compare(value, ref) -> (flag, reason)`；保留 `ComputeFlag` 兼容包装。
- `flag_reason` 枚举：`ocr_miss`、`ocr_merge`、`extractor_pairing`、`no_reference`、`qualitative_unsupported`、`unverified`、`narrative`、`informational`、`indeterminate`、`scalar_ref`、`other`。
- 抽取端设置 reason；修正后服务端重算 flag+reason。
- `FlagCounts` 增加 `abnormal`；unknown 仍计 flag=unknown。

### 3. Contract / DB / UI

- `schemas/report.schema.json`、`schemas/openapi.yaml`：flag 加 abnormal，item 加 `flag_reason`，FlagCounts 加 abnormal。
- 新 migration：`report_items.flag_reason` + flag CHECK 加 abnormal。
- Go storage/types/httpapi 读写 reason；`review.go` 重算。
- Mobile：`flagLabel` 加 abnormal，`FLAG_COLORS` 加 abnormal，`flagBadges/countFlagCounts` 加 abnormal；`ReviewItemCard` 显示 reason 文案 + 动作；`isBackfillItem` reason-aware。
- 生成 `apps/mobile/src/api/schema.d.ts`。

### 4. 提取器（A，按复杂度取舍）

- 只针对审计样例：`cbc_04` 行合并 7 项 + `bd3d` 值粘连、`tft_03` 范围未归位、`98c10b`/`b208` 值未归位、`ua_05` 微白蛋白；若规则开始膨胀则停在“高置信自动修 + 人工补录”，不重写通用表格引擎。
- 实际落地：`98c10b`/`b208` 的“数值% 熔 unit”与 `-`/`-` 进 value/ref 已修；`cbc_04`/`tft_03`/`ua_05` 需要列级重排/跨行配对，按“避免过度复杂”停在本迭代之外，保留 `flag_reason` + 补录/修正路径。
- 用 golden 语料锁定。

## 验收

- `schemas/testdata/flag.vectors.json` 覆盖：`-`/`-` normal、`+`/`-` abnormal、标量 0、单侧值、区间值、定性 normal/abnormal/informational、未知 reason；Go/Python 双侧通过。
- `samples/expected/report/*.json` 重新生成后，审计中的非 OCR unknown 按预期下降；至少 `cbc_04`、`tft_03`、`98c10b`、`b208`、`ua_05` 有行为测试。
- `go test ./...`、`cd recognizer && uv run pytest`、`npm run typecheck`、`npm test`、`npx expo lint` 全绿。
- App：`cbc_04` 的 abnormal/normal 与 unknown reason 卡片可见；`-`/`-` 显示正常；信息项不显示“未知”。
- 生产代码改动均可追溯到本 spec；F/G 不做；不新增依赖。

## 分阶段

1. policy + Compare + vectors → 验证：Go/Python flag tests。
2. contract/DB/API/UI → 验证：go test、typecheck、unit。
3. golden 重生成 + 提取器（按复杂度）→ 验证：pytest、golden diff、审计脚本复核。
4. App E2E → 验证：`npm run e2e`。
