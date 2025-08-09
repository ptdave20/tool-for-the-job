package main

import (
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	tp, tpErr := JaegerTracerProvider()
	if tpErr != nil {
		log.Fatal(tpErr)
	}
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	r := gin.Default()
	r.Use(otelgin.Middleware("golang-gin-gonic"))
	r.Use(PostgresMiddleware())

	tracer := otel.Tracer("ping-handler")
	r.GET("/ping", func(c *gin.Context) {

		_, span := tracer.Start(c.Request.Context(), "Ping Handler")
		defer span.End()
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	r.Run()
}
