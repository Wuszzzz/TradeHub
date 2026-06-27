import uuid
from django.db import models
from django.contrib.auth import get_user_model

User = get_user_model()


class Fund(models.Model):
    """基金模型"""

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    fund_code = models.CharField(max_length=10, unique=True, db_index=True)
    fund_name = models.CharField(max_length=100)
    fund_type = models.CharField(max_length=50, null=True, blank=True)
    company = models.ForeignKey(
        'FundCompany',
        null=True,
        blank=True,
        on_delete=models.SET_NULL,
        related_name='funds',
    )
    short_name = models.CharField(max_length=100, null=True, blank=True)
    inception_date = models.DateField(null=True, blank=True)
    fund_size = models.DecimalField(max_digits=20, decimal_places=4, null=True, blank=True, help_text='基金规模，单位亿元')
    fund_size_text = models.CharField(max_length=64, null=True, blank=True)
    tracking_index = models.CharField(max_length=100, null=True, blank=True)
    risk_level = models.CharField(max_length=50, null=True, blank=True)
    fund_status = models.CharField(max_length=50, null=True, blank=True)
    purchase_status = models.CharField(max_length=50, null=True, blank=True)
    redemption_status = models.CharField(max_length=50, null=True, blank=True)
    management_fee = models.DecimalField(max_digits=10, decimal_places=4, null=True, blank=True)
    custody_fee = models.DecimalField(max_digits=10, decimal_places=4, null=True, blank=True)
    profile_source = models.CharField(max_length=50, null=True, blank=True)
    profile_updated_at = models.DateTimeField(null=True, blank=True)
    profile_raw = models.JSONField(default=dict, blank=True)

    # 净值数据（由数据源更新）
    latest_nav = models.DecimalField(
        max_digits=10, decimal_places=4,
        null=True, blank=True,
        help_text='最新净值'
    )
    latest_nav_date = models.DateField(
        null=True, blank=True,
        help_text='最新净值日期'
    )

    # 实时估值数据（缓存）
    estimate_nav = models.DecimalField(
        max_digits=10, decimal_places=4,
        null=True, blank=True,
        help_text='实时估值净值'
    )
    estimate_growth = models.DecimalField(
        max_digits=10, decimal_places=4,
        null=True, blank=True,
        help_text='估值涨跌幅（%）'
    )
    estimate_time = models.DateTimeField(
        null=True, blank=True,
        help_text='估值更新时间'
    )

    # 元数据
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        db_table = 'fund'
        verbose_name = '基金'
        verbose_name_plural = '基金'

    def __str__(self):
        return f'{self.fund_code} - {self.fund_name}'


class FundCompany(models.Model):
    """基金公司"""

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    company_code = models.CharField(max_length=64, unique=True, db_index=True)
    company_name = models.CharField(max_length=200, db_index=True)
    short_name = models.CharField(max_length=100, null=True, blank=True)
    source = models.CharField(max_length=50, default='manual')
    raw_data = models.JSONField(default=dict, blank=True)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        db_table = 'fund_company'
        verbose_name = '基金公司'
        verbose_name_plural = '基金公司'
        ordering = ['company_name']

    def __str__(self):
        return self.company_name


class FundManager(models.Model):
    """基金经理"""

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    manager_code = models.CharField(max_length=64, unique=True, db_index=True)
    manager_name = models.CharField(max_length=100, db_index=True)
    company = models.ForeignKey(FundCompany, null=True, blank=True, on_delete=models.SET_NULL, related_name='managers')
    source = models.CharField(max_length=50, default='manual')
    raw_data = models.JSONField(default=dict, blank=True)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        db_table = 'fund_manager'
        verbose_name = '基金经理'
        verbose_name_plural = '基金经理'
        ordering = ['manager_name']

    def __str__(self):
        return self.manager_name


