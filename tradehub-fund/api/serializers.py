"""
序列化器

用于 API 数据的序列化和反序列化
"""
from rest_framework import serializers
from django.contrib.auth import get_user_model
from datetime import date
from .models import (
    Fund, Account, Position, PositionOperation,
    Watchlist, WatchlistItem, EstimateAccuracy, FundNavHistory,
    FundAllocationSnapshot, FundCompany, FundDailyFact, FundHoldingItem,
    FundHoldingSnapshot, FundManager, FundManagerTenure, FundEvaluationSnapshot,
    FundPerformanceRankSnapshot, FundSectorMarketSnapshot,
    UserSourceCredential, AIConfig, AIPromptTemplate,
    NotificationChannel, NotificationRule, NotificationLog,
)

User = get_user_model()


class FundSerializer(serializers.ModelSerializer):
    """基金序列化器"""
    company_name = serializers.CharField(source='company.company_name', read_only=True)

    class Meta:
        model = Fund
        fields = [
            'id', 'fund_code', 'fund_name', 'fund_type', 'company', 'company_name',
            'short_name', 'inception_date', 'fund_size', 'fund_size_text',
            'tracking_index', 'risk_level', 'fund_status',
            'purchase_status', 'redemption_status', 'management_fee', 'custody_fee',
            'profile_source', 'profile_updated_at',
            'latest_nav', 'latest_nav_date',
            'return_this_year', 'return_30d', 'return_1m', 'return_3m', 'return_1y',
            'estimate_nav', 'estimate_growth', 'estimate_time',
            'created_at', 'updated_at'
        ]
        read_only_fields = ['id', 'created_at', 'updated_at']


class AccountSerializer(serializers.ModelSerializer):
    """账户序列化器"""

    parent = serializers.PrimaryKeyRelatedField(
        queryset=Account.objects.all(),
        required=False,
        allow_null=True
    )

    # 汇总字段
    holding_cost = serializers.DecimalField(max_digits=20, decimal_places=2, read_only=True)
    holding_value = serializers.DecimalField(max_digits=20, decimal_places=2, read_only=True)
    pnl = serializers.DecimalField(max_digits=20, decimal_places=2, read_only=True)
    pnl_rate = serializers.DecimalField(max_digits=10, decimal_places=4, read_only=True, allow_null=True)
    estimate_value = serializers.DecimalField(max_digits=20, decimal_places=2, read_only=True, allow_null=True)
    estimate_pnl = serializers.DecimalField(max_digits=20, decimal_places=2, read_only=True, allow_null=True)
    estimate_pnl_rate = serializers.DecimalField(max_digits=10, decimal_places=4, read_only=True, allow_null=True)
    today_pnl = serializers.DecimalField(max_digits=20, decimal_places=2, read_only=True, allow_null=True)
    today_pnl_rate = serializers.DecimalField(max_digits=10, decimal_places=4, read_only=True, allow_null=True)

    # 父账户专用：子账户列表
    children = serializers.SerializerMethodField()

    class Meta:
        model = Account
        fields = [
            'id', 'name', 'parent', 'is_default',
            'holding_cost', 'holding_value', 'pnl', 'pnl_rate',
            'estimate_value', 'estimate_pnl', 'estimate_pnl_rate',
            'today_pnl', 'today_pnl_rate',
            'children',
            'created_at', 'updated_at'
        ]
        read_only_fields = ['id', 'created_at', 'updated_at']

    def get_children(self, obj):
        """获取子账户列表（仅父账户）"""
        if obj.parent is not None:
            return None
        children = obj.children.all()
        return AccountSerializer(children, many=True, context=self.context).data

    def to_representation(self, instance):
        """序列化时将 UUID 转为字符串，移除子账户的 children 字段"""
        data = super().to_representation(instance)
        if data.get('parent'):
            data['parent'] = str(data['parent'])

        # 子账户不返回 children 字段
        if instance.parent is not None:
            data.pop('children', None)

        return data

    def validate(self, data):
        """验证账户名唯一性"""
        user = self.context['request'].user
        name = data.get('name')

        # 更新时排除自己
        if self.instance:
            if Account.objects.filter(user=user, name=name).exclude(id=self.instance.id).exists():
                raise serializers.ValidationError({'name': '账户名已存在'})
        else:
            if Account.objects.filter(user=user, name=name).exists():
                raise serializers.ValidationError({'name': '账户名已存在'})

        return data


