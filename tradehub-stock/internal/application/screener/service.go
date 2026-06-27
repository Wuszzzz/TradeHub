package screener

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"stock-etf-monitor/backend/model"
)

type ScreenRepository interface {
	ScreenByIndicator(period, indicatorCode, field, op string, threshold float64, limit int) ([]map[string]any, error)
	ScreenByPattern(period, patternCode, direction string, limit int) ([]map[string]any, error)
}

type TemplateRepository interface {
	ListScreeningTemplates(enabledOnly bool) ([]model.ScreeningTemplate, error)
	UpsertScreeningTemplate(model.ScreeningTemplate) error
	DeleteScreeningTemplate(templateID string) error
	ListScreeningResults(taskID, templateID string, limit int) ([]model.ScreeningResult, error)
}

type Service struct {
	screenRepo   ScreenRepository
	templateRepo TemplateRepository
}

type TemplateInput struct {
	TemplateID  string         `json:"template_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Conditions  map[string]any `json:"conditions"`
	Enabled     *bool          `json:"enabled"`
	CreatedBy   string         `json:"created_by"`
}

func NewService(screenRepo ScreenRepository, templateRepo ...TemplateRepository) *Service {
	service := &Service{screenRepo: screenRepo}
	if len(templateRepo) > 0 {
		service.templateRepo = templateRepo[0]
	}
	return service
}

func (s *Service) Indicator(_ context.Context, period, indicatorCode, field, op string, threshold float64, limit int) ([]map[string]any, error) {
	period = normalizePeriod(period)
	indicatorCode = strings.ToUpper(strings.TrimSpace(indicatorCode))
	field = strings.TrimSpace(field)
	op = normalizeOp(op)
	if indicatorCode == "" || field == "" {
		return nil, fmt.Errorf("indicator_code and field are required")
	}
	if op == "" {
		return nil, fmt.Errorf("op is invalid")
	}
	return s.screenRepo.ScreenByIndicator(period, indicatorCode, field, op, threshold, normalizeLimit(limit))
}

func (s *Service) Pattern(_ context.Context, period, patternCode, direction string, limit int) ([]map[string]any, error) {
	period = normalizePeriod(period)
	patternCode = strings.TrimSpace(patternCode)
	direction = strings.ToLower(strings.TrimSpace(direction))
	if patternCode == "" {
		return nil, fmt.Errorf("pattern_code is required")
	}
	if direction != "" && direction != "bullish" && direction != "bearish" && direction != "neutral" {
		return nil, fmt.Errorf("direction is invalid")
	}
	return s.screenRepo.ScreenByPattern(period, patternCode, direction, normalizeLimit(limit))
}

func (s *Service) ListTemplates(_ context.Context, enabledOnly bool) ([]model.ScreeningTemplate, error) {
	if s.templateRepo == nil {
		return nil, fmt.Errorf("screening template repository is not configured")
	}
	return s.templateRepo.ListScreeningTemplates(enabledOnly)
}

func (s *Service) SaveTemplate(_ context.Context, input TemplateInput) (model.ScreeningTemplate, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return model.ScreeningTemplate{}, fmt.Errorf("name is required")
	}
	conditions := input.Conditions
	if conditions == nil {
		return model.ScreeningTemplate{}, fmt.Errorf("conditions are required")
	}
	rawConditions, err := json.Marshal(conditions)
	if err != nil {
		return model.ScreeningTemplate{}, err
	}
	now := time.Now().UTC()
	templateID := strings.TrimSpace(input.TemplateID)
	if templateID == "" {
		templateID = fmt.Sprintf("screen_tpl_%d", now.UnixNano())
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	createdBy := strings.TrimSpace(input.CreatedBy)
	if createdBy == "" {
		createdBy = "system"
	}
	template := model.ScreeningTemplate{
		TemplateID:     templateID,
		Name:           name,
		Description:    strings.TrimSpace(input.Description),
		ConditionsJSON: string(rawConditions),
		Enabled:        enabled,
		CreatedBy:      createdBy,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if s.templateRepo == nil {
		return model.ScreeningTemplate{}, fmt.Errorf("screening template repository is not configured")
	}
	return template, s.templateRepo.UpsertScreeningTemplate(template)
}

func (s *Service) DeleteTemplate(_ context.Context, templateID string) error {
	if s.templateRepo == nil {
		return fmt.Errorf("screening template repository is not configured")
	}
	templateID = strings.TrimSpace(templateID)
	if templateID == "" {
		return fmt.Errorf("template_id is required")
	}
	return s.templateRepo.DeleteScreeningTemplate(templateID)
}

func (s *Service) ListResults(_ context.Context, taskID, templateID string, limit int) ([]model.ScreeningResult, error) {
	if s.templateRepo == nil {
		return nil, fmt.Errorf("screening template repository is not configured")
	}
	return s.templateRepo.ListScreeningResults(strings.TrimSpace(taskID), strings.TrimSpace(templateID), normalizeLimit(limit))
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
	if limit > 1000 {
		return 1000
	}
	return limit
}

func normalizeOp(op string) string {
	switch strings.ToLower(strings.TrimSpace(op)) {
	case "gt", "gte", "lt", "lte", "eq":
		return strings.ToLower(strings.TrimSpace(op))
	default:
		return ""
	}
}
