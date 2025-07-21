package builder

import (
	"fmt"
	v3cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	v3listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	v3route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	v3runtime "github.com/envoyproxy/go-control-plane/envoy/service/runtime/v3"
	"github.com/go-errors/errors"
	pstruct "github.com/golang/protobuf/ptypes/struct"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/builder/cluster"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/builder/common"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/builder/listener"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/builder/routeconfig"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"google.golang.org/protobuf/types/known/structpb"
	"sync"
)

var (
	logger logging.Logger
)

const (
	EgressGateway = "egress-gateway"
)

func init() {
	logger = logging.GetLogger("EnvoyConfigBuilderImpl")
}

//go:generate mockgen -source=builder.go -destination=../../../test/mock/envoy/cache/builder/stub_builder.go -package=mock_builder
type EnvoyConfigBuilder interface {
	BuildListener(listener *domain.Listener, namespaceMapping string, withTls bool) (*v3listener.Listener, error)
	BuildCluster(nodeGroup string, clusterEntity *domain.Cluster) (*v3cluster.Cluster, error)
	BuildRouteConfig(routeConfig *domain.RouteConfiguration) (*v3route.RouteConfiguration, error)
	BuildRuntime(_ string, runtimeName string) (*v3runtime.Runtime, error)
	RegisterGateway(gateway *domain.NodeGroup) error
}

type EnvoyConfigBuilderImpl struct {
	mutex *sync.RWMutex

	dao   dao.Dao
	props *common.EnvoyProxyProperties

	clusterBuilder            cluster.ClusterBuilder
	facadeListenerBuilder     listener.ListenerBuilder
	gatewayListenerBuilder    listener.ListenerBuilder
	meshVirtualHostBuilder    routeconfig.VirtualHostBuilder
	ingressVirtualHostBuilder routeconfig.VirtualHostBuilder
	egressVirtualHostBuilder  routeconfig.VirtualHostBuilder

	listenerBuilders    map[string]listener.ListenerBuilder
	virtualHostBuilders map[string]routeconfig.VirtualHostBuilder
	clusterBuilders     map[domain.GatewayType]cluster.ClusterBuilder
}

func NewEnvoyConfigBuilder(dao dao.Dao, props *common.EnvoyProxyProperties, facadeListenerBuilder, gatewayListenerBuilder listener.ListenerBuilder, meshVirtualHostBuilder, gatewayVirtualHostBuilder, ingressVirtualHostBuilder, egressVirtualHostBuilder routeconfig.VirtualHostBuilder) *EnvoyConfigBuilderImpl {
	builder := EnvoyConfigBuilderImpl{
		mutex:                     &sync.RWMutex{},
		dao:                       dao,
		props:                     props,
		clusterBuilder:            cluster.NewDefaultClusterBuilder(dao, props.Routes),
		facadeListenerBuilder:     facadeListenerBuilder,
		gatewayListenerBuilder:    gatewayListenerBuilder,
		meshVirtualHostBuilder:    meshVirtualHostBuilder,
		ingressVirtualHostBuilder: ingressVirtualHostBuilder,
		egressVirtualHostBuilder:  egressVirtualHostBuilder,
		listenerBuilders:          make(map[string]listener.ListenerBuilder),
		virtualHostBuilders:       make(map[string]routeconfig.VirtualHostBuilder),
		clusterBuilders:           make(map[domain.GatewayType]cluster.ClusterBuilder)}

	builder.listenerBuilders[domain.PublicGateway] = gatewayListenerBuilder
	builder.listenerBuilders[domain.PrivateGateway] = gatewayListenerBuilder
	builder.listenerBuilders[domain.InternalGateway] = gatewayListenerBuilder

	builder.virtualHostBuilders[domain.PublicGateway] = gatewayVirtualHostBuilder
	builder.virtualHostBuilders[domain.PrivateGateway] = gatewayVirtualHostBuilder
	builder.virtualHostBuilders[domain.InternalGateway] = gatewayVirtualHostBuilder

	builder.clusterBuilders[domain.Egress] = cluster.NewEgressClusterBuilder(dao, props.Routes)
	return &builder
}

func (ecb *EnvoyConfigBuilderImpl) RegisterGateway(gateway *domain.NodeGroup) error {
	if gateway.GatewayType == "" || common.IsReservedGatewayName(gateway.Name) {
		return nil
	}
	ecb.mutex.Lock()
	defer ecb.mutex.Unlock()

	switch gateway.GatewayType {
	case domain.Ingress:
		ecb.listenerBuilders[gateway.Name] = ecb.gatewayListenerBuilder
		ecb.virtualHostBuilders[gateway.Name] = ecb.ingressVirtualHostBuilder
		return nil
	case domain.Egress:
		ecb.listenerBuilders[gateway.Name] = ecb.facadeListenerBuilder
		ecb.virtualHostBuilders[gateway.Name] = ecb.egressVirtualHostBuilder
		return nil
	case domain.Mesh:
		ecb.listenerBuilders[gateway.Name] = ecb.facadeListenerBuilder
		ecb.virtualHostBuilders[gateway.Name] = ecb.meshVirtualHostBuilder
		return nil
	default:
		msg := fmt.Sprintf("builder: unknown gateway type %s passed to register in EnvoyConfigBuilder", gateway.GatewayType)
		logger.Errorf(msg)
		return errors.New(msg)
	}
}

