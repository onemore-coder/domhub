# ---- 前端构建 ----
FROM node:22-alpine AS web-builder
WORKDIR /app/web
COPY web/package.json web/package-lock.json* ./
RUN npm install --no-audit --no-fund
COPY web/ .
RUN npm run build

# ---- 后端构建 ----
FROM golang:1.26-alpine AS go-builder
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /app/web/dist ./web/dist
# 版本号：构建时传入（CI 传 git tag，本地默认 dev）
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux \
    go build -ldflags="-s -w -X github.com/domhub-io/domhub/internal/pkg/version.Version=${VERSION}" \
    -o /domhub ./cmd/server

# ---- 运行镜像 ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 domhub
WORKDIR /app
COPY --from=go-builder /domhub /app/domhub
USER domhub
EXPOSE 8080
ENTRYPOINT ["/app/domhub"]
