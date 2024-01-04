package main

import (
	"context"
	"time"

	"github.com/go-stack/stack"
	"github.com/karlmutch/kv"
	"golang.org/x/exp/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"

	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracer trace.Tracer
)

func newMeterProvider(ctx context.Context, res *resource.Resource) (meterProvider *metric.MeterProvider, err kv.Error) {
	metricExporter, errGo := otlpmetricgrpc.New(ctx)
	if errGo != nil {
		return nil, kv.Wrap(errGo).With("stack", stack.Trace().TrimRuntime())
	}

	meterProvider = metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(metricExporter,
			// Default is 1m. Set to 3s for demonstrative purposes.
			metric.WithInterval(3*time.Second))),
	)

	// Handle shutdown properly so nothing leaks.
	go func() {
		<-ctx.Done()
		if errGo := meterProvider.Shutdown(context.Background()); errGo != nil {
			slog.WarnCtx(ctx, kv.Wrap(errGo).With("stack", stack.Trace().TrimRuntime()).Error())
		}
	}()

	// Register as global meter provider so that it can be used via otel.Meter
	// and accessed using otel.GetMeterProvider.
	// Most instrumentation libraries use the global meter provider as default.
	// If the global meter provider is not set then a no-op implementation
	// is used, which fails to generate data.
	otel.SetMeterProvider(meterProvider)

	return meterProvider, nil
}

func initTracer(ctx context.Context) (tracer trace.Tracer, err kv.Error) {
	exporter, errGo := otlptracegrpc.New(context.Background())
	if errGo != nil {
		return nil, kv.Wrap(errGo).With("stack", stack.Trace().TrimRuntime())
	}

	rsc := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("example/pingbuf"),
		// any attributes you want to set on your resource
	)

	bsp := sdktrace.NewBatchSpanProcessor(exporter)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(rsc),
		sdktrace.WithSpanProcessor(bsp),
	)

	// Handle shutdown properly so nothing leaks.
	go func() {
		<-ctx.Done()
		defer func() { _ = tp.Shutdown(ctx) }()
	}()

	otel.SetTracerProvider(tp)

	// Finally, set the tracer that can be used for this package.
	tracer = tp.Tracer("main/pingbuf")

	_, err = newMeterProvider(ctx, rsc)
	if err != nil {
		return nil, kv.Wrap(err).With("stack", stack.Trace().TrimRuntime())
	}

	return tracer, nil
}

func initO11y(ctx context.Context) (tp trace.Tracer, err kv.Error) {

	_, err = initTracer(ctx)
	if err != nil {
		return nil, kv.Wrap(err).With("stack", stack.Trace().TrimRuntime())
	}
	return tracer, nil
}
