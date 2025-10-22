# 阶段 1：构建
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 复制 go.mod 和 go.sum 文件并下载依赖项
COPY go.mod go.sum ./
# 使用 -o vendor 确保依赖被下载到 vendor 目录
RUN go mod download
RUN go mod vendor

# 复制源代码
COPY . .

# 构建 Go 应用
# CGO_ENABLED=0 禁用 CGO，构建静态二进制文件
# -o /app/main 指定输出
RUN CGO_ENABLED=0 GOOS=linux go build -v -o /app/main -mod=vendor .

# 阶段 2：运行
FROM alpine:latest

WORKDIR /app

# 从构建器阶段复制编译好的二进制文件
COPY --from=builder /app/main .

# 复制 .env 文件 (虽然不推荐，但对于简单部署可行)
# 更好的方式是使用 docker-compose 的 environment
# COPY .env .

# 暴露端口
EXPOSE 8080

# 运行应用
CMD ["/app/main"]