class FundManagerTenure(models.Model):
    """基金经理任职关系"""

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    fund = models.ForeignKey(Fund, on_delete=models.CASCADE, related_name='manager_tenures')
    manager = models.ForeignKey(FundManager, on_delete=models.CASCADE, related_name='tenures')
    start_date = models.DateField(null=True, blank=True)
    end_date = models.DateField(null=True, blank=True)
    source = models.CharField(max_length=50, default='manual')
    raw_data = models.JSONField(default=dict, blank=True)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        db_table = 'fund_manager_tenure'
        verbose_name = '基金经理任职'
        verbose_name_plural = '基金经理任职'
        unique_together = [['fund', 'manager', 'start_date']]
        indexes = [
            models.Index(fields=['fund', 'end_date']),
            models.Index(fields=['manager', 'start_date']),
        ]

    def __str__(self):
        return f'{self.fund.fund_code} - {self.manager.manager_name}'


class FundHoldingSnapshot(models.Model):
    """基金披露持仓快照"""

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    fund = models.ForeignKey(Fund, on_delete=models.CASCADE, related_name='holding_snapshots')
    report_date = models.DateField(help_text='披露/快照日期')
    source = models.CharField(max_length=50, default='eastmoney')
    target_code = models.CharField(max_length=32, null=True, blank=True)
    total_weight = models.DecimalField(max_digits=10, decimal_places=4, null=True, blank=True)
    raw_data = models.JSONField(default=dict, blank=True)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        db_table = 'fund_holding_snapshot'
        verbose_name = '基金持仓快照'
        verbose_name_plural = '基金持仓快照'
        unique_together = [['fund', 'report_date', 'source', 'target_code']]
        ordering = ['-report_date', '-updated_at']
        indexes = [
            models.Index(fields=['fund', '-report_date']),
            models.Index(fields=['report_date', 'source']),
        ]

    def __str__(self):
        return f'{self.fund.fund_code} - {self.report_date} - {self.source}'


class FundHoldingItem(models.Model):
    """基金披露持仓明细"""

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    snapshot = models.ForeignKey(FundHoldingSnapshot, on_delete=models.CASCADE, related_name='items')
    holding_type = models.CharField(max_length=32, default='stock')
    asset_code = models.CharField(max_length=32, db_index=True)
    asset_name = models.CharField(max_length=200)
    weight = models.DecimalField(max_digits=10, decimal_places=4, null=True, blank=True)
    price = models.DecimalField(max_digits=20, decimal_places=4, null=True, blank=True)
    change_percent = models.DecimalField(max_digits=10, decimal_places=4, null=True, blank=True)
    contribution = models.DecimalField(max_digits=10, decimal_places=4, null=True, blank=True)
    sort_order = models.IntegerField(default=0)
    raw_data = models.JSONField(default=dict, blank=True)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        db_table = 'fund_holding_item'
        verbose_name = '基金持仓明细'
        verbose_name_plural = '基金持仓明细'
        unique_together = [['snapshot', 'asset_code', 'holding_type']]
        ordering = ['sort_order', '-weight']
        indexes = [
            models.Index(fields=['asset_code']),
            models.Index(fields=['holding_type']),
        ]

    def __str__(self):
        return f'{self.snapshot.fund.fund_code} - {self.asset_name}'


class FundAllocationSnapshot(models.Model):
    """基金板块/资产配置快照"""

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    fund = models.ForeignKey(Fund, on_delete=models.CASCADE, related_name='allocation_snapshots')
    report_date = models.DateField()
    allocation_type = models.CharField(max_length=32, db_index=True)
    name = models.CharField(max_length=100)
    ratio = models.DecimalField(max_digits=10, decimal_places=4, null=True, blank=True)
    source = models.CharField(max_length=50, default='tencent_fund')
    raw_data = models.JSONField(default=dict, blank=True)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        db_table = 'fund_allocation_snapshot'
        verbose_name = '基金板块配置快照'
        verbose_name_plural = '基金板块配置快照'
        unique_together = [['fund', 'report_date', 'allocation_type', 'name', 'source']]
        ordering = ['-report_date', 'allocation_type', '-ratio']
        indexes = [
            models.Index(fields=['fund', '-report_date']),
            models.Index(fields=['allocation_type', 'name']),
        ]


