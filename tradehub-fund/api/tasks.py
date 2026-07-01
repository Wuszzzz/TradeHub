"""
Celery 任务

定义所有后台异步任务
"""
from celery import shared_task
from django.core.management import call_command
import logging
import requests
from datetime import date, timedelta

logger = logging.getLogger(__name__)


def _active_fund_codes(limit=200):
    """基金自动任务默认只处理持仓和自选里的活跃基金。"""
    from django.db.models import Q
    from api.models import Fund, FundHoldingSnapshot, FundNavHistory, Position, WatchlistItem

    position_codes = Position.objects.values_list('fund__fund_code', flat=True)
    watchlist_codes = WatchlistItem.objects.values_list('fund__fund_code', flat=True)
    codes = list(
        Fund.objects
        .filter(fund_code__in=list(position_codes) + list(watchlist_codes))
        .order_by('fund_code')
        .values_list('fund_code', flat=True)
        .distinct()
        [:limit]
    )
    if codes:
        return codes

    touched_codes = set(FundHoldingSnapshot.objects.values_list('fund__fund_code', flat=True))
    touched_codes.update(FundNavHistory.objects.values_list('fund__fund_code', flat=True))
    touched = list(
        Fund.objects
        .filter(Q(fund_code__in=touched_codes) | Q(profile_updated_at__isnull=False) | Q(latest_nav_date__isnull=False))
        .order_by('fund_code')
        .values_list('fund_code', flat=True)
        .distinct()
        [:limit]
    )
    if touched:
        return touched

    return list(Fund.objects.order_by('fund_code').values_list('fund_code', flat=True)[:limit])


@shared_task
def sync_funds(if_empty=False):
    """每日全量刷新基金基础列表。"""
    try:
        args = ['--if-empty'] if if_empty else []
        call_command('sync_funds', *args)
        logger.info('基金基础列表同步完成')
        return {'success': True, 'if_empty': if_empty}
    except Exception as e:
        logger.error(f'基金基础列表同步失败: {str(e)}')
        raise


@shared_task
def update_fund_nav():
    """
    定时更新基金净值（昨日净值）
    
    默认从数据源获取最新可用的历史净值并同步到基金主表。
    """
    try:
        call_command('update_nav')
        logger.info('基金昨日/最新净值同步完成')
        return '净值同步完成'
    except Exception as e:
        logger.error(f'基金净值自动更新失败: {str(e)}')
        raise


@shared_task
def update_fund_today_nav():
    """
    定时更新基金当日确认净值
    
    每天晚间执行，尝试从确权接口抓取今日净值。
    """
    try:
        call_command('update_nav', '--today')
        logger.info('基金今日净值确权完成')
        return '当日净值更新完成'
    except Exception as e:
        logger.error(f'基金当日净值确权失败: {str(e)}')
        raise


@shared_task
def sync_active_fund_profiles_and_holdings(limit=200):
    """
    自动同步活跃基金资料和披露持仓。

    活跃范围：已有持仓和自选列表中的基金，避免每天扫描全量基金库。
    """
    from api.services.fund_profile import sync_fund_basic_profile, sync_fund_holdings_snapshot

    codes = _active_fund_codes(limit=limit)
    success = 0
    skipped = 0
    failed = 0
    errors = []

    for fund_code in codes:
        try:
            sync_fund_basic_profile(fund_code, source_name='tencent_fund')
            result = sync_fund_holdings_snapshot(fund_code, source_name='tencent_fund')
            if result.get('success'):
                success += 1
            else:
                skipped += 1
        except Exception as exc:
            failed += 1
            errors.append({'fund_code': fund_code, 'error': str(exc)})
            logger.warning(f'同步基金资料/持仓失败：{fund_code}, 错误：{exc}')

    summary = {
        'total': len(codes),
        'success': success,
        'skipped': skipped,
        'failed': failed,
        'errors': errors[:20],
    }
    logger.info(f'活跃基金资料/持仓同步完成：{summary}')
    return summary


