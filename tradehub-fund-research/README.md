# tradehub-fund-research

Go implementation of the TradeHub fund research workspace.

This service recreates the fund-focused workflows inspired by
`axiaoxin-com/investool` inside TradeHub:

- 4433 fund screening
- custom strict screening
- single/multiple fund diagnostics
- fund holding similarity
- stock-to-fund lookup
- fund manager screening

The implementation is not a vendored copy of the upstream Go web app. It is a
TradeHub-native Go service that calls EastMoney endpoints and reads the existing
TradeHub PostgreSQL fund tables when available.

Upstream attribution: see `THIRD_PARTY_NOTICES.md`.

## API

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/health` | service health |
| `GET` | `/api/fund-research/v1/summary` | feature and source summary |
| `GET` | `/api/fund-research/v1/funds/4433` | 4433 fund screening |
| `GET` | `/api/fund-research/v1/funds/filter` | custom strict fund screening |
| `POST` | `/api/fund-research/v1/funds/check` | fund diagnostics |
| `POST` | `/api/fund-research/v1/funds/similarity` | fund holding similarity |
| `GET` | `/api/fund-research/v1/funds/by-stock` | stock-to-fund lookup |
| `GET` | `/api/fund-research/v1/managers` | fund manager screening |

## Local Run

```bash
FUND_RESEARCH_ADDR=:17081 go run ./cmd/fund-research
```

Example:

```bash
curl -X POST http://127.0.0.1:17081/api/fund-research/v1/funds/check \
  -H 'Content-Type: application/json' \
  -d '{"codes":["260104"]}'
```

## Upstream Fund Workflow Coverage

The fund-side upstream workflows in `axiaoxin-com/investool` are mapped into
TradeHub APIs as follows:

| investool workflow | TradeHub implementation |
| --- | --- |
| 基金 4433 筛选 | `GET /api/fund-research/v1/funds/4433` |
| 自定义基金筛选 / 4433 严选 | `GET /api/fund-research/v1/funds/filter` |
| 基金检测 | `POST /api/fund-research/v1/funds/check` |
| 股票选基 | `GET /api/fund-research/v1/funds/by-stock` |
| 股票持仓相似度检测 | `POST /api/fund-research/v1/funds/similarity` |
| 基金经理筛选 | `GET /api/fund-research/v1/managers` |
