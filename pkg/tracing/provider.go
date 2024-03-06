package tracing

import (
	"context"
	"log"
	"time"

	otelpyroscope "github.com/pyroscope-io/otel-profiling-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

const (
	environment string = "dev"
)

var (
	Tracer = otel.Tracer("")
)

type Provider struct {
	serviceName       string
	exporterURL       string
	decoratedProvider sdktrace.TracerProvider
	originProvider    *trace.TracerProvider
}

// https://levelup.gitconnected.com/golang-opentelemetry-and-sentry-the-underrated-distributed-tracing-stack-69dcda886ffe
func NewProvider(ctx context.Context, serviceName, exporterURL string) (*Provider, error) {
	// setup exporter
	e, err := otlptrace.New(ctx, otlptracegrpc.NewClient(
		otlptracegrpc.WithEndpoint(exporterURL),
		otlptracegrpc.WithInsecure(), // as connection is not need to be secured in this project
		otlptracegrpc.WithTimeout(time.Second*2),
	))
	if err != nil {
		return nil, err
	}
	// Create the Jaeger exporter
	// e, err := jaeger.New(
	// 	jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(exporterURL)),
	// )
	// if err != nil {
	// 	log.Fatalln("Couldn't initialize exporter", err)
	// }

	// Create stdout exporter to be able to retrieve the collected spans.
	_, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		log.Fatalln("Couldn't initialize exporter", err)
	}

	// serup resource
	originProvider := trace.NewTracerProvider(
		trace.WithBatcher(e),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String("0.0.1"),
			attribute.String("environment", environment),
		)),
	)

	decoratedProvider := otelpyroscope.NewTracerProvider(originProvider,
		otelpyroscope.WithAppName(serviceName),
		otelpyroscope.WithPyroscopeURL(exporterURL),
		otelpyroscope.WithRootSpanOnly(true),
		otelpyroscope.WithAddSpanName(true),
		otelpyroscope.WithProfileURL(true),
		otelpyroscope.WithProfileBaselineURL(true),
	)

	return &Provider{
		serviceName:       serviceName,
		exporterURL:       exporterURL,
		decoratedProvider: decoratedProvider,
		originProvider:    originProvider,
	}, nil
}

func (p *Provider) RegisterAsGlobal() (shutFunc, error) {
	// set global provider
	otel.SetTracerProvider(p.originProvider)

	// set global propagator to tracecontext (the default is no-op).
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return p.originProvider.Shutdown, nil
}
