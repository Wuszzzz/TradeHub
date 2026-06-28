"""
同步基金历史净值管理命令
"""
from django.core.management.base import BaseCommand
from datetime import date, datetime, timedelta

from api.services.nav_history import sync_nav_history, batch_sync_nav_history
from api.models import Fund


class Command(BaseCommand):
    help = '同步基金历史净值'

    def add_arguments(self, parser):
        parser.add_argument(
            '--fund-code',
            type=str,
            help='基金代码（可选，不指定则同步所有基金）'
        )
        parser.add_argument(
            '--start-date',
            type=str,
            help='开始日期（格式：YYYY-MM-DD）'
        )
        parser.add_argument(
            '--end-date',
            type=str,
            help='结束日期（格式：YYYY-MM-DD）'
        )
        parser.add_argument(
            '--days',
            type=int,
            help='回看天数。未指定 start-date 时生效，例如 --days 365'
        )
        parser.add_argument(
            '--limit',
            type=int,
            help='最多同步多少只基金。适合首部署分批回补'
        )
        parser.add_argument(
            '--offset',
            type=int,
            default=0,
            help='从基金列表偏移位置开始同步，配合 --limit 分片'
        )
        parser.add_argument(
            '--batch-size',
            type=int,
            default=200,
            help='批次大小，用于输出进度和降低单次失败影响'
        )
        parser.add_argument(
            '--force',
            action='store_true',
            help='强制全量同步'
        )

    def handle(self, *args, **options):
        fund_code = options.get('fund_code')
        start_date = options.get('start_date')
        end_date = options.get('end_date')
        days = options.get('days')
        limit = options.get('limit')
        offset = options.get('offset') or 0
        batch_size = max(options.get('batch_size') or 200, 1)
        force = options.get('force', False)

        # 转换日期
        if start_date:
            start_date = datetime.strptime(start_date, '%Y-%m-%d').date()
        if end_date:
            end_date = datetime.strptime(end_date, '%Y-%m-%d').date()
        else:
            end_date = date.today()
        if days and not start_date:
            start_date = end_date - timedelta(days=days)

        if fund_code:
            # 同步单个基金
            self.stdout.write(f'开始同步基金 {fund_code}，区间 {start_date or "增量"} ~ {end_date}...')
            count = sync_nav_history(fund_code, start_date, end_date, force)
            self.stdout.write(
                self.style.SUCCESS(f'同步完成，新增/更新 {count} 条记录')
            )
        else:
            # 同步所有基金
            fund_codes_qs = Fund.objects.order_by('fund_code').values_list('fund_code', flat=True)
            if offset:
                fund_codes_qs = fund_codes_qs[offset:]
            if limit:
                fund_codes_qs = fund_codes_qs[:limit]
            fund_codes = list(fund_codes_qs)
            total = len(fund_codes)
            self.stdout.write(
                f'开始同步 {total} 个基金，区间 {start_date or "增量"} ~ {end_date}，'
                f'offset={offset}，limit={limit or "全部"}，batch_size={batch_size}'
            )

            results = {}
            for index in range(0, total, batch_size):
                batch = fund_codes[index:index + batch_size]
                batch_no = index // batch_size + 1
                batch_total = (total + batch_size - 1) // batch_size
                self.stdout.write(f'批次 {batch_no}/{batch_total}: {batch[0]} ~ {batch[-1]}，数量 {len(batch)}')
                batch_results = batch_sync_nav_history(batch, start_date, end_date)
                results.update(batch_results)
                batch_success = sum(1 for r in batch_results.values() if r['success'])
                batch_records = sum(r.get('count', 0) for r in batch_results.values() if r['success'])
                batch_failed = len(batch_results) - batch_success
                self.stdout.write(
                    f'批次 {batch_no}/{batch_total} 完成：成功 {batch_success}，失败 {batch_failed}，新增/更新 {batch_records} 条'
                )

            success_count = sum(1 for r in results.values() if r['success'])
            total_records = sum(r.get('count', 0) for r in results.values() if r['success'])
            failed_count = len(results) - success_count

            self.stdout.write(
                self.style.SUCCESS(
                    f'同步完成：成功 {success_count}/{total} 个基金，失败 {failed_count} 个基金，'
                    f'新增/更新 {total_records} 条记录，区间 {start_date or "增量"} ~ {end_date}'
                )
            )
