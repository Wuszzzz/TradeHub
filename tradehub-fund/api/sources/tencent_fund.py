import requests
from decimal import Decimal
from datetime import date


class TencentFundSource:
    """腾讯基金详情页资产/重仓股接口"""

    ASSET_URL = 'https://zxg.txfund.com/ifzqgtimg/appstock/fund/baseInfo/asset'
    RANK_INFO_URL = 'https://web.ifzq.gtimg.cn/fund/newfund/fundBase/getRankInfo'

    def get_source_name(self) -> str:
        return 'tencent_fund'

    def fetch_asset(self, symbol: str) -> dict | None:
        try:
            resp = requests.get(
                self.ASSET_URL,
                params={'code': symbol},
                headers={
                    'User-Agent': 'Mozilla/5.0',
                    'Referer': 'https://gu.qq.com/',
                },
                timeout=15,
            )
            resp.raise_for_status()
            data = resp.json()
            if data.get('code') != 0:
                return None
            return data.get('data') or None
        except Exception:
            return None

    def fetch_rank_info(self, symbol: str) -> dict | None:
        try:
            resp = requests.get(
                self.RANK_INFO_URL,
                params={'symbol': symbol},
                headers={
                    'User-Agent': 'Mozilla/5.0',
                    'Referer': 'https://gu.qq.com/',
                },
                timeout=15,
            )
            resp.raise_for_status()
            data = resp.json()
            if data.get('code') != 0:
                return None
            return data.get('data') or None
        except Exception:
            return None

    def fetch_profile(self, symbol: str) -> dict:
        asset = self.fetch_asset(symbol) or {}
        rank_info = self.fetch_rank_info(symbol) or {}
        report_date = None
        if asset.get('report_time'):
            try:
                report_date = date.fromisoformat(asset['report_time'])
            except Exception:
                report_date = None
        return {
            'symbol': symbol,
            'source': 'tencent_fund',
            'report_date': report_date,
            'asset': asset.get('asset') or [],
            'industry': asset.get('industry') or [],
            'rank_info': rank_info,
            'raw_data': {
                'asset': asset,
                'rank_info': rank_info,
            },
        }

    def fetch_index_holdings(self, symbol: str) -> list:
        data = self.fetch_asset(symbol)
        if not data:
            return []

        result = []
        for item in data.get('stock') or []:
            result.append({
                'stock_code': item.get('code'),
                'stock_name': item.get('name'),
                'weight': Decimal(str(item.get('ratio'))),
                'price': None,
                'change_percent': Decimal(str(item.get('rate'))) if item.get('rate') not in (None, '', '--') else None,
                'holding_type': 'stock',
            })

        # 贵金属 ETF 等没有股票仓位时，退化到 product
        if not result:
          for item in data.get('product') or []:
              ratio = item.get('ratio')
              if ratio in (None, '', '--'):
                  continue
              result.append({
                  'stock_code': item.get('code'),
                  'stock_name': item.get('name'),
                  'weight': Decimal(str(ratio)),
                  'price': None,
                  'change_percent': Decimal(str(item.get('rate'))) if item.get('rate') not in (None, '', '--') else None,
                  'holding_type': 'product',
              })
        return result
