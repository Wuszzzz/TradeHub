"""
更新基金净值命令

从数据源更新基金的最新净值
"""
import logging
from datetime import date
from concurrent.futures import ThreadPoolExecutor, as_completed
from django.core.management.base import BaseCommand
from api.sources import SourceRegistry
from api.models import Fund

logger = logging.getLogger(__name__)

DEFAULT_NAV_SOURCES = ('eastmoney', 'tencent_fund', 'yangjibao', 'xiaobeiyangji')
EXCLUDED_NAV_SOURCES = {'sina'}


def _source_name(source):
    try:
        return source.get_source_name()
    except Exception:
        return source.__class__.__name__


def _resolve_sources(source_option):
    """
    Resolve NAV sources in a stable order.

    Default keeps EastMoney first, then other fund NAV sources as fallback.
    Passing --source all includes all registered non-market quote sources.
    """
    registered = SourceRegistry.list_sources()
    if source_option == 'all':
        source_names = [name for name in registered if name not in EXCLUDED_NAV_SOURCES]
    else:
        requested = [item.strip() for item in source_option.split(',') if item.strip()]
        source_names = []
        for name in requested:
            if name == 'all':
                source_names.extend([item for item in registered if item not in EXCLUDED_NAV_SOURCES])
            else:
                source_names.append(name)

    sources = []
    seen = set()
    for name in source_names:
        if name in seen or name in EXCLUDED_NAV_SOURCES:
            continue
        seen.add(name)
        source = SourceRegistry.get_source(name)
        if source:
            sources.append(source)
        else:
            logger.warning(f'净值数据源未注册，已跳过：{name}')
    return sources


def _fetch_nav_from_source(source, fund_code, use_today, today):
    """从单个数据源获取净值，单源失败不影响其他源"""
    source_name = _source_name(source)
    try:
        if use_today:
            # 某些数据源可能没有 fetch_today_nav，需要判断
            if hasattr(source, 'fetch_today_nav'):
                data = source.fetch_today_nav(fund_code)
            else:
                data = source.fetch_realtime_nav(fund_code)
            
            if not data or data['nav_date'] != today:
                return None
        else:
            data = source.fetch_realtime_nav(fund_code)
            if not data:
                return None
        return {'source': source_name, 'data': data, 'error': None}
    except Exception as exc:
        return {'source': source_name, 'data': None, 'error': str(exc)}


def _fetch_best_nav(fund_code, use_today, today, sources):
    """
    并发从多个数据源获取净值，返回 nav_date 最新的那条。
    """
    results = []
    failures = []
    if not sources:
        return None, failures

    with ThreadPoolExecutor(max_workers=len(sources) or 1) as executor:
        futures = {executor.submit(_fetch_nav_from_source, s, fund_code, use_today, today): s for s in sources}
        for future in as_completed(futures):
            try:
                result = future.result()
                if result.get('data'):
                    data = result['data']
                    data['_source'] = result.get('source')
                    results.append(data)
                elif result.get('error'):
                    failures.append({'source': result.get('source'), 'error': result.get('error')})
            except Exception as exc:
                source = futures[future]
                failures.append({'source': _source_name(source), 'error': str(exc)})

    if not results:
        return None, failures

    # 取 nav_date 最新的
    return max(results, key=lambda d: d['nav_date']), failures


class Command(BaseCommand):
    help = '更新基金净值'

    def add_arguments(self, parser):
        parser.add_argument(
            '--fund_code',
            type=str,
            help='指定基金代码（可选，不指定则更新所有基金）',
        )
        parser.add_argument(
            '--today',
            action='store_true',
            help='获取当日确认净值（从历史净值接口），只有当日净值才更新',
        )
        parser.add_argument(
            '--source',
            type=str,
            default=','.join(DEFAULT_NAV_SOURCES),
            help='净值数据源，逗号分隔；默认 eastmoney,tencent_fund,yangjibao,xiaobeiyangji，可传 all',
        )

    def handle(self, *args, **options):
        fund_code = options.get('fund_code')
        use_today = options.get('today', False)
        source_option = options.get('source') or ','.join(DEFAULT_NAV_SOURCES)
        sources = _resolve_sources(source_option)
        source_names = [_source_name(source) for source in sources]

        if not sources:
            self.stdout.write(self.style.ERROR(f'没有可用净值数据源：{source_option}'))
            return

        if fund_code:
            self.stdout.write(f'开始更新基金 {fund_code} 的净值，数据源：{", ".join(source_names)}')
            funds = Fund.objects.filter(fund_code=fund_code)
            if not funds.exists():
                self.stdout.write(self.style.ERROR(f'基金 {fund_code} 不存在'))
                return
        else:
            mode = '当日净值' if use_today else '昨日净值'
            self.stdout.write(f'开始更新所有基金的{mode}（多源容错取最新），数据源：{", ".join(source_names)}')
            funds = Fund.objects.all()

        today = date.today()
        success_count = 0
        error_count = 0
        skip_count = 0
        source_failure_count = 0

        for fund in funds:
            try:
                data, source_failures = _fetch_best_nav(fund.fund_code, use_today, today, sources)
                source_failure_count += len(source_failures)
                if source_failures:
                    logger.warning(
                        f'基金 {fund.fund_code} 部分净值源失败，已继续其他源：{source_failures[:5]}'
                    )

                if not data:
                    skip_count += 1
                    continue

                new_date = data.get('nav_date')
                # 核心保护：只在日期更新或相等时保存，防止旧数据覆盖新数据
                if not fund.latest_nav_date or (new_date and new_date >= fund.latest_nav_date):
                    fund.latest_nav = data['nav']
                    fund.latest_nav_date = new_date
                    fund.save(update_fields=['latest_nav', 'latest_nav_date', 'updated_at'])
                    from api.services.daily_fact import upsert_daily_fact_from_latest_nav
                    upsert_daily_fact_from_latest_nav(fund, source=data.get('_source') or 'update_nav')
                    success_count += 1
                else:
                    skip_count += 1

                if fund_code:
                    self.stdout.write(
                        f'  {fund.fund_code}: {data["nav"]} ({data["nav_date"]}, 来源：{data.get("_source") or "unknown"})'
                    )

            except Exception as e:
                error_count += 1
                logger.error(f'更新基金 {fund.fund_code} 净值失败: {e}')
                if fund_code:
                    self.stdout.write(self.style.ERROR(f'  更新失败: {e}'))

        self.stdout.write(self.style.SUCCESS(
            f'更新完成：成功 {success_count} 个，跳过 {skip_count} 个，失败 {error_count} 个，单源失败 {source_failure_count} 次'
        ))
