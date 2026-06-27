package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Meta struct {
	RequestID string `json:"request_id"`
	Timestamp string `json:"timestamp"`
	Version   string `json:"version,omitempty"`
}

type ErrorDetail struct {
	Field  string `json:"field,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type Response struct {
	Success bool         `json:"success"`
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Data    any          `json:"data"`
	Meta    Meta         `json:"meta"`
	Error   *ErrorDetail `json:"error,omitempty"`
}

const RequestIDHeader = "X-Request-ID"

func RequestID(r *http.Request) string {
	if value := r.Header.Get(RequestIDHeader); value != "" {
		return value
	}
	return fmt.Sprintf("req_%s_%d", time.Now().Format("20060102_150405"), time.Now().UnixNano()%1_000_000)
}

func responseMeta(r *http.Request, version string) Meta {
	return Meta{
		RequestID: RequestID(r),
		Timestamp: time.Now().Format(time.RFC3339),
		Version:   version,
	}
}

func WriteJSON(w http.ResponseWriter, r *http.Request, status int, code string, message string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set(RequestIDHeader, RequestID(r))
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Response{
		Success: status >= 200 && status < 300,
		Code:    code,
		Message: message,
		Data:    data,
		Meta:    responseMeta(r, "v1"),
	})
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, code string, message string, field string, detail string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set(RequestIDHeader, RequestID(r))
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Response{
		Success: false,
		Code:    code,
		Message: message,
		Data:    nil,
		Meta:    responseMeta(r, "v1"),
		Error: &ErrorDetail{
			Field:  field,
			Detail: detail,
		},
	})
}
