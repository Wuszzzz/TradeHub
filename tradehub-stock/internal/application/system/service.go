package system

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Health() map[string]any {
	return map[string]any{
		"status":  "ok",
		"service": "stock-api",
	}
}

func (s *Service) Overview() map[string]any {
	return map[string]any{
		"project": "tradehub",
		"service": "stock-api",
		"architecture": map[string]any{
			"mode": "modular_large_service_with_process_level_decoupling",
			"modules": []string{
				"instrument",
				"market",
				"watchlist",
				"alert",
				"paper",
				"task",
				"quant",
				"backtest",
				"ai",
			},
		},
	}
}
