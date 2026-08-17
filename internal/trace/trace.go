package trace

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials/insecure"
)

const defaultTracerName = "janusgate/tracer"

var globalTracer trace.Tracer

type Config struct {
	ServiceName string
	Environment string
	Endpoint    string
	Insecure    bool
	SampleRatio float64
}

func InitTracer(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "janusgate"
	}
	if cfg.Environment == "" {
		cfg.Environment = "development"
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = "localhost:4317"
	}

	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
	}

	if cfg.Insecure {
		opts = append(opts,
			otlptracegrpc.WithTLSCredentials(insecure.NewCredentials()),
		)
	}

	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.DeploymentEnvironmentKey.String(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace resource: %w", err)
	}

	bsp := sdktrace.NewBatchSpanProcessor(exporter)

	var sampler sdktrace.Sampler
	if cfg.Environment == "production" {
		ratio := cfg.SampleRatio
		if ratio == 0 {
			ratio = 0.01
		}
		sampler = sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
	} else {
		sampler = sdktrace.AlwaysSample()
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(bsp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	globalTracer = tp.Tracer(defaultTracerName)

	return tp.Shutdown, nil
}

func GetTracer() trace.Tracer {
	if globalTracer == nil {
		return otel.GetTracerProvider().Tracer(defaultTracerName)
	}
	return globalTracer
}
