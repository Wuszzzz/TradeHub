# Integration

外部券商 / 同花顺半自动执行的接入层（stub 阶段）。

## 现状

当前仅提供：

- `Broker` 接口（`PlaceOrder` / `CancelOrder` / `QueryPositions` 等）
- `NoopBroker` 占位实现，所有方法返回 `broker not connected`

API 层提供只读探测接口：

```
GET /api/v1/broker/status
→ { "broker": "noop", "connected": false, "note": "..." }
```

## 后续接入计划

- 同花顺客户端 dll（Windows 容器 / Wine）
- 模拟券商 SDK（如 easytrader）
- 自研撮合层 + 行情桥

接入时只需新建 `ths.go` / `easytrader.go` 实现 `Broker` 接口，并在 `backend/api/main.go` 里替换 `NoopBroker` 即可，模拟盘逻辑保持不变。
