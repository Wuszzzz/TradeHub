# Generated manually for fund sector market snapshots.

import uuid
from django.db import migrations, models


class Migration(migrations.Migration):

    dependencies = [
        ('api', '0017_fund_allocation_rank_snapshots'),
    ]

    operations = [
        migrations.CreateModel(
            name='FundSectorMarketSnapshot',
            fields=[
                ('id', models.UUIDField(default=uuid.uuid4, editable=False, primary_key=True, serialize=False)),
                ('snapshot_time', models.DateTimeField(db_index=True)),
                ('trade_date', models.DateField(db_index=True)),
                ('board_code', models.CharField(db_index=True, default='aStock', max_length=32)),
                ('sector_code', models.CharField(db_index=True, max_length=32)),
                ('sector_name', models.CharField(db_index=True, max_length=100)),
                ('latest_price', models.DecimalField(blank=True, decimal_places=4, max_digits=20, null=True)),
                ('change_amount', models.DecimalField(blank=True, decimal_places=4, max_digits=20, null=True)),
                ('change_percent', models.DecimalField(blank=True, decimal_places=4, max_digits=10, null=True)),
                ('turnover_rate', models.DecimalField(blank=True, decimal_places=4, max_digits=10, null=True)),
                ('amplitude', models.DecimalField(blank=True, decimal_places=4, max_digits=10, null=True)),
                ('volume', models.CharField(blank=True, max_length=64, null=True)),
                ('amount', models.CharField(blank=True, max_length=64, null=True)),
                ('leading_stock_code', models.CharField(blank=True, max_length=32, null=True)),
                ('leading_stock_name', models.CharField(blank=True, max_length=100, null=True)),
                ('five_day_change', models.DecimalField(blank=True, decimal_places=4, max_digits=10, null=True)),
                ('twenty_day_change', models.DecimalField(blank=True, decimal_places=4, max_digits=10, null=True)),
                ('sixty_day_change', models.DecimalField(blank=True, decimal_places=4, max_digits=10, null=True)),
                ('fifty_two_week_change', models.DecimalField(blank=True, decimal_places=4, max_digits=10, null=True)),
                ('ytd_change', models.DecimalField(blank=True, decimal_places=4, max_digits=10, null=True)),
                ('source', models.CharField(default='tencent_market', max_length=50)),
                ('is_close_snapshot', models.BooleanField(db_index=True, default=False)),
                ('raw_data', models.JSONField(blank=True, default=dict)),
                ('created_at', models.DateTimeField(auto_now_add=True)),
            ],
            options={
                'verbose_name': '基金板块市场快照',
                'verbose_name_plural': '基金板块市场快照',
                'db_table': 'fund_sector_market_snapshot',
                'ordering': ['-snapshot_time', '-change_percent'],
            },
        ),
        migrations.AddIndex(
            model_name='fundsectormarketsnapshot',
            index=models.Index(fields=['trade_date', 'board_code', '-snapshot_time'], name='fund_sector_trade_d_a0c8dd_idx'),
        ),
        migrations.AddIndex(
            model_name='fundsectormarketsnapshot',
            index=models.Index(fields=['sector_code', '-snapshot_time'], name='fund_sector_sector__26bb68_idx'),
        ),
        migrations.AddIndex(
            model_name='fundsectormarketsnapshot',
            index=models.Index(fields=['is_close_snapshot', 'trade_date'], name='fund_sector_is_clos_5d67fe_idx'),
        ),
        migrations.AlterUniqueTogether(
            name='fundsectormarketsnapshot',
            unique_together={('snapshot_time', 'board_code', 'sector_code', 'source')},
        ),
    ]
