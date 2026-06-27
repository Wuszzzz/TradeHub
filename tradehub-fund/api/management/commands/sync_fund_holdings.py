from django.core.management.base import BaseCommand

from api.models import Fund
from api.services.fund_profile import sync_fund_holdings_snapshot


class Command(BaseCommand):
    help = '同步基金披露持仓快照'

    def add_arguments(self, parser):
        parser.add_argument('--fund_code', type=str, help='指定基金代码')
        parser.add_argument('--limit', type=int, default=100, help='批量同步数量上限')
        parser.add_argument('--source', type=str, default='tencent_fund', help='数据源')

    def handle(self, *args, **options):
        fund_code = options.get('fund_code')
        source = options.get('source') or 'tencent_fund'
        limit = options.get('limit') or 100

        funds = Fund.objects.all().order_by('fund_code')
        if fund_code:
            funds = funds.filter(fund_code=fund_code)
        else:
            funds = funds[:limit]

        success = 0
        skipped = 0
        failed = 0
        for fund in funds:
            try:
                result = sync_fund_holdings_snapshot(fund.fund_code, source_name=source)
                if result.get('success'):
                    success += 1
                    self.stdout.write(self.style.SUCCESS(
                        f'{fund.fund_code} synced {result.get("count", 0)} holdings'
                    ))
                else:
                    skipped += 1
                    self.stdout.write(f'{fund.fund_code} skipped: no holdings')
            except Exception as exc:
                failed += 1
                self.stdout.write(self.style.ERROR(f'{fund.fund_code} failed: {exc}'))

        self.stdout.write(self.style.SUCCESS(
            f'完成：成功 {success}，跳过 {skipped}，失败 {failed}'
        ))
