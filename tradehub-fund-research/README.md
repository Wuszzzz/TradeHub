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
- related sector mapping and sector quote lookup
- fund tag recommendation based on related sectors
- sector metadata sync into PostgreSQL
- fund evaluation metric calculation persisted into PostgreSQL

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
| `GET` | `/api/fund-research/v1/sectors/related` | fund-to-related-sector lookup |
| `GET` | `/api/fund-research/v1/sectors/quotes` | EastMoney sector/index quotes |
| `GET` | `/api/fund-research/v1/tags/recommend` | recommended fund tags |
| `GET` | `/api/fund-research/v1/sync/status` | metadata sync status |
| `POST` | `/api/fund-research/v1/sync/sector-map` | sync sector mappings into PostgreSQL |
| `POST` | `/api/fund-research/v1/sync/evaluations` | calculate fund risk/evaluation metrics from PostgreSQL NAV data and upsert snapshots |

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

Calculate evaluation snapshots:

```bash
curl -X POST http://127.0.0.1:17081/api/fund-research/v1/sync/evaluations \
  -H 'Content-Type: application/json' \
  -d '{"limit":500,"window_days":370}'
```

The evaluation sync reads `fund`, `fund_nav_history`, and
`fund_performance_rank_snapshot`, then upserts `fund_evaluation_snapshot`.
Django ranking and compare APIs read those snapshots first and only fall back
to Python request-time calculation when a snapshot is missing.

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

## real-time-fund Engineering Ideas Adopted

`hzm0321/real-time-fund` uses a practical metadata chain:

1. fund code -> related sector name
2. related sector name -> EastMoney `secid`
3. batched sector quote lookup
4. recommended tags derived from sector/topic metadata
5. sync state separated from live quote fetching

TradeHub adopts that design shape in Go, but does not vendor the upstream
Next.js/Supabase implementation. The service keeps a small built-in seed map
for common indices and sectors, can optionally persist mappings into
PostgreSQL, and exposes APIs that the TradeHub fund workspace can call from the
same `/api/fund-research/v1/*` namespace.
