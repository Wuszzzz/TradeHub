import uuid
from django.db import migrations, models
import django.db.models.deletion


class Migration(migrations.Migration):

    dependencies = [
        ('api', '0019_rank_snapshot_type'),
    ]

    operations = [
        migrations.CreateModel(
            name='FundEvaluationSnapshot',
            fields=[
                ('id', models.UUIDField(default=uuid.uuid4, editable=False, primary_key=True, serialize=False)),
                ('evaluation_date', models.DateField(db_index=True)),
                ('window_days', models.IntegerField(db_index=True, default=370)),
                ('nav_count', models.IntegerField(default=0)),
                ('start_date', models.DateField(blank=True, null=True)),
                ('end_date', models.DateField(blank=True, null=True)),
                ('return_1m', models.DecimalField(blank=True, decimal_places=4, max_digits=12, null=True)),
                ('return_3m', models.DecimalField(blank=True, decimal_places=4, max_digits=12, null=True)),
                ('return_6m', models.DecimalField(blank=True, decimal_places=4, max_digits=12, null=True)),
                ('return_1y', models.DecimalField(blank=True, decimal_places=4, max_digits=12, null=True)),
                ('max_drawdown', models.DecimalField(blank=True, decimal_places=4, max_digits=12, null=True)),
                ('volatility', models.DecimalField(blank=True, decimal_places=4, max_digits=12, null=True)),
                ('sharpe', models.DecimalField(blank=True, decimal_places=4, max_digits=12, null=True)),
                ('score', models.IntegerField(default=0)),
                ('level', models.CharField(default='谨慎', max_length=32)),
                ('reasons', models.JSONField(blank=True, default=list)),
                ('source', models.CharField(default='go_fund_research', max_length=50)),
                ('raw_data', models.JSONField(blank=True, default=dict)),
                ('created_at', models.DateTimeField(auto_now_add=True)),
                ('updated_at', models.DateTimeField(auto_now=True)),
                ('fund', models.ForeignKey(on_delete=django.db.models.deletion.CASCADE, related_name='evaluation_snapshots', to='api.fund')),
            ],
            options={
                'verbose_name': '基金评估指标快照',
                'verbose_name_plural': '基金评估指标快照',
                'db_table': 'fund_evaluation_snapshot',
                'ordering': ['-evaluation_date', '-score'],
            },
        ),
        migrations.AddIndex(
            model_name='fundevaluationsnapshot',
            index=models.Index(fields=['fund', '-evaluation_date'], name='api_fundeva_fund_id_7d4e12_idx'),
        ),
        migrations.AddIndex(
            model_name='fundevaluationsnapshot',
            index=models.Index(fields=['evaluation_date', '-score'], name='api_fundeva_evaluat_3d71cd_idx'),
        ),
        migrations.AddIndex(
            model_name='fundevaluationsnapshot',
            index=models.Index(fields=['level', '-evaluation_date'], name='api_fundeva_level_7a11bd_idx'),
        ),
        migrations.AddIndex(
            model_name='fundevaluationsnapshot',
            index=models.Index(fields=['source', '-evaluation_date'], name='api_fundeva_source_c0b66d_idx'),
        ),
        migrations.AlterUniqueTogether(
            name='fundevaluationsnapshot',
            unique_together={('fund', 'evaluation_date', 'window_days', 'source')},
        ),
    ]
