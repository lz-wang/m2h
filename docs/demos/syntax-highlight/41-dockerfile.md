---
title: Dockerfile 语法高亮示例
description: 以 dockerfile 围栏代码块渲染 Dockerfile，验证 Dockerfile 的语法高亮效果。
tags:
  - 语法高亮
  - 配置文件
create_date: 2026-09-06
update_date: 2026-09-06
---

# Dockerfile 语法高亮示例

以下 `Dockerfile` 示例使用 ` ```dockerfile ` 围栏代码块渲染：

```dockerfile
# Dockerfile 示例文件 - 多阶段构建镜像定义，指令全大写
# syntax=docker/dockerfile:1

# ================ 阶段 1：构建阶段 ================
FROM golang:1.22-alpine AS builder

# ARG：构建时变量（仅构建期可用，不保留到运行镜像）
ARG VERSION=dev
# ENV：环境变量（构建期与运行期均可用）
ENV CGO_ENABLED=0 \
    GOOS=linux

WORKDIR /src

# 先拷依赖文件以利用 Docker 层缓存
COPY go.mod go.sum ./
RUN go mod download

# 拷源码并编译
COPY . .
RUN go build -ldflags="-X main.version=${VERSION}" -o /out/app ./cmd/server

# ================ 阶段 2：运行阶段 ================
FROM alpine:3.19

# LABEL：镜像元数据标签
LABEL maintainer="dev@example.com" \
      version="1.0" \
      description="demo service"

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# COPY：从构建阶段拷贝产物（--from 指定来源阶段）
COPY --from=builder /out/app /app/app
COPY config.yaml /app/config.yaml

# 声明端口与卷（文档性质，需运行时 -p / -v 映射）
EXPOSE 8080
VOLUME ["/app/data"]

# ================ ENTRYPOINT 与 CMD 的区别 ================
# ENTRYPOINT：固定可执行文件，不易被覆盖
# CMD：默认参数，可被 docker run 后的参数覆盖
ENTRYPOINT ["/app/app"]
CMD ["--config", "/app/config.yaml"]
```
