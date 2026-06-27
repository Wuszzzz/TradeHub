package quant

import (
	"context"
	"fmt"
	"strings"
	"time"

	"stock-etf-monitor/backend/model"
)

type Repository interface {
	ListIndicatorDefinitions(category string, enabledOnly bool) ([]model.IndicatorDefinition, error)
	UpsertIndicatorDefinition(model.IndicatorDefinition) error
	ListPatternDefinitions(category string, enabledOnly bool) ([]model.PatternDefinition, error)
	UpsertPatternDefinition(model.PatternDefinition) error
}

type ResultRepository interface {
	QueryIndicatorValues(symbol, period, indicatorCode string, limit int) ([]map[string]any, error)
	QueryPatternHits(symbol, period, patternCode string, limit int) ([]map[string]any, error)
}

type Service struct {
	repo    Repository
	results ResultRepository
}

type IndicatorInput struct {
	IndicatorCode string `json:"indicator_code"`
	Name          string `json:"name"`
	Category      string `json:"category"`
	Description   string `json:"description"`
	ParamsSchema  string `json:"params_schema"`
	OutputFields  string `json:"output_fields"`
	Enabled       *bool  `json:"enabled"`
}

type PatternInput struct {
	PatternCode   string `json:"pattern_code"`
	Name          string `json:"name"`
	Category      string `json:"category"`
	TALibFunction string `json:"talib_function"`
	Direction     string `json:"direction"`
	Description   string `json:"description"`
	ParamsSchema  string `json:"params_schema"`
	Enabled       *bool  `json:"enabled"`
}

func NewService(repo Repository, results ...ResultRepository) *Service {
	service := &Service{repo: repo}
	if len(results) > 0 {
		service.results = results[0]
	}
	return service
}

func (s *Service) ListIndicators(_ context.Context, category string, enabledOnly bool) ([]model.IndicatorDefinition, error) {
	return s.repo.ListIndicatorDefinitions(strings.TrimSpace(category), enabledOnly)
}

func (s *Service) ListIndicatorValues(_ context.Context, symbol, period, indicatorCode string, limit int) ([]map[string]any, error) {
	if s.results == nil {
		return nil, fmt.Errorf("quant result repository is not configured")
	}
	symbol = strings.TrimSpace(symbol)
	indicatorCode = strings.ToUpper(strings.TrimSpace(indicatorCode))
	if symbol == "" || indicatorCode == "" {
		return nil, fmt.Errorf("symbol and indicator_code are required")
	}
	period = normalizePeriod(period)
	return s.results.QueryIndicatorValues(symbol, period, indicatorCode, normalizeLimit(limit))
}

func (s *Service) CreateIndicator(_ context.Context, input IndicatorInput) (model.IndicatorDefinition, error) {
	code := strings.ToUpper(strings.TrimSpace(input.IndicatorCode))
	name := strings.TrimSpace(input.Name)
	if code == "" || name == "" {
		return model.IndicatorDefinition{}, fmt.Errorf("indicator_code and name are required")
	}
	category := strings.TrimSpace(input.Category)
	if category == "" {
		category = "technical"
	}
	now := time.Now().UTC()
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	definition := model.IndicatorDefinition{
		IndicatorCode: code,
		Name:          name,
		Category:      category,
		Description:   strings.TrimSpace(input.Description),
		ParamsSchema:  defaultJSON(input.ParamsSchema, "{}"),
		OutputFields:  defaultJSON(input.OutputFields, "[]"),
		Enabled:       enabled,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	return definition, s.repo.UpsertIndicatorDefinition(definition)
}

func (s *Service) ListPatterns(_ context.Context, category string, enabledOnly bool) ([]model.PatternDefinition, error) {
	return s.repo.ListPatternDefinitions(strings.TrimSpace(category), enabledOnly)
}

func (s *Service) ListPatternHits(_ context.Context, symbol, period, patternCode string, limit int) ([]map[string]any, error) {
	if s.results == nil {
		return nil, fmt.Errorf("quant result repository is not configured")
	}
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	period = normalizePeriod(period)
	return s.results.QueryPatternHits(symbol, period, strings.TrimSpace(patternCode), normalizeLimit(limit))
}

func (s *Service) CreatePattern(_ context.Context, input PatternInput) (model.PatternDefinition, error) {
	code := strings.TrimSpace(input.PatternCode)
	name := strings.TrimSpace(input.Name)
	function := strings.ToUpper(strings.TrimSpace(input.TALibFunction))
	if code == "" || name == "" || function == "" {
		return model.PatternDefinition{}, fmt.Errorf("pattern_code, name and talib_function are required")
	}
	category := strings.TrimSpace(input.Category)
	if category == "" {
		category = "candlestick"
	}
	direction := strings.TrimSpace(input.Direction)
	if direction == "" {
		direction = "both"
	}
	now := time.Now().UTC()
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	definition := model.PatternDefinition{
		PatternCode:   code,
		Name:          name,
		Category:      category,
		TALibFunction: function,
		Direction:     direction,
		Description:   strings.TrimSpace(input.Description),
		ParamsSchema:  defaultJSON(input.ParamsSchema, "{}"),
		Enabled:       enabled,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	return definition, s.repo.UpsertPatternDefinition(definition)
}

func defaultJSON(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func normalizePeriod(period string) string {
	period = strings.TrimSpace(period)
	if period == "" {
		return "1d"
	}
	return period
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 200
	}
	if limit > 5000 {
		return 5000
	}
	return limit
}