class FundSectorMarketSnapshot(models.Model):
    """基金板块市场行情快照"""

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    snapshot_time = models.DateTimeField(db_index=True)
    trade_date = models.DateField(db_index=True)
    board_code = models.CharField(max_length=32, default='aStock', db_index=True)
    sector_code = models.CharField(max_length=32, db_index=True)
    sector_name = models.CharField(max_length=100, db_index=True)
    latest_price = models.DecimalField(max_digits=20, decimal_places=4, null=True, blank=True)
    change_amount = models.DecimalField(max_digits=20, decimal_places=4, null=True, blank=True)
    change_percent = models.DecimalField(max_digits=10, decimal_places=4, null=True, blank=True)
    turnover_rate = models.DecimalField(max_digits=10, decimal_places=4, null=True, blank=True)
    amplitude = models.DecimalField(max_digits=10, decimal_places=4, null=True, blank=True)
    volume = models.CharField(max_length=64, null=True, blank=True)
    amount = models.CharField(max_length=64, null=True, blank=True)
    leading_stock_code = models.CharField(max_length=32, null=True, blank=True)
    leading_stock_name = models.CharField(max_length=100, null=True, blank=True)
    five_day_change = models.DecimalField(max_digits=10, decimal_places=4, null=True, blank=True)
    twenty_day_change = models.DecimalField(max_digits=10, decimal_places=4, null=True, blank=True)
    sixty_day_change = models.DecimalField(max_digits=10, decimal_places=4, null=True, blank=True)
    fifty_two_week_change = models.DecimalField(max_digits=10, decimal_places=4, null=True, blank=True)
    ytd_change = models.DecimalField(max_digits=10, decimal_places=4, null=True, blank=True)
    source = models.CharField(max_length=50, default='tencent_market')
    is_close_snapshot = models.BooleanField(default=False, db_index=True)
    raw_data = models.JSONField(default=dict, blank=True)
    created_at = models.DateTimeField(auto_now_add=True)

    class Meta:
        db_table = 'fund_sector_market_snapshot'
        verbose_name = '基金板块市场快照'
        verbose_name_plural = '基金板块市场快照'
        unique_together = [['snapshot_time', 'board_code', 'sector_code', 'source']]
        ordering = ['-snapshot_time', '-change_percent']
        indexes = [
            models.Index(fields=['trade_date', 'board_code', '-snapshot_time']),
            models.Index(fields=['sector_code', '-snapshot_time']),
            models.Index(fields=['is_close_snapshot', 'trade_date']),
        ]


class FundPerformanceRankSnapshot(models.Model):
    """基金区间业绩排行快照"""

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    fund = models.ForeignKey(Fund, on_delete=models.CASCADE, related_name='performance_rank_snapshots')
    rank_type = models.CharField(max_length=32, default='performance', db_index=True)
    rank_date = models.DateField()
    period = models.CharField(max_length=32, db_index=True)
    growth = models.DecimalField(max_digits=12, decimal_places=4, null=True, blank=True)
    rank = models.IntegerField(null=True, blank=True)
    total = models.IntegerField(null=True, blank=True)
    quartile = models.IntegerField(null=True, blank=True)
    source = models.CharField(max_length=50, default='tencent_fund')
    raw_data = models.JSONField(default=dict, blank=True)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        db_table = 'fund_performance_rank_snapshot'
        verbose_name = '基金业绩排行快照'
        verbose_name_plural = '基金业绩排行快照'
        unique_together = [['fund', 'rank_type', 'rank_date', 'period', 'source']]
        ordering = ['-rank_date', 'period']
        indexes = [
            models.Index(fields=['fund', '-rank_date']),
            models.Index(fields=['rank_type', 'period', 'rank_date']),
            models.Index(fields=['period', 'rank_date']),
            models.Index(fields=['rank_date', 'rank']),
        ]


