package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"janusgate/internal/auth"
)

const (
	HeaderAuthorization = "Authorization"
	HeaderXUserID       = "X-User-Id"
	HeaderXUserName     = "X-User-Name"
	HeaderXUserRoles    = "X-User-Roles"
)

type errorResponse struct {
	Error     string    `json:"error"`
	Message   string    `json:"message"`
	Path      string    `json:"path"`
	Code      int       `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}

func Authenticate(validator auth.TokenValidator) Middleware {
	if validator == nil {
		panic("Authenticate middleware initialized with nil TokenValidator")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get(HeaderAuthorization)
			if authHeader == "" {
				slog.WarnContext(r.Context(), "Missing Authorization header", "path", r.URL.Path, "ip", extractClientIP(r))
				writeJSONError(w, r, "Authorization header is required", http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				slog.WarnContext(r.Context(), "Invalid Authorization header format", "path", r.URL.Path, "ip", extractClientIP(r))
				writeJSONError(w, r, "Authorization header format must be 'Bearer <token>'", http.StatusUnauthorized)
				return
			}

			tokenStr := strings.TrimSpace(parts[1])
			if tokenStr == "" {
				slog.WarnContext(r.Context(), "Empty Bearer token provided", "path", r.URL.Path, "ip", extractClientIP(r))
				writeJSONError(w, r, "Token cannot be empty", http.StatusUnauthorized)
				return
			}

			claims, err := validator.ValidateToken(r.Context(), tokenStr)
			if err != nil {
				slog.WarnContext(r.Context(), "Invalid or expired JWT token",
					"error", err.Error(),
					"path", r.URL.Path,
					"ip", extractClientIP(r),
				)
				writeJSONError(w, r, "Invalid or expired authentication token", http.StatusUnauthorized)
				return
			}

			ctx := auth.InjectClaims(r.Context(), claims)
			r = r.WithContext(ctx)

			r.Header.Del(HeaderAuthorization)

			r.Header.Del(HeaderXUserID)
			r.Header.Del(HeaderXUserName)
			r.Header.Del(HeaderXUserRoles)

			r.Header.Set(HeaderXUserID, claims.UserID)
			r.Header.Set(HeaderXUserName, claims.Username)
			if len(claims.Roles) > 0 {
				r.Header.Set(HeaderXUserRoles, strings.Join(claims.Roles, ","))
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeJSONError(w http.ResponseWriter, r *http.Request, message string, statusCode int) {
	if statusCode == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", `Bearer realm="JanusGate"`)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := errorResponse{
		Error:     http.StatusText(statusCode),
		Message:   message,
		Path:      r.URL.Path,
		Code:      statusCode,
		Timestamp: time.Now().UTC(),
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.ErrorContext(r.Context(), "Failed to encode error response", "error", err.Error())
	}
}