@shared_task
def sync_active_fund_nav_history_and_facts(limit=200, days=14):
    """
    自动同步活跃基金历史净值，并写入每日事实表。

    日常增量默认回看 14 天，用于补齐节假日、晚到净值和修正数据。
    """
    from api.services.nav_history import batch_sync_nav_history

    codes = _active_fund_codes(limit=limit)
    end_date = date.today()
    start_date = end_date - timedelta(days=days)
    results = batch_sync_nav_history(codes, start_date=start_date, end_date=end_date)
    success = sum(1 for item in results.values() if item.get('success'))
    failed = len(results) - success
    count = sum(item.get('count', 0) for item in results.values() if item.get('success'))
    summary = {
        'total': len(codes),
        'success': success,
        'failed': failed,
        'count': count,
        'start_date': start_date.isoformat(),
        'end_date': end_date.isoformat(),
    }
    logger.info(f'活跃基金净值/日事实同步完成：{summary}')
    return summary


@shared_task
def sync_all_fund_nav_history_and_facts(days=7, batch_size=500, limit=None):
    """
    每日全量同步所有基金历史净值，并写入每日事实表。

    全量基金数量较大，按 batch_size 分批执行，单只基金内部仍复用幂等 upsert。
    """
    from api.models import Fund
    from api.services.nav_history import batch_sync_nav_history

    codes_qs = Fund.objects.order_by('fund_code').values_list('fund_code', flat=True)
    if limit:
        codes_qs = codes_qs[:limit]
    codes = list(codes_qs)
    end_date = date.today()
    start_date = end_date - timedelta(days=days)
    success = 0
    failed = 0
    count = 0
    errors = []

    for offset in range(0, len(codes), batch_size):
        batch = codes[offset:offset + batch_size]
        results = batch_sync_nav_history(batch, start_date=start_date, end_date=end_date)
        success += sum(1 for item in results.values() if item.get('success'))
        failed += sum(1 for item in results.values() if not item.get('success'))
        count += sum(item.get('count', 0) for item in results.values() if item.get('success'))
        errors.extend([
            {'fund_code': fund_code, 'error': item.get('error')}
            for fund_code, item in results.items()
            if not item.get('success')
        ])

    summary = {
        'total': len(codes),
        'success': success,
        'failed': failed,
        'count': count,
        'start_date': start_date.isoformat(),
        'end_date': end_date.isoformat(),
        'batch_size': batch_size,
        'errors': errors[:50],
    }
    logger.info(f'全量基金净值/日事实同步完成：{summary}')
    return summary


@shared_task
def sync_all_fund_profiles_and_holdings(batch_size=300, limit=None, sync_holdings=True):
    """
    每日全量同步基金资料、基金公司、规模、板块配置和披露持仓。

    sync_holdings 默认开启，按基金代码分批顺序执行；失败基金记录错误并继续下一只。
    """
    from api.models import Fund
    from api.services.fund_profile import sync_fund_basic_profile, sync_fund_holdings_snapshot

    codes_qs = Fund.objects.order_by('fund_code').values_list('fund_code', flat=True)
    if limit:
        codes_qs = codes_qs[:limit]
    codes = list(codes_qs)
    profile_success = 0
    holding_success = 0
    skipped = 0
    failed = 0
    errors = []

    for offset in range(0, len(codes), batch_size):
        batch = codes[offset:offset + batch_size]
        for fund_code in batch:
            try:
                sync_fund_basic_profile(fund_code, source_name='tencent_fund')
                profile_success += 1
                if sync_holdings:
                    result = sync_fund_holdings_snapshot(fund_code, source_name='tencent_fund')
                    if result.get('success'):
                        holding_success += 1
                    else:
                        skipped += 1
            except Exception as exc:
                failed += 1
                errors.append({'fund_code': fund_code, 'error': str(exc)})
                logger.warning(f'全量同步基金资料/持仓失败：{fund_code}, 错误：{exc}')

    summary = {
        'total': len(codes),
        'profile_success': profile_success,
        'holding_success': holding_success,
        'skipped': skipped,
        'failed': failed,
        'batch_size': batch_size,
        'sync_holdings': sync_holdings,
        'errors': errors[:50],
    }
    logger.info(f'全量基金资料/持仓同步完成：{summary}')
    return summary


@shared_task
def backfill_fund_daily_facts_task(limit=500):
    """低频回填每日事实表，修复历史净值和估值准确率之间的缺口。"""
    from api.services.daily_fact import backfill_daily_facts

    count = backfill_daily_facts(limit=limit)
    logger.info(f'基金日事实低频回填完成：{count}')
    return {'count': count, 'limit': limit}


