package main

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

// newExporter picks where spans go, and it is the only place in the service
// that knows. OTEL_EXPORTER_OTLP_ENDPOINT set means there is a Collector to
// talk to; unset means local development, where printing to the log beats
// failing to reach a Collector that was never started.
//
// The endpoint is a value at runtime rather than a constant because the
// Collector runs as a DaemonSet — a separate pod with its own IP, on whichever
// node the scheduler happened to pick. The pod learns that node's IP from the
// downward API (see k8s/app.yaml). localhost would be the pod's own loopback,
// where nothing is listening.
func newExporter(ctx context.Context) (sdktrace.SpanExporter, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		return stdouttrace.New(stdouttrace.WithPrettyPrint())
	}

	// WithEndpointURL derives transport security from the scheme, so an
	// http:// endpoint is plaintext. That is deliberate here: this hop never
	// leaves the node, and terminating TLS at a local agent buys nothing while
	// costing certificate rotation the project does not need yet.
	return otlptracegrpc.New(ctx, otlptracegrpc.WithEndpointURL(endpoint))
}

// newTracerProvider wires up the trace half of OpenTelemetry: where spans go
// (the exporter), who they say they came from (the resource), and how trace
// context crosses a network hop (the propagator).
func newTracerProvider(ctx context.Context) (*sdktrace.TracerProvider, error) {
	exp, err := newExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("trace exporter: %w", err)
	}

	// Attributes describing the producer of the telemetry, attached once to
	// every span instead of being repeated per span. Without service.name,
	// the backend files everything under "unknown_service:app".
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(envOr("OTEL_SERVICE_NAME", "chaos-gym-app")),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("build resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		// Batch rather than export one span per request: the exporter runs on
		// its own goroutine so a slow backend adds latency to nothing.
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)

	// Registering globally is what lets otelhttp and any library find this
	// provider without it being threaded through every call site.
	otel.SetTracerProvider(tp)

	// W3C traceparent header in, traceparent header out. This is the only
	// reason a trace can span two services instead of stopping at this one.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, nil
}
