# HealthyDuoduo

家庭健康与育儿记录助手：家长拍照归档检查单，系统 OCR 成结构化指标，支持时间线、核对、项目、趋势和可选解读；同时记录喂养、尿布、睡眠、辅食等育儿事件，提供今天、统计和本机提醒。当前面向家庭局域网使用，不提供诊断或治疗建议。

- **健康档案**：检查单上传、报告详情、核对、按项目浏览、趋势、解读。
- **育儿**：喂养、尿布、睡眠、辅食记录，今天、统计和提醒。
- **客户端**：同一套代码支持 Android / iOS；可以用安装包，也可以用 Expo 开发客户端。

## 快速开始

### 1. 前置条件

| 依赖 | 要求 | 用途 |
| --- | --- | --- |
| Docker + Docker Compose v2 | 当前稳定版 | 运行 PostgreSQL、SeaweedFS、识别服务和数据服务。 |
| Android / iOS 客户端 | APK、IPA 或 Expo 开发客户端 | 日常使用；安装包构建不在本文展开。 |

只部署后端时，Docker 是唯一必需的依赖。App 使用安装包或开发客户端即可；日常使用不需要 Expo CLI 或开发环境。

### 2. 启动后端

在仓库根执行：

```bash
docker compose -p deploy -f deploy/docker-compose.yml up -d --build
```

检查状态和健康端点：

```bash
docker compose -p deploy -f deploy/docker-compose.yml ps

curl -fsS http://127.0.0.1:8080/api/v1/health
# {"ok":true,"version":"0.1.0"}

curl -fsS http://127.0.0.1:8000/healthz
# {"status":"ok"}
```

首次启动时 PostgreSQL 先就绪；SeaweedFS 首启需要约 15–20 秒，`hdd` 在依赖未就绪时会退出并由 compose 自动重启。两个端点都返回 200 即可使用。

### 3. 连接 App

在 App 的「设置」页填写后端地址：

```text
http://<部署主机地址>:8080/api/v1
```

保存后应通过健康检查。首次使用：

1. 新建一个成员；孩子成员会出现在育儿页。
2. 在健康档案上传检查单，或在育儿页记录喂养、尿布、睡眠、辅食。
3. 打开报告核对指标，之后查看项目、趋势和解读。

## 产品架构

![HealthyDuoduo 产品架构](docs/assets/architecture.svg)

App 只访问 `hdd` 的 `/api/v1`。`hdd` 负责成员、报告、育儿事件、分析编排、数据库迁移和图像代理；识别服务无状态、不落库；PostgreSQL 保存业务数据，SeaweedFS 保存原图；配置 OpenAI 兼容端点与模型后才启用解释层，API key 可选。

## 核心流程

![HealthyDuoduo 核心使用流程](docs/assets/flows.svg)

- **检查单归档**：App 上传 → `hdd` 幂等编排 → 识别服务 OCR + 规则后处理 → 报告、OCR 落库，原图写入对象存储 → App 读取报告、核对、看趋势。
- **育儿记录**：App 记录喂养、尿布、睡眠、辅食 → `hdd` 写入 `care_events` → 今天和统计读取事件；提醒由本机根据最新喂养记录派生。
- **报告解读**：`hdd` 先本地生成事实层和框架层并保存分析快照；配置 OpenAI 兼容端点与模型时再调用解释层，未配置时结果页仍可展示本地部分。

## 使用

### 健康档案

- 上传检查单后，报告会出现在时间线；`partial` 或低置信项可进入核对，`failed` 报告会显示失败原因，可删除后重新上传。
- 核对时确认解析结果或修正名称、数值、单位、参考范围；修正后服务端重新计算异常标记。
- 按项目浏览用于看一组相关指标，趋势用于看同一指标跨报告的变化。
- 「解读」先展示本地事实层和框架层；配置 OpenAI 兼容端点与模型后追加解释层。

### 育儿

- 育儿页只消费当前孩子；单孩子家庭自动选中，多孩子在本机切换并记住。
- 记录类型为喂养、尿布、睡眠、辅食；睡眠支持结束后补记。
- 今天页展示时间线和提醒入口，统计页展示最近七天的记录汇总。
- 提醒设置保存在本机，根据最新喂养记录重排；不写入服务端。

## 配置

### 端口

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `HDD_SERVE_PORT` | `8080` | 数据服务宿主端口，App 使用这个端口。 |
| `RECOGNIZER_PORT` | `8000` | 识别服务宿主端口。 |
| `POSTGRES_PORT` | `5432` | PostgreSQL 宿主端口。 |
| `S3_API_PORT` | `8333` | SeaweedFS S3 API 宿主端口。 |
| `HDD_LLM_SECRET_DIR` | `$HOME/.config/healthyduoduo` | 可选 LLM key 目录，只读挂载到容器 `/run/secrets`。 |

### 服务配置