@shared_task
def sync_top_fund_rankings_task(limit=500, sync_profiles_limit=500):
    """每日同步全市场基金各周期 Top 排行，写入本地排行快照表。"""
    from api.services.fund_rankings import sync_top_fund_rankings

    summary = sync_top_fund_rankings(limit=limit, sync_profiles_limit=sync_profiles_limit)
    logger.info(f'基金 Top 排行同步完成：{summary}')
    return summary


@shared_task
def sync_top_fund_rankings_intraday_task(limit=1000, sync_profiles_limit=1000):
    """盘中每 10 分钟同步前 1000 名基金排行，并补齐基础资料。"""
    from api.services.fund_rankings import sync_top_fund_rankings

    summary = sync_top_fund_rankings(limit=limit, sync_profiles_limit=sync_profiles_limit)
    logger.info(f'基金 Top 排行盘中同步完成：{summary}')
    return summary


@shared_task
def sync_top_fund_intraday_estimates_task(limit=1000, source_name='eastmoney'):
    """盘中刷新前排基金的今日实时估值涨幅，用于当天日榜。"""
    from concurrent.futures import ThreadPoolExecutor, as_completed
    from django.utils import timezone
    from api.models import Fund, FundPerformanceRankSnapshot
    from api.sources import SourceRegistry
    from api.services.daily_fact import upsert_latest_estimate_fact

    source = SourceRegistry.get_source(source_name) or SourceRegistry.get_source('eastmoney')
    if not source:
        return {'success': False, 'error': 'no estimate source'}

    latest_date = (
        FundPerformanceRankSnapshot.objects
        .filter(rank_type='performance', period='day')
        .order_by('-rank_date')
        .values_list('rank_date', flat=True)
        .first()
    )
    ranked_codes = []
    if latest_date:
        ranked_codes = list(
            FundPerformanceRankSnapshot.objects
            .filter(rank_type='performance', period='day', rank_date=latest_date)
            .order_by('rank')
            .values_list('fund__fund_code', flat=True)
            [:limit]
        )
    if not ranked_codes:
        ranked_codes = list(Fund.objects.order_by('fund_code').values_list('fund_code', flat=True)[:limit])

    fund_map = {fund.fund_code: fund for fund in Fund.objects.filter(fund_code__in=ranked_codes)}
    now = timezone.now()
    success = 0
    skipped = 0
    failed = 0
    errors = []

    def fetch(code):
        return code, source.fetch_estimate(code)

    with ThreadPoolExecutor(max_workers=12) as executor:
        futures = [executor.submit(fetch, code) for code in ranked_codes]
        for future in as_completed(futures):
            try:
                code, data = future.result()
                fund = fund_map.get(code)
                if not fund or not data or data.get('estimate_nav') is None:
                    skipped += 1
                    continue
                estimate_time = data.get('estimate_time')
                if estimate_time and timezone.is_naive(estimate_time):
                    estimate_time = timezone.make_aware(estimate_time, timezone.get_current_timezone())
                fund.estimate_nav = data.get('estimate_nav')
                fund.estimate_growth = data.get('estimate_growth')
                fund.estimate_time = estimate_time or now
                fund.save(update_fields=['estimate_nav', 'estimate_growth', 'estimate_time'])
                upsert_latest_estimate_fact(fund)
                success += 1
            except Exception as exc:
                failed += 1
                if len(errors) < 20:
                    errors.append(str(exc))

    summary = {
        'success': success,
        'skipped': skipped,
        'failed': failed,
        'limit': limit,
        'source': source_name,
        'rank_date': latest_date.isoformat() if latest_date else None,
        'errors': errors,
    }
    logger.info(f'基金盘中估值涨幅同步完成：{summary}')
    return summary


@shared_task
def sync_ranked_fund_basic_profiles_task(limit=1000):
    """同步已落库排行基金的基础资料、基金公司、规模和板块配置。"""
    from api.models import FundPerformanceRankSnapshot
    from api.services.fund_profile import sync_fund_basic_profiles

    codes = list(
        FundPerformanceRankSnapshot.objects
        .order_by('rank')
        .values_list('fund__fund_code', flat=True)
        .distinct()
        [:limit]
    )
    summary = sync_fund_basic_profiles(codes, limit=limit)
    logger.info(f'排行基金基础资料同步完成：{summary}')
    return summary


