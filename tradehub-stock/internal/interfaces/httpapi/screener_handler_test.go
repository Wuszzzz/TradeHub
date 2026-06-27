package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"stock-etf-monitor/backend/internal/application/screener"
	"stock-etf-monitor/backend/model"
)

type fakeScreenerRepository struct {
	indicatorItems []map[string]any
	patternItems   []map[string]any
	templates      []model.ScreeningTemplate
}

func (r *fakeScreenerRepository) ScreenByIndicator(period, indicatorCode, field, op string, threshold float64, limit int) ([]map[string]any, error) {
	return r.indicatorItems, nil
}

func (r *fakeScreenerRepository) ScreenByPattern(period, patternCode, direction string, limit int) ([]map[string]any, error) {
	return r.patternItems, nil
}

func (r *fakeScreenerRepository) ListScreeningTemplates(enabledOnly bool) ([]model.ScreeningTemplate, error) {
	return r.templates, nil
}

func (r *fakeScreenerRepository) UpsertScreeningTemplate(template model.ScreeningTemplate) error {
	r.templates = append(r.templates, template)
	return nil
}

func (r *fakeScreenerRepository) DeleteScreeningTemplate(templateID string) error {
	return nil
}

func (r *fakeScreenerRepository) ListScreeningResults(taskID, templateID string, limit int) ([]model.ScreeningResult, error) {
	return []model.ScreeningResult{{TaskID: taskID, TemplateID: templateID, Symbol: "600519"}}, nil
}

func TestScreenerIndicatorReturnsUnifiedResponse(t *testing.T) {
	repo := &fakeScreenerRepository{indicatorItems: []map[string]any{{"symbol": "600519"}}}
	handler := NewScreenerHandler(screener.NewService(repo))
	req := httptest.NewRequest(http.MethodGet, "/api/stock/v1/screener/indicator?indicator_code=MACD&field=macd&op=gt&threshold=0", nil)
	rec := httptest.NewRecorder()

	handler.Indicator(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body Response
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success || body.Code != "OK" {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestScreenerPatternRejectsMissingPattern(t *testing.T) {
	handler := NewScreenerHandler(screener.NewService(&fakeScreenerRepository{}))
	req := httptest.NewRequest(http.MethodGet, "/api/stock/v1/screener/pattern", nil)
	rec := httptest.NewRecorder()

	handler.Pattern(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestScreenerTemplatePostReturnsCreated(t *testing.T) {
	repo := &fakeScreenerRepository{}
	handler := NewScreenerHandler(screener.NewService(repo, repo))
	body := []byte(`{"name":"MACD 红柱且吞噬形态","conditions":{"logic":"and","indicator_conditions":[{"period":"1d","indicator_code":"MACD","field":"macd","op":"gt","threshold":0}]}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/stock/v1/screener/templates", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Templates(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var response Response
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Success || response.Code != "OK" {
		t.Fatalf("unexpected response: %+v", response)
	}
	if len(repo.templates) != 1 {
		t.Fatalf("expected one saved template, got %d", len(repo.templates))
	}
}

func TestScreenerTemplateDeleteRequiresTemplateID(t *testing.T) {
	repo := &fakeScreenerRepository{}
	handler := NewScreenerHandler(screener.NewService(repo, repo))
	req := httptest.NewRequest(http.MethodDelete, "/api/stock/v1/screener/templates", nil)
	rec := httptest.NewRecorder()

	handler.Templates(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestScreenerResultsReturnsUnifiedResponse(t *testing.T) {
	repo := &fakeScreenerRepository{}
	handler := NewScreenerHandler(screener.NewService(repo, repo))
	req := httptest.NewRequest(http.MethodGet, "/api/stock/v1/screener/results?task_id=task_1", nil)
	rec := httptest.NewRecorder()

	handler.Results(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response Response
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Success || response.Code != "OK" {
		t.Fatalf("unexpected response: %+v", response)
	}
}
