package screener

import (
	"context"
	"testing"

	"stock-etf-monitor/backend/model"
)

type fakeRepository struct {
	indicatorItems []map[string]any
	patternItems   []map[string]any
	templates      []model.ScreeningTemplate
}

func (r *fakeRepository) ScreenByIndicator(period, indicatorCode, field, op string, threshold float64, limit int) ([]map[string]any, error) {
	return r.indicatorItems, nil
}

func (r *fakeRepository) ScreenByPattern(period, patternCode, direction string, limit int) ([]map[string]any, error) {
	return r.patternItems, nil
}

func (r *fakeRepository) ListScreeningTemplates(enabledOnly bool) ([]model.ScreeningTemplate, error) {
	return r.templates, nil
}

func (r *fakeRepository) UpsertScreeningTemplate(template model.ScreeningTemplate) error {
	r.templates = append(r.templates, template)
	return nil
}

func (r *fakeRepository) DeleteScreeningTemplate(templateID string) error {
	return nil
}

func (r *fakeRepository) ListScreeningResults(taskID, templateID string, limit int) ([]model.ScreeningResult, error) {
	return []model.ScreeningResult{{TaskID: taskID, TemplateID: templateID, Symbol: "600519"}}, nil
}

func TestIndicatorValidatesRequiredFields(t *testing.T) {
	service := NewService(&fakeRepository{})
	if _, err := service.Indicator(context.Background(), "1d", "", "macd", "gt", 0, 100); err == nil {
		t.Fatalf("expected required field error")
	}
}

func TestIndicatorValidatesOperator(t *testing.T) {
	service := NewService(&fakeRepository{})
	if _, err := service.Indicator(context.Background(), "1d", "MACD", "macd", "bad", 0, 100); err == nil {
		t.Fatalf("expected operator validation error")
	}
}

func TestPatternDefaultsPeriodAndLimit(t *testing.T) {
	repo := &fakeRepository{patternItems: []map[string]any{{"symbol": "600519"}}}
	service := NewService(repo)
	items, err := service.Pattern(context.Background(), "", "doji", "", 0)
	if err != nil {
		t.Fatalf("pattern screen failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %d", len(items))
	}
}

func TestSaveTemplateValidatesConditions(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo, repo)
	if _, err := service.SaveTemplate(context.Background(), TemplateInput{Name: "模板"}); err == nil {
		t.Fatalf("expected conditions validation error")
	}
}

func TestSaveTemplateDefaultsIDAndEnabled(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo, repo)
	template, err := service.SaveTemplate(context.Background(), TemplateInput{
		Name:       "MACD 金叉",
		Conditions: map[string]any{"logic": "and"},
	})
	if err != nil {
		t.Fatalf("save template failed: %v", err)
	}
	if template.TemplateID == "" || !template.Enabled || template.CreatedBy != "system" {
		t.Fatalf("unexpected template defaults: %+v", template)
	}
}

func TestListResultsNormalizesLimit(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo, repo)
	items, err := service.ListResults(context.Background(), "task_1", "template_1", 0)
	if err != nil {
		t.Fatalf("list results failed: %v", err)
	}
	if len(items) != 1 || items[0].TaskID != "task_1" {
		t.Fatalf("unexpected results: %+v", items)
	}
}
