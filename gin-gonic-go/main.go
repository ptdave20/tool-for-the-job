package main

import (
	"context"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize PostgreSQL connection pool
	err := InitPgxPool()
	if err != nil {
		log.Fatalf("Failed to initialize database pool: %v", err)
	}
	defer ClosePgxPool()

	// Initialize tracing
	tp, err := OTLPTracerProvider()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	otel.SetTracerProvider(tp)

	// Initialize metrics
	mp, err := OTLPMetricsProvider()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := mp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down meter provider: %v", err)
		}
	}()

	// Setup runtime metrics collection
	if err := SetupRuntimeMetrics(); err != nil {
		log.Printf("Error setting up runtime metrics: %v", err)
	}

	r := gin.Default()

	// Add tracing middleware
	r.Use(otelgin.Middleware("gin-gonic"))

	// Add custom middlewares
	r.Use(PostgresMiddleware())
	r.Use(MetricsMiddleware())

	// Routes
	r.POST("/", PostTodo)
	r.GET("/", GetTodo)
	r.POST("/:id", PatchTodo)

	r.Run()
}
