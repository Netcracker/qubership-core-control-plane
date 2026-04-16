package config

import (
	"github.com/google/uuid"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
)

const cannotSaveErrorMessage = "Can't save virtual host domain to memory database: \n %v"

type createAndSaveGatewayRoutesFunction func(dao.Repository, entity.ServiceInterface, int32, string, string) error

func createAndSaveInternalGatewayRoutes(storage dao.Repository, entityService entity.ServiceInterface, virtualHostId int32, currentDeploymentVersion string,
	initialDeploymentVersion string) error {
	routes := []*domain.Route{
		/*Control Plane*/
		newCommonRoute(virtualHostId, "route-cp", "/api/v1/routes", "/api/v1/routes/internal-gateway-service",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v2/control-plane/routing/details", "/api/v2/control-plane/routing/details", "/api/v2/control-plane/routing/details",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v3/control-plane/routing/details", "/api/v3/control-plane/routing/details", "/api/v3/routing/details",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v3/control-plane/active-active||v1", "/api/v3/control-plane/active-active", "/api/v3/active-active",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v3/control-plane/composite-platform/namespaces||v1", "/api/v3/control-plane/composite-platform/namespaces", "/api/v3/composite-platform/namespaces",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v3/control-plane/versions/microservices||v1", "/api/v3/control-plane/versions/microservices", "/api/v3/versions/microservices",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v3/control-plane/versions/registry||v1", "/api/v3/control-plane/versions/registry", "/api/v3/versions/registry",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v3/control-plane/rate-limits||v1", "/api/v3/control-plane/rate-limits", "/api/v3/rate-limits",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v3/control-plane/http-filters||v1", "/api/v3/control-plane/http-filters", "/api/v3/http-filters",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v3/control-plane/gateways/specs||v1", "/api/v3/control-plane/gateways/specs", "/api/v3/gateways/specs",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		//CS
		newCommonRoute(virtualHostId, configServerRouteKeyPrefix+"||"+"/api/v1/config-server", "/api/v1/config-server", "/",
			configServerClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newGatewayHealthRoute(virtualHostId, "internal-gateway-health", "/health", controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion, 200),
	}
	logger.Infof("Saving routes for internal gateway")
	err := entityService.PutRoutes(storage, routes)
	if err != nil {
		logger.Errorf(cannotSaveErrorMessage, err)
		return err
	}

	return nil
}

func createAndSavePublicGatewayRoutes(storage dao.Repository, entityService entity.ServiceInterface, virtualHostId int32, currentDeploymentVersion string, initialDeploymentVersion string) error {
	routes := []*domain.Route{
		/*Control Plane*/
		newCommonRoute(virtualHostId, "route-cp", "/api/v1/routes", "/api/v1/routes/public-gateway-service",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "", "/api/v3/control-plane/ui", "/api/v3/ui",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		// health for public gateway
		// added "||control-plane||control-plane||8080". Route have to be moved to active version in BG
		newGatewayHealthRoute(virtualHostId, "public-gateway-health", "/health", controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion, 200),
	}
	logger.Infof("Saving routes for public gateway")
	err := entityService.PutRoutes(storage, routes)
	if err != nil {
		logger.Errorf(cannotSaveErrorMessage, err)
		return err
	}

	return nil
}

func createAndSavePrivateGatewayRoutes(storage dao.Repository, entityService entity.ServiceInterface, virtualHostId int32, currentDeploymentVersion string, initialDeploymentVersion string) error {
	routes := []*domain.Route{
		/*Control Plane*/
		newCommonRoute(virtualHostId, "route-cp", "/api/v1/routes", "/api/v1/routes/private-gateway-service",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v1/routes/clusters||v1", "/api/v1/control-plane/routes/clusters", "/api/v1/routes/clusters",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v1/routes/route-configs||v1", "/api/v1/control-plane/routes/route-configs", "/api/v1/routes/route-configs",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),

		newCommonRoute(virtualHostId, "||/api/v2/control-plane/routes||v1", "/api/v2/control-plane/routes", "/api/v2/control-plane/routes",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v2/control-plane/routing/details||v1", "/api/v2/control-plane/routing/details", "/api/v2/control-plane/routing/details",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v2/control-plane/versions||v1", "/api/v2/control-plane/versions", "/api/v2/control-plane/versions",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v2/control-plane/promote", "/api/v2/control-plane/promote", "/api/v2/control-plane/promote",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v2/control-plane/rollback", "/api/v2/control-plane/rollback", "/api/v2/control-plane/rollback",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v2/control-plane/endpoints", "/api/v2/control-plane/endpoints", "/api/v2/control-plane/endpoints",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v2/control-plane/load-balance", "/api/v2/control-plane/load-balance", "/api/v2/control-plane/load-balance",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v2/control-plane/versions/watch||v1", "/api/v2/control-plane/versions/watch", "/api/v2/control-plane/versions/watch",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),

		newCommonRoute(virtualHostId, "||/api/v3/control-plane/routing/details||v1", "/api/v3/control-plane/routing/details", "/api/v3/routing/details",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v3/control-plane/versions||v1", "/api/v3/control-plane/versions", "/api/v3/versions",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v3/control-plane/versions/microservices||v1", "/api/v3/control-plane/versions/microservices", "/api/v3/versions/microservices",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v3/control-plane/promote", "/api/v3/control-plane/promote", "/api/v3/promote",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v3/control-plane/rollback", "/api/v3/control-plane/rollback", "/api/v3/rollback",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v3/control-plane/endpoints", "/api/v3/control-plane/endpoints", "/api/v3/endpoints",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v3/control-plane/load-balance", "/api/v3/control-plane/load-balance", "/api/v3/load-balance",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v3/control-plane/rate-limits", "/api/v3/control-plane/rate-limits", "/api/v3/rate-limits",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v3/control-plane/http-filters", "/api/v3/control-plane/http-filters", "/api/v3/http-filters",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v3/control-plane/gateways/specs", "/api/v3/control-plane/gateways/specs", "/api/v3/gateways/specs",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v3/control-plane/versions/watch||v1", "/api/v3/control-plane/versions/watch", "/api/v3/versions/watch",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v3/control-plane/routes||v1", "/api/v3/control-plane/routes", "/api/v3/routes",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v3/control-plane/domains||v1", "/api/v3/control-plane/domains", "/api/v3/domains",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v3/control-plane/config||v1", "/api/v3/control-plane/config", "/api/v3/config",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "", "/api/v3/control-plane/apply-config", "/api/v3/apply-config",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newRegexRoute(virtualHostId, "||/api/v3/control-plane/([^/]+)/([^/]+)(/.*)?||v1", "/api/v3/control-plane/([^/]+)/([^/]+)(/.*)?", "/api/v3/\\1/\\2\\3",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		newCommonRoute(virtualHostId, "||/api/v3/control-plane/active-active||v1", "/api/v3/control-plane/active-active", "/api/v3/active-active",
			controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		//CS
		newCommonRoute(virtualHostId, configServerRouteKeyPrefix+"||"+"/api/v1/config-server", "/api/v1/config-server", "/",
			configServerClusterName(), currentDeploymentVersion, initialDeploymentVersion),
		//health for private gateway
		//added "||control-plane||control-plane||8080". Route have to be moved to active version in BG
		newGatewayHealthRoute(virtualHostId, "private-gateway-health", "/health", controlPlaneClusterName(), currentDeploymentVersion, initialDeploymentVersion, 200),
	}
	logger.Infof("Saving routes for private gateway")
	err := entityService.PutRoutes(storage, routes)
	if err != nil {
		logger.Errorf(cannotSaveErrorMessage, err)
		return err
	}

	return nil
}

func newGatewayHealthRoute(virtualHostId int32, routeKey, fromPrefix, clusterName, currentDeploymentVersion, initialDeploymentVersion string, directResponse uint32) *domain.Route {
	return &domain.Route{
		Uuid:                     uuid.New().String(),
		VirtualHostId:            virtualHostId,
		RouteKey:                 routeKey,
		Prefix:                   fromPrefix,
		ClusterName:              clusterName,
		DirectResponseCode:       directResponse,
		DeploymentVersion:        currentDeploymentVersion,
		InitialDeploymentVersion: initialDeploymentVersion,
		Version:                  1,
	}
}

func newCommonRoute(virtualHostId int32, routeKey string, fromPrefix string, toPrefix string,
	clusterName string, deploymentVersion string, initialDeploymentVersion string) *domain.Route {

	return &domain.Route{
		Uuid:                     uuid.New().String(),
		VirtualHostId:            virtualHostId,
		RouteKey:                 routeKey,
		Prefix:                   fromPrefix,
		ClusterName:              clusterName,
		PrefixRewrite:            toPrefix,
		DeploymentVersion:        deploymentVersion,
		InitialDeploymentVersion: initialDeploymentVersion,
		Version:                  1,
	}
}

func newRegexRoute(virtualHostId int32, routeKey, fromRegex, toRegex, clusterName, dVersion, initDVersion string) *domain.Route {
	return &domain.Route{
		Uuid:                     uuid.New().String(),
		VirtualHostId:            virtualHostId,
		RouteKey:                 routeKey,
		Regexp:                   fromRegex,
		ClusterName:              clusterName,
		RegexpRewrite:            toRegex,
		Version:                  1,
		DeploymentVersion:        dVersion,
		InitialDeploymentVersion: initDVersion,
	}
}