下表是 `hdd` 自身的默认值；本机 compose 已把数据库、识别服务和 S3 地址改成容器服务名（`postgres` / `recognizer` / `seaweedfs`），通常不需要改。

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `HDD_DATABASE_URL` | `postgres://healthyduoduo:healthyduoduo@localhost:5432/healthyduoduo?sslmode=disable` | PostgreSQL DSN。 |
| `HDD_RECOGNIZER_URL` | `http://localhost:8000` | 识别服务地址。 |
| `HDD_S3_ENDPOINT` | `http://localhost:8333` | S3 端点，必须带 `http://` 或 `https://`。 |
| `HDD_S3_BUCKET` | `raw` | 原图 bucket。 |
| `HDD_FLAG_POLICY` | `recognizer/recognizer/flag_policy.yaml` | 定性比较策略；镜像内为 `/app/flag_policy.yaml`。 |
| `HDD_LLM_BASE_URL` | 空 | OpenAI 兼容端点；与 `HDD_LLM_MODEL` 同时非空才启用解释层。 |
| `HDD_LLM_MODEL` | 空 | 模型名；与 `HDD_LLM_BASE_URL` 同时配置才启用解释层。 |
| `HDD_LLM_REASONING_EFFORT` | `none` | 透传 `reasoning_effort`；显式留空则不发送，兼容不识别该字段的端点。 |
| `HDD_LLM_TIMEOUT_SECONDS` | `30` | 单次模型请求超时。 |
| `HDD_LLM_INPUT_TOKENS` | `8000` | 输入 token 上限。 |
| `HDD_LLM_OUTPUT_TOKENS` | `1500` | 输出 token 上限。 |
| `HDD_LLM_API_KEY_FILE` | 空 | 可选 key 文件，本机 compose 设为 `/run/secrets/llm_api_key`；缺失或为空表示无鉴权。 |
| `HDD_LLM_API_KEY` | 空 | 可选 key；文件型优先，本机 compose 不直接透传该变量。 |
| `HDD_ANALYSIS_DAILY_LIMIT` | `20` | 每日解释层调用上限；事实层和框架层不受影响。 |

本机 compose 会把宿主环境中的 `HDD_LLM_BASE_URL`、`HDD_LLM_MODEL`、`HDD_LLM_REASONING_EFFORT` 传进容器；修改后执行 `up -d hdd` 生效。

### 解释层（可选）

解释层使用 OpenAI 兼容的 `/chat/completions`，不绑定具体模型提供商。只有同时设置 `HDD_LLM_BASE_URL` 和 `HDD_LLM_MODEL` 才启用；API key 可选。

本机 Ollama 示例（模型跑在宿主机）：

```bash
# Ollama 默认上下文为 4096；调到大于 HDD_LLM_INPUT_TOKENS + HDD_LLM_OUTPUT_TOKENS（默认 9500）
OLLAMA_CONTEXT_LENGTH=16384 ollama serve

HDD_LLM_BASE_URL=http://host.docker.internal:11434/v1 \
HDD_LLM_MODEL=qwen3.5:4b \
HDD_LLM_REASONING_EFFORT=none \
docker compose -p deploy -f deploy/docker-compose.yml up -d hdd
```

`host.docker.internal` 适用于 Docker Desktop；其他环境请换成容器可达的宿主机地址。`HDD_LLM_REASONING_EFFORT` 默认 `none`；不识别该字段的端点，把上面命令中的 `HDD_LLM_REASONING_EFFORT=none` 改成 `HDD_LLM_REASONING_EFFORT=` 即可。

需要鉴权的云端 OpenAI 兼容服务，先配置 endpoint / model，再写入 key 并重启 `hdd`：

```bash
HDD_LLM_BASE_URL=https://<endpoint>/v1 \
HDD_LLM_MODEL=<model> \
docker compose -p deploy -f deploy/docker-compose.yml up -d hdd

mkdir -p ~/.config/healthyduoduo && chmod 700 ~/.config/healthyduoduo
read -s KEY
printf '%s' "$KEY" > ~/.config/healthyduoduo/llm_api_key
chmod 600 ~/.config/healthyduoduo/llm_api_key

docker compose -p deploy -f deploy/docker-compose.yml restart hdd
```

`HDD_LLM_BASE_URL` / `HDD_LLM_MODEL` 需要在每次 `up` 时提供；建议写入启动脚本或 shell 环境，否则重建容器后会关闭解释层。升级后只配置 key 不会启用解释层，必须同时配置 endpoint 和 model。

未配置 endpoint / model 时解释层关闭，事实层和框架层照常输出。key 文件缺失或为空表示无鉴权，不影响本地模型；key 文件读取失败（权限、目录错误等）会让 `hdd` 启动失败。`HDD_LLM_SECRET_DIR` 可以改用其他目录，文件名仍须为 `llm_api_key`；改目录本身后需要用 `up -d` 重建容器。

### 识别规则

识别服务的 OCR 参数和词典在 `recognizer/recognizer/` 下：`rapidocr.yaml`、`*_dict.yaml`、`unit_conversions.yaml`、`ocr_forms.yaml`、`flag_policy.yaml`。修改后需要重建镜像：

```bash
docker compose -p deploy -f deploy/docker-compose.yml up -d --build recognizer hdd
```

只改 `flag_policy.yaml` 时也要同时重建 `hdd`，否则 Go 侧仍使用旧策略。

## 运维

