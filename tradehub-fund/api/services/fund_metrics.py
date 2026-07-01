from datetime import date, timedelta
from decimal import Decimal

from api.models import FundEvaluationSnapshot, FundNavHistory


RETURN_PERIODS = {
    '1m': 30,
    '3m': 90,
    '6m': 180,
    '1y': 365,
}

RANK_PERIODS = (
    'week',
    'month',
    'quarter',
    'half_year',
    'this_year',
    'year',
    'since_inception',
)


def calculate_nav_returns(nav_list, today=None):
    today = today or date.today()
    result = {}
    for period_name, days in RETURN_PERIODS.items():
        cutoff = today - timedelta(days=days)
        start_nav = None
        for row in nav_list:
            if row.nav_date <= cutoff:
                start_nav = row
        end_nav = nav_list[-1] if nav_list else None
        if start_nav and end_nav and start_nav.unit_nav > 0:
            value = (end_nav.unit_nav - start_nav.unit_nav) / start_nav.unit_nav * 100
            result[period_name] = str(round(value, 2))
        else:
            result[period_name] = None
    return result


def calculate_rank_period_growth(nav_list, period, latest_date=None):
    if len(nav_list) < 2:
        return None

    latest_row = nav_list[-1]
    latest_date = latest_date or latest_row.nav_date
    start_boundary = _pick_rank_period_start(period, latest_date, nav_list[0].nav_date)
    if not start_boundary:
        return None

    if period == 'since_inception':
        start_row = nav_list[0]
    else:
        start_row = next((row for row in reversed(nav_list) if row.nav_date <= start_boundary), None)
        if start_row is None:
            start_row = next((row for row in nav_list if row.nav_date >= start_boundary), nav_list[0])

    if not start_row or not latest_row or not start_row.unit_nav or start_row.unit_nav <= 0:
        return None

    value = (latest_row.unit_nav - start_row.unit_nav) / start_row.unit_nav * Decimal('100')
    return value.quantize(Decimal('0.0001'))


def calculate_rank_growths(nav_list, latest_date=None):
    return {
        period: calculate_rank_period_growth(nav_list, period, latest_date=latest_date)
        for period in RANK_PERIODS
    }


def calculate_nav_risk_metrics(nav_list):
    if len(nav_list) < 60:
        return {'max_drawdown': None, 'volatility': None, 'sharpe': None}

    vals = [float(row.unit_nav) for row in nav_list if row.unit_nav]
    if len(vals) < 60:
        return {'max_drawdown': None, 'volatility': None, 'sharpe': None}

    peak = vals[0]
    max_dd = 0.0
    for value in vals:
        if value > peak:
            peak = value
        drawdown = (peak - value) / peak if peak > 0 else 0
        if drawdown > max_dd:
            max_dd = drawdown

    daily_returns = [(vals[i] - vals[i - 1]) / vals[i - 1] for i in range(1, len(vals)) if vals[i - 1] > 0]
    if len(daily_returns) < 2:
        return {'max_drawdown': str(round(-max_dd * 100, 2)), 'volatility': None, 'sharpe': None}

    mean = sum(daily_returns) / len(daily_returns)
    variance = sum((value - mean) ** 2 for value in daily_returns) / (len(daily_returns) - 1)
    annual_vol = (variance ** 0.5) * (252 ** 0.5)
    annual_return = mean * 252
    sharpe = (annual_return - 0.02) / annual_vol if annual_vol > 0 else None

    return {
        'max_drawdown': str(round(-max_dd * 100, 2)),
        'volatility': str(round(annual_vol * 100, 2)),
        'sharpe': str(round(sharpe, 2)) if sharpe is not None else None,
    }


