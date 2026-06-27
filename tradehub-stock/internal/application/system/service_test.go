package system

import "testing"

func TestServiceOverviewIncludesPlannedModules(t *testing.T) {
	service := NewService()
	overview := service.Overview()

	if overview["project"] != "tradehub" {
		t.Fatalf("expected project tradehub, got %v", overview["project"])
	}

	architecture, ok := overview["architecture"].(map[string]any)
	if !ok {
		t.Fatalf("expected architecture map, got %T", overview["architecture"])
	}
	if architecture["mode"] != "modular_large_service_with_process_level_decoupling" {
		t.Fatalf("unexpected mode: %v", architecture["mode"])
	}
}
