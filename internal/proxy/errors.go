package proxy

import (
	"encoding/json"
	"net/http"
	"time"
)

type ErrorResponse struct {
	Error     string    `json:"error"`
	Message   string    `json:"message"`
	Path      string    `json:"path"`
	Code      int       `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}

func respondCircuitOpen(w http.ResponseWriter, path string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)

	resp := ErrorResponse{
		Error:     "Service Unavailable",
		Message:   "Upstream service is temporarily unavailable due to high error rates (Circuit Breaker Open).",
		Path:      path,
		Code:      http.StatusServiceUnavailable,
		Timestamp: time.Now().UTC(),
	}

	_ = json.NewEncoder(w).Encode(resp)
}
