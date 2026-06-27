package instrument

import (
	"context"
	"testing"
)

type fakeProvider struct{}

func (p fakeProvider) Search(keyword string) ([]map[string]any, error) {
	return []map[string]any{{"symbol": "600519", "keyword": keyword}}, nil
}

func (p fakeProvider) Profile(symbol string) (map[string]any, error) {
	return map[string]any{"symbol": symbol, "name": "贵州茅台"}, nil
}

func TestServiceSearchValidatesKeyword(t *testing.T) {
	service := NewService(fakeProvider{})
	if _, err := service.Search(context.Background(), " "); err == nil {
		t.Fatalf("expected keyword validation error")
	}
}

func TestServiceSearchUsesProvider(t *testing.T) {
	service := NewService(fakeProvider{})
	items, err := service.Search(context.Background(), "茅台")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(items) != 1 || items[0]["symbol"] != "600519" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestServiceProfileValidatesSymbol(t *testing.T) {
	service := NewService(fakeProvider{})
	if _, err := service.Profile(context.Background(), " "); err == nil {
		t.Fatalf("expected symbol validation error")
	}
}
