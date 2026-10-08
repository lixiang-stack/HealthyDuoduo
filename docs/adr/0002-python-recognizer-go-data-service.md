# 0002 · 双语架构：Python 识别服务 + Go 数据工程服务

MVP 起，系统由两个服务组成。**识别服务**（Python，uv + Python 3.13）打包为 Docker 镜像、通过 HTTP 提供能力：OCR（RapidOCR ≥3.9.0 默认配置，onnxruntime CPU）+ 血常规规则后处理；端点为图 → 结构化报告（`POST /report`）、OCR JSON → 报告（`POST /reparse`）、调试用 `POST /ocr`，镜像内保留调试 CLI。**数据工程服务**（Go 1.27）负责 ingest 编排、对象存储（SeaweedFS，见 ADR-0003）与 PostgreSQL 落库、查询，交付 `hdd` CLI（ingest / reparse / list / show）。存储不经 SQLite 过渡：MVP 即用 docker compose 起 PostgreSQL + SeaweedFS（S3 API 网关）。

理由：维护者更熟 Go，长线目标是服务化架构；认知层（OCR 调参 + 规则迭代 + 样本 golden 回归）收敛在 Python 单库内，试错闭环最短，且后续 LLM 抽取增强只动认知层；Go 的强类型与工程生态（pgx / sqlc / minio-go、单二进制、毫秒启动）恰好压在编排、存储、查询与将来的 API 服务上。

## Considered Options

- 全 Python 单仓、进程内调用（0001）：被否——放弃 Go 工程优势与维护者语言偏好，且 HTTP 服务边界终将需要。
- Python 镜像仅 OCR、后处理放 Go：被否——Go regexp（RE2）无 lookahead/backreference，OCR 文本清洗受限；模糊匹配生态弱；规则迭代要「改码 → 编译 → 起服务 → 重跑」，慢一拍。

## Consequences

- JSON 契约双语各一份（pydantic ↔ Go struct），以 Python golden JSON 作为 Go fixture 互证防漂移。
- CI 双工具链（uv / pytest / ruff + go vet / gofmt / go test）；端到端验收需 docker compose。
- 识别服务无状态、不落库；三层留存（原图 → 对象存储，OCR 结果与报告 → PG）全部由 Go 侧负责。
- 对象存储实现由 MinIO 改为 SeaweedFS（ADR-0003，2026-10-08）；`minio-go` 仅作为通用 S3 客户端库保留。
