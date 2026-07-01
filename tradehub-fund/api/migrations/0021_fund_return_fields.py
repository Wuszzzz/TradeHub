from django.db import migrations, models


class Migration(migrations.Migration):

    dependencies = [
        ('api', '0020_fund_evaluation_snapshot'),
    ]

    operations = [
        migrations.AddField(
            model_name='fund',
            name='return_1y',
            field=models.DecimalField(blank=True, decimal_places=4, help_text='近1年涨幅（%）', max_digits=12, null=True),
        ),
        migrations.AddField(
            model_name='fund',
            name='return_this_year',
            field=models.DecimalField(blank=True, decimal_places=4, help_text='今年以来涨幅（%）', max_digits=12, null=True),
        ),
    ]
