package main

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"log"
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

	r.POST("/", PostTodo)
	r.GET("/", GetTodo)
	r.POST("/:id", PatchTodo)
	r.Run()
}
