FROM golang:1.23 AS builder

WORKDIR /src
COPY go.mod go.sum* ./
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/fund-research ./cmd/fund-research

FROM scratch
WORKDIR /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/fund-research /app/fund-research
ENV FUND_RESEARCH_ADDR=":18081"
EXPOSE 18081
ENTRYPOINT ["/app/fund-research"]
