# 07: 云端脱敏、LLM 客户端与配额

**What to build:** 分析请求能把脱敏后的证据包发给云端服务；超时、失败或配额用尽时自动降级到前两层；家长能看到今日剩余次数。

**Blocked by:** 04 分析事实层与结果页

**Status:** resolved

- [x] 发送内容不含：姓名、出生日期、精确报告日期、机构、科室、医生、原图、OCR 原文（诊断与处理段落除外）、报告编号、主诉、现病史
- [x] 发送内容含：指标规范名、数值、单位、参考范围、flag、相对时间、年龄段、诊断与处理原文、条目原文与 `source_ids`
- [x] 单次 input 上限 8k tokens，超限先裁最旧证据点、不裁条目原文
- [x] 单次 output 上限 1.5k tokens，超限截断并标注
- [x] 每天每家庭 20 次，数值走配置
- [x] 单次超时 30 秒
- [x] 失败重试 1 次后省略解释层并标注原因，前两层不受影响
- [x] 配额用尽时按钮仍可点击，解释层位置提示「今日额度已用完，事实与判读仍可用」
- [x] 响应中返回 `used_today`、`limit_today`、`explanation_available`
- [x] 单测覆盖脱敏字段清单、配额边界、超时降级

## Comments

**实现（2026-11-03，分支 `llm-analysis-growth-facts`）**

- `internal/analysis/evidence.go`：脱敏证据包只含指标规范名、数值、单位、参考范围、flag、相对时间、年龄段、诊断/处理原文、条目原文与 `source_ids`；姓名、生日、精确日期、报告 id、图像哈希、主诉/现病史、整页 OCR 兜底不发送。证据 id 用请求内临时 `e1...` 编号。
- `internal/analysis/explain.go` + `internal/llm/client.go`：OpenAI 兼容 chat/completions；单次超时映射 `ErrLLMTimeout`，失败重试 1 次；`bridge_text` 校验条目 / 证据 id / 数字 / 禁用词 / 200 字。失败只省略解释层并在 `quota.explanation_reason` 标注原因。
- 配额：`analysis_llm_calls` 原子占用每日 20 次（配置 `HDD_ANALYSIS_DAILY_LIMIT`）；input 8k / output 1.5k / timeout 30s 均可配置；超 input 先裁最旧指标点、再裁最旧病历，不裁条目原文。
- 响应 `quota` 返回 `used_today` / `limit_today` / `explanation_available`；App 配额用尽时在解释层位置显示「今日额度已用完，事实与判读仍可用」，分析入口与反馈按钮不禁用。
- 与票 08 的边界：生产侧暂用 `analysis.NoEntries` 空匹配器，不加载 YAML 条目，因此不会真正发起云端调用；脱敏、配额、重试、超时与校验已用 fake 条目 / fake 客户端覆盖。票 08 接入条目后替换匹配器即可。
