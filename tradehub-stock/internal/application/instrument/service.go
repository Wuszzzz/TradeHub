package instrument

import (
	"context"
	"fmt"
	"strings"
)

type Provider interface {
	Search(keyword string) ([]map[string]any, error)
	Profile(symbol string) (map[string]any, error)
}

type Service struct {
	provider Provider
}

func NewService(provider Provider) *Service {
	return &Service{provider: provider}
}

func (s *Service) Search(_ context.Context, keyword string) ([]map[string]any, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, fmt.Errorf("keyword is required")
	}
	return s.provider.Search(keyword)
}

func (s *Service) Profile(_ context.Context, symbol string) (map[string]any, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	return s.provider.Profile(symbol)
}
