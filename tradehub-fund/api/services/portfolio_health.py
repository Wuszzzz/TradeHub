from decimal import Decimal

import requests
from django.db.models import Prefetch

from api.models import (
    Account,
    FundAllocationSnapshot,
    FundEvaluationSnapshot,
    FundHoldingSnapshot,
    Position,
)
from fundval.config import config


def build_portfolio_health_payload(user, account_id=None):
    accounts = Account.objects.filter(user=user)
    account = None
    if account_id:
        account = accounts.filter(id=account_id).first()
        if not account:
            raise ValueError('账户不存在或无权限访问')
        account_ids = [account.id]
        if account.parent is None:
            account_ids = list(account.children.values_list('id', flat=True))
    else:
        account_ids = list(accounts.filter(parent__isnull=False).values_list('id', flat=True))

    positions = list(
        Position.objects
        .filter(account_id__in=account_ids)
        .select_related('account', 'fund', 'fund__company')
        .order_by('account__name', 'fund__fund_code')
    )
    fund_ids = [position.fund_id for position in positions]
    allocation_map = _latest_allocations_by_fund(fund_ids)
    holding_map = _latest_holdings_by_fund(fund_ids)
    evaluation_map = _latest_evaluations_by_fund(fund_ids)

    return {
        'account_id': str(account.id) if account else None,
        'account_name': account.name if account else '全部账户',
        'positions': [
            _position_to_payload(position, allocation_map, holding_map, evaluation_map)
            for position in positions
        ],
    }


def analyze_portfolio_health(user, account_id=None):
    payload = build_portfolio_health_payload(user, account_id=account_id)
    endpoint = config.get('fund_research_url', '') or 'http://fund-research:18081'
    url = f'{endpoint.rstrip("/")}/api/fund-research/v1/portfolio/health'
    response = requests.post(url, json=payload, timeout=30)
    response.raise_for_status()
    body = response.json()
    if not body.get('ok'):
        raise ValueError(body.get('error') or '持仓体检计算失败')
    return body.get('data') or {}


def _position_to_payload(position, allocation_map, holding_map, evaluation_map):
    fund = position.fund
    latest_nav = _float(fund.latest_nav)
    market_value = _float(position.holding_share) * latest_nav if latest_nav else 0
    pnl = market_value - _float(position.holding_cost)
    pnl_rate = (pnl / _float(position.holding_cost) * 100) if _float(position.holding_cost) else 0
    evaluation = evaluation_map.get(fund.id)
    return {
        'fund_code': fund.fund_code,
        'fund_name': fund.fund_name,
        'fund_type': fund.fund_type,
        'company': fund.company.company_name if fund.company else '',
        'holding_share': _float(position.holding_share),
        'holding_cost': _float(position.holding_cost),
        'holding_nav': _float(position.holding_nav),
        'latest_nav': latest_nav,
        'estimate_nav': _float(fund.estimate_nav),
        'market_value': market_value,
        'pnl': pnl,
        'pnl_rate': pnl_rate,
        'return_30d': _float(fund.return_30d),
        'return_1m': _float(fund.return_1m),
        'return_3m': _float(fund.return_3m),
        'return_1y': _float(fund.return_1y),
        'return_this_year': _float(fund.return_this_year),
        'max_drawdown': _float(evaluation.max_drawdown) if evaluation else 0,
        'volatility': _float(evaluation.volatility) if evaluation else 0,
        'asset_allocation': allocation_map.get(fund.id, {}).get('asset', []),
        'industry': allocation_map.get(fund.id, {}).get('industry', []),
        'top_holdings': holding_map.get(fund.id, []),
        'data_flags': _data_flags(fund, allocation_map.get(fund.id), holding_map.get(fund.id), evaluation),
    }


def _latest_allocations_by_fund(fund_ids):
    result = {}
    latest_dates = {}
    rows = (
        FundAllocationSnapshot.objects
        .filter(fund_id__in=fund_ids)
        .order_by('fund_id', '-report_date', 'allocation_type', '-ratio')
    )
    for row in rows:
        latest_dates.setdefault(row.fund_id, row.report_date)
        if row.report_date != latest_dates[row.fund_id]:
            continue
        result.setdefault(row.fund_id, {'asset': [], 'industry': []})
        if row.allocation_type in ('asset', 'industry'):
            result[row.fund_id][row.allocation_type].append({
                'name': row.name,
                'ratio': _float(row.ratio),
            })
    return result


def _latest_holdings_by_fund(fund_ids):
    result = {}
    snapshots = (
        FundHoldingSnapshot.objects
        .filter(fund_id__in=fund_ids)
        .prefetch_related(Prefetch('items'))
        .order_by('fund_id', '-report_date')
    )
    seen = set()
    for snapshot in snapshots:
        if snapshot.fund_id in seen:
            continue
        seen.add(snapshot.fund_id)
        result[snapshot.fund_id] = [
            {
                'code': item.asset_code,
                'name': item.asset_name,
                'weight': _float(item.weight),
                'industry': (item.raw_data or {}).get('industry') or '',
            }
            for item in list(snapshot.items.all())[:10]
        ]
    return result


def _latest_evaluations_by_fund(fund_ids):
    result = {}
    rows = (
        FundEvaluationSnapshot.objects
        .filter(fund_id__in=fund_ids)
        .order_by('fund_id', '-evaluation_date', '-window_days')
    )
    for row in rows:
        result.setdefault(row.fund_id, row)
    return result


def _data_flags(fund, allocations, holdings, evaluation):
    flags = []
    if not fund.latest_nav:
        flags.append('缺少最新净值')
    if not (fund.return_30d or fund.return_1y or fund.return_this_year):
        flags.append('缺少周期收益')
    if not allocations:
        flags.append('缺少资产/行业配置')
    if not holdings:
        flags.append('缺少披露持仓')
    if not evaluation:
        flags.append('缺少风险评估')
    return flags


def _float(value):
    if value is None:
        return 0
    if isinstance(value, Decimal):
        return float(value)
    try:
        return float(value)
    except (TypeError, ValueError):
        return 0
