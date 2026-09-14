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
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /domhub ./cmd/server

# ---- 运行镜像 ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 domhub
WORKDIR /app
COPY --from=go-builder /domhub /app/domhub
USER domhub
EXPOSE 8080
ENTRYPOINT ["/app/domhub"]
