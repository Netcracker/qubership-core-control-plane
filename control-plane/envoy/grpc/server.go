package grpc

import (
	"context"
	"net"
	"os"
	"strings"
	"time"

	v3core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	v3clusterservice "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	v3discoveryservice "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	v3endpointservice "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	v3listenerservice "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	v3routeservice "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	v3runtimeservice "github.com/envoyproxy/go-control-plane/envoy/service/runtime/v3"
	v3secretservice "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
	v3cache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	v3server "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	envoy "github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/builder"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/listener"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/tlsmode"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-core-lib-go/v3/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

type ClusterHash struct{}

func (ClusterHash) ID(node *v3core.Node) string {
	if node == nil {
		return ""
	}
	return stripNamespaceSuffixFromServiceCluster(node.Cluster)
}

func stripNamespaceSuffixFromServiceCluster(serviceCluster string) string {
	ns := strings.TrimSpace(configloader.GetOrDefaultString("microservice.namespace", ""))
	if ns == "" {
		ns = strings.TrimSpace(os.Getenv("CLOUD_NAMESPACE"))
	}
	// Skip placeholder defaults from application.yaml (real ns comes from MICROSERVICE_NAMESPACE / CLOUD_NAMESPACE).
	if ns == "" || ns == "unknown" || ns == "default" || ns == "local" || serviceCluster == "" {
		return serviceCluster
	}
	suffix := "-" + ns
	if strings.HasSuffix(serviceCluster, suffix) && len(serviceCluster) > len(suffix) {
		return strings.TrimSuffix(serviceCluster, suffix)
	}
	return serviceCluster
}

var (
	ctx = context.Background()
)

type XDSServer struct {
	SnapshotCache v3cache.SnapshotCache
	logger        logging.Logger
	grpcServer    *grpc.Server
	dao           dao.Dao
	updateManager *envoy.UpdateManager
}

func NewGRPCServer(eventBus bus.BusSubscriber, dao dao.Dao, envoyConfigBuilder builder.EnvoyConfigBuilder) *XDSServer {
	this := &XDSServer{}
	this.dao = dao
	this.logger = logging.GetLogger("XDS-server")
	this.SnapshotCache = v3cache.NewSnapshotCache(false, ClusterHash{}, this.logger)
	server := v3server.NewServer(ctx, this.SnapshotCache, &DebugCallbacks{logger: this.logger})

	keepAliveEnforcementPolicy := keepalive.EnforcementPolicy{
		MinTime: 15 * time.Second, // If a client pings more than once every MinTime, terminate the connection
	}

	if tlsmode.GetMode() == tlsmode.Disabled {
		this.grpcServer = grpc.NewServer(grpc.KeepaliveEnforcementPolicy(keepAliveEnforcementPolicy))
	} else {
		this.grpcServer = grpc.NewServer(
			grpc.Creds(credentials.NewTLS(utils.GetTlsConfig())),
			grpc.KeepaliveEnforcementPolicy(keepAliveEnforcementPolicy))
	}

	v3discoveryservice.RegisterAggregatedDiscoveryServiceServer(this.grpcServer, server)
	v3endpointservice.RegisterEndpointDiscoveryServiceServer(this.grpcServer, server)
	v3clusterservice.RegisterClusterDiscoveryServiceServer(this.grpcServer, server)
	v3routeservice.RegisterRouteDiscoveryServiceServer(this.grpcServer, server)
	v3listenerservice.RegisterListenerDiscoveryServiceServer(this.grpcServer, server)
	v3secretservice.RegisterSecretDiscoveryServiceServer(this.grpcServer, server)
	v3runtimeservice.RegisterRuntimeDiscoveryServiceServer(this.grpcServer, server)

	this.updateManager = envoy.DefaultUpdateManager(dao, this.SnapshotCache, envoyConfigBuilder)
	changeEventListener := listener.NewChangeEventListener(this.updateManager)
	multipleChangeEventListener := listener.NewMultipleChangeEventListener(this.updateManager)
	reloadEventListener := listener.NewReloadEventListener(this.updateManager)
	eventBus.Subscribe(bus.TopicChanges, changeEventListener.HandleEvent)
	eventBus.Subscribe(bus.TopicMultipleChanges, multipleChangeEventListener.HandleEvent)
	eventBus.Subscribe(bus.TopicReload, reloadEventListener.HandleEvent)

	return this
}

func (xdsServer *XDSServer) GetUpdateManager() *envoy.UpdateManager {
	return xdsServer.updateManager
}

func (xdsServer *XDSServer) ListenAndServe() {
	bindAddr := configloader.GetKoanf().String("grpc.server.bind")
	lis, _ := net.Listen("tcp", ":"+bindAddr)
	xdsServer.logger.InfoC(ctx, "Start GRPC server on :%v", bindAddr)
	if err := xdsServer.grpcServer.Serve(lis); err != nil {
		// error handling
	}
}

func (xdsServer *XDSServer) GracefulStop() {

	xdsServer.grpcServer.GracefulStop()

}

type DebugCallbacks struct {
	logger logging.Logger
}

var callbackCtx = context.Background()

func (c *DebugCallbacks) OnDeltaStreamOpen(ctx2 context.Context, i int64, s string) error {
	return nil
}

func (c *DebugCallbacks) OnDeltaStreamClosed(i int64, node *v3core.Node) {
	// nothing to do
}

func (c *DebugCallbacks) OnStreamDeltaRequest(i int64, request *v3discoveryservice.DeltaDiscoveryRequest) error {
	return nil
}

func (c *DebugCallbacks) OnStreamDeltaResponse(i int64, request *v3discoveryservice.DeltaDiscoveryRequest, response *v3discoveryservice.DeltaDiscoveryResponse) {
	// nothing to do
}

func (c *DebugCallbacks) OnStreamOpen(context.Context, int64, string) error {
	return nil
}

func (c *DebugCallbacks) OnStreamClosed(int64, *v3core.Node) {
	// nothing to do
}

func (c *DebugCallbacks) OnStreamRequest(version int64, request *v3discoveryservice.DiscoveryRequest) error {
	return nil
}

func (c *DebugCallbacks) OnStreamResponse(context context.Context, version int64, request *v3discoveryservice.DiscoveryRequest, response *v3discoveryservice.DiscoveryResponse) {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Errorf("Handle Envoy error caused err: %v", r)
		}
	}()
	if request != nil && request.ErrorDetail != nil {
		var message string
		if len(request.ErrorDetail.Message) > 500 {
			message = request.ErrorDetail.Message[:500]
		} else {
			message = request.ErrorDetail.Message
		}
		c.logger.ErrorC(callbackCtx, "EnvoyProxy's sent error: %s", message)
		if response != nil {
			response.Resources = make([]*any.Any, 0)
			response.VersionInfo = "-1"
		}
	}
}

func (c *DebugCallbacks) OnFetchRequest(context.Context, *v3discoveryservice.DiscoveryRequest) error {
	return nil
}

func (c *DebugCallbacks) OnFetchResponse(*v3discoveryservice.DiscoveryRequest, *v3discoveryservice.DiscoveryResponse) {
	// nothing to do
}
