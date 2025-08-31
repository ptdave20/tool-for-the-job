package main

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"log"
)

func main() {
	tp, tpErr := OTLPTracerProvider() // Changed from JaegerTracerProvider
	if tpErr != nil {
		log.Fatal(tpErr)
	}
	mp, mpErr := OTLPMetricsProvider()
	if mpErr != nil {
		log.Fatal(mpErr)
	}

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	r := gin.Default()
	r.Use(otelgin.Middleware("golang-gin-gonic", otelgin.WithTracerProvider(tp), otelgin.WithMeterProvider(mp)))
	r.Use(PostgresMiddleware())
	r.Use(MetricsMiddleware())

	r.POST("/", PostTodo)
	r.GET("/", GetTodo)
	r.POST("/:id", PatchTodo)
	r.Run()
}