class PositionSerializer(serializers.ModelSerializer):
    """持仓序列化器"""

    fund_code = serializers.CharField(source='fund.fund_code', read_only=True)
    fund_name = serializers.CharField(source='fund.fund_name', read_only=True)
    fund_type = serializers.CharField(source='fund.fund_type', read_only=True)
    account_name = serializers.CharField(source='account.name', read_only=True)
    pnl = serializers.DecimalField(max_digits=20, decimal_places=2, read_only=True)

    # 添加基金的估值和净值信息
    fund = serializers.SerializerMethodField()

    def get_fund(self, obj):
        """返回基金的详细信息"""
        return {
            'fund_code': obj.fund.fund_code,
            'fund_name': obj.fund.fund_name,
            'fund_type': obj.fund.fund_type,
            'latest_nav': str(obj.fund.latest_nav) if obj.fund.latest_nav else None,
            'latest_nav_date': obj.fund.latest_nav_date.isoformat() if obj.fund.latest_nav_date else None,
            'estimate_nav': str(obj.fund.estimate_nav) if obj.fund.estimate_nav else None,
            'estimate_growth': str(obj.fund.estimate_growth) if obj.fund.estimate_growth else None,
            'estimate_time': obj.fund.estimate_time.isoformat() if obj.fund.estimate_time else None,
        }

    class Meta:
        model = Position
        fields = [
            'id', 'account', 'account_name', 'fund', 'fund_code', 'fund_name', 'fund_type',
            'holding_share', 'holding_cost', 'holding_nav', 'pnl',
            'updated_at'
        ]
        read_only_fields = [
            'id', 'holding_share', 'holding_cost', 'holding_nav', 'updated_at'
        ]


class PositionOperationSerializer(serializers.ModelSerializer):
    """持仓操作序列化器"""

    fund_code = serializers.CharField(write_only=True)
    fund_name = serializers.CharField(source='fund.fund_name', read_only=True)
    account_name = serializers.CharField(source='account.name', read_only=True)

    class Meta:
        model = PositionOperation
        fields = [
            'id', 'account', 'account_name', 'fund', 'fund_code', 'fund_name',
            'operation_type', 'operation_date', 'before_15',
            'amount', 'share', 'nav',
            'created_at'
        ]
        read_only_fields = ['id', 'fund', 'created_at']

    def validate(self, data):
        """验证并设置 fund"""
        fund_code = data.pop('fund_code', None)
        if not fund_code:
            raise serializers.ValidationError({'fund_code': '基金代码不能为空'})

        try:
            fund = Fund.objects.get(fund_code=fund_code)
            data['fund'] = fund
        except Fund.DoesNotExist:
            raise serializers.ValidationError({'fund_code': '基金不存在'})

        return data

    def create(self, validated_data):
        """创建操作并自动重算持仓"""
        from .services import recalculate_position

        operation = super().create(validated_data)

        # 自动重算持仓
        recalculate_position(operation.account.id, operation.fund.id)

        return operation


class WatchlistItemSerializer(serializers.ModelSerializer):
    """自选列表项序列化器"""

    fund_code = serializers.CharField(source='fund.fund_code', read_only=True)
    fund_name = serializers.CharField(source='fund.fund_name', read_only=True)
    fund_type = serializers.CharField(source='fund.fund_type', read_only=True)

    class Meta:
        model = WatchlistItem
        fields = ['id', 'fund', 'fund_code', 'fund_name', 'fund_type', 'order', 'created_at']
        read_only_fields = ['id', 'created_at']


class WatchlistSerializer(serializers.ModelSerializer):
    """自选列表序列化器"""

    items = WatchlistItemSerializer(many=True, read_only=True)

    class Meta:
        model = Watchlist
        fields = ['id', 'name', 'items', 'created_at']
        read_only_fields = ['id', 'created_at']

    def validate(self, data):
        """验证自选列表名唯一性"""
        user = self.context['request'].user
        name = data.get('name')

        if self.instance:
            if Watchlist.objects.filter(user=user, name=name).exclude(id=self.instance.id).exists():
                raise serializers.ValidationError({'name': '自选列表名已存在'})
        else:
            if Watchlist.objects.filter(user=user, name=name).exists():
                raise serializers.ValidationError({'name': '自选列表名已存在'})

        return data


