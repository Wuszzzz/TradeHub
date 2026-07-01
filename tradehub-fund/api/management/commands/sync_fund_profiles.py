"""
同步基金基础画像和第三方周期收益字段。
"""
from django.core.management.base import BaseCommand

from api.models import Fund
from api.services.fund_profile import sync_fund_basic_profile


class Command(BaseCommand):
    help = '同步基金基础画像，包括腾讯源的近1月/近3月/近1年/今年以来收益'

    def add_arguments(self, parser):
        parser.add_argument('--fund-code', type=str, help='指定基金代码')
        parser.add_argument('--source', type=str, default='tencent_fund', help='画像数据源，默认 tencent_fund')
        parser.add_argument('--limit', type=int, help='最多处理多少只基金')
        parser.add_argument('--offset', type=int, default=0, help='偏移量')

    def handle(self, *args, **options):
        fund_code = options.get('fund_code')
        source_name = options.get('source') or 'tencent_fund'
        offset = options.get('offset') or 0
        limit = options.get('limit')

        queryset = Fund.objects.order_by('fund_code').values_list('fund_code', flat=True)
        if fund_code:
            queryset = queryset.filter(fund_code=fund_code)
        if offset:
            queryset = queryset[offset:]
        if limit:
            queryset = queryset[:limit]

        fund_codes = list(queryset)
        self.stdout.write(f'开始同步 {len(fund_codes)} 只基金画像，source={source_name}')

        success_count = 0
        failed_count = 0
        for code in fund_codes:
            try:
                result = sync_fund_basic_profile(code, source_name=source_name)
                success_count += 1
                self.stdout.write(
                    f'{code}: company={result.get("company") or "-"} '
                    f'ranks={result.get("rank_count", 0)} allocations={result.get("allocation_count", 0)}'
                )
            except Exception as exc:
                failed_count += 1
                self.stdout.write(self.style.ERROR(f'{code}: 同步失败：{exc}'))

        self.stdout.write(self.style.SUCCESS(
            f'同步完成：成功 {success_count}，失败 {failed_count}'
        ))
