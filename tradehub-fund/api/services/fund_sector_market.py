import os
from decimal import Decimal

import requests
from django.db import transaction
from django.utils import timezone

from ..models import FundAllocationSnapshot, FundSectorMarketSnapshot


STOCK_API_BASE = os.environ.get('STOCK_API_URL', 'http://stock-api:8000').rstrip('/')


def _decimal_or_none(value):
    if value in (None, '', '--'):
        return None
    try:
        return Decimal(str(value).replace('%', '').replace(',', ''))
    except Exception:
        return None


def fetch_stock_api_sector_boards(board_type='industry', limit=500):
    response = requests.get(
        f'{STOCK_API_BASE}/api/stock/v1/market/sector-boards',
        params={'board_type': board_type, 'limit': limit},
        headers={'User-Agent': 'Mozilla/5.0'},
        timeout=20,
    )
    response.raise_for_status()
    return response.json()


def sync_fund_sector_market_snapshots(
    board_codes=None,
    board_code=None,
    sort_type='f3',
    direct='down',
    count=500,
    is_close_snapshot=False,
):
    del sort_type, direct  # stock-api 当前直接返回按数据源默认排序的板块列表

    snapshot_time = timezone.now().replace(microsecond=0)
    trade_date = timezone.localdate()
    saved = 0
    total = 0
    errors = []
    board_codes = board_codes or ([board_code] if board_code else ['industry', 'concept'])

    with transaction.atomic():
        for current_board_code in board_codes:
            try:
                data = fetch_stock_api_sector_boards(board_type=current_board_code, limit=count)
            except Exception as exc:
                errors.append({'board_code': current_board_code, 'error': str(exc)})
                continue
            rows = data.get('items') or []
            if data.get('error'):
                errors.append({'board_code': current_board_code, 'error': data.get('error')})
            total += data.get('total') or len(rows)
            for row in rows:
                sector_code = str(row.get('sector_code') or '').strip()
                sector_name = str(row.get('sector_name') or '').strip()
                if not sector_code or not sector_name:
                    continue
                FundSectorMarketSnapshot.objects.update_or_create(
                    snapshot_time=snapshot_time,
                    board_code=current_board_code,
                    sector_code=sector_code,
                    source='stock_api_akshare',
                    defaults={
                        'trade_date': trade_date,
                        'sector_name': sector_name,
                        'latest_price': _decimal_or_none(row.get('latest_price')),
                        'change_amount': _decimal_or_none(row.get('change_amount')),
                        'change_percent': _decimal_or_none(row.get('change_percent')),
                        'turnover_rate': _decimal_or_none(row.get('turnover_rate')),
                        'amplitude': _decimal_or_none(row.get('amplitude')),
                        'volume': str(row.get('up_count') or ''),
                        'amount': str(row.get('down_count') or ''),
                        'leading_stock_code': '',
                        'leading_stock_name': row.get('leading_stock_name') or '',
                        'five_day_change': None,
                        'twenty_day_change': None,
                        'sixty_day_change': None,
                        'fifty_two_week_change': None,
                        'ytd_change': None,
                        'is_close_snapshot': is_close_snapshot,
                        'raw_data': row,
                    },
                )
                saved += 1

    return {
        'count': saved,
        'total': total,
        'snapshot_time': snapshot_time.isoformat(),
        'trade_date': trade_date.isoformat(),
        'board_codes': board_codes,
        'source': 'stock_api_akshare',
        'is_close_snapshot': is_close_snapshot,
        'errors': errors,
    }


def _normalize_sector_name(value):
    return (value or '').strip().replace(' ', '')


def _build_sector_index(sector_rows):
    exact = {}
    normalized = {}
    for row in sector_rows:
        exact[row.sector_name or ''] = row
        normalized[_normalize_sector_name(row.sector_name)] = row
    return exact, normalized


def _match_sector_row(name, exact_map, normalized_map):
    if not name:
        return None
    if name in exact_map:
        return exact_map[name]
    normalized = _normalize_sector_name(name)
    if normalized in normalized_map:
        return normalized_map[normalized]
    for key, row in exact_map.items():
        if name in key or key in name:
            return row
    return None


def _candidate_sector_keywords(name):
    mapping = {
        '制造业': ['半导体', '电子', '汽车', '电池', '消费电子', '机械', '家电', '食品饮料', '医药'],
        '采矿业': ['煤炭', '有色金属', '贵金属', '油气', '石油'],
        '金融业': ['银行', '保险', '证券'],
        '信息传输、软件和信息技术服务业': ['软件', '互联网', '通信', '计算机', '人工智能'],
        '科学研究和技术服务业': ['创新药', '医药', '研发', '检测'],
        '电力、热力、燃气及水生产和供应业': ['电力', '新能源', '光伏', '风电', '燃气'],
        '交通运输、仓储和邮政业': ['物流', '航运', '航空', '铁路', '港口'],
        '房地产业': ['房地产', '物业'],
        '建筑业': ['建筑', '基建', '水泥', '工程'],
        '批发和零售业': ['零售', '商业', '电商'],
        '农、林、牧、渔业': ['农业', '养殖', '种植', '猪肉'],
        '文化、体育和娱乐业': ['传媒', '游戏', '影视'],
    }
    return mapping.get(name, [])


def _fallback_sector_rows(name, sector_rows, limit=3):
    keywords = _candidate_sector_keywords(name)
    if not keywords:
        return []
    matches = []
    for row in sector_rows:
        sector_name = row.sector_name or ''
        if any(keyword in sector_name for keyword in keywords):
            matches.append(row)
    matches.sort(key=lambda row: float(row.change_percent or 0), reverse=True)
    return matches[:limit]