class UserRegisterSerializer(serializers.Serializer):
    """用户注册序列化器"""

    username = serializers.CharField(max_length=150)
    password = serializers.CharField(write_only=True, min_length=8)
    password_confirm = serializers.CharField(write_only=True)

    def validate_username(self, value):
        """验证用户名唯一性"""
        if User.objects.filter(username=value).exists():
            raise serializers.ValidationError('用户名已存在')
        return value

    def validate(self, data):
        """验证密码一致性"""
        if data['password'] != data['password_confirm']:
            raise serializers.ValidationError({'password_confirm': '两次密码不一致'})
        return data

    def create(self, validated_data):
        """创建用户"""
        validated_data.pop('password_confirm')
        user = User.objects.create_user(**validated_data)
        return user


class FundNavHistorySerializer(serializers.ModelSerializer):
    """基金历史净值序列化器"""

    fund_code = serializers.CharField(source='fund.fund_code', read_only=True)
    fund_name = serializers.CharField(source='fund.fund_name', read_only=True)

    class Meta:
        model = FundNavHistory
        fields = [
            'id',
            'fund_code',
            'fund_name',
            'nav_date',
            'unit_nav',
            'accumulated_nav',
            'daily_growth',
            'created_at',
            'updated_at',
        ]
        read_only_fields = fields


class FundDailyFactSerializer(serializers.ModelSerializer):
    fund_code = serializers.CharField(source='fund.fund_code', read_only=True)
    fund_name = serializers.CharField(source='fund.fund_name', read_only=True)

    class Meta:
        model = FundDailyFact
        fields = [
            'id', 'fund', 'fund_code', 'fund_name', 'trade_date',
            'unit_nav', 'accumulated_nav', 'daily_growth',
            'estimate_nav', 'estimate_growth', 'estimate_time',
            'estimate_error_rate', 'source', 'created_at', 'updated_at',
        ]
        read_only_fields = fields


class FundCompanySerializer(serializers.ModelSerializer):
    fund_count = serializers.IntegerField(read_only=True, required=False)
    manager_count = serializers.IntegerField(read_only=True, required=False)

    class Meta:
        model = FundCompany
        fields = [
            'id', 'company_code', 'company_name', 'short_name', 'source',
            'fund_count', 'manager_count', 'created_at', 'updated_at',
        ]
        read_only_fields = fields


class FundManagerSerializer(serializers.ModelSerializer):
    company_name = serializers.CharField(source='company.company_name', read_only=True)
    fund_count = serializers.IntegerField(read_only=True, required=False)

    class Meta:
        model = FundManager
        fields = [
            'id', 'manager_code', 'manager_name', 'company', 'company_name',
            'source', 'fund_count', 'created_at', 'updated_at',
        ]
        read_only_fields = fields


class FundManagerTenureSerializer(serializers.ModelSerializer):
    fund_code = serializers.CharField(source='fund.fund_code', read_only=True)
    fund_name = serializers.CharField(source='fund.fund_name', read_only=True)
    manager_name = serializers.CharField(source='manager.manager_name', read_only=True)
    company_name = serializers.CharField(source='manager.company.company_name', read_only=True)

    class Meta:
        model = FundManagerTenure
        fields = [
            'id', 'fund', 'fund_code', 'fund_name', 'manager', 'manager_name',
            'company_name', 'start_date', 'end_date', 'source', 'created_at', 'updated_at',
        ]
        read_only_fields = fields


class FundHoldingItemSerializer(serializers.ModelSerializer):
    class Meta:
        model = FundHoldingItem
        fields = [
            'id', 'holding_type', 'asset_code', 'asset_name', 'weight',
            'price', 'change_percent', 'contribution', 'sort_order',
            'created_at', 'updated_at',
        ]
        read_only_fields = fields


class FundHoldingSnapshotSerializer(serializers.ModelSerializer):
    fund_code = serializers.CharField(source='fund.fund_code', read_only=True)
    fund_name = serializers.CharField(source='fund.fund_name', read_only=True)
    items = FundHoldingItemSerializer(many=True, read_only=True)
    item_count = serializers.IntegerField(read_only=True, required=False)

    class Meta:
        model = FundHoldingSnapshot
        fields = [
            'id', 'fund', 'fund_code', 'fund_name', 'report_date', 'source',
            'target_code', 'total_weight', 'item_count', 'items',
            'created_at', 'updated_at',
        ]
        read_only_fields = fields


class FundAllocationSnapshotSerializer(serializers.ModelSerializer):
    fund_code = serializers.CharField(source='fund.fund_code', read_only=True)
    fund_name = serializers.CharField(source='fund.fund_name', read_only=True)

    class Meta:
        model = FundAllocationSnapshot
        fields = [
            'id', 'fund', 'fund_code', 'fund_name', 'report_date',
            'allocation_type', 'name', 'ratio', 'source',
            'created_at', 'updated_at',
        ]
        read_only_fields = fields