@shared_task
def sync_quarterly_fund_holdings_and_evaluations_task(limit=2000, source_name='tencent_fund', window_days=370):
    """每季度 1 号同步前 2000 只基金持仓，并触发 Go 评估快照重算。"""
    from api.models import Fund
    from api.services.fund_profile import sync_fund_holdings_snapshot

    codes = list(Fund.objects.order_by('fund_code').values_list('fund_code', flat=True)[:limit])
    holdings_success = 0
    holdings_skipped = 0
    holdings_failed = 0
    holding_errors = []

    for fund_code in codes:
        try:
            result = sync_fund_holdings_snapshot(fund_code, source_name=source_name)
            if result.get('success'):
                holdings_success += 1
            else:
                holdings_skipped += 1
        except Exception as exc:
            holdings_failed += 1
            holding_errors.append({'fund_code': fund_code, 'error': str(exc)})
            logger.warning(f'季度持仓同步失败：{fund_code}, 错误：{exc}')

    go_summary = {'triggered': False}
    endpoint = config.get('fund_research_url', '') or 'http://fund-research:18081'
    try:
        resp = requests.post(
            f'{endpoint.rstrip("/")}/api/fund-research/v1/sync/evaluations',
            json={'limit': limit, 'window_days': window_days},
            timeout=1800,
        )
        resp.raise_for_status()
        go_summary = {'triggered': True, 'response': resp.json()}
    except Exception as exc:
        go_summary = {'triggered': False, 'error': str(exc)}
        logger.error(f'季度 Go 评估重算失败: {exc}')

    summary = {
        'limit': limit,
        'source_name': source_name,
        'holdings_success': holdings_success,
        'holdings_skipped': holdings_skipped,
        'holdings_failed': holdings_failed,
        'holding_errors': holding_errors[:50],
        'go_evaluation': go_summary,
    }
    logger.info(f'季度基金持仓与评估同步完成：{summary}')
    return summary


@shared_task
def sync_fund_sector_market_snapshots_task(is_close_snapshot=False):
    """同步市场板块涨跌快照，用于基金板块管理。"""
    from api.services.fund_sector_market import sync_fund_sector_market_snapshots

    summary = sync_fund_sector_market_snapshots(
        board_codes=['industry', 'concept'],
        sort_type='f3',
        direct='down',
        count=500,
        is_close_snapshot=is_close_snapshot,
    )
    logger.info(f'基金板块市场快照同步完成：{summary}')
    return summary


@shared_task
def analyze_fund_sector_rotation_task(fund_codes=None, trade_date=None, board_code='industry', close_only=True, page_size=300):
    """按盘后板块涨幅分析基金当前归属板块强弱和切换信号。"""
    from api.services.fund_sector_market import analyze_fund_sector_rotation

    summary = analyze_fund_sector_rotation(
        fund_codes=fund_codes,
        trade_date=trade_date,
        board_code=board_code,
        close_only=close_only,
        page_size=page_size,
    )
    logger.info(
        '基金板块轮动分析完成：trade_date=%s board=%s count=%s',
        summary.get('trade_date'),
        board_code,
        summary.get('count'),
    )
    return summary


@shared_task
def capture_estimate_snapshot():
    """
    捕捉 15:00 收盘估值快照
    
    每个交易日 15:05 执行，将收盘估值锁定，用于晚间与真实净值对比计算误差。
    """
    from api.models import Fund, EstimateAccuracy
    from api.utils.trading_calendar import is_trading_day
    from django.utils import timezone

    today = timezone.localdate()
    if not is_trading_day(today):
        logger.info(f'{today} 不是交易日，跳过估值捕捉')
        return '非交易日'

    funds = Fund.objects.exclude(estimate_nav__isnull=True)
    count = 0
    for fund in funds:
        # 只捕捉当天的预估
        if fund.estimate_time and fund.estimate_time.date() == today:
            EstimateAccuracy.objects.update_or_create(
                source_name='eastmoney',
                fund=fund,
                estimate_date=today,
                defaults={
                    'estimate_nav': fund.estimate_nav
                }
            )
            count += 1

    logger.info(f'已捕捉 {count} 个基金的收盘估值快照')
    return f'捕捉完成：{count}'