### 状态与日志

```bash
docker compose -p deploy -f deploy/docker-compose.yml ps
docker compose -p deploy -f deploy/docker-compose.yml logs -f --tail=200 hdd recognizer
docker compose -p deploy -f deploy/docker-compose.yml restart hdd
```

### 健康检查

```bash
curl -fsS http://127.0.0.1:8080/api/v1/health
curl -fsS http://127.0.0.1:8000/healthz
docker compose -p deploy -f deploy/docker-compose.yml exec postgres \
  pg_isready -U healthyduoduo -d healthyduoduo
```

### 数据与重置

业务数据在 Docker 卷 `deploy_pgdata`，原图在 `deploy_seaweeddata`。升级或重置前先备份这两个卷。

清空本机验证数据：

```bash
deploy/reset-data.sh        # 二次确认
deploy/reset-data.sh --yes  # 跳过确认
```

脚本会删除 PostgreSQL 全部业务数据和 `raw` bucket 的原始图像，不可恢复；只作用于 compose 项目 `deploy`。如果改过 `HDD_SERVE_PORT` / `S3_API_PORT`，先在同一个 shell 导出对应值。重置后 App 可能仍保存旧的成员 ID，请在成员列表重新点选成员。

### 升级

```bash
docker compose -p deploy -f deploy/docker-compose.yml up -d --build
curl -fsS http://127.0.0.1:8080/api/v1/health
```

`hdd` 启动时会自动执行内嵌的 goose 迁移；升级后先看 `hdd` 日志中的迁移结果。数据库迁移通常只能向前，异常时按数据卷备份恢复。

## 常见问题

先看状态、日志和健康端点：

```bash
docker compose -p deploy -f deploy/docker-compose.yml ps
docker compose -p deploy -f deploy/docker-compose.yml logs --tail=200 hdd recognizer
curl -fsS http://127.0.0.1:8080/api/v1/health
```

| 现象 | 检查与处理 |
| --- | --- |
| 端口被占用 | `lsof -nP -iTCP:8080 -iTCP:8000 -iTCP:5432 -iTCP:8333 -sTCP:LISTEN`；改端口后同步 App 的 Base URL。 |
| `hdd` 反复重启 | 看 `hdd` / `postgres` / `seaweedfs` 日志；SeaweedFS 首启需等 15–20 秒，或稍后重启 `hdd`。 |
| PostgreSQL 认证失败 | 检查 `HDD_DATABASE_URL` 与卷中已有数据库密码是否一致；密码建议只用字母数字。 |
| S3 / bucket 报错 | 看 `seaweedfs` 日志；确认 `HDD_S3_ENDPOINT` 带 scheme，bucket `raw` 已创建。 |
| 识别服务不可用 | `curl http://127.0.0.1:8000/healthz`；看 `recognizer` 日志。支持 jpg、jpeg、png、bmp、webp、tif、tiff。 |
| App 连不上后端 | 确认 Base URL 含 `/api/v1`、端口正确；用浏览器打开 health 验证。 |
| OCR 不准或待核对项多 | 在报告详情看原文和低置信标记；优先补识别词典、单位别名，再改解析规则。 |
| 解读不可用 | 未配置 endpoint / model 时只展示事实层和框架层；配置后重启 `hdd`。endpoint 不可达时看 `hdd` 日志；key 可选，每日额度默认 20 次。 |
| 重置后 App 看不到成员 | App 保存的成员 ID 已失效；在成员列表重新选择成员。 |

### 上传接口状态码

| 状态 | 含义 | 处理 |
| --- | --- | --- |
| 201 | 新报告已归档 | 正常；报告状态可能是 `success`、`partial` 或 `failed`。 |
| 200 | 同一图像已存在 | 正常，不重复识别；需要重跑时使用 `override=true`。 |
| 409 | 已有核对决策 | 确认要重置归档后再传 `override=true`。 |
| 400 | 字段缺失、`member_id` 非法、图像为空或格式错误 | 检查 `image` / `member_id` 和图像格式。 |
| 404 | 成员或报告不存在 | 先选对成员；报告 ID 是 64 位 sha256。 |
| 500 | 识别、数据库或对象存储失败 | 依次看 `hdd`、`recognizer`、`postgres`、`seaweedfs` 日志。 |

报告返回 `failed` 不一定是 HTTP 错误：图像里没有识别出指标项时仍会归档并返回 201，App 会显示失败原因。

## 脚本与工具

| 场景 | 命令 |
| --- | --- |
| 清空本机数据 | `deploy/reset-data.sh` |
| 查看报告与原始识别 | `docker compose -p deploy -f deploy/docker-compose.yml exec hdd hdd show <sha256>` |
| 存储 / 迁移测试 | `bash scripts/test-db.sh` |
| OCR / 规则测试 | `cd recognizer && uv run pytest` |
| 数据链路 E2E | `uv run --project recognizer python tools/e2e.py verify` |

测试命令另需 Go、Python + uv；只部署和日常使用不需要这些工具链。E2E 使用本机 `deploy` 栈；改动共享数据前先 `snapshot`，`clean` / `restore` 会要求人工确认。
