package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"janusgate/internal/auth"
)

type authErrorResponse struct {
	Error     string    `json:"error"`
	Message   string    `json:"message"`
	Path      string    `json:"path"`
	Code      int       `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}

func Authenticate(tokenMgr auth.TokenManager) Middleware {
	if tokenMgr == nil {
		panic("Authenticate middleware initialized with nil TokenManager")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				slog.Warn("Missing Authorization header", "path", r.URL.Path, "ip", extractClientIP(r))
				writeAuthError(w, r, "Authorization header is required", http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				slog.Warn("Invalid Authorization header format", "path", r.URL.Path, "ip", extractClientIP(r))
				writeAuthError(w, r, "Authorization header format must be 'Bearer <token>'", http.StatusUnauthorized)
				return
			}

			tokenStr := strings.TrimSpace(parts[1])
			if tokenStr == "" {
				slog.Warn("Empty Bearer token provided", "path", r.URL.Path, "ip", extractClientIP(r))
				writeAuthError(w, r, "Token cannot be empty", http.StatusUnauthorized)
				return
			}

			claims, err := tokenMgr.ValidateToken(tokenStr)
			if err != nil {
				slog.Warn("Invalid or expired JWT token",
					"error", err.Error(),
					"path", r.URL.Path,
					"ip", extractClientIP(r),
				)
				writeAuthError(w, r, "Invalid or expired authentication token", http.StatusUnauthorized)
				return
			}

			ctx := auth.InjectClaims(r.Context(), claims)
			r = r.WithContext(ctx)

			r.Header.Del("Authorization")

			r.Header.Set("X-User-Id", claims.UserID)
			r.Header.Set("X-User-Name", claims.Username)
			if len(claims.Roles) > 0 {
				r.Header.Set("X-User-Roles", strings.Join(claims.Roles, ","))
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeAuthError(w http.ResponseWriter, r *http.Request, message string, statusCode int) {
	if statusCode == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", `Bearer realm="JanusGate"`)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := authErrorResponse{
		Error:     http.StatusText(statusCode),
		Message:   message,
		Path:      r.URL.Path,
		Code:      statusCode,
		Timestamp: time.Now().UTC(),
	}

	_ = json.NewEncoder(w).Encode(resp)
}