def analyze_fund_sector_rotation(
    fund_codes=None,
    trade_date=None,
    board_code='industry',
    close_only=True,
    page_size=200,
):
    allocation_qs = FundAllocationSnapshot.objects.select_related('fund').filter(allocation_type='industry')
    if fund_codes:
        allocation_qs = allocation_qs.filter(fund__fund_code__in=fund_codes)

    latest_report_dates = {}
    for row in allocation_qs.order_by('fund__fund_code', '-report_date', '-ratio'):
        latest_report_dates.setdefault(row.fund.fund_code, row.report_date)

    if not latest_report_dates:
        return {
            'trade_date': trade_date,
            'board_code': board_code,
            'count': 0,
            'items': [],
            'generated_at': timezone.now(),
        }

    allocation_rows = []
    for fund_code, report_date in latest_report_dates.items():
        allocation_rows.extend(
            list(allocation_qs.filter(fund__fund_code=fund_code, report_date=report_date).order_by('-ratio', 'name'))
        )

    sector_qs = FundSectorMarketSnapshot.objects.filter(board_code=board_code)
    if trade_date:
        sector_qs = sector_qs.filter(trade_date=trade_date)
    base_sector_qs = sector_qs
    if close_only:
        sector_qs = sector_qs.filter(is_close_snapshot=True)
    latest_snapshot = sector_qs.order_by('-snapshot_time').values_list('snapshot_time', flat=True).first()
    if close_only and not latest_snapshot:
        sector_qs = base_sector_qs
        latest_snapshot = sector_qs.order_by('-snapshot_time').values_list('snapshot_time', flat=True).first()
    if latest_snapshot:
        sector_qs = sector_qs.filter(snapshot_time=latest_snapshot)
    sector_rows = list(sector_qs.order_by('-change_percent', 'sector_name'))
    exact_map, normalized_map = _build_sector_index(sector_rows)

    grouped = {}
    for item in allocation_rows:
        matched_rows = []
        matched = _match_sector_row(item.name, exact_map, normalized_map)
        if matched:
            matched_rows = [matched]
        else:
            matched_rows = _fallback_sector_rows(item.name, sector_rows)
        if not matched_rows:
            continue
        payload = grouped.setdefault(item.fund.fund_code, {
            'fund_code': item.fund.fund_code,
            'fund_name': item.fund.fund_name,
            'fund_type': item.fund.fund_type,
            'fund_size': item.fund.fund_size,
            'fund_size_text': item.fund.fund_size_text,
            'report_date': item.report_date,
            'trade_date': matched_rows[0].trade_date,
            'snapshot_time': matched_rows[0].snapshot_time,
            'board_code': board_code,
            'analysis_mode': 'close' if close_only else 'latest',
            'sectors': [],
        })
        for matched in matched_rows:
            payload['sectors'].append({
                'allocation_name': item.name,
                'allocation_ratio': item.ratio,
                'matched_sector_name': matched.sector_name,
                'matched_sector_code': matched.sector_code,
                'change_percent': matched.change_percent,
                'five_day_change': matched.five_day_change,
                'twenty_day_change': matched.twenty_day_change,
                'sixty_day_change': matched.sixty_day_change,
                'ytd_change': matched.ytd_change,
                'leading_stock_name': matched.leading_stock_name,
                'leading_stock_code': matched.leading_stock_code,
                'source': matched.source,
            })

    items = []
    for _, payload in grouped.items():
        sectors_sorted = sorted(
            payload['sectors'],
            key=lambda row: (
                -(float(row['allocation_ratio'] or 0)),
                -(float(row['change_percent'] or 0)),
            ),
        )
        total_ratio = sum(float(row['allocation_ratio'] or 0) for row in sectors_sorted) or 0
        weighted_change = Decimal('0')
        if total_ratio > 0:
            for row in sectors_sorted:
                ratio = Decimal(str(row['allocation_ratio'] or 0))
                change = Decimal(str(row['change_percent'] or 0))
                weighted_change += (ratio / Decimal(str(total_ratio))) * change

        top_sector = sectors_sorted[0] if sectors_sorted else None
        payload['sectors'] = sectors_sorted[:8]
        payload['sector_count'] = len(sectors_sorted)
        payload['weighted_change_percent'] = weighted_change
        payload['rotation_signal'] = (
            '板块走强'
            if weighted_change >= Decimal('1.5')
            else '板块走弱'
            if weighted_change <= Decimal('-1.5')
            else '板块震荡'
        )
        payload['primary_sector'] = top_sector['matched_sector_name'] if top_sector else ''
        payload['primary_sector_change_percent'] = top_sector['change_percent'] if top_sector else None
        payload['analysis_summary'] = (
            f"{payload['primary_sector'] or '未识别主板块'}"
            f" / 加权涨跌 {weighted_change.quantize(Decimal('0.01'))}%"
            f" / 命中 {payload['sector_count']} 个归属板块"
        )
        items.append(payload)

    items.sort(
        key=lambda row: (
            -(float(row['weighted_change_percent'] or 0)),
            -(float(row['fund_size'] or 0)),
            row['fund_code'],
        )
    )

    return {
        'trade_date': trade_date or (items[0]['trade_date'] if items else None),
        'board_code': board_code,
        'snapshot_time': latest_snapshot,
        'count': len(items),
        'items': items[:page_size],
        'generated_at': timezone.now(),
    }
