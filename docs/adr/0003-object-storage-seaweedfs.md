# 0003 · 对象存储：MinIO → SeaweedFS

**状态：** 已采用（2026-10-08，维护者决策）

对象存储由 MinIO 改为 **SeaweedFS**：compose 用 `chrislusf/seaweedfs` 镜像、单进程 `server -dir=/data -s3` 起 S3 API 网关（master + volume + filer + S3），S3 流量端口 8333。**不变的部分**：bucket `raw`、键 = 图像 sha256、`minio-go` 作为通用 S3 客户端库、三层持久化与追溯设计（ADR-0002）；契约与验收仅需把「MinIO 健康端点 200」换成「master `/cluster/status` 200 且 S3 API 200」。

## 理由

- 官方 `minio/minio` / `minio/mc` 镜像 2026-09 起从 Docker Hub 撤下、改为源码分发（quay.io 同步收紧），本地一键起 MinIO 的路径断了。
- 社区构建镜像（coollabsio/minio 等）虽可用，但引入长期第三方维护风险；P0 需要的是**可一键复现的本地基础设施**。
- SeaweedFS 提供 S3 API 兼容层，本仓库对对象存储的全部诉求（放原图对象、按 sha256 键、S3 语义）都落在 S3 API 之内，替换不触碰契约。

## Considered Options

- `coollabsio/minio`（社区从官方源码自动构建多架构镜像）：本次过渡可用（P0 首轮空转即用它验收），但维护主体是个人/社区，弃。
- PostgreSQL 大对象/bytea 存图：违背已定的 bucket + sha256 键对象粒度，原图与报告耦合进同一库，否。
- Garage / RustFS 等 S3 兼容新生态：可行但生态更年轻、对 minio-go 兼容性验证弱，保守弃。

## Consequences

- compose 服务名从 `minio` 改为 `seaweedfs`；本地验收命令同步更新（`docker compose up -d postgres seaweedfs`）。
- `minio-go` 保留：它是 S3 客户端库而非 MinIO 服务依赖，P2 的 Go 代码不需要换 SDK。
- 未来若 SeaweedFS 亦有变故，S3 API 这一抽象边界保证可再换实现而不动契约（NF-05 可演进）。
