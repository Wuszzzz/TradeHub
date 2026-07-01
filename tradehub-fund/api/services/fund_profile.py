from datetime import date
from decimal import Decimal
import re
from django.db import transaction
from django.utils import timezone

from ..models import (
    Fund,
    FundAllocationSnapshot,
    FundCompany,
    FundHoldingItem,
    FundHoldingSnapshot,
    FundManager,
    FundManagerTenure,
    FundPerformanceRankSnapshot,
)
from ..sources import SourceRegistry


def _decimal_or_none(value):
    if value in (None, '', '--'):
        return None
    return Decimal(str(value))


def _json_safe(value):
    if isinstance(value, Decimal):
        return str(value)
    if isinstance(value, dict):
        return {key: _json_safe(item) for key, item in value.items()}
    if isinstance(value, list):
        return [_json_safe(item) for item in value]
    return value


def _market_symbol(code):
    if code.startswith(('00', '01', '02', '03', '15', '16', '18', '30')):
        return f'sz{code}'
    if code.startswith(('50', '51', '52', '56', '58', '59', '60', '68')):
        return f'sh{code}'
    return code


def infer_company_name(fund_name):
    if not fund_name:
        return None
    known_companies = [
        '易方达', '华夏', '广发', '南方', '嘉实', '博时', '富国', '招商',
        '汇添富', '鹏华', '工银瑞信', '交银施罗德', '中欧', '兴证全球',
        '景顺长城', '银华', '国泰', '华安', '天弘', '建信', '农银汇理',
        '平安', '永赢', '东方红', '睿远', '朱雀', '摩根', '海富通',
        '财通', '华商', '华泰柏瑞', '长安', '红土创新', '德邦', '大成',
        '东财', '信澳', '诺德', '宏利', '交银', '兴证资管', '财通资管',
        '招商资管', '先锋', '东方', '银华', '万家', '国联安', '前海开源',
        '华泰柏瑞', '中航',
    ]
    for company in sorted(known_companies, key=len, reverse=True):
        if fund_name.startswith(company) or fund_name.endswith(company) or company in fund_name:
            return company
    return None


def parse_size_text(value):
    if not value:
        return None
    text = str(value)
    match = re.search(r'([\d.]+)', text)
    if not match:
        return None
    amount = Decimal(match.group(1))
    if '万' in text and '亿' not in text:
        return (amount / Decimal('10000')).quantize(Decimal('0.0001'))
    return amount


def _int_or_none(value):
    if value in (None, '', '--'):
        return None
    try:
        return int(value)
    except (TypeError, ValueError):
        return None


def upsert_fund_company(company_name, company_code=None, short_name=None, source='manual', raw_data=None):
    if not company_name:
        return None
    code = company_code or company_name
    company, _ = FundCompany.objects.update_or_create(
        company_code=code,
        defaults={
            'company_name': company_name,
            'short_name': short_name,
            'source': source,
            'raw_data': raw_data or {},
        },
    )
    return company


def upsert_fund_manager(manager_name, manager_code=None, company=None, source='manual', raw_data=None):
    if not manager_name:
        return None
    code = manager_code or manager_name
    manager, _ = FundManager.objects.update_or_create(
        manager_code=code,
        defaults={
            'manager_name': manager_name,
            'company': company,
            'source': source,
            'raw_data': raw_data or {},
        },
    )
    return manager


def upsert_fund_manager_tenure(fund, manager, start_date=None, end_date=None, source='manual', raw_data=None):
    if not fund or not manager:
        return None
    tenure, _ = FundManagerTenure.objects.update_or_create(
        fund=fund,
        manager=manager,
        start_date=start_date,
        defaults={
            'end_date': end_date,
            'source': source,
            'raw_data': raw_data or {},
        },
    )
    return tenure


def sync_fund_allocations(fund, profile, source='tencent_fund'):
    report_date = profile.get('report_date') or date.today()
    count = 0
    rows = [
        ('asset', profile.get('asset') or []),
        ('industry', profile.get('industry') or []),
    ]
    for allocation_type, items in rows:
        for item in items:
            name = item.get('name')
            if not name:
                continue
            FundAllocationSnapshot.objects.update_or_create(
                fund=fund,
                report_date=report_date,
                allocation_type=allocation_type,
                name=name,
                source=source,
                defaults={
                    'ratio': _decimal_or_none(item.get('ratio')),
                    'raw_data': _json_safe(item),
                },
            )
            count += 1
    return count