class Account(models.Model):
    """账户模型"""

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    user = models.ForeignKey(User, on_delete=models.CASCADE, related_name='accounts')
    name = models.CharField(max_length=100)
    parent = models.ForeignKey('self', null=True, blank=True, on_delete=models.CASCADE, related_name='children')
    is_default = models.BooleanField(default=False)

    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        db_table = 'account'
        verbose_name = '账户'
        verbose_name_plural = '账户'
        unique_together = [['user', 'name']]
        constraints = [
            models.CheckConstraint(
                condition=models.Q(is_default=False) | models.Q(parent__isnull=True),
                name='default_account_must_be_parent',
                violation_error_message='默认账户必须是父账户'
            ),
        ]

    def __str__(self):
        return f'{self.user.username} - {self.name}'

    def clean(self):
        """模型验证"""
        from django.core.exceptions import ValidationError

        # 验证：默认账户必须是父账户
        if self.is_default and self.parent is not None:
            raise ValidationError('默认账户必须是父账户（parent 必须为 NULL）')

        # 验证：每个用户只能有一个默认账户
        if self.is_default:
            existing_default = Account.objects.filter(
                user=self.user,
                is_default=True
            ).exclude(id=self.id).first()

            if existing_default:
                raise ValidationError(f'用户 {self.user.username} 已有默认账户：{existing_default.name}')

        # 验证：最多两层（父账户 -> 子账户）
        if self.parent is not None and self.parent.parent is not None:
            raise ValidationError('账户层级最多两层：父账户 -> 子账户，不支持孙账户')

    def save(self, *args, **kwargs):
        """保存前自动处理默认账户切换"""
        # 如果设置为默认账户，自动取消同用户的其他默认账户
        if self.is_default:
            Account.objects.filter(
                user=self.user,
                is_default=True
            ).exclude(id=self.id).update(is_default=False)

        # 调用 clean 进行验证
        self.full_clean()
        super().save(*args, **kwargs)

    # 汇总字段（@property）
    @property
    def holding_cost(self):
        """持仓成本"""
        from decimal import Decimal
        if self.parent is None:
            # 父账户：汇总所有子账户
            return sum(
                (child.holding_cost for child in self.children.all()),
                Decimal('0')
            )
        else:
            # 子账户：汇总所有持仓
            return sum(
                (pos.holding_cost for pos in self.positions.all()),
                Decimal('0')
            )

    @property
    def holding_value(self):
        """持仓市值（latest_nav）"""
        from decimal import Decimal
        if self.parent is None:
            # 父账户：汇总所有子账户
            return sum(
                (child.holding_value for child in self.children.all()),
                Decimal('0')
            )
        else:
            # 子账户：汇总所有持仓
            return sum(
                (pos.fund.latest_nav * pos.holding_share
                 for pos in self.positions.all() if pos.fund.latest_nav),
                Decimal('0')
            )

    @property
    def pnl(self):
        """总盈亏"""
        from decimal import Decimal
        return self.holding_value - self.holding_cost

    @property
    def pnl_rate(self):
        """收益率"""
        from decimal import Decimal
        if self.holding_cost == 0:
            return None
        return (self.pnl / self.holding_cost).quantize(Decimal('0.0001'))

    @property
    def estimate_value(self):
        """预估市值"""
        from decimal import Decimal
        if self.parent is None:
            # 父账户：汇总所有子账户，跳过 None
            values = [child.estimate_value for child in self.children.all()]
            non_null = [v for v in values if v is not None]
            if not non_null:
                return Decimal('0')
            return sum(non_null, Decimal('0'))
        else:
            # 子账户：汇总所有持仓，跳过缺失估值的持仓
            total = Decimal('0')
            has_any = False
            for pos in self.positions.all():
                if pos.fund.estimate_nav is None:
                    continue
                total += pos.fund.estimate_nav * pos.holding_share
                has_any = True
            return total if has_any else (Decimal('0') if not self.positions.exists() else Decimal('0'))

    @property
    def estimate_pnl(self):
        """预估盈亏"""
        from decimal import Decimal
        if self.estimate_value is None:
            return None
        return self.estimate_value - self.holding_cost

    @property
    def estimate_pnl_rate(self):
        """预估收益率"""
        from decimal import Decimal
        if self.estimate_pnl is None or self.holding_cost == 0:
            return None
        return (self.estimate_pnl / self.holding_cost).quantize(Decimal('0.0001'))

    @property
    def today_pnl(self):
        """今日盈亏"""
        from decimal import Decimal
        if self.parent is None:
            # 父账户：汇总所有子账户，跳过 None
            values = [child.today_pnl for child in self.children.all()]
            non_null = [v for v in values if v is not None]
            if not non_null:
                return Decimal('0')
            return sum(non_null, Decimal('0'))
        else:
            # 子账户：汇总所有持仓，跳过缺失估值的持仓
            total = Decimal('0')
            has_any = False
            for pos in self.positions.all():
                if pos.fund.estimate_nav is None or pos.fund.latest_nav is None:
                    continue  # 跳过缺失估值的持仓，不影响其他持仓的计算
                total += pos.holding_share * (pos.fund.estimate_nav - pos.fund.latest_nav)
                has_any = True
            return total if has_any else (Decimal('0') if self.positions.exists() else Decimal('0'))

    @property
    def today_pnl_rate(self):
        """今日收益率"""
        from decimal import Decimal
        if self.today_pnl is None or self.holding_value == 0:
            return None
        return (self.today_pnl / self.holding_value).quantize(Decimal('0.0001'))


