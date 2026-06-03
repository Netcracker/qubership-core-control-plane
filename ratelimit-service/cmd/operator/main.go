package main

import (
    "context"
    "flag"
    "os"
    "os/signal"
    "syscall"
    "time"

    "ratelimit-service/pkg/api"
    "ratelimit-service/pkg/controller"
    "ratelimit-service/pkg/metrics"
    "ratelimit-service/pkg/ratelimit"
    "ratelimit-service/pkg/utils"

    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
    "k8s.io/client-go/tools/clientcmd"
    "k8s.io/klog/v2"
)

func main() {
    klog.InitFlags(nil)
    flag.Parse()

    redisAddr := utils.GetEnv("REDIS_ADDR", "localhost:6379")
    apiPort := utils.GetEnv("API_PORT", "8082")
    grpcPort := utils.GetEnv("GRPC_PORT", "8081")
    metricsPort := utils.GetEnv("METRICS_PORT", "9090")
    namespace := utils.GetEnv("NAMESPACE", "core-1-core")

    // Create Redis client
    redisClient, err := ratelimit.NewRedisClient(redisAddr, "", 0)
    if err != nil {
        klog.Fatalf("Failed to create Redis client: %v", err)
    }
    defer redisClient.Close()

    // Create rate limit manager
    rateLimitManager := ratelimit.NewRateLimitManager(redisClient)
    redisClient.SetManager(rateLimitManager)

    // Create metrics collector
    metricsCollector := metrics.NewDefaultMetricsCollector()
    metrics.SetGlobalMetrics(metricsCollector)

    // Start metrics HTTP server for Prometheus scraping
    metricsServer := metrics.NewMetricsServer(metricsCollector, metricsPort)
    if err := metricsServer.Start(); err != nil {
        klog.Fatalf("Failed to start metrics server: %v", err)
    }
    defer metricsServer.Stop()

    // Create metrics collector service (periodically collects Redis stats)
    metricsService := metrics.NewMetricsCollectorService(redisClient, metricsCollector, 30*time.Second)

    // Create Kubernetes client
    var clientset *kubernetes.Clientset
    var config *rest.Config
    
    // Try in-cluster config first
    config, err = rest.InClusterConfig()
    if err != nil {
        klog.Warningf("Failed to get in-cluster config: %v, falling back to kubeconfig", err)
        kubeconfig := utils.GetEnv("KUBECONFIG", utils.GetEnv("HOME", "")+"/.kube/config")
        config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
        if err != nil {
            klog.Fatalf("Failed to build kubeconfig: %v", err)
        }
    }
    
    clientset, err = kubernetes.NewForConfig(config)
    if err != nil {
        klog.Fatalf("Failed to create k8s client: %v", err)
    }
    
    klog.Info("Kubernetes client created successfully")
    klog.Infof("Using namespace: %s", namespace)

    // Create controller
    configMapController := controller.NewConfigMapController(clientset, redisClient, rateLimitManager)

    // Create API server
    apiServer := api.NewServer(redisClient, configMapController, rateLimitManager)

    // Start gRPC server for Envoy integration
    grpcServer, err := ratelimit.StartGRPCServer(grpcPort, rateLimitManager)
    if err != nil {
        klog.Fatalf("Failed to start gRPC server: %v", err)
    }
    defer grpcServer.GracefulStop()

    // Start all services
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Start metrics collection service (periodic Redis stats)
    go metricsService.Start(ctx)

    // Start ConfigMap controller
    go configMapController.Run(ctx)

    // Start API server
    go func() {
        if err := apiServer.Run(":" + apiPort); err != nil {
            klog.Errorf("API server error: %v", err)
        }
    }()

    klog.Infof("========================================")
    klog.Infof("All services started successfully:")
    klog.Infof("  - HTTP API: http://localhost:%s", apiPort)
    klog.Infof("  - gRPC API: localhost:%s (for Envoy)", grpcPort)
    klog.Infof("  - Metrics API: http://localhost:%s/metrics", metricsPort)
    klog.Infof("  - Redis: %s", redisAddr)
    klog.Infof("  - Metrics collector: running (interval: 30s)")
    klog.Infof("  - ConfigMap controller: watching namespace '%s'", namespace)
    klog.Infof("========================================")

    // Wait for shutdown signal
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh

    klog.Info("Shutting down...")
    cancel()
    time.Sleep(2 * time.Second)
    klog.Info("Shutdown complete")
}