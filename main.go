package main

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	mc "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var (
	tracer  trace.Tracer
	counter mc.Int64Counter
	meter   mc.Meter
	logger  otellog.Logger
)

func initLoggerProvider(ctx context.Context, r *resource.Resource) *sdklog.LoggerProvider {
	exporter, err := otlploggrpc.New(ctx, otlploggrpc.WithInsecure())
	if err != nil {
		log.Fatalf("new otlp log grpc exporter failed: %v", zap.Error(err))
	}
	provider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(r),
	)
	return provider
}

func initMeter(ctx context.Context, r *resource.Resource) *metric.MeterProvider {
	exporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithInsecure())
	if err != nil {
		log.Fatalf("new otlp metric grpc exporter failed: %v", zap.Error(err))
	}
	provider := metric.NewMeterProvider(metric.WithReader(metric.NewPeriodicReader(exporter)), metric.WithResource(r))
	return provider
}

func initTracerProvider(ctx context.Context, r *resource.Resource) *sdktrace.TracerProvider {
	// Create exporter.
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure())
	if err != nil {
		log.Fatalf("failed to construct new exporter: %v", err)
	}

	// Create tracer provider.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(r),
	)

	// Set tracer provider and propagator.
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return tp
}
func main() {

	ctx := context.Background()
	// Create resource.
	res, err := resource.New(ctx,
		resource.WithContainer(),
		resource.WithFromEnv(),
	)
	if err != nil {
		log.Fatalf("failed to create resource: %v", err)
	}
	tp := initTracerProvider(ctx, res)
	tracer = tp.Tracer("workshop-tracer")

	defer func() {
		if err := tp.Shutdown(ctx); err != nil {
			log.Fatalf("Error shutting down tracer provider: %v", err)
		}
	}()

	lp := initLoggerProvider(ctx, res)
	defer func() {
		if err := lp.Shutdown(ctx); err != nil {
			log.Fatalf("Error shutting down logger provider: %v", err)
		}
	}()
	global.SetLoggerProvider(lp)
	logger = global.Logger("workshop-logger")

	mp := initMeter(ctx, res)
	defer func() {
		if err := mp.Shutdown(ctx); err != nil {
			log.Fatalf("Error shutting down meter provider: %v", err)
		}
	}()

	meter = mp.Meter("workshop-meter")
	counter, err = meter.Int64Counter("otel.workshop.metric")
	if err != nil {
		panic(err)
	}

	for {
		makeMetricSpanLog()
		time.Sleep(15 * time.Second)
	}
}

func makeMetricSpanLog() {
	ctx := context.Background()
	_, span := tracer.Start(ctx, "work", trace.WithAttributes(attribute.String("name", "custom-resource")))
	counter.Add(ctx, 1, mc.WithAttributes(
		attribute.String("workshop", "ok"),
	))
	span.End()

	var record otellog.Record
	record.SetBody(otellog.StringValue("workshop in progress"))
	logger.Emit(ctx, record)
}