class Position(models.Model):
    """持仓汇总模型（只读，由流水计算）"""

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    account = models.ForeignKey(Account, on_delete=models.CASCADE, related_name='positions')
    fund = models.ForeignKey(Fund, on_delete=models.CASCADE, related_name='positions')

    # 汇总数据（只读，由流水计算）
    holding_share = models.DecimalField(max_digits=20, decimal_places=4, default=0)
    holding_cost = models.DecimalField(max_digits=20, decimal_places=2, default=0)
    holding_nav = models.DecimalField(max_digits=10, decimal_places=4, default=0)

    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        db_table = 'position'
        verbose_name = '持仓'
        verbose_name_plural = '持仓'
        unique_together = [['account', 'fund']]

    def __str__(self):
        return f'{self.account.name} - {self.fund.fund_name}'

    def clean(self):
        """模型验证"""
        from django.core.exceptions import ValidationError

        # 验证：持仓账户必须是子账户（parent 不能为 NULL）
        if self.account.parent is None:
            raise ValidationError('持仓只能创建在子账户上，父账户不能持有持仓')

    def save(self, *args, **kwargs):
        """保存前验证"""
        self.full_clean()
        super().save(*args, **kwargs)

    @property
    def pnl(self):
        """盈亏（实时计算）"""
        if not self.fund.latest_nav or self.holding_share == 0:
            return 0
        return (self.fund.latest_nav - self.holding_nav) * self.holding_share


class PositionOperation(models.Model):
    """持仓操作流水"""

    OPERATION_TYPE_CHOICES = [
        ('BUY', '建仓/加仓'),
        ('SELL', '减仓'),
    ]

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    account = models.ForeignKey(Account, on_delete=models.CASCADE, related_name='operations')
    fund = models.ForeignKey(Fund, on_delete=models.CASCADE, related_name='operations')

    operation_type = models.CharField(max_length=10, choices=OPERATION_TYPE_CHOICES)
    operation_date = models.DateField()
    before_15 = models.BooleanField(default=True, help_text='是否 15:00 前操作')

    amount = models.DecimalField(max_digits=20, decimal_places=2)
    share = models.DecimalField(max_digits=20, decimal_places=4)
    nav = models.DecimalField(max_digits=10, decimal_places=4, help_text='操作时的净值')

    created_at = models.DateTimeField(auto_now_add=True)

    class Meta:
        db_table = 'position_operation'
        verbose_name = '持仓操作'
        verbose_name_plural = '持仓操作'
        ordering = ['operation_date', 'created_at']

    def __str__(self):
        return f'{self.get_operation_type_display()} - {self.fund.fund_name} - {self.operation_date}'

    def clean(self):
        """模型验证"""
        from django.core.exceptions import ValidationError

        # 验证：操作账户必须是子账户（parent 不能为 NULL）
        if self.account.parent is None:
            raise ValidationError('持仓操作只能在子账户上进行，父账户不能进行持仓操作')

    def save(self, *args, **kwargs):
        """保存前验证，新建操作时自动重算持仓"""
        self.full_clean()
        is_new = self._state.adding
        super().save(*args, **kwargs)

        # 新建操作后自动重算持仓
        if is_new:
            from .services import recalculate_position
            recalculate_position(self.account.id, self.fund.id)


class Watchlist(models.Model):
    """自选列表"""

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    user = models.ForeignKey(User, on_delete=models.CASCADE, related_name='watchlists')
    name = models.CharField(max_length=100)

    created_at = models.DateTimeField(auto_now_add=True)

    class Meta:
        db_table = 'watchlist'
        verbose_name = '自选列表'
        verbose_name_plural = '自选列表'
        unique_together = [['user', 'name']]

    def __str__(self):
        return f'{self.user.username} - {self.name}'


