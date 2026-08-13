package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"janusgate/internal/auth"
)

type contextKey string

const (
	UserClaimsKey contextKey = "user_claims"
)

type authErrorResponse struct {
	Error     string    `json:"error"`
	Message   string    `json:"message"`
	Path      string    `json:"path"`
	Code      int       `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}

func Authenticate(tokenMgr auth.TokenManager) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if tokenMgr == nil {
				slog.Error("Auth middleware called but TokenManager is nil")
				writeAuthError(w, r, "Internal authentication error", http.StatusInternalServerError)
				return
			}

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
					"error", err,
					"path", r.URL.Path,
					"ip", extractClientIP(r),
				)
				writeAuthError(w, r, "Invalid or expired authentication token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserClaims(ctx context.Context) (*auth.Claims, bool) {
	claims, ok := ctx.Value(UserClaimsKey).(*auth.Claims)
	return claims, ok
}

func writeAuthError(w http.ResponseWriter, r *http.Request, message string, statusCode int) {
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

func GetUserID(ctx context.Context) (string, bool) {
	if claims, ok := GetUserClaims(ctx); ok && claims != nil {
		return claims.UserID, true
	}
	return "", false
}

func GetUsername(ctx context.Context) (string, bool) {
	if claims, ok := GetUserClaims(ctx); ok && claims != nil {
		return claims.Username, true
	}
	return "", false
}

func GetUserRoles(ctx context.Context) ([]string, bool) {
	if claims, ok := GetUserClaims(ctx); ok && claims != nil {
		return claims.Roles, true
	}
	return nil, false
}

func HasRole(ctx context.Context, role string) bool {
	roles, ok := GetUserRoles(ctx)
	if !ok {
		return false
	}
	for _, r := range roles {
		if strings.EqualFold(r, role) {
			return true
		}
	}
	return false
}
