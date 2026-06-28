#!/usr/bin/env bash
# Bootstrap one year of fund NAV history, then calculate Go evaluation snapshots.
#
# Run from the TradeHub repository root on a Docker Compose deployment host.

set -euo pipefail

COMPOSE=${COMPOSE:-docker compose}
DAYS=${DAYS:-365}
WINDOW_DAYS=${WINDOW_DAYS:-370}
PARTS=${PARTS:-4}
BATCH_SIZE=${BATCH_SIZE:-200}
VERIFY_LIMIT=${VERIFY_LIMIT:-20}
LOG_DIR=${LOG_DIR:-logs}
FUND_TOTAL=${FUND_TOTAL:-}

mkdir -p "$LOG_DIR"

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}

compose_exec() {
  # shellcheck disable=SC2086
  $COMPOSE exec -T "$@"
}

require_service() {
  local service="$1"
  if ! $COMPOSE ps --services --filter "status=running" | grep -qx "$service"; then
    log "service not running: $service"
    log "start services first: docker compose up -d"
    exit 1
  fi
}

wait_pid() {
  local pid="$1"
  local label="$2"
  if wait "$pid"; then
    log "$label finished"
    return 0
  fi
  log "$label failed"
  return 1
}

require_service fund-backend
require_service fund-research
require_service postgres

if [[ -z "$FUND_TOTAL" ]]; then
  FUND_TOTAL=$(compose_exec postgres psql -U postgres -d fundval -Atc "select count(*) from fund;")
fi

if [[ "$FUND_TOTAL" -le 0 ]]; then
  log "fund table is empty; run fund bootstrap/sync first"
  exit 1
fi

log "fund_total=$FUND_TOTAL days=$DAYS parts=$PARTS batch_size=$BATCH_SIZE"

if [[ "$VERIFY_LIMIT" -gt 0 ]]; then
  log "running small verification batch: limit=$VERIFY_LIMIT"
  compose_exec fund-backend python -u manage.py sync_nav_history \
    --days "$DAYS" \
    --limit "$VERIFY_LIMIT" \
    --batch-size "$VERIFY_LIMIT"
fi

part_size=$(( (FUND_TOTAL + PARTS - 1) / PARTS ))
pids=()

for ((part = 0; part < PARTS; part++)); do
  offset=$(( part * part_size ))
  if [[ "$offset" -ge "$FUND_TOTAL" ]]; then
    break
  fi
  limit="$part_size"
  if [[ $(( offset + limit )) -gt "$FUND_TOTAL" ]]; then
    limit=$(( FUND_TOTAL - offset ))
  fi
  log_file="$LOG_DIR/sync_nav_history_1y_part_${offset}.log"
  log "starting part=$part offset=$offset limit=$limit log=$log_file"
  (
    compose_exec fund-backend python -u manage.py sync_nav_history \
      --days "$DAYS" \
      --offset "$offset" \
      --limit "$limit" \
      --batch-size "$BATCH_SIZE"
  ) >"$log_file" 2>&1 &
  pids+=("$!")
done

failed=0
for i in "${!pids[@]}"; do
  if ! wait_pid "${pids[$i]}" "part_$i"; then
    failed=1
  fi
done

if [[ "$failed" -ne 0 ]]; then
  log "one or more NAV sync parts failed; inspect $LOG_DIR/sync_nav_history_1y_part_*.log"
  exit 1
fi

log "NAV sync completed; calculating Go fund evaluations"
curl -fsS -X POST http://127.0.0.1:17081/api/fund-research/v1/sync/evaluations \
  -H 'Content-Type: application/json' \
  -d "{\"limit\":$FUND_TOTAL,\"window_days\":$WINDOW_DAYS}" \
  >"$LOG_DIR/sync_go_evaluations_1y.json"

log "database summary"
compose_exec postgres psql -U postgres -d fundval -c "
select
  count(*) as nav_rows,
  count(distinct fund_id) as fund_count,
  min(nav_date) as min_date,
  max(nav_date) as max_date
from fund_nav_history
where nav_date >= current_date - interval '$DAYS days';
"
compose_exec postgres psql -U postgres -d fundval -c "
select
  count(*) as eval_count,
  count(*) filter (where nav_count >= 60) as evaluable,
  min(nav_count) as min_nav,
  max(nav_count) as max_nav,
  max(evaluation_date) as latest_eval_date
from fund_evaluation_snapshot
where source = 'go_fund_research';
"

log "done"