class WatchlistItem(models.Model):
    """自选列表项"""

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    watchlist = models.ForeignKey(Watchlist, on_delete=models.CASCADE, related_name='items')
    fund = models.ForeignKey(Fund, on_delete=models.CASCADE, related_name='watchlist_items')
    order = models.IntegerField(default=0, help_text='排序')

    created_at = models.DateTimeField(auto_now_add=True)

    class Meta:
        db_table = 'watchlist_item'
        verbose_name = '自选项'
        verbose_name_plural = '自选项'
        unique_together = [['watchlist', 'fund']]
        ordering = ['order']

    def __str__(self):
        return f'{self.watchlist.name} - {self.fund.fund_name}'


class EstimateAccuracy(models.Model):
    """估值准确率记录"""

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    source_name = models.CharField(max_length=50, db_index=True)
    fund = models.ForeignKey(Fund, on_delete=models.CASCADE, related_name='accuracy_records')

    estimate_date = models.DateField()
    estimate_nav = models.DecimalField(max_digits=10, decimal_places=4)
    actual_nav = models.DecimalField(max_digits=10, decimal_places=4, null=True, blank=True)

    error_rate = models.DecimalField(max_digits=10, decimal_places=6, null=True, blank=True, help_text='误差率')

    created_at = models.DateTimeField(auto_now_add=True)

    class Meta:
        db_table = 'estimate_accuracy'
        verbose_name = '估值准确率'
        verbose_name_plural = '估值准确率'
        unique_together = [['source_name', 'fund', 'estimate_date']]
        indexes = [
            models.Index(fields=['fund', 'estimate_date']),
            models.Index(fields=['source_name', 'estimate_date']),
        ]

    def __str__(self):
        return f'{self.source_name} - {self.fund.fund_code} - {self.estimate_date}'

    def calculate_error_rate(self):
        """计算误差率"""
        if self.actual_nav and self.actual_nav > 0:
            # 去掉 abs()，改用 (估值 - 实际) / 实际
            error = self.estimate_nav - self.actual_nav
            self.error_rate = error / self.actual_nav
            self.save()


class FundNavHistory(models.Model):
    """基金历史净值"""

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    fund = models.ForeignKey(Fund, on_delete=models.CASCADE, related_name='nav_history')

    # 净值数据
    nav_date = models.DateField(help_text='净值日期')
    unit_nav = models.DecimalField(
        max_digits=10, decimal_places=4,
        help_text='单位净值'
    )
    accumulated_nav = models.DecimalField(
        max_digits=10, decimal_places=4,
        null=True, blank=True,
        help_text='累计净值'
    )
    daily_growth = models.DecimalField(
        max_digits=10, decimal_places=4,
        null=True, blank=True,
        help_text='日增长率（%）'
    )

    # 元数据
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        db_table = 'fund_nav_history'
        verbose_name = '基金历史净值'
        verbose_name_plural = '基金历史净值'
        unique_together = [['fund', 'nav_date']]
        ordering = ['-nav_date']
        indexes = [
            models.Index(fields=['fund', '-nav_date']),
            models.Index(fields=['nav_date']),
        ]

    def __str__(self):
        return f'{self.fund.fund_code} - {self.nav_date}'


class FundDailyFact(models.Model):
    """基金每日事实表：统一实际净值、实际收益、估值与误差"""

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    fund = models.ForeignKey(Fund, on_delete=models.CASCADE, related_name='daily_facts')
    trade_date = models.DateField()
    unit_nav = models.DecimalField(max_digits=10, decimal_places=4, null=True, blank=True)
    accumulated_nav = models.DecimalField(max_digits=10, decimal_places=4, null=True, blank=True)
    daily_growth = models.DecimalField(max_digits=10, decimal_places=4, null=True, blank=True)
    estimate_nav = models.DecimalField(max_digits=10, decimal_places=4, null=True, blank=True)
    estimate_growth = models.DecimalField(max_digits=10, decimal_places=4, null=True, blank=True)
    estimate_time = models.DateTimeField(null=True, blank=True)
    estimate_error_rate = models.DecimalField(max_digits=10, decimal_places=6, null=True, blank=True)
    source = models.CharField(max_length=50, default='system')
    raw_data = models.JSONField(default=dict, blank=True)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        db_table = 'fund_daily_fact'
        verbose_name = '基金每日事实'
        verbose_name_plural = '基金每日事实'
        unique_together = [['fund', 'trade_date']]
        ordering = ['-trade_date']
        indexes = [
            models.Index(fields=['fund', '-trade_date']),
            models.Index(fields=['trade_date']),
        ]

    def __str__(self):
        return f'{self.fund.fund_code} - {self.trade_date}'


