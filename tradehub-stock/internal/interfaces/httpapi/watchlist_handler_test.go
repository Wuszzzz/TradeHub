package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stock-etf-monitor/backend/internal/application/watchlist"
	"stock-etf-monitor/backend/model"
)

type fakeWatchlistRepository struct {
	groups []model.WatchlistGroup
	items  []model.WatchlistItem
}

func (r *fakeWatchlistRepository) ListWatchlistGroups() ([]model.WatchlistGroup, error) {
	return r.groups, nil
}

func (r *fakeWatchlistRepository) UpsertWatchlistGroup(group model.WatchlistGroup) error {
	r.groups = append(r.groups, group)
	return nil
}

func (r *fakeWatchlistRepository) DeleteWatchlistGroup(groupID string) error {
	return nil
}

func (r *fakeWatchlistRepository) ListWatchlistItems(groupID string) ([]model.WatchlistItem, error) {
	return r.items, nil
}

func (r *fakeWatchlistRepository) UpsertWatchlistItem(item model.WatchlistItem) error {
	r.items = append(r.items, item)
	return nil
}

func (r *fakeWatchlistRepository) DeleteWatchlistItem(itemID string) error {
	return nil
}

func TestWatchlistGroupsPostReturnsUnifiedResponse(t *testing.T) {
	handler := NewWatchlistHandler(watchlist.NewService(&fakeWatchlistRepository{}))
	req := httptest.NewRequest(http.MethodPost, "/api/stock/v1/watchlist/groups", strings.NewReader(`{"name":"重点"}`))
	rec := httptest.NewRecorder()

	handler.Groups(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	var body Response
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success || body.Code != "OK" {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestWatchlistItemsPostRejectsEmptySymbol(t *testing.T) {
	handler := NewWatchlistHandler(watchlist.NewService(&fakeWatchlistRepository{}))
	req := httptest.NewRequest(http.MethodPost, "/api/stock/v1/watchlist/items", strings.NewReader(`{"symbol":" "}`))
	rec := httptest.NewRecorder()

	handler.Items(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
