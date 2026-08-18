package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type contextKey string

const (
	RequestIDKey contextKey = "request_id"
)

const (
	HeaderXRequestID string = "X-Request-ID"
)

func RequestID() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := r.Header.Get(HeaderXRequestID)

			if reqID == "" || len(reqID) > 64 || !isValidRequestID(reqID) {
				reqID = uuid.New().String()
			}

			r.Header.Set(HeaderXRequestID, reqID)
			w.Header().Set(HeaderXRequestID, reqID)

			ctx := context.WithValue(r.Context(), RequestIDKey, reqID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

func isValidRequestID(id string) bool {
	for _, c := range id {
		if c < 32 || c > 126 {
			return false
		}
	}
	return !strings.ContainsAny(id, "<>\"'\\")
}
