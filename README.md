# HealthyDuoduo

家庭健康档案助手：家长把医院检查单拍照归档，系统 OCR 成结构化指标，支持时间线、核对、按项目浏览与趋势。仅面向家庭局域网自用，不提供诊断或治疗建议。

## 架构

```text
apps/mobile (Expo / React Native)
        │ HTTP /api/v1
        ▼
cmd/hdd + internal/ (Go 数据服务) ──► PostgreSQL
        │                         └─► SeaweedFS (S3)
        │ 内部 HTTP
        ▼
recognizer/ (Python FastAPI + RapidOCR)
        │
schemas/ (OpenAPI / 报告契约) ──► App 类型与 Python/Go 契约
```

- 识别端无状态、不落库；原图、OCR 结果与报告由 Go 侧留存。
- 双语架构与存储选型见 `docs/adr/0002-python-recognizer-go-data-service.md`、`docs/adr/0003-object-storage-seaweedfs.md`。
- 局域网先行、不做鉴权是明确边界，见 `docs/adr/0006-lan-first-no-auth.md`。

## 仓库结构

```text
cmd/hdd/          Go 数据服务 CLI / HTTP API
internal/         Go 领域、存储、管道与 HTTP 层
recognizer/       Python OCR + 规则后处理
apps/mobile/      Expo / React Native App
schemas/          OpenAPI 与识别契约
samples/          脱敏图像与 golden 样本
deploy/           本地 compose 栈与镜像
docs/             ADR、设计文档、agent 约定
scripts/          文档检查与 Git 策略脚本
.scratch/         本地 spec / issue tracker
```

## 快速开始

前置：Go 1.27、Python 3.13 + uv、Node 22、Docker（本地栈与 E2E 需要）。

数据服务：

```bash
go test ./...
bash scripts/test-db.sh   # 真库测试（需 Docker；每个测试独立数据库）
```

识别服务：

```bash
cd recognizer
uv run pytest
```

App：

```bash
cd apps/mobile
npm ci
npm test
npm run typecheck
npm run lint
npm start          # Expo 开发服务器
```

OpenAPI 变更后重新生成 App 类型：

```bash
cd apps/mobile
npm run codegen
```

文档检查：

```bash
python3 scripts/check-docs.py
```

本地栈：

```bash
docker compose -p deploy -f deploy/docker-compose.yml up -d
```

NAS 局域网常驻部署（端口 19080、数据盘、USB 备份与验收）见 `deploy/NAS.md`。

解释层是可选能力：同时设置 `HDD_LLM_BASE_URL` 与 `HDD_LLM_MODEL` 才启用，未设置时只输出事实层与框架层。`internal/llm` 只依赖 OpenAI 兼容的 `/chat/completions`，不绑定具体模型提供商。

本机 Ollama 示例：

```bash
# Ollama 默认上下文为 4096；先调到大于 HDD_LLM_INPUT_TOKENS + HDD_LLM_OUTPUT_TOKENS（默认 9500）。
export OLLAMA_CONTEXT_LENGTH=16384
ollama serve

export HDD_LLM_BASE_URL=http://localhost:11434/v1
export HDD_LLM_MODEL=qwen3.5:4b
export HDD_LLM_REASONING_EFFORT=none   # 留空则省略；qwen 等 thinking 模型用 none 关闭 reasoning
go run ./cmd/hdd serve
```

compose 内的 `hdd` 要把 `localhost` 换成 `host.docker.internal`：

```bash
HDD_LLM_BASE_URL=http://host.docker.internal:11434/v1 \
HDD_LLM_MODEL=qwen3.5:4b \
docker compose -p deploy -f deploy/docker-compose.yml up -d hdd
```

需要鉴权的云端 OpenAI 兼容服务再额外提供 key；推荐文件型 secret，不建议把 key 写进命令行或 `environment:`：

```bash
mkdir -p ~/.config/healthyduoduo && chmod 700 ~/.config/healthyduoduo
read -s KEY
printf '%s' "$KEY" > ~/.config/healthyduoduo/llm_api_key
chmod 600 ~/.config/healthyduoduo/llm_api_key
```

基础 compose 会把 `~/.config/healthyduoduo` 只读挂到 `/run/secrets`，并设置 `HDD_LLM_API_KEY_FILE=/run/secrets/llm_api_key`。文件缺失或为空表示没有 key（本地模型不需要），不影响启动；是否启用解释层只取决于 endpoint 与模型名。设置 `HDD_LLM_SECRET_DIR` 可换挂载目录（目录内文件名仍须为 `llm_api_key`）。

App 屏幕测试随 `npm test` 运行；合并 UI 改动前按 `apps/mobile/ANDROID.md` 做真机 smoke。

## 文档索引

- `CONTEXT.md`：统一语言与领域模型
- `docs/adr/`：架构与技术决策
- `docs/design/`：设计与实施计划
- `docs/agents/e2e.md`：Pipeline E2E
- `docs/agents/documentation.md`：文档约定
- `docs/agents/complexity.md`：增量复杂度信号
- `docs/agents/prototype-driven-development.md`：前端方案选型与原型规范
- `deploy/NAS.md`：NAS 局域网部署、USB 备份与验收
- `.scratch/<feature>/`：本地 spec 与 ticket

## 开发约定

- 每个需求在 `.worktree/<需求名>` 开发，完成后走 PR；不直接改 `main`。
- 提交前测试必须全绿；本地 hooks 会检查分支 / worktree 与 commit 前缀。
- push 由用户明确授权；具体 agent 工作流见 `AGENTS.md`。

## 范围与安全边界

- 当前只部署在家庭局域网，HTTP API 与图像代理不做鉴权；公网暴露前必须重新评审。
- 产品用于归档、核对与查看检查报告，不提供诊断、治疗或复查建议。
