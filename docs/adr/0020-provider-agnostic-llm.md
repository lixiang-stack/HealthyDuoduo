# 0020 · 解释层模型改为可配置的 OpenAI 兼容端点

**状态：** 采纳（2026-11-25）；已实现。

## 背景

- 解释层此前通过 `internal/llm` 调用 DeepSeek：`HDD_LLM_BASE_URL` / `HDD_LLM_MODEL` 带 DeepSeek 默认值，且只有配置 API key 才创建 `analysis.Explainer`。
- 家庭环境已通过 Ollama 部署 `qwen3.5:4b`。本地端点不需要 API key；继续把 key 当作启用开关，会把「端点是否需要鉴权」误当成「是否配置了模型」。
- `internal/llm` 只需要 OpenAI 兼容的 `/chat/completions`。Ollama 提供 `/v1/chat/completions`，不需要新增 provider 分支或适配器。
- ADR-0018 决策 4 写定「聊天模型仍走云端 DeepSeek」，本 ADR 修订该点。

## 决策

1. `internal/llm` 保持单一、provider-neutral 的 OpenAI 兼容客户端；代码中不出现 provider 枚举、分支或默认值。
2. 提供商选择完全在装配层（`cmd/hdd` 环境变量）：`HDD_LLM_BASE_URL` 与 `HDD_LLM_MODEL` 都非空才启用解释层；API key 可选，为空时不发送 `Authorization`。
3. `HDD_LLM_REASONING_EFFORT` 作为通用请求字段透传，默认 `none`；显式留空则不发送，兼容不识别该字段的端点。`qwen3.5:4b` 是 thinking 模型，关闭 reasoning 避免输出 token 被思考过程耗尽。
4. 不再保留 DeepSeek 的代码默认值。本地 Ollama 与云端兼容服务走同一客户端，切换只改环境变量。
5. 文件型 key 仍受支持：存在则发送 Bearer；缺失或为空视为无鉴权，不再关闭解释层。

## 后果

- 隐私边界变化：默认部署不再把脱敏证据包发往公网；代价是解释层依赖本机 / NAS 的 Ollama 可用性、显存与推理速度。
- Ollama 默认上下文为 4096，小于默认输入 + 输出上限（9500）。部署时需设 `OLLAMA_CONTEXT_LENGTH` 或下调 `HDD_LLM_INPUT_TOKENS` / `HDD_LLM_OUTPUT_TOKENS`；客户端不发送 provider-specific 的 `num_ctx`。
- 没装 Ollama 的环境默认关闭解释层；配置了但端点不可达时，解释层按 `llm_error` 降级，事实层与框架层不受影响。
- 云端服务仍可使用同一客户端，但需要手动配置 URL、模型与 key；没有 provider-specific fallback。
- 升级影响：仅配置 key 的旧部署在升级后解释层关闭；需补 `HDD_LLM_BASE_URL` 与 `HDD_LLM_MODEL`。
- 验收：`internal/llm` 单测覆盖可选 `Authorization` 与 `reasoning_effort`；`cmd/hdd` 覆盖 keyless 启用与缺 endpoint / model 时关闭；真实 Ollama `qwen3.5:4b` 返回合法 JSON。
- App 首次解读确认文案不再声称「云端服务」，改为 provider-neutral 的「解读模型」；已同意的本地记录继续有效。

## 备选方案

- **Provider 接口 + DeepSeek / Ollama 两个适配器**：Ollama 的 OpenAI 兼容端点已覆盖协议，新增适配器没有收益。否。
- **保留 DeepSeek 默认值，仅新增 Ollama 配置**：仍把 provider 默认值写进代码，且 key 仍被当作启用开关。否。
- **始终发送 `reasoning_effort=none`**：对不识别该字段的 provider 不友好；改为可配置透传。否。

## 关联

- 原决策：`docs/adr/0018-rag-explanation-layer.md`
- 模型端口：`internal/analysis/explain.go`
- 客户端：`internal/llm/client.go`
- 配置装配：`cmd/hdd/config.go`
