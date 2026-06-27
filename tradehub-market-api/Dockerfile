# market-api：聚合腾讯/东方财富/搜狐/同花顺/雪球的本地行情网关，监听 :18080。
# 由 docker-compose 单独编排成 service `market-api`，供 backend 内网调用。
FROM golang:1.23 AS builder

WORKDIR /src
COPY go.mod go.sum* ./
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/market-api ./cmd/market-api

# 运行镜像使用 scratch，避免依赖 gcr.io/distroless 拉取；只拷贝 HTTPS 证书和静态二进制。
FROM scratch
WORKDIR /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/market-api /app/market-api
ENV MARKET_API_ADDR=":18080"
EXPOSE 18080
ENTRYPOINT ["/app/market-api"]
