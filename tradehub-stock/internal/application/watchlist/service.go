package watchlist

import (
	"context"
	"fmt"
	"strings"
	"time"

	"stock-etf-monitor/backend/model"
)

type Repository interface {
	ListWatchlistGroups() ([]model.WatchlistGroup, error)
	UpsertWatchlistGroup(model.WatchlistGroup) error
	DeleteWatchlistGroup(groupID string) error
	ListWatchlistItems(groupID string) ([]model.WatchlistItem, error)
	UpsertWatchlistItem(model.WatchlistItem) error
	DeleteWatchlistItem(itemID string) error
}

type Service struct {
	repo Repository
}

type GroupInput struct {
	GroupID   string `json:"group_id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

type ItemInput struct {
	ItemID    string `json:"item_id"`
	GroupID   string `json:"group_id"`
	Symbol    string `json:"symbol"`
	Name      string `json:"name"`
	Market    string `json:"market"`
	Note      string `json:"note"`
	SortOrder int    `json:"sort_order"`
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListGroups(_ context.Context) ([]model.WatchlistGroup, error) {
	return s.repo.ListWatchlistGroups()
}

func (s *Service) CreateGroup(_ context.Context, input GroupInput) (model.WatchlistGroup, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return model.WatchlistGroup{}, fmt.Errorf("name is required")
	}
	now := time.Now().UTC()
	groupID := strings.TrimSpace(input.GroupID)
	if groupID == "" {
		groupID = fmt.Sprintf("grp_%d", now.UnixNano())
	}
	group := model.WatchlistGroup{
		GroupID:   groupID,
		Name:      name,
		SortOrder: input.SortOrder,
		CreatedAt: now,
	}
	return group, s.repo.UpsertWatchlistGroup(group)
}

func (s *Service) DeleteGroup(_ context.Context, groupID string) error {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return fmt.Errorf("group_id is required")
	}
	return s.repo.DeleteWatchlistGroup(groupID)
}

func (s *Service) ListItems(_ context.Context, groupID string) ([]model.WatchlistItem, error) {
	return s.repo.ListWatchlistItems(strings.TrimSpace(groupID))
}

func (s *Service) CreateItem(_ context.Context, input ItemInput) (model.WatchlistItem, error) {
	symbol := strings.TrimSpace(input.Symbol)
	if symbol == "" {
		return model.WatchlistItem{}, fmt.Errorf("symbol is required")
	}
	groupID := strings.TrimSpace(input.GroupID)
	if groupID == "" {
		groupID = "default"
	}
	now := time.Now().UTC()
	itemID := strings.TrimSpace(input.ItemID)
	if itemID == "" {
		itemID = fmt.Sprintf("wli_%d", now.UnixNano())
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = symbol
	}
	market := strings.TrimSpace(input.Market)
	if market == "" {
		market = "CN-A"
	}
	item := model.WatchlistItem{
		ItemID:    itemID,
		GroupID:   groupID,
		Symbol:    symbol,
		Name:      name,
		Market:    market,
		Note:      input.Note,
		SortOrder: input.SortOrder,
		CreatedAt: now,
	}
	return item, s.repo.UpsertWatchlistItem(item)
}

func (s *Service) DeleteItem(_ context.Context, itemID string) error {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return fmt.Errorf("item_id is required")
	}
	return s.repo.DeleteWatchlistItem(itemID)
}
