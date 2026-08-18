package middleware

import (
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	internalTrace "janusgate/internal/trace"
)

func Trace() Middleware {
	tracer := internalTrace.GetTracer()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			pathPattern := r.Pattern
			if pathPattern == "" {
				pathPattern = "unknown_route"
			}
			spanName := fmt.Sprintf("HTTP %s %s", r.Method, pathPattern)

			ctx, span := tracer.Start(
				ctx,
				spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPRequestMethodKey.String(r.Method),
					semconv.URLPathKey.String(r.URL.Path),
					semconv.HTTPRouteKey.String(pathPattern),
					semconv.UserAgentOriginalKey.String(r.UserAgent()),
					semconv.ClientAddressKey.String(extractClientIP(r)),
				),
			)
			defer span.End()

			rw := getRecorder(w)

			next.ServeHTTP(rw, r.WithContext(ctx))

			status := rw.StatusCode
			if status == 0 {
				status = http.StatusOK
			}

			span.SetAttributes(semconv.HTTPResponseStatusCodeKey.Int(status))

			if status >= 500 {
				span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d error", status))
			} else {
				span.SetStatus(codes.Ok, "OK")
			}
		})
	}
}