class UserSourceCredential(models.Model):
    """用户数据源凭证"""

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    user = models.ForeignKey(User, on_delete=models.CASCADE, related_name='source_credentials')
    source_name = models.CharField(max_length=50, help_text='数据源名称（如 yangjibao）')
    token = models.TextField(help_text='加密存储的 token')
    is_active = models.BooleanField(default=True, help_text='是否激活')

    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        db_table = 'user_source_credential'
        verbose_name = '用户数据源凭证'
        verbose_name_plural = '用户数据源凭证'
        unique_together = [['user', 'source_name']]
        indexes = [
            models.Index(fields=['user', 'source_name', 'is_active']),
        ]

    def __str__(self):
        return f'{self.user.username} - {self.source_name}'


class UserPreference(models.Model):
    """用户偏好设置"""

    SOURCE_CHOICES = [
        ('eastmoney', '东方财富'),
        ('yangjibao', '养基宝'),
    ]

    THEME_CHOICES = [
        ('light', '浅色'),
        ('dark', '深色'),
    ]

    user = models.OneToOneField(User, on_delete=models.CASCADE, related_name='preference')
    preferred_source = models.CharField(
        max_length=50,
        choices=SOURCE_CHOICES,
        default='eastmoney',
    )
    theme_mode = models.CharField(
        max_length=10,
        choices=THEME_CHOICES,
        default='light',
    )
    report_enabled = models.BooleanField(default=False, help_text='是否开启投资报告')
    report_frequency = models.CharField(
        max_length=30,
        default='monthly',
        help_text='逗号分隔: weekly,monthly,yearly',
    )

    class Meta:
        db_table = 'user_preference'
        verbose_name = '用户偏好'
        verbose_name_plural = '用户偏好'

    def __str__(self):
        return f'{self.user.username} - {self.preferred_source}'


class AIConfig(models.Model):
    """用户AI配置"""

    user = models.OneToOneField(User, on_delete=models.CASCADE, related_name='ai_config')
    api_endpoint = models.CharField(max_length=500, help_text='OpenAI协议接口地址')
    api_key = models.CharField(max_length=500, help_text='API Key')
    model_name = models.CharField(max_length=100, default='gpt-4o-mini', help_text='模型名称')

    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        db_table = 'ai_config'
        verbose_name = 'AI配置'
        verbose_name_plural = 'AI配置'

    def __str__(self):
        return f'{self.user.username} - {self.model_name}'


class AIPromptTemplate(models.Model):
    """AI提示词模板"""

    CONTEXT_CHOICES = [
        ('fund', '基金分析'),
        ('position', '持仓分析'),
    ]

    user = models.ForeignKey(User, on_delete=models.CASCADE, related_name='ai_templates')
    name = models.CharField(max_length=100, help_text='模板名称')
    context_type = models.CharField(max_length=20, choices=CONTEXT_CHOICES, help_text='分析维度')
    system_prompt = models.TextField(help_text='系统提示词')
    user_prompt = models.TextField(help_text='用户提示词（含占位符）')
    is_default = models.BooleanField(default=False, help_text='是否为该类型的默认模板')

    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        db_table = 'ai_prompt_template'
        verbose_name = 'AI提示词模板'
        verbose_name_plural = 'AI提示词模板'
        unique_together = [['user', 'name']]

    def __str__(self):
        return f'{self.user.username} - {self.name}'


