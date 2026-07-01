from ..models import Fund, FundNavHistory
from .fund_metrics import calculate_rank_period_growth


def recalculate_fund_returns(fund: Fund, include_source_periods: bool = False) -> dict:
    """
    Recalculate cached return fields on Fund.

    By default the true 30 calendar day return and YTD return are calculated
    from local NAV history. The source-provided windows (1m/3m/1y) are
    preserved because third-party fund ranking APIs define those periods
    independently from our exact NAV window calculation.
    """
    nav_list = list(FundNavHistory.objects.filter(fund=fund).order_by('nav_date'))
    result = {
        'return_30d': calculate_rank_period_growth(nav_list, '30d'),
        'return_this_year': calculate_rank_period_growth(nav_list, 'this_year'),
        'return_1m': fund.return_1m,
        'return_3m': fund.return_3m,
        'return_1y': fund.return_1y,
    }

    update_values = {
        'return_30d': result['return_30d'],
        'return_this_year': result['return_this_year'],
    }
    if include_source_periods:
        nav_return_values = {
            'return_1m': calculate_rank_period_growth(nav_list, 'month'),
            'return_3m': calculate_rank_period_growth(nav_list, 'quarter'),
            'return_1y': calculate_rank_period_growth(nav_list, 'year'),
        }
        result.update(nav_return_values)
        update_values.update(nav_return_values)

    Fund.objects.filter(pk=fund.pk).update(**update_values)
    for field_name, value in update_values.items():
        setattr(fund, field_name, value)
    fund.return_30d = result['return_30d']
    return result


def recalculate_fund_returns_by_code(fund_code: str, include_source_periods: bool = False) -> dict:
    fund = Fund.objects.get(fund_code=fund_code)
    return recalculate_fund_returns(fund, include_source_periods=include_source_periods)