class FundPerformanceRankSnapshotSerializer(serializers.ModelSerializer):
    fund_code = serializers.CharField(source='fund.fund_code', read_only=True)
    fund_name = serializers.CharField(source='fund.fund_name', read_only=True)
    fund_type = serializers.CharField(source='fund.fund_type', read_only=True)
    fund_size = serializers.DecimalField(source='fund.fund_size', max_digits=20, decimal_places=4, read_only=True)
    fund_size_text = serializers.CharField(source='fund.fund_size_text', read_only=True)
    max_drawdown = serializers.SerializerMethodField()
    volatility = serializers.SerializerMethodField()
    sharpe = serializers.SerializerMethodField()
    evaluation = serializers.SerializerMethodField()
    top5_holding_ratio = serializers.SerializerMethodField()
    top10_holding_ratio = serializers.SerializerMethodField()

    class Meta:
        model = FundPerformanceRankSnapshot
        fields = [
            'id', 'fund', 'fund_code', 'fund_name', 'fund_type', 'rank_type',
            'fund_size', 'fund_size_text', 'rank_date', 'period',
            'growth', 'rank', 'total', 'quartile', 'source',
            'top5_holding_ratio', 'top10_holding_ratio',
            'max_drawdown', 'volatility', 'sharpe', 'evaluation',
            'created_at', 'updated_at',
        ]
        read_only_fields = fields

    def _metrics(self, obj):
        metrics_map = self.context.get('fund_metrics') or {}
        return metrics_map.get(obj.fund_id) or {}

    def get_max_drawdown(self, obj):
        return self._metrics(obj).get('max_drawdown')

    def get_volatility(self, obj):
        return self._metrics(obj).get('volatility')

    def get_sharpe(self, obj):
        return self._metrics(obj).get('sharpe')

    def get_evaluation(self, obj):
        from .services.fund_metrics import evaluate_fund_choice
        metrics = self._metrics(obj)
        if metrics.get('evaluation'):
            return metrics['evaluation']
        return evaluate_fund_choice({
            'growth': str(obj.growth) if obj.growth is not None else None,
            'rank': obj.rank,
            'total': obj.total,
            **metrics,
        })

    def get_top5_holding_ratio(self, obj):
        raw = self._metrics(obj).get('raw_data') or {}
        return raw.get('top5_holding_ratio')

    def get_top10_holding_ratio(self, obj):
        raw = self._metrics(obj).get('raw_data') or {}
        return raw.get('top10_holding_ratio')


class FundEvaluationSnapshotSerializer(serializers.ModelSerializer):
    fund_code = serializers.CharField(source='fund.fund_code', read_only=True)
    fund_name = serializers.CharField(source='fund.fund_name', read_only=True)
    fund_type = serializers.CharField(source='fund.fund_type', read_only=True)

    class Meta:
        model = FundEvaluationSnapshot
        fields = [
            'id', 'fund', 'fund_code', 'fund_name', 'fund_type',
            'evaluation_date', 'window_days', 'nav_count', 'start_date', 'end_date',
            'return_1m', 'return_3m', 'return_6m', 'return_1y',
            'max_drawdown', 'volatility', 'sharpe',
            'score', 'level', 'reasons', 'source', 'raw_data',
            'created_at', 'updated_at',
        ]
        read_only_fields = fields


class FundSectorMarketSnapshotSerializer(serializers.ModelSerializer):
    class Meta:
        model = FundSectorMarketSnapshot
        fields = [
            'id', 'snapshot_time', 'trade_date', 'board_code',
            'sector_code', 'sector_name', 'latest_price',
            'change_amount', 'change_percent', 'turnover_rate', 'amplitude',
            'volume', 'amount', 'leading_stock_code', 'leading_stock_name',
            'five_day_change', 'twenty_day_change', 'sixty_day_change',
            'fifty_two_week_change', 'ytd_change', 'source',
            'is_close_snapshot', 'created_at',
        ]
        read_only_fields = fields


class QueryNavSerializer(serializers.Serializer):
    """查询持仓操作净值序列化器"""

    fund_code = serializers.CharField(max_length=10)
    operation_date = serializers.DateField()
    before_15 = serializers.BooleanField()

    def validate_operation_date(self, value):
        """验证操作日期不能是未来"""
        if value > date.today():
            raise serializers.ValidationError('操作日期不能是未来')
        return value