@shared_task
def check_notification_rules():
    """
    检查通知规则并发送通知

    每 5 分钟执行一次，检查所有激活的通知规则，
    判断是否触发条件，发送通知并记录日志。
    """
    from django.utils import timezone
    from datetime import timedelta
    from decimal import Decimal
    from api.models import NotificationRule, NotificationLog
    from api.notifications import ChannelRegistry

    rules = NotificationRule.objects.filter(is_active=True).select_related(
        'fund', 'user'
    ).prefetch_related('channels')

    triggered = 0
    sent = 0

    for rule in rules:
        fund = rule.fund
        if fund.estimate_growth is None:
            continue

        growth = Decimal(str(fund.estimate_growth))

        # 判断是否触发
        triggered_flag = False
        if rule.rule_type == 'growth_up' and growth >= rule.threshold:
            triggered_flag = True
        elif rule.rule_type == 'growth_down' and growth <= -rule.threshold:
            triggered_flag = True

        if not triggered_flag:
            continue

        triggered += 1

        # 检查冷却时间
        cooldown_cutoff = timezone.now() - timedelta(minutes=rule.cooldown_minutes)
        recent_log = NotificationLog.objects.filter(
            rule=rule,
            trigger_time__gte=cooldown_cutoff,
            status='success',
        ).exists()

        if recent_log:
            logger.debug(f'规则 {rule.id} 在冷却期内，跳过')
            continue

        # 构建通知内容
        direction = '涨幅' if rule.rule_type == 'growth_up' else '跌幅'
        title = f'基金{direction}提醒：{fund.fund_name}'
        content = (
            f'{fund.fund_name}（{fund.fund_code}）当前{direction} {abs(growth):.2f}%，'
            f'已超过您设定的阈值 {rule.threshold}%。'
        )

        # 逐渠道发送
        for channel_obj in rule.channels.filter(is_active=True):
            channel_impl = ChannelRegistry.get_channel(channel_obj.channel_type)
            if not channel_impl:
                logger.warning(f'未找到渠道实现：{channel_obj.channel_type}')
                continue

            success = False
            error_msg = None
            try:
                success = channel_impl.send(title, content, channel_obj.config)
            except Exception as e:
                error_msg = str(e)
                logger.error(f'发送通知异常：rule={rule.id}, channel={channel_obj.id}, 错误：{e}')

            NotificationLog.objects.create(
                rule=rule,
                channel=channel_obj,
                fund_code=fund.fund_code,
                fund_name=fund.fund_name,
                growth=growth,
                status='success' if success else 'failed',
                error_message=error_msg,
            )

            if success:
                sent += 1

    logger.info(f'通知检查完成：触发 {triggered} 条规则，发送 {sent} 条通知')
    return f'触发 {triggered} 条，发送 {sent} 条'


@shared_task
def audit_accuracy():
    """
    审计估值准确率

    每个交易晚间执行，计算所有捕捉到的快照与最终净值的误差。
    """
    from api.utils.trading_calendar import is_trading_day
    from django.utils import timezone

    today = timezone.localdate()
    if not is_trading_day(today):
        logger.info(f'{today} 不是交易日，跳过准确率审计')
        return '非交易日'

    try:
        call_command('calculate_accuracy', date=today.isoformat())
        logger.info(f'{today} 准确率审计完成')
        return '审计完成'
    except Exception as e:
        logger.error(f'准确率审计失败: {str(e)}')
        raise


@shared_task
def capture_intraday_snapshots():
    """
    盘中定时抓取估值快照

    交易日内每 5 分钟执行一次（9:30-15:00），为所有有持仓/自选的基金抓取估值快照，
    用于绘制当日估值曲线。当天收盘后保留 7 天自动清理。
    """
    from datetime import timedelta
    from django.utils import timezone
    from api.models import Fund, EstimateSnapshot, Position
    from api.sources import SourceRegistry
    from api.utils.trading_calendar import is_trading_day

    today = timezone.localdate()
    if not is_trading_day(today):
        logger.info(f'{today} 不是交易日，跳过估值快照抓取')
        return '非交易日'

    now = timezone.now()
    # 只在交易时段执行
    market_open = now.replace(hour=9, minute=30, second=0)
    market_close = now.replace(hour=15, minute=5, second=0)
    if now < market_open or now > market_close:
        logger.info(f'{now.time()} 不在交易时段')
        return '非交易时段'

    # 清理 7 天前的旧快照
    cutoff = today - timedelta(days=7)
    deleted, _ = EstimateSnapshot.objects.filter(timestamp__date__lt=cutoff).delete()
    if deleted:
        logger.info(f'清理了 {deleted} 条过期快照')

    # 获取所有有持仓的基金
    fund_ids = Position.objects.values_list('fund_id', flat=True).distinct()
    funds = Fund.objects.filter(id__in=fund_ids)

    count = 0
    for fund in funds:
        source = SourceRegistry.get_source('eastmoney')
        if not source:
            continue
        try:
            data = source.fetch_estimate(fund.fund_code)
            if data and data.get('estimate_nav'):
                EstimateSnapshot.objects.create(
                    fund=fund,
                    source='eastmoney',
                    timestamp=now,
                    estimate_nav=data['estimate_nav'],
                    estimate_growth=data.get('estimate_growth'),
                )
                count += 1
        except Exception as e:
            logger.warning(f'抓取 {fund.fund_code} 估值快照失败: {e}')

    logger.info(f'已抓取 {count} 个基金的估值快照')
    return f'已抓取 {count} 个快照'


