from datetime import date, timedelta

from api.models import FundEvaluationSnapshot, FundNavHistory


RETURN_PERIODS = {
    '1m': 30,
    '3m': 90,
    '6m': 180,
    '1y': 365,
}


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
