FROM golang:1.23 AS builder

WORKDIR /src

COPY . /src

WORKDIR /src

ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
RUN go build -o /out/backend-worker ./worker

FROM python:3.11-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends postgresql-client && rm -rf /var/lib/apt/lists/*
COPY requirements.txt /app/requirements.txt
RUN pip install --no-cache-dir -r /app/requirements.txt

COPY --from=builder /out/backend-worker /app/backend-worker
COPY network /app/network

CMD ["/app/backend-worker"]
