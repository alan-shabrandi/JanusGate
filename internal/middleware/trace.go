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

			spanName := fmt.Sprintf("HTTP %s %s", r.Method, r.URL.Path)

			ctx, span := tracer.Start(
				ctx,
				spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPRequestMethodKey.String(r.Method),
					semconv.URLPathKey.String(r.URL.Path),
					semconv.UserAgentOriginalKey.String(r.UserAgent()),
					semconv.ClientAddressKey.String(r.RemoteAddr),
				),
			)
			defer span.End()

			rw := newResponseWriter(w)

			next.ServeHTTP(rw, r.WithContext(ctx))

			span.SetAttributes(semconv.HTTPResponseStatusCodeKey.Int(rw.statusCode))

			if rw.statusCode >= 500 {
				span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d error", rw.statusCode))
			} else {
				span.SetStatus(codes.Ok, "OK")
			}
		})
	}
}