func (ecb *EnvoyConfigBuilderImpl) BuildListener(listener *domain.Listener, namespaceMapping string, withTls bool) (*v3listener.Listener, error) {
	ecb.mutex.RLock()
	defer ecb.mutex.RUnlock()

	err := ecb.dao.WithRTx(func(repo dao.Repository) error {
		wasmFiltersPtr, err := repo.FindWasmFilterByListenerId(listener.Id)
		if err != nil {
			return err
		}
		listener.WasmFilters = make([]domain.WasmFilter, len(wasmFiltersPtr))
		for i, p := range wasmFiltersPtr {
			listener.WasmFilters[i] = *p
		}
		listener.ExtAuthzFilter, err = repo.FindExtAuthzFilterByNodeGroup(listener.NodeGroupId)
		return err
	})
	if err != nil {
		logger.Errorf("BuildListener could not load filters using DAO:\n %v", err)
		return nil, err
	}

	if builder, found := ecb.listenerBuilders[listener.NodeGroupId]; found {
		return builder.BuildListener(listener, namespaceMapping, withTls)
	} else {
		return ecb.facadeListenerBuilder.BuildListener(listener, namespaceMapping, withTls)
	}
}

func (ecb *EnvoyConfigBuilderImpl) BuildCluster(nodeGroup string, clusterEntity *domain.Cluster) (*v3cluster.Cluster, error) {
	logger.Infof("Build cluster '%s' for node group '%s'", clusterEntity.Name, nodeGroup)
	gatewayType, err := ecb.getGatewayType(nodeGroup, clusterEntity)
	if err != nil {
		return nil, err
	}

	if builder, found := ecb.clusterBuilders[gatewayType]; found {
		logger.Infof("Found cluster builder for '%s' gateway type", gatewayType)
		return builder.BuildCluster(nodeGroup, clusterEntity, ecb.props.Routes)
	}
	return ecb.clusterBuilder.BuildCluster(nodeGroup, clusterEntity, ecb.props.Routes)
}

func (ecb *EnvoyConfigBuilderImpl) getGatewayType(nodeGroup string, clusterEntity *domain.Cluster) (domain.GatewayType, error) {
	if nodeGroup == EgressGateway {
		return domain.Egress, nil
	}

	existingNodegroup, err := ecb.dao.FindNodeGroupByName(nodeGroup)
	if err != nil {
		logger.Errorf("Could not find existing node group by name %s due to DAO error:\n %v", nodeGroup, err)
		return "", err
	}
	logger.Infof("Found node group %+v for cluster '%s'", existingNodegroup, clusterEntity.Name)
	if existingNodegroup == nil {
		return "", nil
	}

	return existingNodegroup.GatewayType, nil
}

func (ecb *EnvoyConfigBuilderImpl) BuildRouteConfig(routeConfig *domain.RouteConfiguration) (*v3route.RouteConfiguration, error) {
	ecb.mutex.RLock()
	defer ecb.mutex.RUnlock()

	virtualHostsFromDao, err := ecb.dao.FindVirtualHostsByRouteConfigurationId(routeConfig.Id)
	if err != nil {
		logger.Errorf("Failed to load virtual hosts for routeConfig %v using DAO: %v", routeConfig.Id, err)
		return nil, err
	}
	routeConfig.VirtualHosts = virtualHostsFromDao
	var virtualHosts []*v3route.VirtualHost
	if builder, found := ecb.virtualHostBuilders[routeConfig.NodeGroupId]; found {
		virtualHosts, err = builder.BuildVirtualHosts(routeConfig)
	} else {
		virtualHosts, err = ecb.meshVirtualHostBuilder.BuildVirtualHosts(routeConfig)
	}
	if err != nil {
		return nil, err
	}
	return &v3route.RouteConfiguration{
		Name:         routeConfig.Name,
		VirtualHosts: virtualHosts,
	}, nil
}

func (ecb *EnvoyConfigBuilderImpl) BuildRuntime(_ string, runtimeName string) (*v3runtime.Runtime, error) {
	return &v3runtime.Runtime{
		Name: runtimeName,
		Layer: &pstruct.Struct{
			Fields: map[string]*pstruct.Value{
				"re2.max_program_size.error_level": {
					Kind: &structpb.Value_StringValue{StringValue: ecb.props.Googlere2.Maxsize},
				},
				"re2.max_program_size.warn_level": {
					Kind: &structpb.Value_StringValue{StringValue: ecb.props.Googlere2.WarnSize},
				},
			},
		},
	}, nil
}
