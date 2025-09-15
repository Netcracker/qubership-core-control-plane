package cache

import (
	v3cache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/builder"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/builder/common"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/builder/listener"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/builder/routeconfig"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
)

const (
	EgressGateway = "egress-gateway"
)

func DefaultRouteTransformer() *routeconfig.RouteTransformer {
	return routeconfig.NewRouteTransformer(
		routeconfig.NewSimpleRouteTransformationRule(),
		routeconfig.NewGenericVersionedRouteTransformRule(),
		routeconfig.NewNoActiveRouteTransformer())
}

func DefaultRoutePreparer(dao dao.Repository, entitySrv *entity.Service) routeconfig.RoutePreparer {
	routeTransformer := DefaultRouteTransformer()
	routeStatefulSessionPreparer := routeconfig.NewRouteStatefulSessionPreparer(dao, entitySrv)
	routeMultiVersionPreparer := routeconfig.NewRouteMultiVersionPreparer(routeStatefulSessionPreparer, dao, routeTransformer)
	extraSlashRoutePreparer := routeconfig.NewExtraSlashRoutePreparer(routeMultiVersionPreparer)
	sortRoutePreparer := routeconfig.NewSortRoutePreparer(extraSlashRoutePreparer)
	return sortRoutePreparer
}

func DefaultEnvoyConfigurationBuilder(dao dao.Dao, entitySrv *entity.Service, provider routeconfig.VersionAliasesProvider) builder.EnvoyConfigBuilder {
	envoyProxyProps := common.NewEnvoyProxyProperties()

	facadeListenerBuilder := listener.NewFacadeListenerBuilder(envoyProxyProps.Tracing)
	gatewayListenerBuilder := listener.NewGatewayListenerBuilder(envoyProxyProps)

	routePreparer := DefaultRoutePreparer(dao, entitySrv)
	meshRouteBuilder := routeconfig.NewMeshRouteBuilder(dao, envoyProxyProps, routePreparer)
	gatewayRouteBuilder := routeconfig.NewRouteBuilder(dao, envoyProxyProps, routePreparer)
	egressRouteBuilder := routeconfig.NewEgressRouteBuilder(dao, envoyProxyProps, routePreparer)

	meshVirtualHostBuilder := routeconfig.NewMeshVirtualHostBuilder(dao, entitySrv, meshRouteBuilder)
	gatewayVirtualHostBuilder := routeconfig.NewGatewayVirtualHostBuilder(dao, gatewayRouteBuilder, provider)
	ingressVirtualHostBuilder := routeconfig.NewIngressVirtualHostBuilder(dao, entitySrv, gatewayRouteBuilder)
	egressVirtualHostBuilder := routeconfig.NewEgressVirtualHostBuilder(dao, egressRouteBuilder, provider)

	return builder.NewEnvoyConfigBuilder(dao, envoyProxyProps,
		facadeListenerBuilder, gatewayListenerBuilder,
		meshVirtualHostBuilder, gatewayVirtualHostBuilder, ingressVirtualHostBuilder, egressVirtualHostBuilder)
}

func DefaultUpdateManager(dao dao.Dao, snapshotCache v3cache.SnapshotCache, envoyConfigBuilder builder.EnvoyConfigBuilder) *UpdateManager {
	return NewUpdateManager(dao, snapshotCache, envoyConfigBuilder)
}
