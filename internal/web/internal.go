package internal

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

//go:embed template/*
var templateContent embed.FS

type WebServer struct {
	TemplateData *TemplateData
}

func NewServer(templateData *TemplateData) *WebServer {
	return &WebServer{
		TemplateData: templateData,
	}

}

func (we *WebServer) CreateServer() *chi.Mux {
	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Logger)
	router.Use(middleware.Timeout(60 * time.Second))
	//promhttp
	router.Handle("/metrics", promhttp.Handler())
	router.Get("/", we.HandleRequest)
	return router
}

type TemplateData struct {
	Title              string
	BackgroundColor    string
	ResponseTime       time.Duration
	ExternalCallMethod string
	ExternalCallURL    string
	Contet             string
	RequestNameOTEL    string
	OTELTracer         trace.Tracer
}

func (h *WebServer) HandleRequest(w http.ResponseWriter, r *http.Request) {
	carrier := propagation.HeaderCarrier(r.Header)

	ctx := r.Context()
	ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)

	// cria span

	ctx, spanInicial := h.TemplateData.OTELTracer.Start(ctx, "SPAN_INICIAL "+h.TemplateData.RequestNameOTEL)
	time.Sleep(time.Second)
	spanInicial.End()

	ctx, span := h.TemplateData.OTELTracer.Start(ctx, "CHAMADA_EXTERNA "+h.TemplateData.RequestNameOTEL)
	defer span.End()

	time.Sleep(time.Millisecond * h.TemplateData.ResponseTime)
	if h.TemplateData.ExternalCallURL != "" {
		var req *http.Request
		var err error
		if h.TemplateData.ExternalCallMethod == "GET" {
			req, err = http.NewRequestWithContext(ctx, "GET", h.TemplateData.ExternalCallURL, nil)
		} else if h.TemplateData.ExternalCallMethod == "POST" {
			req, err = http.NewRequestWithContext(ctx, "POST", h.TemplateData.ExternalCallURL, nil)
		} else {
			http.Error(w, "Invalid externalCallMethod", http.StatusInternalServerError)
			return
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return

		}

		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return

		}

		defer resp.Body.Close()
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		h.TemplateData.Contet = string(bodyBytes)
	}

	tpl := template.Must(template.New("index.html").ParseFS(templateContent, "template/index.html"))
	err := tpl.Execute(w, h.TemplateData)

	if err != nil {
		http.Error(w, fmt.Sprintf("Error executing template: %v", err), http.StatusInternalServerError)
		return
	}

}
