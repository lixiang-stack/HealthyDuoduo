# NAS 局域网部署与验收

**状态：** 局域网常驻部署说明（2026-11-25）。外网访问不在本文件范围；当前边界仍是 ADR-0006 的“家庭局域网、无鉴权”。公网化前必须先补鉴权/TLS/审计并修订 ADR。

## 1. 目标形态

```text
手机（Android / iPhone）
  │ http://<NAS 局域网 IP>:19080/api/v1
  ▼
hdd（唯一发布宿主端口）
  │ Docker 内网
  ├── postgres
  ├── seaweedfs
  └── recognizer
```

- 只有 hdd 发布宿主端口，缺省 **19080**，避开 8xxx。
- Postgres / SeaweedFS / recognizer 不发布宿主端口。
- 数据落 NAS 数据盘；USB 用 restic 备份。
- 不假设公网可访问；不做端口映射，不开 UPnP，IPv6 入站默认拒绝。

## 2. 前置条件

- NAS 为 amd64（若是 arm64，按目标架构构建镜像即可）。
- Docker + Docker Compose 可用。
- 一个数据盘目录，例如 `/vol1/1000/docker/healthyduoduo`。
- 一块 USB 硬盘（建议 ext4），挂载到例如 `/mnt/usb`。
- NAS 上可用 `docker`、`rsync`；备份使用 `restic/restic` 镜像。
- 路由器给 NAS 做 DHCP 保留，保证 App 里的 IP 不变。

## 3. 文件

| 文件 | 用途 |
|---|---|
| `deploy/docker-compose.nas.yml` | NAS 常驻栈：只暴露 hdd 19080，restart/healthcheck/日志/时区 |
| `deploy/.env.example` | 环境变量模板；复制为 `deploy/.env` 并改值 |
| `deploy/backup-usb.sh` | Postgres + SeaweedFS 备份到 USB（restic） |
| `deploy/restore-usb.sh` | 从 USB 恢复（破坏性，需 `RESTORE_YES=1`） |

## 4. 构建镜像

### 方式 A：NAS 本机构建（简单）

```bash
cd /path/to/HealthyDuoduo
docker compose -p healthyduoduo -f deploy/docker-compose.nas.yml --env-file deploy/.env build
```

首次会拉 Go/Python 基础镜像并下载依赖，耗时和构建缓存取决于 NAS 网络/CPU。

### 方式 B：Mac 交叉构建后导入（推荐用于 amd64）

在 Mac（arm64）上构建 `linux/amd64`：

```bash
docker buildx build --platform linux/amd64 \
  -f deploy/hdd.Dockerfile -t healthyduoduo/hdd:nas .

docker buildx build --platform linux/amd64 \
  -f deploy/recognizer.Dockerfile -t healthyduoduo/recognizer:nas .
```

保存并传到 NAS：

```bash
docker save healthyduoduo/hdd:nas healthyduoduo/recognizer:nas | gzip > /tmp/hdd-images.tgz
scp /tmp/hdd-images.tgz nas:/tmp/
ssh nas 'gunzip -c /tmp/hdd-images.tgz | docker load'
```

在 NAS 上确认架构：

```bash
docker image inspect healthyduoduo/hdd:nas --format '{{.Architecture}}'
# 期望 linux/amd64
```

NAS 的 `deploy/.env` 中设置：

```dotenv
HDD_IMAGE=healthyduoduo/hdd:nas
RECOGNIZER_IMAGE=healthyduoduo/recognizer:nas
```

然后 `up -d --no-build`，避免 NAS 重新构建。

## 5. 首次配置

```bash
cp deploy/.env.example deploy/.env
chmod 600 deploy/.env
```

编辑 `deploy/.env`：

- `POSTGRES_PASSWORD`：改成长随机、URL 安全的密码（建议只字母数字，避免拼进 DSN 时转义）；
- `HDD_DATA_DIR`：NAS 数据盘目录；
- `HDD_LLM_SECRET_DIR`：secret 目录；不需要鉴权时可留空目录；
- `HDD_LLM_BASE_URL` / `HDD_LLM_MODEL`：可选；两者都设置才启用解释层；
- `HDD_SERVE_PORT=19080`；
- `BACKUP_ROOT`：USB 挂载点下的备份目录；
- `RESTIC_PASSWORD_FILE`：restic 密码文件路径（首次创建，600 权限）。

创建目录：

