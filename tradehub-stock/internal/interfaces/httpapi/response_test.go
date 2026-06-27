package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSONUsesUnifiedShapeAndRequestID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/system/v1/health", nil)
	req.Header.Set(RequestIDHeader, "req_test")
	rec := httptest.NewRecorder()

	WriteJSON(rec, req, http.StatusOK, "OK", "ok", map[string]any{"status": "ok"})

	if rec.Header().Get(RequestIDHeader) != "req_test" {
		t.Fatalf("expected request id header to be propagated")
	}

	var body Response
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success || body.Code != "OK" || body.Meta.RequestID != "req_test" {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestWriteErrorUsesUnifiedShape(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/system/v1/health", nil)
	rec := httptest.NewRecorder()

	WriteError(rec, req, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "", "")

	var body Response
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Success || body.Code != "METHOD_NOT_ALLOWED" || body.Error == nil {
		t.Fatalf("unexpected error response: %+v", body)
	}
}
