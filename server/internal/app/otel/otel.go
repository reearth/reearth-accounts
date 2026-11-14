package otel

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/reearth/reearth-accounts/server/pkg/applog"
	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"golang.org/x/oauth2/google"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
)

type ExporterType string

const (
	ExporterTypeJaeger ExporterType = "jaeger"
	ExporterTypeGCP    ExporterType = "gcp"
	ExporterTypeOTLP   ExporterType = "otlp"
)

const (
	gcpProjectIDAttribute = "gcp.project_id"
	gcpCloudTraceEndpoint = "telemetry.googleapis.com:443"
)

type OtelServiceName string

const (
	OtelAccountsServiceName OtelServiceName = "reearth-accounts-api"
)

type Config struct {
	Enabled            bool
	Endpoint           string
	ExporterType       ExporterType
	Insecure           bool // skip TLS for otlp/jaeger; ignored for gcp (always TLS)
	MaxExportBatchSize int
	BatchTimeout       time.Duration
	MaxQueueSize       int

	SamplingRatio float64

	ServiceName OtelServiceName
}

type TracerProvider interface {
	Tracer(name string, options ...trace.TracerOption) trace.Tracer
	Shutdown(ctx context.Context) error
}

type noopTracerProvider struct {
	trace.TracerProvider
}

func (n *noopTracerProvider) Shutdown(ctx context.Context) error {
	return nil
}

func InitTracer(ctx context.Context, cfg *Config) (TracerProvider, error) {
	if !cfg.Enabled {
		applog.Info(ctx, "OpenTelemetry tracing is disabled")
		return &noopTracerProvider{TracerProvider: noop.NewTracerProvider()}, nil
	}

	if (cfg.ExporterType == ExporterTypeOTLP || cfg.ExporterType == ExporterTypeJaeger) && cfg.Endpoint == "" {
		return nil, fmt.Errorf("endpoint is required for exporter type %s", cfg.ExporterType)
	}
	var exporter sdktrace.SpanExporter
	var err error

	// resolvedEndpoint is the actual endpoint used by the exporter (may differ from cfg.Endpoint for GCP).
	resolvedEndpoint := cfg.Endpoint
	switch cfg.ExporterType {
	case ExporterTypeGCP:
		resolvedEndpoint = gcpCloudTraceEndpoint
		applog.Info(ctx, "Initializing GCP Cloud Trace exporter via OTLP", applog.Endpoint(resolvedEndpoint))
		exporter, err = createGCPExporter(ctx, cfg)
	case ExporterTypeJaeger:
		applog.Info(ctx, "Initializing Jaeger exporter via OTLP", applog.Endpoint(cfg.Endpoint), applog.Insecure(cfg.Insecure))
		exporter, err = createOTLPExporter(ctx, cfg, false)
	case ExporterTypeOTLP:
		applog.Info(ctx, "Initializing OTLP exporter", applog.Endpoint(cfg.Endpoint), applog.Insecure(cfg.Insecure))
		exporter, err = createOTLPExporter(ctx, cfg, false)
	default:
		applog.Warn(ctx, "Unknown exporter type, defaulting to OTLP", applog.Exporter(cfg.ExporterType), applog.Insecure(cfg.Insecure))
		exporter, err = createOTLPExporter(ctx, cfg, false)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	res, err := createResource(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	applog.Info(ctx, "resource attributes", applog.Resource(res.Attributes()))

	sampler := createSampler(ctx, cfg)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(
			exporter,
			sdktrace.WithMaxExportBatchSize(cfg.MaxExportBatchSize),
			sdktrace.WithBatchTimeout(cfg.BatchTimeout),
			sdktrace.WithMaxQueueSize(cfg.MaxQueueSize),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	otel.SetTracerProvider(tp)

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	applog.Info(ctx, "OpenTelemetry tracing initialized successfully", applog.Endpoint(resolvedEndpoint), applog.Exporter(cfg.ExporterType), applog.Service(string(cfg.ServiceName)), applog.MaxExportBatchSize(cfg.MaxExportBatchSize), applog.BatchTimeout(cfg.BatchTimeout), applog.MaxQueueSize(cfg.MaxQueueSize), applog.SamplingRatio(cfg.SamplingRatio))

	return tp, nil
}

func createResource(ctx context.Context, cfg *Config) (*resource.Resource, error) {
	baseAttributes := []resource.Option{}

	if cfg.ExporterType == ExporterTypeGCP {
		gcpDetector := gcp.NewDetector()
		if gcpDetector != nil {
			applog.Info(ctx, "GCP resource detector initialized")
			baseAttributes = append(baseAttributes, resource.WithDetectors(gcpDetector))
		}

		if metadata.OnGCE() {
			if id, err := metadata.ProjectIDWithContext(ctx); err == nil {
				baseAttributes = append(baseAttributes, resource.WithAttributes(
					attribute.String(gcpProjectIDAttribute, id),
				))
			}
		}
	}

	baseAttributes = append(baseAttributes, []resource.Option{
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceName(string(cfg.ServiceName)),
		),
	}...)

	return resource.New(ctx, baseAttributes...)
}

func createSampler(ctx context.Context, cfg *Config) sdktrace.Sampler {
	// Honor an upstream service's sampling decision when there is a remote
	// parent span; apply the ratio only to new root spans.
	ratio := cfg.SamplingRatio
	var root sdktrace.Sampler
	switch {
	case ratio < 0:
		applog.Warn(ctx, "OTEL sampling ratio is below 0; clamping to 0", applog.SamplingRatio(ratio))
		root = sdktrace.NeverSample()
	case ratio == 0:
		root = sdktrace.NeverSample()
	case ratio > 1:
		applog.Warn(ctx, "OTEL sampling ratio exceeds 1; clamping to 1", applog.SamplingRatio(ratio))
		root = sdktrace.AlwaysSample()
	case ratio == 1:
		root = sdktrace.AlwaysSample()
	default:
		root = sdktrace.TraceIDRatioBased(ratio)
	}
	return sdktrace.ParentBased(root)
}

func createOTLPExporter(ctx context.Context, cfg *Config, useGCPAuth bool) (sdktrace.SpanExporter, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
	}

	switch {
	case useGCPAuth:
		creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/trace.append")
		if err != nil {
			return nil, fmt.Errorf("failed to get GCP credentials: %w", err)
		}

		perRPCCreds := oauth.TokenSource{TokenSource: creds.TokenSource}

		opts = append(opts,
			otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, "")),
			otlptracegrpc.WithDialOption(grpc.WithPerRPCCredentials(perRPCCreds)),
		)
	case cfg.Insecure:
		opts = append(opts, otlptracegrpc.WithInsecure())
	default:
		// TLS using the system trust store.
		opts = append(opts, otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, "")))
	}

	return otlptracegrpc.New(ctx, opts...)
}

func createGCPExporter(ctx context.Context, cfg *Config) (sdktrace.SpanExporter, error) {
	gcpCfg := &Config{
		Enabled:       true,
		Endpoint:      gcpCloudTraceEndpoint,
		ExporterType:  ExporterTypeGCP,
		SamplingRatio: cfg.SamplingRatio,
	}

	return createOTLPExporter(ctx, gcpCfg, true)
}
