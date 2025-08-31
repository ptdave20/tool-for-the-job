package main

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp" // Changed from jaeger
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"time"
)

// Updated to use OTLP trace exporter instead of deprecated Jaeger exporter
func OTLPTracerProvider() (*sdktrace.TracerProvider, error) {
	// Create OTLP HTTP trace exporter
	exp, err := otlptracehttp.New(
		context.Background(),
		// The endpoint is configured via OTEL_EXPORTER_OTLP_TRACES_ENDPOINT environment variable
		// or defaults to http://localhost:4318/v1/traces
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("golang"),
			semconv.DeploymentEnvironmentKey.String("development"),
		)),
	)
	return tp, nil
}

func OTLPMetricsProvider() (*sdkmetric.MeterProvider, error) {
	// Create OTLP HTTP exporter for metrics
	exporter, err := otlpmetrichttp.New(context.Background())
	if err != nil {
		return nil, err
	}

	// Create a periodic reader that will push metrics every 30 seconds
	reader := sdkmetric.NewPeriodicReader(
		exporter,
		sdkmetric.WithInterval(30*time.Second),
	)

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("golang"),
			semconv.DeploymentEnvironmentKey.String("development"),
		)),
	)

	otel.SetMeterProvider(provider)
	return provider, nil
}

func PostgresMiddleware() gin.HandlerFunc {
	tracer := otel.Tracer("postgres-middleware")
	var con *pgx.Conn = nil
	var err error

	test := func(ctx context.Context) error {
		_, span := tracer.Start(ctx, "Ping Postgres")
		defer span.End()
		err = con.Ping(ctx)
		if err != nil {
			span.RecordError(err)
			return err
		}
		return nil
	}

	connect := func(ctx context.Context) error {
		_, span := tracer.Start(ctx, "Verify Postgres Connection")
		defer span.End()
		if con == nil {
			con, err = pgx.Connect(context.Background(), "postgres://local:local@localhost")
			if err != nil {
				span.RecordError(err)
				con = nil
				return err
			}
		}
		return nil
	}

	return func(c *gin.Context) {
		ctx, span := tracer.Start(c.Request.Context(), "PostgresMiddleware")
		for i := 0; i < 3; i++ {
			time.Sleep(time.Millisecond * 500 * time.Duration(i))

			err = connect(ctx)
			if err != nil {
				continue
			}
			err = test(ctx)
			if err != nil {
				continue
			}
			break
		}

		// If we still have an error after 3 tries, abort
		if err != nil {
			c.AbortWithError(500, err)
			span.End()
			return
		}
		span.End()

		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "pgxConn", con))
		c.Next()
	}
}

// Add metrics middleware for Gin
func MetricsMiddleware() gin.HandlerFunc {
	meter := otel.Meter("gin-gonic-metrics")

	// Create metrics instruments
	requestCounter, _ := meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
	)

	requestDuration, _ := meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("HTTP request duration in seconds"),
	)

	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start).Seconds()

		// Create attributes
		attrs := []attribute.KeyValue{
			attribute.String("method", c.Request.Method),
			attribute.String("route", c.FullPath()),
			attribute.Int("status_code", c.Writer.Status()),
		}

		// Record metrics
		requestCounter.Add(c.Request.Context(), 1, metric.WithAttributes(attrs...))
		requestDuration.Record(c.Request.Context(), duration, metric.WithAttributes(attrs...))
	}
}

func GetPostgresConn(c *gin.Context) (*pgx.Conn, error) {
	pgxInt := c.Request.Context().Value("pgxConn")
	if pgxInt == nil {
		return nil, errors.New("No Postgres Connection")
	}
	// Validate that it is what we think it should be
	pgxConn, ok := pgxInt.(*pgx.Conn)
	if !ok {
		return nil, errors.New("Postgres Connection is not a *pgx.Conn")
	}
	return pgxConn, nil
}
