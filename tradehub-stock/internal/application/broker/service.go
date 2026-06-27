package broker

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Status() map[string]any {
	return map[string]any{
		"broker":    "noop",
		"connected": false,
		"note":      "同花顺/券商对接尚未启用，当前仅模拟交易（paper trading）",
	}
}
