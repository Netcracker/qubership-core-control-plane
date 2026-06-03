package setup

import (
    "context"
    "ratelimit-service/pkg/api"
    "time"

    "ratelimit-service/pkg/controller"
    "ratelimit-service/pkg/metrics"
    "ratelimit-service/pkg/ratelimit"

    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/clientcmd"
    "k8s.io/klog/v2"
)

type LocalOperator struct {
    controller       *controller.ConfigMapController
    apiServer        *api.Server
    redisClient      *ratelimit.RedisClient
    rateLimitManager *ratelimit.RateLimitManager
    metricsService   *metrics.MetricsCollectorService
    metricsServer    *metrics.MetricsServer 
    metricsCollector metrics.MetricsCollector
    cancel           context.CancelFunc
}

func NewLocalOperator(kubeconfigPath string) (*LocalOperator, error) {
    config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
    if err != nil {
        return nil, err
    }

    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return nil, err
    }

    redisAddr := "localhost:6379"
    
    // Create Redis client
    redisClient, err := ratelimit.NewRedisClient(redisAddr, "", 0)
    if err != nil {
        return nil, err
    }

    // Create rate limit manager
    rateLimitManager := ratelimit.NewRateLimitManager(redisClient)
    redisClient.SetManager(rateLimitManager)

    // Add default rule for testing
    if err := rateLimitManager.AddRule(&ratelimit.Rule{
        Name:      "api_strict",
        Pattern:   ".*/test.*",
        Limit:     2,
        Window:    10 * time.Second,
        Algorithm: ratelimit.FixedWindow,
        Priority:  50,
    }); err != nil {
        klog.Warningf("Failed to add default rule: %v", err)
    }

    // Create metrics collector
    metricsCollector := metrics.NewDefaultMetricsCollector()
    metrics.SetGlobalMetrics(metricsCollector)

    // Create metrics service
    metricsService := metrics.NewMetricsCollectorService(redisClient, metricsCollector, 5*time.Second)

    // Create metrics HTTP server
    metricsServer := metrics.NewMetricsServer(metricsCollector, "9090")

    // Create controller
    controller := controller.NewConfigMapController(clientset, redisClient, rateLimitManager)
    
    // Create API server
    apiServer := api.NewServer(redisClient, controller, rateLimitManager)

    return &LocalOperator{
        controller:       controller,
        apiServer:        apiServer,
        redisClient:      redisClient,
        rateLimitManager: rateLimitManager,
        metricsService:   metricsService,
        metricsServer:    metricsServer,
        metricsCollector: metricsCollector,
    }, nil
}

func (op *LocalOperator) Start(ctx context.Context, port string) error {
    ctx, cancel := context.WithCancel(ctx)
    op.cancel = cancel

    // Start metrics collection service
    go op.metricsService.Start(ctx)

    // Start metrics HTTP server
    if err := op.metricsServer.Start(); err != nil {
        klog.Warningf("Failed to start metrics server: %v", err)
    }

    // Start controller
    go op.controller.Run(ctx)

    // Start API server
    go func() {
        if err := op.apiServer.Run(":" + port); err != nil {
            klog.Errorf("API server error: %v", err)
        }
    }()

    // Wait for services to be ready
    time.Sleep(2 * time.Second)

    klog.Info("Local operator started successfully")
    return nil
}

func (op *LocalOperator) Stop() {
    if op.cancel != nil {
        op.cancel()
    }
    if op.metricsService != nil {
        op.metricsService.Stop()
    }
    if op.metricsServer != nil {
        op.metricsServer.Stop()
    }
    if op.controller != nil {
        op.controller.Stop()
    }
    if op.apiServer != nil {
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        if err := op.apiServer.Shutdown(shutdownCtx); err != nil {
            klog.Warningf("API server shutdown error: %v", err)
        }
    }
    if op.redisClient != nil {
        op.redisClient.Close()
    }
    klog.Info("Local operator stopped")
}

func (op *LocalOperator) GetURL() string {
    return "http://localhost:8083"
}