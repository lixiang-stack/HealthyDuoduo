package analysis

import _ "embed"

// 解释层 system prompt 是模型看到的精确 payload;文本文件便于 review / diff,
// go:embed 保证运行时没有外部文件依赖。改 prompt 必须同时跑解释层测试。
//
//go:embed prompts/explanation_system.txt
var explanationSystemPrompt string

// explanationPromptVersion 随快照记录,便于区分「规则变了」和「prompt 变了」。
const explanationPromptVersion = "v3"
