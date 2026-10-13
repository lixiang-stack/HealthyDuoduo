# 数据服务镜像:hdd serve 常驻端点(/api/v1),随既有 compose 栈拉起。
# CGO 关闭 → 纯静态二进制;两段构建兼顾构建层缓存(go.mod 先行)。
# 构建上下文按根 .dockerignore 白名单截取(仓库根为 recognizer/hdd 两镜像共享,
# 白名单放宽不改变识别端镜像内容——其 Dockerfile 只 COPY recognizer/)。
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o /out/hdd ./cmd/hdd

FROM alpine:3.20
COPY --from=build /out/hdd /usr/local/bin/hdd
EXPOSE 8080
# serve 即常驻入口;--addr 缺省 :8080 绑定全部网卡,compose 端口映射直出 LAN。
ENTRYPOINT ["hdd"]
CMD ["serve"]
