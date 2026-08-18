package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				if err == http.ErrAbortHandler {
					panic(err)
				}

				stackTrace := string(debug.Stack())

				slog.Error("CRITICAL: Recovered from a panic",
					slog.Any("error", err),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.String("stack_trace", stackTrace),
				)

				if rec := getRecorder(w); rec != nil && rec.StatusCode != 0 {
					return
				}

				WriteJSONError(w, r, http.StatusInternalServerError, "An unexpected internal error occurred.")
			}
		}()

		next.ServeHTTP(w, r)
	})
}