@shared_task
def generate_investment_reports():
    """
    定时生成投资报告

    遍历所有开启了报告的用户，生成 AI 投资周报/月报/年报并推送。
    根据用户设置的 report_frequency 判断是否应该生成（周报=周一，月报=1日，年报=1月1日）。
    """
    from django.contrib.auth import get_user_model
    from api.models import AIConfig, UserPreference
    from api.views import build_report_context, _replace_placeholders

    today = date.today()
    generated = 0
    skip_ai = 0
    skip_disabled = 0

    for pref in UserPreference.objects.filter(report_enabled=True).select_related('user'):
        user = pref.user

        # 检查频率是否匹配今天（支持逗号分隔多选）
        frequencies = [f.strip() for f in pref.report_frequency.split(',')]
        should_run = False
        if 'weekly' in frequencies and today.weekday() == 0:
            should_run = True
        if 'monthly' in frequencies and today.day == 1:
            should_run = True
        if 'yearly' in frequencies and today.month == 1 and today.day == 1:
            should_run = True

        if not should_run:
            continue

        ai_config = AIConfig.objects.filter(user=user).first()
        if not ai_config:
            skip_ai += 1
            continue

        try:
            context_data = build_report_context(user, pref.report_frequency)
            system_prompt = '你是一位专业的基金投资顾问，请根据提供的持仓数据，生成一份结构清晰、客观专业的投资报告。使用 Markdown 格式，报告标题下方标注生成日期。'
            user_prompt = (
                f'请根据以下数据生成一份投资报告（报告日期：{today.strftime("%Y年%m月%d日")}）：\n\n'
                f'## 账户总览\n{context_data.get("account_summary", "")}\n\n'
                f'## 持仓明细\n{context_data.get("position_summary", "")}\n\n'
                f'## 期间表现\n{context_data.get("period_pnl", "")}\n\n'
                f'## 表现最佳\n{context_data.get("top_performers", "")}\n\n'
                f'## 表现最差\n{context_data.get("worst_performers", "")}\n'
            )

            endpoint = ai_config.api_endpoint.rstrip('/')
            resp = requests.post(
                f'{endpoint}/chat/completions',
                headers={'Authorization': f'Bearer {ai_config.api_key}', 'Content-Type': 'application/json'},
                json={'model': ai_config.model_name, 'messages': [
                    {'role': 'system', 'content': system_prompt},
                    {'role': 'user', 'content': user_prompt},
                ]},
                timeout=120,
            )
            resp.raise_for_status()
            result = resp.json()
            content = result['choices'][0]['message']['content']

            # 推送报告到用户通知渠道
            from api.models import NotificationChannel
            from api.notifications import ChannelRegistry
            channels = NotificationChannel.objects.filter(user=user, is_active=True)
            for ch in channels:
                impl = ChannelRegistry.get_channel(ch.channel_type)
                if impl:
                    try:
                        impl.send(
                            f'Fundval 投资{pref.report_frequency}报',
                            content[:4000],  # 截断避免过长
                            ch.config,
                        )
                    except Exception as e:
                        logger.warning(f'推送报告到渠道 {ch.id} 失败: {e}')

            generated += 1
        except Exception as e:
            logger.error(f'为用户 {user.username} 生成报告失败: {e}')

    summary = f'{generated} reports generated, {skip_ai} skipped (no AI config), {skip_disabled} skipped (disabled)'
    logger.info(summary)
    return summary
