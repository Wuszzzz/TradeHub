from django.core.management.base import BaseCommand

from api.services.daily_fact import backfill_daily_facts


class Command(BaseCommand):
    help = '从历史净值和估值准确率回填基金每日事实表'

    def add_arguments(self, parser):
        parser.add_argument('--fund_code', action='append', dest='fund_codes', help='可重复传入基金代码')
        parser.add_argument('--limit', type=int, help='限制基金数量')

    def handle(self, *args, **options):
        count = backfill_daily_facts(
            fund_codes=options.get('fund_codes'),
            limit=options.get('limit'),
        )
        self.stdout.write(self.style.SUCCESS(f'回填完成：{count} 条事实记录'))