```bash
mkdir -p "$HDD_DATA_DIR/pgdata" "$HDD_DATA_DIR/seaweed" "$HDD_LLM_SECRET_DIR"
```

如果 Postgres 启动时报 `pgdata` 权限错误，先查容器用户：

```bash
docker compose -p healthyduoduo -f deploy/docker-compose.nas.yml --env-file deploy/.env run --rm postgres id
```

按输出的 UID/GID 修正 `pgdata` 属主（postgres:17-alpine 常见为 `70:70`，以实际为准）。

解释层使用 OpenAI 兼容端点，任意提供商均可；本机 Ollama 示例：

```dotenv
HDD_LLM_BASE_URL=http://<NAS_IP>:11434/v1
HDD_LLM_MODEL=qwen3.5:4b
HDD_LLM_REASONING_EFFORT=none
```

留空 `HDD_LLM_REASONING_EFFORT` 则省略该字段，兼容不识别它的端点。

Ollama 默认上下文为 4096，小于本项目默认的输入 + 输出上限（9500）。启动 Ollama 前设置 `OLLAMA_CONTEXT_LENGTH=16384`，或把 `HDD_LLM_INPUT_TOKENS` / `HDD_LLM_OUTPUT_TOKENS` 调到总和不超上下文。容器通过 NAS 局域网 IP 访问时，Ollama 还需监听 `0.0.0.0`（如 `OLLAMA_HOST=0.0.0.0`）。

需要鉴权的云端服务再添加 key 文件：

```bash
read -s KEY
printf '%s' "$KEY" > "$HDD_LLM_SECRET_DIR/llm_api_key"
chmod 600 "$HDD_LLM_SECRET_DIR/llm_api_key"
```

未配置 endpoint / model 时解释层关闭，事实层/框架层照常可用；key 文件缺失或为空表示无鉴权，不影响启动。

## 6. 启动与检查

```bash
docker compose -p healthyduoduo -f deploy/docker-compose.nas.yml --env-file deploy/.env up -d
```

查看状态：

```bash
docker compose -p healthyduoduo -f deploy/docker-compose.nas.yml --env-file deploy/.env ps
docker compose -p healthyduoduo -f deploy/docker-compose.nas.yml --env-file deploy/.env logs -f hdd
```

健康检查：

```bash
curl -fsS http://127.0.0.1:19080/api/v1/health
# {"ok":true,"version":"0.1.0"}
```

确认只有 hdd 发布端口：

```bash
docker compose -p healthyduoduo -f deploy/docker-compose.nas.yml --env-file deploy/.env ps --format 'table {{.Name}}\t{{.Ports}}'
```

预期只有 hdd 有 `0.0.0.0:19080->8080/tcp`；postgres/seaweedfs/recognizer 没有宿主端口。

## 7. 网络与暴露面

- 路由器给 NAS 做 DHCP 保留；App 地址用固定局域网 IP。
- 只在可信 LAN 开放 `19080`；不要把 `19080/5432/8333/9333/8000` 转发到公网。
- 关掉 UPnP；确认路由器 IPv6 入站默认拒绝，或只允许明确需要的端口。
- Docker 在 Linux 上会绕过常见 UFW/firewalld 规则，不要依赖主机防火墙挡住已发布端口。
- 访客 Wi-Fi / 智能家居如果不算可信网络，建议把 NAS 放独立 VLAN，或后续给 LAN 也加 token。
- 当前无鉴权是 ADR-0006 的明确边界，不是遗漏；任何能连到 `19080` 的 LAN 设备都能读写数据。

## 8. App 配置（当前，无二维码）

1. 构建 Android release APK（或后续 iOS Ad Hoc）。
2. 设置页手输：

```text
http://<NAS 局域网 IP>:19080/api/v1
```

3. 点“保存并校验”，看到 health 版本即通。
4. 上传路径：**医院用系统相机拍照，回家从系统相册选择上传**。App 内相机只用于“已经在家、当场拍当场传”；不做本地离线队列，退出上传页不保留待传照片。
5. 上传失败时，照片仍在系统相册；回家后重新选择同一张图即可。服务端按 sha256 幂等，重复上传会返回 duplicate，不会重复归档。

> 二维码配对（#11）尚未实现；当前每个手机手输一次 URL。做 #11 时二维码内容应指向同一 `http://<NAS IP>:19080`。

## 9. USB 备份

首次创建 restic 密码文件：