class NotificationChannel(models.Model):
    """通知渠道配置"""

    CHANNEL_TYPE_CHOICES = [
        ('webhook', 'Webhook'),
        ('email', 'Email'),
    ]

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    user = models.ForeignKey(User, on_delete=models.CASCADE, related_name='notification_channels')
    channel_type = models.CharField(max_length=20, choices=CHANNEL_TYPE_CHOICES)
    config = models.JSONField(help_text='渠道配置（webhook_url 或 email）')
    is_active = models.BooleanField(default=True)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        db_table = 'notification_channel'
        verbose_name = '通知渠道'
        verbose_name_plural = '通知渠道'

    def __str__(self):
        return f'{self.user.username} - {self.get_channel_type_display()}'


class NotificationRule(models.Model):
    """通知规则"""

    RULE_TYPE_CHOICES = [
        ('growth_up', '涨幅超过'),
        ('growth_down', '跌幅超过'),
    ]

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    user = models.ForeignKey(User, on_delete=models.CASCADE, related_name='notification_rules')
    fund = models.ForeignKey(Fund, on_delete=models.CASCADE, related_name='notification_rules')
    rule_type = models.CharField(max_length=20, choices=RULE_TYPE_CHOICES)
    threshold = models.DecimalField(
        max_digits=10, decimal_places=2,
        help_text='阈值（百分比，如 5.00 表示 5%）'
    )
    channels = models.ManyToManyField(NotificationChannel, related_name='rules')
    is_active = models.BooleanField(default=True)
    cooldown_minutes = models.IntegerField(default=60, help_text='冷却时间（分钟）')
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        db_table = 'notification_rule'
        verbose_name = '通知规则'
        verbose_name_plural = '通知规则'
        constraints = [
            models.CheckConstraint(
                condition=models.Q(threshold__gte=0),
                name='threshold_non_negative',
                violation_error_message='阈值必须非负'
            ),
            models.CheckConstraint(
                condition=models.Q(cooldown_minutes__gte=0),
                name='cooldown_non_negative',
                violation_error_message='冷却时间必须非负'
            ),
        ]

    def __str__(self):
        return f'{self.user.username} - {self.fund.fund_name} {self.get_rule_type_display()} {self.threshold}%'


class NotificationLog(models.Model):
    """通知记录"""

    STATUS_CHOICES = [
        ('success', '成功'),
        ('failed', '失败'),
    ]

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    rule = models.ForeignKey(NotificationRule, on_delete=models.CASCADE, related_name='logs')
    channel = models.ForeignKey(NotificationChannel, on_delete=models.CASCADE, related_name='logs')
    trigger_time = models.DateTimeField(auto_now_add=True, db_index=True)
    fund_code = models.CharField(max_length=10)
    fund_name = models.CharField(max_length=100)
    growth = models.DecimalField(max_digits=10, decimal_places=2, help_text='触发时的涨跌幅（%）')
    status = models.CharField(max_length=20, choices=STATUS_CHOICES)
    error_message = models.TextField(null=True, blank=True)

    class Meta:
        db_table = 'notification_log'
        verbose_name = '通知记录'
        verbose_name_plural = '通知记录'
        indexes = [
            models.Index(fields=['rule', 'trigger_time']),
        ]

    def __str__(self):
        return f'{self.fund_name} {self.growth}% - {self.get_status_display()}'


class EstimateSnapshot(models.Model):
    """盘中估值快照（用于绘制当日估值曲线）"""

    fund = models.ForeignKey(Fund, on_delete=models.CASCADE, related_name='estimate_snapshots')
    source = models.CharField(max_length=50, default='eastmoney')
    timestamp = models.DateTimeField(db_index=True)
    estimate_nav = models.DecimalField(max_digits=10, decimal_places=4)
    estimate_growth = models.DecimalField(max_digits=10, decimal_places=4, null=True, blank=True)

    class Meta:
        db_table = 'estimate_snapshot'
        verbose_name = '估值快照'
        verbose_name_plural = '估值快照'
        ordering = ['timestamp']
        indexes = [
            models.Index(fields=['fund', 'timestamp']),
            models.Index(fields=['timestamp']),
        ]


# Signal handlers
from django.db.models.signals import post_delete
from django.dispatch import receiver


@receiver(post_delete, sender=PositionOperation)
def recalculate_position_on_delete(sender, instance, **kwargs):
    """删除操作后自动重算持仓"""
    from .services import recalculate_position
    recalculate_position(instance.account.id, instance.fund.id)
