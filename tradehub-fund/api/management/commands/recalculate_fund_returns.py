from django.core.management.base import BaseCommand

from api.models import Fund
from api.services.fund_returns import recalculate_fund_returns


class Command(BaseCommand):
    help = '重算基金主表区间收益字段'

    def add_arguments(self, parser):
        parser.add_argument('--fund-code', type=str, help='指定基金代码')
        parser.add_argument('--limit', type=int, help='最多处理多少只基金')
        parser.add_argument('--offset', type=int, default=0, help='偏移量')
        parser.add_argument(
            '--include-source-periods',
            action='store_true',
            help='同时用本地净值历史覆盖 1月/3月/1年。默认只重算最近30天和今年以来，保留第三方同步的1月/3月/1年。',
        )

    def handle(self, *args, **options):
        fund_code = options.get('fund_code')
        offset = options.get('offset') or 0
        limit = options.get('limit')
        include_source_periods = options.get('include_source_periods', False)

        queryset = Fund.objects.order_by('fund_code')
        if fund_code:
            queryset = queryset.filter(fund_code=fund_code)
        if offset:
            queryset = queryset[offset:]
        if limit:
            queryset = queryset[:limit]

        funds = list(queryset)
        mode = '全字段NAV重算' if include_source_periods else '重算最近30天和今年以来'
        self.stdout.write(f'开始重算 {len(funds)} 只基金的区间收益（{mode}）')
        for fund in funds:
            result = recalculate_fund_returns(fund, include_source_periods=include_source_periods)
            self.stdout.write(
                f'{fund.fund_code}: 30d={result["return_30d"]} '
                f'1m={result["return_1m"]} 3m={result["return_3m"]} '
                f'1y={result["return_1y"]} ytd={result["return_this_year"]}'
            )
        self.stdout.write(self.style.SUCCESS('重算完成'))
