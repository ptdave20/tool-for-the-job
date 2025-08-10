package main

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"time"
)

func JaegerTracerProvider() (*sdktrace.TracerProvider, error) {
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint("http://localhost:14268/api/traces")))
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