def sync_fund_performance_ranks(fund, profile, source='tencent_fund'):
    rank_info = profile.get('rank_info') or {}
    rank_date = None
    if rank_info.get('zxrq'):
        try:
            rank_date = date.fromisoformat(rank_info['zxrq'])
        except Exception:
            rank_date = None
    rank_date = rank_date or date.today()
    total = _int_or_none(rank_info.get('total'))
    growth_map = rank_info.get('jzzf') or {}
    rank_map = rank_info.get('jz_rank') or {}
    quartile_map = rank_info.get('ratio_level') or {}
    period_alias = {
        'd': 'day',
        'day': 'day',
        'w1': 'week',
        'w4': 'month',
        'w13': 'quarter',
        'w26': 'half_year',
        'w52': 'year',
        'year': 'this_year',
        'year3': 'three_year',
        'total': 'since_inception',
    }
    periods = set(growth_map.keys()) | set(rank_map.keys()) | set(quartile_map.keys())
    count = 0
    for source_period in periods:
        period = period_alias.get(source_period, source_period)
        FundPerformanceRankSnapshot.objects.update_or_create(
            fund=fund,
            rank_date=rank_date,
            period=period,
            source=source,
            defaults={
                'growth': _decimal_or_none(growth_map.get(source_period)),
                'rank': _int_or_none(rank_map.get(source_period)),
                'total': total,
                'quartile': _int_or_none(quartile_map.get(source_period)),
                'raw_data': {
                    'source_period': source_period,
                    'growth': growth_map.get(source_period),
                    'rank': rank_map.get(source_period),
                    'quartile': quartile_map.get(source_period),
                    'total': total,
                },
            },
        )
        count += 1
    return count


def sync_fund_basic_profile(fund_code, source_name='tencent_fund', target_code=None):
    fund = Fund.objects.get(fund_code=fund_code)
    company_name = infer_company_name(fund.fund_name)
    company = None
    if company_name:
        company = upsert_fund_company(
            company_name=company_name,
            company_code=company_name,
            short_name=company_name,
            source='name_infer',
            raw_data={'fund_code': fund_code, 'fund_name': fund.fund_name},
        )

    source = SourceRegistry.get_source(source_name)
    profile = {}
    if source and hasattr(source, 'fetch_profile'):
        profile = source.fetch_profile(_market_symbol(target_code or fund_code)) or {}

    update_fields = ['profile_source', 'profile_updated_at', 'profile_raw']
    fund.profile_source = source_name
    fund.profile_updated_at = timezone.now()
    fund.profile_raw = profile.get('raw_data') or profile
    if company:
        fund.company = company
        update_fields.append('company')
    asset_raw = ((profile.get('raw_data') or {}).get('asset') or {})
    if isinstance(asset_raw, dict) and asset_raw.get('total_money'):
        fund.fund_size_text = asset_raw.get('total_money')
        fund.fund_size = parse_size_text(asset_raw.get('total_money'))
        update_fields.extend(['fund_size_text', 'fund_size'])
    rank_info = profile.get('rank_info') or {}
    growth_map = rank_info.get('jzzf') or {}
    direct_return_fields = {
        'return_1m': growth_map.get('w4'),
        'return_3m': growth_map.get('w13'),
        'return_1y': growth_map.get('w52'),
    }
    for field_name, raw_value in direct_return_fields.items():
        if raw_value in (None, ''):
            continue
        setattr(fund, field_name, _decimal_or_none(raw_value))
        update_fields.append(field_name)
    fund.save(update_fields=list(dict.fromkeys(update_fields)))
    allocation_count = sync_fund_allocations(fund, profile, source=source_name)
    rank_count = sync_fund_performance_ranks(fund, profile, source=source_name)

    return {
        'fund_code': fund_code,
        'company': company.company_name if company else None,
        'source': source_name,
        'allocation_count': allocation_count,
        'rank_count': rank_count,
        'profile': profile,
    }


