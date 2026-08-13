package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

type panicErrorResponse struct {
	Error     string    `json:"error"`
	Message   string    `json:"message"`
	Path      string    `json:"path"`
	Code      int       `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				stackTrace := string(debug.Stack())

				slog.Error("CRITICAL: Recovered from a panic",
					slog.Any("error", err),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.String("stack_trace", stackTrace),
				)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)

				resp := panicErrorResponse{
					Error:     "Internal Server Error",
					Message:   "An unexpected internal error occurred.",
					Path:      r.URL.Path,
					Code:      http.StatusInternalServerError,
					Timestamp: time.Now().UTC(),
				}

				_ = json.NewEncoder(w).Encode(resp)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
