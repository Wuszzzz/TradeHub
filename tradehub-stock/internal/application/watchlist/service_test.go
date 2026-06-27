package watchlist

import (
	"context"
	"testing"

	"stock-etf-monitor/backend/model"
)

type fakeRepository struct {
	groups []model.WatchlistGroup
	items  []model.WatchlistItem
}

func (r *fakeRepository) ListWatchlistGroups() ([]model.WatchlistGroup, error) {
	return r.groups, nil
}

func (r *fakeRepository) UpsertWatchlistGroup(group model.WatchlistGroup) error {
	r.groups = append(r.groups, group)
	return nil
}

func (r *fakeRepository) DeleteWatchlistGroup(groupID string) error {
	return nil
}

func (r *fakeRepository) ListWatchlistItems(groupID string) ([]model.WatchlistItem, error) {
	return r.items, nil
}

func (r *fakeRepository) UpsertWatchlistItem(item model.WatchlistItem) error {
	r.items = append(r.items, item)
	return nil
}

func (r *fakeRepository) DeleteWatchlistItem(itemID string) error {
	return nil
}

func TestCreateGroupValidatesName(t *testing.T) {
	service := NewService(&fakeRepository{})
	if _, err := service.CreateGroup(context.Background(), GroupInput{Name: " "}); err == nil {
		t.Fatalf("expected name validation error")
	}
}

func TestCreateItemDefaultsGroupNameAndMarket(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo)
	item, err := service.CreateItem(context.Background(), ItemInput{Symbol: "600519"})
	if err != nil {
		t.Fatalf("create item failed: %v", err)
	}
	if item.GroupID != "default" || item.Name != "600519" || item.Market != "CN-A" {
		t.Fatalf("unexpected item defaults: %+v", item)
	}
}

func TestDeleteItemValidatesID(t *testing.T) {
	service := NewService(&fakeRepository{})
	if err := service.DeleteItem(context.Background(), " "); err == nil {
		t.Fatalf("expected item_id validation error")
	}
}
