from django.urls import path, include
from rest_framework.routers import DefaultRouter
from . import views, viewsets

# 创建主路由器
router = DefaultRouter()
router.register(r'funds', viewsets.FundViewSet, basename='fund')
router.register(r'accounts', viewsets.AccountViewSet, basename='account')
router.register(r'positions', viewsets.PositionViewSet, basename='position')
router.register(r'watchlists', viewsets.WatchlistViewSet, basename='watchlist')
router.register(r'sources', viewsets.SourceViewSet, basename='source')
router.register(r'users', viewsets.UserViewSet, basename='user')
router.register(r'nav-history', viewsets.FundNavHistoryViewSet, basename='nav-history')
router.register(r'fund-daily-facts', viewsets.FundDailyFactViewSet, basename='fund-daily-fact')
router.register(r'fund-companies', viewsets.FundCompanyViewSet, basename='fund-company')
router.register(r'fund-managers', viewsets.FundManagerViewSet, basename='fund-manager')
router.register(r'fund-manager-tenures', viewsets.FundManagerTenureViewSet, basename='fund-manager-tenure')
router.register(r'fund-holding-snapshots', viewsets.FundHoldingSnapshotViewSet, basename='fund-holding-snapshot')
router.register(r'fund-allocation-snapshots', viewsets.FundAllocationSnapshotViewSet, basename='fund-allocation-snapshot')
router.register(r'fund-performance-ranks', viewsets.FundPerformanceRankSnapshotViewSet, basename='fund-performance-rank')
router.register(r'fund-sector-market-snapshots', viewsets.FundSectorMarketSnapshotViewSet, basename='fund-sector-market-snapshot')
router.register(r'source-credentials', viewsets.SourceCredentialViewSet, basename='source-credential')
router.register(r'ai/templates', viewsets.AIPromptTemplateViewSet, basename='ai-template')
router.register(r'notification-channels', viewsets.NotificationChannelViewSet, basename='notification-channel')
router.register(r'notification-rules', viewsets.NotificationRuleViewSet, basename='notification-rule')
router.register(r'notification-logs', viewsets.NotificationLogViewSet, basename='notification-log')

urlpatterns = [
    # 系统管理
    path('health/', views.health, name='health'),

    # Bootstrap 初始化
    path('admin/bootstrap/verify', views.bootstrap_verify, name='bootstrap_verify'),
    path('admin/bootstrap/initialize', views.bootstrap_initialize, name='bootstrap_initialize'),

    # 认证
    path('auth/login', views.login, name='login'),
    path('auth/refresh', views.refresh_token, name='refresh_token'),
    path('auth/me', views.get_current_user, name='get_current_user'),
    path('auth/password', views.change_password, name='change_password'),
    path('tencent-fund/<str:symbol>/', views.tencent_fund_detail, name='tencent_fund_detail'),
    path('tencent-market/board/', views.tencent_market_board, name='tencent_market_board'),
    path('tencent-market/indices/', views.tencent_market_indices, name='tencent_market_indices'),
    path('tencent-market/us/', views.tencent_market_us, name='tencent_market_us'),
    path('tencent-market/fx/', views.tencent_market_fx, name='tencent_market_fx'),
    path('tencent-market/futures/', views.tencent_market_futures, name='tencent_market_futures'),
    path('funds/<str:fund_code>/market-kline/', views.fund_market_kline, name='fund_market_kline'),

    # 持仓操作（单独路由）
    path('positions/operations/', viewsets.PositionOperationViewSet.as_view({
        'get': 'list',
        'post': 'create'
    })),
    path('positions/operations/batch_delete/', viewsets.PositionOperationViewSet.as_view({
        'post': 'batch_delete'
    })),
    path('positions/operations/<uuid:pk>/', viewsets.PositionOperationViewSet.as_view({
        'get': 'retrieve',
        'delete': 'destroy'
    })),

    # 用户偏好
    path('preferences/', viewsets.UserPreferenceViewSet.as_view({
        'get': 'list',
        'put': 'update',
    })),

    # AI配置
    path('ai/config/', viewsets.AIConfigViewSet.as_view({
        'get': 'list',
        'put': 'update',
    })),

    # AI分析
    path('ai/analyze/', views.ai_analyze, name='ai_analyze'),
    path('ai/report-preview/', views.ai_report_preview, name='ai_report_preview'),

    # 管理员
    path('admin/users/', viewsets.AdminViewSet.as_view({'get': 'list'})),
    path('admin/users/<int:user_id>/toggle/', viewsets.AdminViewSet.as_view({'post': 'toggle_active'})),
    path('admin/users/<int:user_id>/reset-password/', viewsets.AdminViewSet.as_view({'post': 'reset_password'})),
    path('admin/stats/', viewsets.AdminViewSet.as_view({'get': 'stats'})),
    path('admin/tasks/<str:task_name>/', viewsets.AdminViewSet.as_view({'post': 'trigger_task'})),

    # API 路由
    path('', include(router.urls)),
]
