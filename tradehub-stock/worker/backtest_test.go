package main

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"stock-etf-monitor/backend/model"
)

func TestEvaluateBacktestTradeAppliesRules(t *testing.T) {
	base := time.Date(2026, 6, 1, 9, 30, 0, 0, time.UTC)
	bars := []BacktestBar{
		{TS: base, Close: 100},
		{TS: base.AddDate(0, 0, 1), Close: 108},
		{TS: base.AddDate(0, 0, 2), Close: 116},
		{TS: base.AddDate(0, 0, 3), Close: 120},
	}

	trade := evaluateBacktestTrade("600519", "1d", bars, 3, 0.001, 0.001, 0.05, 0.10, "000300", 0.03)

	if trade.ExitTime != bars[2].TS {
		t.Fatalf("expected take-profit exit on third bar, got %s", trade.ExitTime)
	}
	if trade.Meta["exit_reason"] != "take_profit" {
		t.Fatalf("unexpected exit reason: %+v", trade.Meta)
	}
	expected := ((116*0.999)-(100*1.001))/(100*1.001) - 0.002
	if math.Abs(trade.ReturnPct-expected) > 1e-9 {
		t.Fatalf("unexpected net return: got %v want %v", trade.ReturnPct, expected)
	}
	if math.Abs(trade.ExcessReturnPct-(trade.ReturnPct-0.03)) > 1e-9 {
		t.Fatalf("unexpected excess return: %v", trade.ExcessReturnPct)
	}
}

func TestSummarizeBacktestTrades(t *testing.T) {
	trades := []BacktestTrade{
		{ReturnPct: 0.10, ExcessReturnPct: 0.05, BenchmarkSymbol: "000300", BenchmarkReturnPct: 0.05},
		{ReturnPct: -0.20, ExcessReturnPct: -0.25, BenchmarkSymbol: "000300", BenchmarkReturnPct: 0.05},
		{ReturnPct: 0.05, ExcessReturnPct: 0.00, BenchmarkSymbol: "000300", BenchmarkReturnPct: 0.05},
	}

	summary := summarizeBacktestTrades("task_1", trades, map[string]any{"period": "1d"})

	if summary.TotalTrades != 3 {
		t.Fatalf("unexpected total trades: %d", summary.TotalTrades)
	}
	if math.Abs(summary.WinRate-(2.0/3.0)) > 1e-9 {
		t.Fatalf("unexpected win rate: %v", summary.WinRate)
	}
	expectedTotal := (1.10 * 0.80 * 1.05) - 1
	if math.Abs(summary.TotalReturnPct-expectedTotal) > 1e-9 {
		t.Fatalf("unexpected total return: got %v want %v", summary.TotalReturnPct, expectedTotal)
	}
	if math.Abs(summary.MaxDrawdownPct-0.20) > 1e-9 {
		t.Fatalf("unexpected max drawdown: %v", summary.MaxDrawdownPct)
	}
	if summary.ReturnDistribution["gain_gt_10"] != 1 || summary.ReturnDistribution["loss_gt_10"] != 1 || summary.ReturnDistribution["gain_5_10"] != 1 {
		t.Fatalf("unexpected distribution: %+v", summary.ReturnDistribution)
	}
}

func TestApplyStrategyTemplateParamsDoesNotOverrideTaskParams(t *testing.T) {
	repo := &PostgresRepository{}
	backtestParams, _ := json.Marshal(map[string]any{"hold_bars": 30, "period": "1d"})
	riskParams, _ := json.Marshal(map[string]any{"stop_loss": 0.08})
	template := model.StrategyTemplate{
		StrategyID:          "strategy_1",
		Name:                "模板策略",
		ScreeningTemplateID: "screen_1",
		BacktestParamsJSON:  string(backtestParams),
		RiskParamsJSON:      string(riskParams),
	}
	params := map[string]any{
		"strategy_id": "strategy_1",
		"hold_bars":   10,
	}
	runner := &TaskRunner{repo: repo}
	repoGetStrategyTemplate = func(_ *PostgresRepository, strategyID string) (*model.StrategyTemplate, error) {
		if strategyID != "strategy_1" {
			t.Fatalf("unexpected strategy id: %s", strategyID)
		}
		return &template, nil
	}
	defer func() { repoGetStrategyTemplate = (*PostgresRepository).GetStrategyTemplate }()

	if err := runner.applyStrategyTemplateParams(params); err != nil {
		t.Fatalf("apply strategy failed: %v", err)
	}
	if params["hold_bars"].(int) != 10 {
		t.Fatalf("task param should win, got %+v", params["hold_bars"])
	}
	if params["period"] != "1d" || params["stop_loss"] == nil || params["template_id"] != "screen_1" {
		t.Fatalf("template params not applied: %+v", params)
	}
}

func TestApplyStrategyTemplateParamsSetsConditions(t *testing.T) {
	repo := &PostgresRepository{}
	conditions, _ := json.Marshal(map[string]any{"logic": "and"})
	template := model.StrategyTemplate{
		StrategyID:     "strategy_2",
		ConditionsJSON: string(conditions),
	}
	params := map[string]any{"strategy_id": "strategy_2"}
	repoGetStrategyTemplate = func(_ *PostgresRepository, strategyID string) (*model.StrategyTemplate, error) {
		if strategyID != "strategy_2" {
			t.Fatalf("unexpected strategy id: %s", strategyID)
		}
		return &template, nil
	}
	defer func() { repoGetStrategyTemplate = (*PostgresRepository).GetStrategyTemplate }()

	runner := &TaskRunner{repo: repo}
	if err := runner.applyStrategyTemplateParams(params); err != nil {
		t.Fatalf("apply strategy failed: %v", err)
	}
	if params["conditions"] == nil {
		t.Fatalf("expected conditions from strategy template")
	}
}

func TestApplyStrategyTemplateParamsPrefersSnapshot(t *testing.T) {
	params := map[string]any{
		"strategy_id": "strategy_3",
		"strategy_snapshot": map[string]any{
			"strategy_id":           "strategy_3",
			"name":                  "快照策略",
			"screening_template_id": "screen_snapshot",
			"backtest_params_json":  `{"hold_bars":55}`,
			"risk_params_json":      `{"stop_loss":0.12}`,
		},
	}
	runner := &TaskRunner{repo: &PostgresRepository{}}
	if err := runner.applyStrategyTemplateParams(params); err != nil {
		t.Fatalf("apply strategy snapshot failed: %v", err)
	}
	if params["template_id"] != "screen_snapshot" || params["hold_bars"] != float64(55) && params["hold_bars"] != 55 {
		t.Fatalf("snapshot params not applied: %+v", params)
	}
}