def sync_fund_basic_profiles(fund_codes, source_name='tencent_fund', limit=None):
    codes = list(fund_codes)
    if limit:
        codes = codes[:limit]
    success = 0
    failed = 0
    errors = []
    for fund_code in codes:
        try:
            sync_fund_basic_profile(fund_code, source_name=source_name)
            success += 1
        except Exception as exc:
            failed += 1
            errors.append({'fund_code': fund_code, 'error': str(exc)})
    return {
        'total': len(codes),
        'success': success,
        'failed': failed,
        'errors': errors[:20],
    }


def sync_fund_holdings_snapshot(fund_code, source_name='tencent_fund', report_date=None, target_code=None):
    fund = Fund.objects.get(fund_code=fund_code)
    source = SourceRegistry.get_source(source_name) or SourceRegistry.get_source('eastmoney')
    if not source:
        return {'success': False, 'fund_code': fund_code, 'error': 'no source'}

    profile = {}
    if source and hasattr(source, 'fetch_profile'):
        profile = source.fetch_profile(_market_symbol(target_code or fund_code)) or {}
        if profile.get('report_date') and not report_date:
            report_date = profile['report_date']

    candidate_codes = [target_code] if target_code else [fund_code]
    holdings = []
    resolved_code = target_code or fund_code
    resolved_source = source_name

    # Reuse FundViewSet target-code resolution when available without importing view logic.
    if not target_code:
        from ..viewsets import FundViewSet
        resolver = FundViewSet()
        candidate_codes = resolver._resolve_holdings_target_code(fund)

    for candidate in candidate_codes:
        code_for_source = candidate
        if source_name == 'tencent_fund':
            code_for_source = _market_symbol(candidate)
        data = source.fetch_index_holdings(code_for_source)
        if data:
            holdings = data
            resolved_code = code_for_source
            break

    if not holdings and source_name != 'tencent_fund':
        tencent = SourceRegistry.get_source('tencent_fund')
        if tencent:
            for candidate in candidate_codes:
                market_code = candidate
                market_code = _market_symbol(candidate)
                data = tencent.fetch_index_holdings(market_code)
                if data:
                    holdings = data
                    resolved_code = market_code
                    resolved_source = 'tencent_fund'
                    break

    if not holdings:
        return {
            'success': False,
            'fund_code': fund_code,
            'target_code': resolved_code,
            'source': resolved_source,
            'count': 0,
        }

    report_date = report_date or date.today()
    total_weight = sum((_decimal_or_none(item.get('weight')) or Decimal('0')) for item in holdings)
    sync_fund_basic_profile(fund_code, source_name='tencent_fund', target_code=resolved_code)

    with transaction.atomic():
        snapshot, _ = FundHoldingSnapshot.objects.update_or_create(
            fund=fund,
            report_date=report_date,
            source=resolved_source,
            target_code=resolved_code,
            defaults={
                'total_weight': total_weight,
                'raw_data': {
                    'count': len(holdings),
                    'asset': profile.get('asset') or [],
                    'industry': profile.get('industry') or [],
                    'rank_info': profile.get('rank_info') or {},
                    'profile_raw': profile.get('raw_data') or {},
                },
            },
        )
        snapshot.items.all().delete()
        for index, item in enumerate(holdings):
            weight = _decimal_or_none(item.get('weight'))
            change_percent = _decimal_or_none(item.get('change_percent'))
            contribution = None
            if weight is not None and change_percent is not None:
                contribution = (weight * change_percent / Decimal('100')).quantize(Decimal('0.0001'))
            FundHoldingItem.objects.create(
                snapshot=snapshot,
                holding_type=item.get('holding_type') or 'stock',
                asset_code=item.get('stock_code') or item.get('asset_code') or '',
                asset_name=item.get('stock_name') or item.get('asset_name') or '',
                weight=weight,
                price=_decimal_or_none(item.get('price')),
                change_percent=change_percent,
                contribution=contribution,
                sort_order=index,
                raw_data=_json_safe(item),
            )

    return {
        'success': True,
        'fund_code': fund_code,
        'target_code': resolved_code,
        'source': resolved_source,
        'snapshot_id': str(snapshot.id),
        'report_date': report_date.isoformat(),
        'total_weight': str(total_weight),
        'count': len(holdings),
    }
