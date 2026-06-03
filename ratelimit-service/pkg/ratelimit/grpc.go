package ratelimit

import (
    "context"
    "net"
    "strings"

    pb "github.com/envoyproxy/go-control-plane/envoy/service/ratelimit/v3"
    "google.golang.org/grpc"
    "k8s.io/klog/v2"
)

type GRPCServer struct {
    pb.UnimplementedRateLimitServiceServer
    manager *RateLimitManager
}

func NewGRPCServer(manager *RateLimitManager) *GRPCServer {
    return &GRPCServer{
        manager: manager,
    }
}

func (s *GRPCServer) ShouldRateLimit(ctx context.Context, req *pb.RateLimitRequest) (*pb.RateLimitResponse, error) {
    // Build key from request
    key := buildKeyFromRequest(req)
    
    klog.V(4).Infof("gRPC rate limit request: domain=%s, key=%s", req.Domain, key)
    klog.V(5).Infof("gRPC request details: descriptors=%+v", req.Descriptors)
    
    // Check rate limit using manager
    allowed, current, err := s.manager.Check(ctx, key)
    if err != nil {
        klog.Errorf("gRPC rate limit check failed: %v", err)
        return &pb.RateLimitResponse{
            OverallCode: pb.RateLimitResponse_UNKNOWN,
        }, nil
    }
    
    // Get matching rule for logging
    rule, _ := s.manager.GetRule(key)
    if rule != nil {
        klog.V(4).Infof("Matched rule: name=%s, limit=%d, window=%v, algorithm=%s", 
            rule.Name, rule.Limit, rule.Window, rule.Algorithm)
        klog.V(4).Infof("Rate limit result: allowed=%v, current=%d, limit=%d", 
            allowed, current, rule.Limit)
    } else {
        klog.V(4).Infof("No matching rule found for key=%s, using default (allow all)", key)
    }
    
    code := pb.RateLimitResponse_OK
    if !allowed {
        code = pb.RateLimitResponse_OVER_LIMIT
        klog.V(4).Infof("Rate limit exceeded for key=%s (current=%d)", key, current)
    }
    
    return &pb.RateLimitResponse{
        OverallCode: code,
        Statuses:    []*pb.RateLimitResponse_DescriptorStatus{},
    }, nil
}

func buildKeyFromRequest(req *pb.RateLimitRequest) string {
    parts := []string{"domain=" + req.Domain}
    seen := make(map[string]bool)
    
    for _, desc := range req.Descriptors {
        for _, entry := range desc.Entries {
            // Avoid duplicate keys
            if !seen[entry.Key] {
                seen[entry.Key] = true
                parts = append(parts, entry.Key+"="+entry.Value)
            }
        }
    }
    return strings.Join(parts, "|")
}

// StartGRPCServer starts gRPC server on specified port
func StartGRPCServer(port string, manager *RateLimitManager) (*grpc.Server, error) {
    lis, err := net.Listen("tcp", ":"+port)
    if err != nil {
        return nil, err
    }
    
    grpcServer := grpc.NewServer()
    pb.RegisterRateLimitServiceServer(grpcServer, NewGRPCServer(manager))
    
    go func() {
        if err := grpcServer.Serve(lis); err != nil {
            klog.Errorf("gRPC server error: %v", err)
        }
    }()
    
    klog.Infof("gRPC rate limit server listening on port %s", port)
    return grpcServer, nil
}