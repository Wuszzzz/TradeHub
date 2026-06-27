package broker

import "testing"

func TestStatusReturnsNoopBroker(t *testing.T) {
	status := NewService().Status()
	if status["broker"] != "noop" || status["connected"] != false {
		t.Fatalf("unexpected status: %+v", status)
	}
}
