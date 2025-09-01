package main

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"net/http"
	"runtime"
	"time"
)

// Global connection pool variable
var pgPool *pgxpool.Pool

// Initialize connection pool
func InitPgxPool() error {
	// Configure connection pool
	config, err := pgxpool.ParseConfig("postgres://local:local@localhost/postgres?pool_max_conns=20&pool_min_conns=5&pool_max_conn_lifetime=1h&pool_max_conn_idle_time=30m")
	if err != nil {
		return err
	}

	// Create the pool
	pgPool, err = pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return err
	}

	// Test the connection
	err = pgPool.Ping(context.Background())
	if err != nil {
		return err
	}

	return nil
}

// Close connection pool
func ClosePgxPool() {
	if pgPool != nil {
		pgPool.Close()
	}
}

func OTLPTracerProvider() (*sdktrace.TracerProvider, error) {
	// Create stdout exporter to be able to retrieve
	// the collected spans.
	exporter, err := otlptrace.New(context.Background(), otlptracehttp.NewClient())
	if err != nil {
		return nil, err
	}

	// For the demonstration, use sdktrace.AlwaysSample sampler to sample all traces.
	// In a production application, use sdktrace.ProbabilitySampler with a desired probability.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return tp, err
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

// Runtime metrics collection
func SetupRuntimeMetrics() error {
	meter := otel.Meter("golang-runtime")

	// Memory metrics
	memoryUsage, err := meter.Int64ObservableGauge(
		"runtime_memory_usage_bytes",
		metric.WithDescription("Current memory usage in bytes"),
	)
	if err != nil {
		return err
	}

	memoryAllocated, err := meter.Int64ObservableGauge(
		"runtime_memory_allocated_bytes",
		metric.WithDescription("Total allocated memory in bytes"),
	)
	if err != nil {
		return err
	}

	memorySystem, err := meter.Int64ObservableGauge(
		"runtime_memory_system_bytes",
		metric.WithDescription("System memory obtained from OS"),
	)
	if err != nil {
		return err
	}

	// GC metrics
	gcCount, err := meter.Int64ObservableGauge(
		"runtime_gc_count_total",
		metric.WithDescription("Total number of GC cycles"),
	)
	if err != nil {
		return err
	}

	gcPauseTime, err := meter.Float64ObservableGauge(
		"runtime_gc_pause_seconds",
		metric.WithDescription("GC pause time in seconds"),
	)
	if err != nil {
		return err
	}

	// Goroutine metrics
	goroutineCount, err := meter.Int64ObservableGauge(
		"runtime_goroutines_count",
		metric.WithDescription("Current number of goroutines"),
	)
	if err != nil {
		return err
	}

	// CPU metrics (approximate)
	cpuGoroutines, err := meter.Int64ObservableGauge(
		"runtime_cpu_goroutines",
		metric.WithDescription("Number of logical CPUs available"),
	)
	if err != nil {
		return err
	}

	// Connection pool metrics
	poolTotalConns, err := meter.Int64ObservableGauge(
		"postgres_pool_total_conns",
		metric.WithDescription("Total number of connections in pool"),
	)
	if err != nil {
		return err
	}

	poolIdleConns, err := meter.Int64ObservableGauge(
		"postgres_pool_idle_conns",
		metric.WithDescription("Number of idle connections in pool"),
	)
	if err != nil {
		return err
	}

	poolAcquiredConns, err := meter.Int64ObservableGauge(
		"postgres_pool_acquired_conns",
		metric.WithDescription("Number of acquired connections in pool"),
	)
	if err != nil {
		return err
	}

	// Register callback to collect runtime stats
	_, err = meter.RegisterCallback(
		func(ctx context.Context, o metric.Observer) error {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			// Memory metrics
			o.ObserveInt64(memoryUsage, int64(m.Alloc))
			o.ObserveInt64(memoryAllocated, int64(m.TotalAlloc))
			o.ObserveInt64(memorySystem, int64(m.Sys))

			// GC metrics
			o.ObserveInt64(gcCount, int64(m.NumGC))
			o.ObserveFloat64(gcPauseTime, float64(m.PauseNs[(m.NumGC+255)%256])/1e9)

			// Goroutine count
			o.ObserveInt64(goroutineCount, int64(runtime.NumGoroutine()))

			// CPU info
			o.ObserveInt64(cpuGoroutines, int64(runtime.NumCPU()))

			// Pool metrics
			if pgPool != nil {
				stat := pgPool.Stat()
				o.ObserveInt64(poolTotalConns, int64(stat.TotalConns()))
				o.ObserveInt64(poolIdleConns, int64(stat.IdleConns()))
				o.ObserveInt64(poolAcquiredConns, int64(stat.AcquiredConns()))
			}

			return nil
		},
		memoryUsage, memoryAllocated, memorySystem, gcCount, gcPauseTime, goroutineCount, cpuGoroutines,
		poolTotalConns, poolIdleConns, poolAcquiredConns,
	)

	return err
}

// Simplified PostgreSQL middleware that just ensures pool is available
func PostgresMiddleware() gin.HandlerFunc {
	tracer := otel.Tracer("postgres-middleware")

	return func(c *gin.Context) {
		ctx, span := tracer.Start(c.Request.Context(), "PostgresMiddleware")
		defer span.End()

		// Check if pool is available
		if pgPool == nil {
			c.AbortWithError(http.StatusInternalServerError, errors.New("Database pool not initialized"))
			return
		}

		// Test pool health (optional - you might skip this in production for performance)
		err := pgPool.Ping(ctx)
		if err != nil {
			span.RecordError(err)
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		// Pool is healthy, continue
		c.Request = c.Request.WithContext(context.WithValue(ctx, "pgPool", pgPool))
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

func GetPostgresConn(c *gin.Context) (*pgxpool.Pool, error) {
	pgp := c.Request.Context().Value("pgPool")
	if pgp == nil {
		return nil, errors.New("Database pool not initialized")
	}
	pgPool := pgp.(*pgxpool.Pool)
	if pgPool == nil {
		return nil, errors.New("Database pool is nil")
	}
	return pgPool, nil
}
