# 分析框架层、云端脱敏与观测（票 05/06/07/09）

**日期：** 2026-11-03
**分支：** `llm-analysis-growth-facts`
**范围：** 肝功 / 血常规 / BMI 框架层；云端脱敏证据包、LLM 客户端与每日配额；分析反馈与降级事件。解释层条目库与四段式医学原文（票 08）不在本次范围。

---

## 为什么做

事实层只陈述数据，家长仍然不知道 R 值分型、转氨酶分级、贫血形态与 BMI 位置。这些结论可以由代码稳定算出，不应交给模型自由发挥；模型只应接收脱敏后的证据包，把条目与证据连接起来。配额与降级事件则保证云端不可用时产品仍可用，并且每周能看出是内容错还是链路坏。

## 做什么

### 框架层（票 05/06）

- **肝功**：R 值分型、转氨酶升高分级、AST/ALT 比值。R 值只在 ALT 与 ALP 同时高于 ULN 时分型，边界固定为 `R ≥ 5` 肝细胞型、`R ≤ 2` 胆汁淤积型、`2 < R < 5` 混合型；分级固定为 `< 2×ULN` 临界、`2–5×` 轻度、`5–15×` 中度、`> 15×` 重度，未超过 ULN 不输出分级。ULN 从单据参考范围解析，缺失时不输出对应标签。
- **血常规**：MCV/MCH/MCHC + RDW 组合输出形态学标签；WHO 2024 Hb 切点按年龄段（6–23 月、24–59 月、5–11 岁、12–14 岁、15–65 岁按性别）输出，年龄按报告日期计算，并同时注明单据参考范围。切点标签不是诊断。
- **BMI**：用最近一次生长记录的测量时点身高体重计算；6–18 岁按 WS/T 586—2018 半岁年龄别界值判定，5–19 岁同时输出 WHO 2007 z 值与百分位。缺出生日期或性别时不输出百分位；`source = manual` 只输出 BMI 数值（该来源已从产品移除，领域层保留守卫）。
- 标签只由代码生成，每个标签附固定解释与来源；App 在结果页渲染「判读标签」。

### 云端脱敏与配额（票 07）

- `internal/analysis/evidence.go` 生成脱敏证据包：指标规范名、数值、单位、参考范围、flag、相对时间、年龄段、诊断/处理原文、条目原文与 `source_ids`；不发送姓名、生日、精确报告日期、报告 id、图像哈希、主诉/现病史、整页 OCR 兜底。
- 证据 id 使用本次请求内的 `e1...` 临时编号，模型输出后映射回真实证据 id，避免把报告 sha 片段发给云端。
- 单次 input 8k tokens、output 1.5k tokens、每日 20 次、单次超时 30 秒均为配置项（`HDD_LLM_*` / `HDD_ANALYSIS_DAILY_LIMIT`）。超限先裁最旧指标点，再裁最旧病历，条目原文不裁。
- 失败重试 1 次；仍失败则省略解释层并在 `quota.explanation_reason` 标注原因，事实层与框架层照常返回。配额用尽时 App 在解释层位置显示「今日额度已用完，事实与判读仍可用」，并展示今日剩余解读次数。
- 07 的 App 解释层只渲染 `bridge_text`、来源与截断标注；四段式医学原文与「查看依据」由票 08 追加。
- `bridge_text` 校验：条目必须存在、证据 id 必须存在、数字必须能在引用证据中找到、禁用推断词（可能/提示/考虑/建议/应该）、长度 ≤ 200 字。校验失败按降级处理。

**与票 08 的边界：** 本次生产侧使用 `analysis.NoEntries` 空匹配器，不加载 YAML 条目，因此不会真正发起云端调用；LLM 客户端、脱敏、配额、重试与校验已通过注入 fake 条目 / fake 客户端测试。票 08 接入条目后，把匹配器换成真实实现并打开 `RecordEntryMiss` 即可。

### 反馈与观测（票 09）

- `POST /api/v1/analyses/{analysis_id}/feedback`：记录 `analysis_id`、可选 `entry_id`、理由与时间；只写入，不自动降级条目。
- `analysis_events` 记录 LLM 超时、JSON 不合法、`bridge_text` 校验失败、配额用尽、信任门控全未通过、条目未命中、其他 LLM 错误与 input 超限。
- `GET /api/v1/analyses/metrics?from&to`：返回分析数、反馈数、反馈率、降级事件数、降级率与按 kind 的分布，供每周查看。
- App 结果页提供「这不对」按钮与理由弹层；提交后显示「已记录，谢谢反馈」。

## 怎么实施

| 层 | 文件 |
| --- | --- |
| 领域规则 | `internal/analysis/framework.go`、`framework_reference.go`、`who_bmi_data.go` |
| 脱敏 / LLM / 配额 | `internal/analysis/evidence.go`、`explain.go`、`internal/llm/client.go` |
| 存储 | `internal/storage/store_analysis.go`、`0016_analysis_observability.sql` |
| HTTP | `cmd/hdd/serve_analysis.go`、`serve_analysis_feedback.go`、`schemas/openapi.yaml` |
| App | `apps/mobile/src/analysis/{components,framework,FeedbackModal}.tsx`、`app/analysis/[analysis_id].tsx` |
| 契约镜像 | `internal/httpapi/analysis.go`、`schemas/testdata/http/analysis.*.json`、`apps/mobile/src/api/schema.d.ts` |

## 怎么验收

- `go test ./... && go vet ./...`
- `bash scripts/test-db.sh`（0016 迁移、配额原子占用、反馈/事件级联）
- `cd apps/mobile && npm run typecheck && npm test && npm run lint -- --max-warnings=0`
- `python3 scripts/check-docs.py`
- 单测覆盖：R 值 1.9/2.0/4.9/5.0、分级各档边界、ULN 缺失、Hb 年龄段切点、形态学缺 Hb/MCV、BMI WS/T 界值上下、缺生日/性别、manual 来源、脱敏字段清单、配额边界、超时重试、`bridge_text` 各失败路径。

## 真机验收（2026-11-03）

- 设备：Android 16 真机（USB + Expo Go SDK 58）。
- 隔离栈：`bash scripts/android-acceptance.sh --seed`；另上传 `samples/cbc_04.png` 并修正 MCV 参考范围 / 单位、血红蛋白值，构造 Hb 108 g/L + MCV 79.7 fL 的形态学用例。
- 已验证：肝功 R 值 / 分级 / AST/ALT；CBC 形态学与 WHO 2024 Hb 切点；BMI 与 WHO 2007 z / 百分位；今日剩余解读次数；配额用尽提示且反馈按钮仍可点；「这不对」提交后 `analysis_feedback` 入库；`GET /analyses/metrics` 返回 5 次分析 / 1 条反馈 / 2 条 trust_gate_failed。
- 真机发现并修复：框架标签的长 `value` 会挤压标签标题，改为标题与值纵向排列（`apps/mobile/src/analysis/components.tsx`）。
- 验收后已恢复 App Base URL 为共享栈 `http://127.0.0.1:8080/api/v1`，移除隔离栈、reverse 与一次性数据库。
