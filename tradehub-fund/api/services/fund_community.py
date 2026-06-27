import re
import time
from html import unescape
from urllib.parse import urljoin

import requests


EASTMONEY_GUBA_BASE = 'https://guba.eastmoney.com/'
COMMUNITY_CACHE_TTL_SECONDS = 60
_COMMUNITY_CACHE = {}


def _clean_text(value):
    text = re.sub(r'<[^>]+>', '', value or '')
    text = unescape(text)
    return re.sub(r'\s+', ' ', text).strip()


def _parse_number(value):
    value = _clean_text(value)
    if not value or value == '-':
        return None
    try:
        return int(float(value.replace(',', '').replace('万', '')) * (10000 if '万' in value else 1))
    except Exception:
        return None


def fetch_eastmoney_fund_community(fund_code, limit=20):
    """Fetch recent Eastmoney fund bar posts dynamically. Nothing is persisted."""
    code = str(fund_code or '').strip()
    cache_key = ('eastmoney_guba', code, int(limit))
    cached = _COMMUNITY_CACHE.get(cache_key)
    if cached and time.time() - cached['ts'] < COMMUNITY_CACHE_TTL_SECONDS:
        payload = dict(cached['payload'])
        payload['cache_hit'] = True
        return payload

    page_url = urljoin(EASTMONEY_GUBA_BASE, f'list,of{code}.html')
    headers = {
        'User-Agent': 'Mozilla/5.0',
        'Referer': 'https://fund.eastmoney.com/',
    }
    response = requests.get(page_url, headers=headers, timeout=12)
    response.raise_for_status()
    response.encoding = response.apparent_encoding or 'utf-8'
    html = response.text

    items = []
    rows = re.findall(
        r'<div[^>]*class="[^"]*articleh[^"]*"[^>]*>(.*?)</div>',
        html,
        re.IGNORECASE | re.DOTALL,
    )
    if not rows:
        rows = re.findall(
            r'<tr[^>]*class="[^"]*(?:listitem|articleh)[^"]*"[^>]*>(.*?)</tr>',
            html,
            re.IGNORECASE | re.DOTALL,
        )

    for row in rows:
        if 'settop' in row:
            continue
        link_match = re.search(r'<a[^>]+href="([^"]+)"[^>]*title="([^"]+)"[^>]*>', row, re.IGNORECASE | re.DOTALL)
        if not link_match:
            link_match = re.search(r'<a[^>]+href="([^"]+)"[^>]*>(.*?)</a>', row, re.IGNORECASE | re.DOTALL)
        if not link_match:
            continue
        href, title_html = link_match.groups()
        if f'news,of{code},' not in href:
            continue
        title = _clean_text(title_html)
        if not title or title in {'标题', '帖子'}:
            continue
        cells = re.findall(r'<(?:td|span)[^>]*class="l([1-5])"[^>]*>(.*?)</(?:td|span)>', row, re.IGNORECASE | re.DOTALL)
        cell_map = {idx: value for idx, value in cells}
        author = ''
        posted_at = ''
        read_count = None
        reply_count = None
        if cell_map:
            read_count = _parse_number(cell_map.get('1'))
            reply_count = _parse_number(cell_map.get('2'))
            author = _clean_text(cell_map.get('4'))
            posted_at = _clean_text(cell_map.get('5'))
        else:
            texts = [_clean_text(part) for part in re.findall(r'<span[^>]*>(.*?)</span>', row, re.IGNORECASE | re.DOTALL)]
            texts = [item for item in texts if item]
            if texts:
                posted_at = texts[-1]
            if len(texts) > 1:
                author = texts[-2]
        items.append({
            'title': title,
            'url': urljoin(EASTMONEY_GUBA_BASE, href),
            'author': author,
            'posted_at': posted_at,
            'read_count': read_count,
            'reply_count': reply_count,
            'source': 'eastmoney_guba',
            'source_name': '东方财富基金吧',
        })
        if len(items) >= limit:
            break

    payload = {
        'fund_code': code,
        'source': 'eastmoney_guba',
        'source_name': '东方财富基金吧',
        'community_url': page_url,
        'items': items,
        'count': len(items),
        'cache_hit': False,
    }
    _COMMUNITY_CACHE[cache_key] = {'ts': time.time(), 'payload': payload}
    return payload


def fetch_fund_community(fund_code, source='eastmoney_guba', limit=20):
    if source not in ('eastmoney_guba', 'eastmoney'):
        raise ValueError(f'unsupported community source: {source}')
    return fetch_eastmoney_fund_community(fund_code, limit=limit)
