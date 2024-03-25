package main

// sdktrace "go.opentelemetry.io/otel/sdk/trace"
// "go.opentelemetry.io/otel"
import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	web "github.com/lucasferreirajs/lab-observabilidade/internal"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"

	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func initProvider(serviceName, collectorUrl string) (func(context.Context) error, error) {

	ctx := context.Background()
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, collectorUrl,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to collector: %w", err)
	}

	// config exporter
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))

	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	//batch span processor
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // amostracer, controle da quantidade de trace enviados
		sdktrace.WithResource(res),                    // microservices
		sdktrace.WithSpanProcessor(bsp),               // batch processar informação
	)

	otel.SetTracerProvider(traceProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return traceExporter.Shutdown, nil
}

// load env
func init() {

	viper.AutomaticEnv()

}

func main() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	shutdown, err := initProvider(viper.GetString("OTEL_SERVICE_NAME"), viper.GetString("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		err = shutdown(ctx)
		if err != nil {
			log.Fatal("failed to shutdown tracerprovider: %w", err)
		}
	}()

	tracer := otel.Tracer("microservices-tracer")
	templateData := &web.TemplateData{
		Title:              viper.GetString("TITLE"),
		BackgroundColor:    viper.GetString("BACKGROUND_COLOR"),
		ResponseTime:       time.Duration(viper.GetInt("RESPONSE_TIME")),
		ExternalCallURL:    viper.GetString("EXTERNAL_CALL_URL"),
		ExternalCallMethod: viper.GetString("EXTERNAL_CALL_METHOD"),
		RequestNameOTEL:    viper.GetString("REQUEST_NAME_OTEL"),
		OTELTracer:         tracer,
	}

	server := web.NewServer(templateData)
	router := server.CreateServer()

	go func() {
		log.Println("Starting server on port: ", viper.GetString("HTTP_PORT"))
		err := http.ListenAndServe(viper.GetString("HTTP_PORT"), router)
		if err != nil {
			log.Fatal(err)
		}

	}()

	select {
	case <-sigCh:
		log.Println("Shutting down gracefully,CTRL+C pressed...")
	case <-ctx.Done():

		log.Println("Shutting down due to other reason...")
	}

	// create time out

	_, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
}
