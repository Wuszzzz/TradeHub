import json
import re
from datetime import date
from decimal import Decimal

import requests
from django.db import transaction
from django.db.models import Count, F

from ..models import Fund, FundPerformanceRankSnapshot
from ..sources import SourceRegistry


RANK_PERIODS = {
    'day': {'sc': 'rzdf', 'field_index': 6},
    'week': {'sc': 'zzf', 'field_index': 7},
    'month': {'sc': '1yzf', 'field_index': 8},
    'quarter': {'sc': '3yzf', 'field_index': 9},
    'half_year': {'sc': '6yzf', 'field_index': 10},
    'this_year': {'sc': 'jnzf', 'field_index': 14},
    'year': {'sc': '1nzf', 'field_index': 11},
    'two_year': {'sc': '2nzf', 'field_index': 12},
    'three_year': {'sc': '3nzf', 'field_index': 13},
    'since_inception': {'sc': 'lnzf', 'field_index': 15},
}


def _decimal_or_none(value):
    if value in (None, '', '--'):
        return None
    return Decimal(str(value).replace('%', ''))


def _date_or_today(value):
    try:
        return date.fromisoformat(value)
    except Exception:
        return date.today()


def fetch_eastmoney_rankings(period='year', limit=500):
    config = RANK_PERIODS[period]
    response = requests.get(
        'https://fund.eastmoney.com/data/rankhandler.aspx',
        params={
            'op': 'ph',
            'dt': 'kf',
            'ft': 'all',
            'rs': '',
            'gs': '0',
            'sc': config['sc'],
            'st': 'desc',
            'sd': '',
            'ed': '',
            'qdii': '',
            'tabSubtype': ',,,,,',
            'pi': '1',
            'pn': str(limit),
            'dx': '1',
            'v': '0.1',
        },
        headers={
            'User-Agent': 'Mozilla/5.0',
            'Referer': 'https://fund.eastmoney.com/data/fundranking.html',
        },
        timeout=30,
    )
    response.raise_for_status()
    match = re.search(r'var rankData = (.*);?$', response.text.strip())
    if not match:
        return []
    payload = match.group(1).rstrip(';')
    payload = re.sub(r'(\w+):', r'"\1":', payload)
    data = json.loads(payload)
    rows = []
    for index, raw in enumerate(data.get('datas') or [], start=1):
        parts = raw.split(',')
        if len(parts) <= config['field_index']:
            continue
        rows.append({
            'fund_code': parts[0],
            'fund_name': parts[1],
            'fund_type': None,
            'rank_date': _date_or_today(parts[3]),
            'growth': _decimal_or_none(parts[config['field_index']]),
            'rank': index,
            'total': data.get('allRecords'),
            'raw_data': {'raw': raw, 'source_period': period, 'sort_field': config['sc']},
        })
    return rows


def sync_top_fund_rankings(periods=None, limit=500, sync_profiles_limit=500):
    periods = periods or list(RANK_PERIODS.keys())
    type_map = {}
    try:
        eastmoney = SourceRegistry.get_source('eastmoney')
        if eastmoney:
            type_map = {
                item.get('fund_code'): item.get('fund_type')
                for item in eastmoney.fetch_fund_list()
            }
    except Exception:
        type_map = {}
    summary = {}
    ranked_codes = []
    for period in periods:
        rows = fetch_eastmoney_rankings(period=period, limit=limit)
        saved = 0
        with transaction.atomic():
            for row in rows:
                if row['fund_code'] not in ranked_codes:
                    ranked_codes.append(row['fund_code'])
                fund, created = Fund.objects.get_or_create(
                    fund_code=row['fund_code'],
                    defaults={'fund_name': row['fund_name']},
                )
                if fund.fund_name != row['fund_name']:
                    fund.fund_name = row['fund_name']
                    update_fields = ['fund_name']
                else:
                    update_fields = []
                fund_type = type_map.get(row['fund_code'])
                if fund_type and not fund.fund_type:
                    fund.fund_type = fund_type
                    update_fields.append('fund_type')
                if update_fields:
                    fund.save(update_fields=update_fields)
                FundPerformanceRankSnapshot.objects.update_or_create(
                    fund=fund,
                    rank_type='performance',
                    rank_date=row['rank_date'],
                    period=period,
                    source='eastmoney_rank',
                    defaults={
                        'growth': row['growth'],
                        'rank': row['rank'],
                        'total': row['total'],
                        'quartile': None,
                        'raw_data': row['raw_data'],
                    },
                )
                saved += 1
        summary[period] = {'count': saved}
    popular_summary = sync_system_popular_rankings(limit=limit)
    summary['popular'] = popular_summary
    if sync_profiles_limit:
        from .fund_profile import sync_fund_basic_profile

        profile_success = 0
        profile_failed = 0
        for fund_code in ranked_codes[:sync_profiles_limit]:
            try:
                sync_fund_basic_profile(fund_code, source_name='tencent_fund')
                profile_success += 1
            except Exception:
                profile_failed += 1
        summary['profiles'] = {
            'count': min(len(ranked_codes), sync_profiles_limit),
            'success': profile_success,
            'failed': profile_failed,
        }
    return summary


def sync_system_popular_rankings(limit=500):
    """按系统内持仓和自选热度生成每日人气榜快照。"""
    rank_date = date.today()
    queryset = (
        Fund.objects
        .annotate(position_count=Count('positions', distinct=True), watchlist_count=Count('watchlist_items', distinct=True))
        .annotate(popularity_score=F('position_count') + F('watchlist_count'))
        .filter(popularity_score__gt=0)
        .order_by('-popularity_score', 'fund_code')[:limit]
    )
    total = Fund.objects.count()
    saved = 0
    with transaction.atomic():
        for index, fund in enumerate(queryset, start=1):
            score = getattr(fund, 'popularity_score', 0) or 0
            FundPerformanceRankSnapshot.objects.update_or_create(
                fund=fund,
                rank_type='popular',
                rank_date=rank_date,
                period='day',
                source='tradehub_system_popular',
                defaults={
                    'growth': Decimal(str(score)),
                    'rank': index,
                    'total': total,
                    'quartile': None,
                    'raw_data': {
                        'position_count': getattr(fund, 'position_count', 0) or 0,
                        'watchlist_count': getattr(fund, 'watchlist_count', 0) or 0,
                        'popularity_score': score,
                    },
                },
            )
            saved += 1
    return {'count': saved, 'rank_date': rank_date.isoformat(), 'source': 'tradehub_system_popular'}
