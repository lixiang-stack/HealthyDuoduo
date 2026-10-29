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

App E2E（会自起专用栈，结束拆除）：

```bash
cd apps/mobile
npm run e2e
```

## 文档索引

- `CONTEXT.md`：统一语言与领域模型
- `docs/adr/`：架构与技术决策
- `docs/design/`：设计与实施计划
- `docs/agents/e2e.md`：Pipeline E2E
- `docs/agents/app-e2e.md`：App E2E
- `docs/agents/documentation.md`：文档约定
- `docs/agents/prototype-driven-development.md`：前端方案选型与原型规范
- `.scratch/<feature>/`：本地 spec 与 ticket

## 开发约定

- 每个需求在 `.worktree/<需求名>` 开发，完成后走 PR；不直接改 `main`。
- 提交前测试必须全绿；本地 hooks 会检查分支 / worktree 与 commit 前缀。
- push 由用户明确授权；具体 agent 工作流见 `AGENTS.md`。

## 范围与安全边界

- 当前只部署在家庭局域网，HTTP API 与图像代理不做鉴权；公网暴露前必须重新评审。
- 产品用于归档、核对与查看检查报告，不提供诊断、治疗或复查建议。