class UserSourceCredentialSerializer(serializers.ModelSerializer):
    """用户数据源凭证序列化器"""

    class Meta:
        model = UserSourceCredential
        fields = [
            'id',
            'source_name',
            'is_active',
            'created_at',
            'updated_at',
        ]
        read_only_fields = fields


class QRCodeLoginSerializer(serializers.Serializer):
    """二维码登录请求序列化器"""

    source_name = serializers.CharField(max_length=50)

    def validate_source_name(self, value):
        """验证数据源是否存在"""
        from .sources import SourceRegistry

        source = SourceRegistry.get_source(value)
        if not source:
            raise serializers.ValidationError(f'数据源 {value} 不存在')

        return value


class AIConfigSerializer(serializers.ModelSerializer):
    """AI配置序列化器"""

    api_key = serializers.CharField(max_length=500)

    class Meta:
        model = AIConfig
        fields = ['id', 'api_endpoint', 'api_key', 'model_name', 'created_at', 'updated_at']
        read_only_fields = ['id', 'created_at', 'updated_at']

    def to_representation(self, instance):
        data = super().to_representation(instance)
        data['api_key'] = '****'
        return data


class AIPromptTemplateSerializer(serializers.ModelSerializer):
    """AI提示词模板序列化器"""

    class Meta:
        model = AIPromptTemplate
        fields = [
            'id', 'name', 'context_type', 'system_prompt',
            'user_prompt', 'is_default', 'created_at', 'updated_at',
        ]
        read_only_fields = ['id', 'created_at', 'updated_at']


class NotificationChannelSerializer(serializers.ModelSerializer):
    """通知渠道序列化器"""

    class Meta:
        model = NotificationChannel
        fields = ['id', 'channel_type', 'config', 'is_active', 'created_at', 'updated_at']
        read_only_fields = ['id', 'created_at', 'updated_at']

    def validate(self, data):
        channel_type = data.get('channel_type', getattr(self.instance, 'channel_type', None))
        config = data.get('config', getattr(self.instance, 'config', {}))

        if channel_type == 'webhook' and not config.get('webhook_url'):
            raise serializers.ValidationError({'config': 'Webhook 渠道需要提供 webhook_url'})
        if channel_type == 'email':
            required = ['smtp_host', 'username', 'password', 'to_email']
            missing = [f for f in required if not config.get(f)]
            if missing:
                raise serializers.ValidationError({'config': f'Email 渠道缺少必要字段：{", ".join(missing)}'})
        return data


class NotificationRuleSerializer(serializers.ModelSerializer):
    """通知规则序列化器"""

    fund_name = serializers.CharField(source='fund.fund_name', read_only=True)
    fund_code = serializers.CharField(source='fund.fund_code', read_only=True)
    channels = NotificationChannelSerializer(many=True, read_only=True)
    channel_ids = serializers.ListField(
        child=serializers.UUIDField(), write_only=True, required=False
    )

    class Meta:
        model = NotificationRule
        fields = [
            'id', 'fund', 'fund_code', 'fund_name',
            'rule_type', 'threshold', 'channels', 'channel_ids',
            'is_active', 'cooldown_minutes', 'created_at', 'updated_at',
        ]
        read_only_fields = ['id', 'created_at', 'updated_at']

    def create(self, validated_data):
        channel_ids = validated_data.pop('channel_ids', [])
        rule = super().create(validated_data)
        if channel_ids:
            channels = NotificationChannel.objects.filter(
                id__in=channel_ids, user=rule.user
            )
            rule.channels.set(channels)
        return rule

    def update(self, instance, validated_data):
        channel_ids = validated_data.pop('channel_ids', None)
        rule = super().update(instance, validated_data)
        if channel_ids is not None:
            channels = NotificationChannel.objects.filter(
                id__in=channel_ids, user=rule.user
            )
            rule.channels.set(channels)
        return rule


class NotificationLogSerializer(serializers.ModelSerializer):
    """通知记录序列化器"""

    channel_type = serializers.CharField(source='channel.channel_type', read_only=True)
    rule_type = serializers.CharField(source='rule.rule_type', read_only=True)

    class Meta:
        model = NotificationLog
        fields = [
            'id', 'fund_code', 'fund_name', 'growth',
            'status', 'error_message', 'trigger_time',
            'channel_type', 'rule_type',
        ]
        read_only_fields = fields
