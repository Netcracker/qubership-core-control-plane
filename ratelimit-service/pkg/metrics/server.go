package metrics

import (
    "net/http"

    "github.com/prometheus/client_golang/prometheus/promhttp"
    "k8s.io/klog/v2"
)

type MetricsServer struct {
    server   *http.Server
    collector MetricsCollector
    port     string
}

func NewMetricsServer(collector MetricsCollector, port string) *MetricsServer {
    return &MetricsServer{
        collector: collector,
        port:      port,
    }
}

func (s *MetricsServer) Start() error {
    mux := http.NewServeMux()
    mux.Handle("/metrics", promhttp.HandlerFor(s.collector.GetRegistry(), promhttp.HandlerOpts{}))
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("OK"))
    })

    s.server = &http.Server{
        Addr:    ":" + s.port,
        Handler: mux,
    }

    go func() {
        klog.Infof("Metrics server listening on port %s", s.port)
        if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            klog.Errorf("Metrics server error: %v", err)
        }
    }()

    return nil
}

func (s *MetricsServer) Stop() error {
    if s.server != nil {
        return s.server.Close()
    }
    return nil
}