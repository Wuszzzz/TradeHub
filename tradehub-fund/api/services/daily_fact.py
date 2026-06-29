from django.utils import timezone

from ..models import EstimateAccuracy, Fund, FundDailyFact, FundNavHistory


def upsert_daily_fact_from_nav(fund, nav_record, source='nav_history'):
    fact, _ = FundDailyFact.objects.update_or_create(
        fund=fund,
        trade_date=nav_record.nav_date,
        defaults={
            'unit_nav': nav_record.unit_nav,
            'accumulated_nav': nav_record.accumulated_nav,
            'daily_growth': nav_record.daily_growth,
            'source': source,
        },
    )
    return fact


def upsert_daily_fact_from_estimate_accuracy(record: EstimateAccuracy):
    defaults = {
        'estimate_nav': record.estimate_nav,
        'unit_nav': record.actual_nav,
        'estimate_error_rate': record.error_rate,
        'source': record.source_name,
    }
    fact, _ = FundDailyFact.objects.update_or_create(
        fund=record.fund,
        trade_date=record.estimate_date,
        defaults={k: v for k, v in defaults.items() if v is not None},
    )
    return fact


def upsert_daily_fact_from_latest_nav(fund, source='latest_nav'):
    if not fund.latest_nav or not fund.latest_nav_date:
        return None
    fact, _ = FundDailyFact.objects.update_or_create(
        fund=fund,
        trade_date=fund.latest_nav_date,
        defaults={
            'unit_nav': fund.latest_nav,
            'source': source,
        },
    )
    return fact


def backfill_daily_facts(fund_codes=None, limit=None):
    funds = Fund.objects.all().order_by('fund_code')
    if fund_codes:
        funds = funds.filter(fund_code__in=fund_codes)
    if limit:
        funds = funds[:limit]

    count = 0
    for fund in funds:
        for nav in FundNavHistory.objects.filter(fund=fund).iterator():
            upsert_daily_fact_from_nav(fund, nav)
            count += 1
        for record in EstimateAccuracy.objects.filter(fund=fund).iterator():
            upsert_daily_fact_from_estimate_accuracy(record)
    return count


def upsert_latest_estimate_fact(fund):
    if not fund.estimate_time:
        return None
    trade_date = timezone.localtime(fund.estimate_time).date()
    fact, _ = FundDailyFact.objects.update_or_create(
        fund=fund,
        trade_date=trade_date,
        defaults={
            'estimate_nav': fund.estimate_nav,
            'estimate_growth': fund.estimate_growth,
            'estimate_time': fund.estimate_time,
            'source': 'latest_estimate',
        },
    )
    return fact