def evaluate_fund_choice(row):
    growth = _to_float(row.get('growth'))
    rank = _to_float(row.get('rank'))
    total = _to_float(row.get('total'))
    max_drawdown = _to_float(row.get('max_drawdown'))
    sharpe = _to_float(row.get('sharpe'))

    score = 0
    reasons = []

    if rank and total and total > 0:
        ratio = rank / total
        if ratio <= 0.25:
            score += 30
            reasons.append('同类排名前25%')
        elif ratio <= 0.5:
            score += 18
            reasons.append('同类排名前50%')
    if growth is not None and growth > 0:
        score += 20
        reasons.append('周期收益为正')
    if sharpe is not None:
        if sharpe >= 1:
            score += 25
            reasons.append('夏普率优秀')
        elif sharpe >= 0.5:
            score += 15
            reasons.append('夏普率尚可')
    if max_drawdown is not None:
        abs_drawdown = abs(max_drawdown)
        if abs_drawdown <= 15:
            score += 20
            reasons.append('回撤控制较好')
        elif abs_drawdown <= 30:
            score += 10
            reasons.append('回撤中等')

    if score >= 75:
        level = '优选'
    elif score >= 50:
        level = '观察'
    else:
        level = '谨慎'

    return {'score': score, 'level': level, 'reasons': reasons}


def metrics_for_funds(fund_ids, max_days=370):
    if not fund_ids:
        return {}
    fund_ids = list(dict.fromkeys(fund_ids))
    snapshots = (
        FundEvaluationSnapshot.objects
        .filter(fund_id__in=fund_ids)
        .order_by('fund_id', '-evaluation_date', '-updated_at')
    )
    result = {}
    for snapshot in snapshots:
        if snapshot.fund_id in result:
            continue
        result[snapshot.fund_id] = _snapshot_to_metrics(snapshot)

    missing_ids = [fund_id for fund_id in fund_ids if fund_id not in result]
    if not missing_ids:
        return result

    cutoff = date.today() - timedelta(days=max_days)
    rows = (
        FundNavHistory.objects
        .filter(fund_id__in=missing_ids, nav_date__gte=cutoff)
        .order_by('fund_id', 'nav_date')
    )
    grouped = {}
    for row in rows:
        grouped.setdefault(row.fund_id, []).append(row)
    for fund_id, nav_list in grouped.items():
        result[fund_id] = calculate_nav_risk_metrics(nav_list)
    return result


def _snapshot_to_metrics(snapshot):
    def decimal_text(value):
        return str(value) if value is not None else None

    return {
        'max_drawdown': decimal_text(snapshot.max_drawdown),
        'volatility': decimal_text(snapshot.volatility),
        'sharpe': decimal_text(snapshot.sharpe),
        'raw_data': snapshot.raw_data or {},
        'returns': {
            '1m': decimal_text(snapshot.return_1m),
            '3m': decimal_text(snapshot.return_3m),
            '6m': decimal_text(snapshot.return_6m),
            '1y': decimal_text(snapshot.return_1y),
        },
        'evaluation': {
            'score': snapshot.score,
            'level': snapshot.level,
            'reasons': snapshot.reasons or [],
            'source': snapshot.source,
            'evaluation_date': snapshot.evaluation_date.isoformat() if snapshot.evaluation_date else None,
            'nav_count': snapshot.nav_count,
        },
    }


def _to_float(value):
    if value in (None, ''):
        return None
    try:
        return float(value)
    except (TypeError, ValueError):
        return None


def _pick_rank_period_start(period, latest_date, first_date):
    if period == '30d':
        return latest_date - timedelta(days=30)
    if period == 'this_year':
        return date(latest_date.year, 1, 1)
    if period == 'month':
        return _shift_months(latest_date, -1)
    if period == 'quarter':
        return _shift_months(latest_date, -3)
    if period == 'half_year':
        return _shift_months(latest_date, -6)
    if period == 'year':
        return _shift_years(latest_date, -1)
    if period == 'week':
        return latest_date - timedelta(days=7)
    if period == 'since_inception':
        return first_date
    return None


def _shift_months(source_date, months):
    month_index = source_date.month - 1 + months
    year = source_date.year + month_index // 12
    month = month_index % 12 + 1
    day = min(source_date.day, _days_in_month(year, month))
    return date(year, month, day)


def _shift_years(source_date, years):
    year = source_date.year + years
    day = min(source_date.day, _days_in_month(year, source_date.month))
    return date(year, source_date.month, day)


def _days_in_month(year, month):
    if month == 12:
        next_month = date(year + 1, 1, 1)
    else:
        next_month = date(year, month + 1, 1)
    return (next_month - date(year, month, 1)).days
