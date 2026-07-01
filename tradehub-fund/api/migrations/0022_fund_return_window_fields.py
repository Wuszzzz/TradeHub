from django.db import migrations, models


class Migration(migrations.Migration):

    dependencies = [
        ('api', '0021_fund_return_fields'),
    ]

    operations = [
        migrations.AddField(
            model_name='fund',
            name='return_1m',
            field=models.DecimalField(blank=True, decimal_places=4, help_text='近1月涨幅（%）', max_digits=12, null=True),
        ),
        migrations.AddField(
            model_name='fund',
            name='return_3m',
            field=models.DecimalField(blank=True, decimal_places=4, help_text='近3月涨幅（%）', max_digits=12, null=True),
        ),
        migrations.AddField(
            model_name='fund',
            name='return_30d',
            field=models.DecimalField(blank=True, decimal_places=4, help_text='近30天涨幅（%）', max_digits=12, null=True),
        ),
    ]
