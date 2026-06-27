FROM golang:1.23 AS builder

WORKDIR /src

COPY . /src

WORKDIR /src/api

ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -o /out/backend-api .

FROM python:3.11-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends postgresql-client && rm -rf /var/lib/apt/lists/*
COPY requirements.txt /app/requirements.txt
RUN pip install --no-cache-dir -r /app/requirements.txt

COPY --from=builder /out/backend-api /app/backend-api
COPY network /app/network

EXPOSE 8000

CMD ["/app/backend-api"]