```bash
mkdir -p "$(dirname "$RESTIC_PASSWORD_FILE")"
openssl rand -base64 32 > "$RESTIC_PASSWORD_FILE"
chmod 600 "$RESTIC_PASSWORD_FILE"
```

确认 USB 已挂载，然后：

```bash
deploy/backup-usb.sh
# 可选：备份后校验
CHECK=1 deploy/backup-usb.sh
```

脚本行为：

1. 停 hdd 与 seaweedfs（备份期间 App 不可用）；
2. `pg_dump -Fc` 到临时目录；
3. restic 备份临时 DB dump + `$HDD_DATA_DIR/seaweed`；
4. `restic forget --prune` 保留 14 日 / 8 周 / 6 月；
5. 恢复 hdd。

建议 cron/systemd timer 每晚执行；USB 必须在独立挂载点，脚本会拒绝落在 `/` 上。单块 USB 不是 3-2-1，最好两块轮换，一块放别处。

### 从 USB 恢复（破坏性）

```bash
RESTORE_YES=1 deploy/restore-usb.sh
```

脚本会：恢复最新快照到临时目录 → 停 hdd/seaweedfs → 覆盖 SeaweedFS 数据 → 启动 seaweedfs 等待 raft → `pg_restore` → 启动 hdd。

恢复后立即抽查报告、原图和一次上传；恢复演练不要直接在正式数据上做，先用临时项目验证。

## 10. 验收清单

### 后端

- [ ] `docker compose config` 无报错；只有 hdd 发布 19080。
- [ ] amd64 镜像架构正确；镜像 tag 记录在案。
- [ ] Postgres/SeaweedFS/recognizer 没有宿主端口。
- [ ] `curl http://127.0.0.1:19080/api/v1/health` 返回 ok。
- [ ] 首次启动 goose 迁移成功；日志无报错。
- [ ] 建成员 → 上传一张真实/脱敏样本 → 时间线 → 详情 → 原图 → 核对。
- [ ] 趋势/项目/分析/护理事件各打开一次。
- [ ] 逐个 `docker restart` 后服务恢复；NAS 整机重启后自动拉起。
- [ ] 数据卷在数据盘；`docker info` 的 Docker Root Dir 不承载业务数据。
- [ ] 日志轮转生效；磁盘/备份失败有告警。
- [ ] USB 备份执行成功；`RESTORE_YES=1 deploy/restore-usb.sh` 在临时栈演练通过。

### 双端 App

- [ ] Android release APK 安装、手输 URL、health 通过。
- [ ] iPhone Ad Hoc 安装（Apple Developer 下来后）、手输 URL、health 通过。
- [ ] 相机拍照上传（在家）与系统相册选图（医院拍完回家）两条路径都通。
- [ ] 上传失败提示明确；照片仍在相册；回家重选上传成功。
- [ ] 报告详情、原图、核对、改归属、删除重传在两端一致。
- [ ] iPhone HEIC/25MiB 边界已验证（必要时 App 压缩转 JPEG）。
- [ ] iOS 本地网络权限/ATS 已在真机验证。

### 网络边界

- [ ] 从另一台 LAN 设备只能访问 hdd `19080`，访问不到 5432/8333/9333/8000。
- [ ] 外网/WAN 访问 19080 不通；IPv6 入站未意外放行。
- [ ] UPnP 关闭；路由器没有为 NAS 自动建映射。
- [ ] 访客 Wi-Fi/IoT 不能访问 NAS（VLAN/防火墙），或已接受风险。

## 11. 升级与回滚

1. 先跑一次备份；
2. 构建新镜像并打新 tag；
3. `up -d` 后看 goose 日志和 health；
4. 出问题：把 `.env` 的 `HDD_IMAGE`/`RECOGNIZER_IMAGE` 指回旧 tag，`up -d --no-build`；
5. 数据库迁移通常只能向前；若新版本改坏 schema，按 USB 备份恢复，不要只手改表。

## 12. 已知边界

- 不做离线队列（#08 已废弃）；医院拍照必须用系统相机，回家从相册选。
- 二维码配对、dev client 双端验收仍属 #11；NAS 常驻验收属 #12。
- 当前无鉴权，仅限可信 LAN；外网访问需要新 ADR + 鉴权/TLS，不在本文件范围。
- `deploy/reset-data.sh` 会删除卷，不要在 NAS 上使用。
