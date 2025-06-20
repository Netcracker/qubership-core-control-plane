package registration

import (
	"context"
	"fmt"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/cluster/clusterkey"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/creator"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
)

type V2RequestProcessor struct {
	dao dao.Dao
}

func NewV2RequestProcessor(dao dao.Dao) V2RequestProcessor {
	return V2RequestProcessor{
		dao: dao,
	}
}

func (p V2RequestProcessor) ProcessRequestV2(ctx context.Context, nodeGroupName string, regRequests []dto.RouteRegistrationRequest, activeVersion string) (ProcessedRequest, error) {
	commonEntityBuilder := NewCommonEntityBuilder(nodeGroupName)
	result := ProcessedRequest{GroupedRoutes: NewGroupedRoutesMap(), ClusterNodeGroups: make(map[string][]string), ClusterTlsConfig: make(map[string]string)}

	// build node group
	result.NodeGroups = []domain.NodeGroup{commonEntityBuilder.CreateNodeGroup()}

	// build listener
	result.Listeners = []domain.Listener{commonEntityBuilder.CreateListener()}

	// build route config
	routeConfiguration := commonEntityBuilder.CreateRouteConfiguration()

	// build virtual host
	virtualHost := commonEntityBuilder.CreateVirtualHost()

	// build virtual host domain
	vhDomain := commonEntityBuilder.CreateVirtualHostDomain()
	virtualHost.Domains = []*domain.VirtualHostDomain{&vhDomain}

	clusters := make(map[string]*domain.Cluster) // map <clusterName>:<cluster> to avoid duplicates
	deploymentVersionsSet := make(map[string]bool)

	for _, regRequest := range regRequests {
		// save deployment version
		logger.InfoC(ctx, "Processing RouteRegistration item. Namespace='%s', Endpoint='%s', Version='%s'", regRequest.Namespace, regRequest.Endpoint, regRequest.Version)

		msAddress := msaddr.NewMicroserviceAddress(regRequest.Endpoint, regRequest.Namespace)
		clusterName := clusterkey.DefaultClusterKeyGenerator.GenerateKey(regRequest.Cluster, msAddress)

		initialDeploymentVersion, deploymentVersion, err := ResolveVersions(p.dao, clusterName, regRequest.Version, activeVersion)
		if err != nil {
			return ProcessedRequest{}, err
		}
		logger.InfoC(ctx, "Resolved initialDeploymentVersion as '%s' and DeploymentVersion as '%s'", initialDeploymentVersion, deploymentVersion)

		if _, found := deploymentVersionsSet[deploymentVersion]; !found {
			deploymentVersionsSet[deploymentVersion] = true
		}

		isAllowed := true
		if regRequest.Allowed != nil {
			isAllowed = *regRequest.Allowed
		}

		// build cluster
		var cluster *domain.Cluster
		alreadyPresent := false
		if cluster, alreadyPresent = clusters[clusterName]; !alreadyPresent {
			cluster = domain.NewCluster(clusterName, false)
			cluster.Endpoints = make([]*domain.Endpoint, 0)
			clusters[clusterName] = cluster
		}
		// build endpoint
		endpoint := resolveEndpoint(cluster, msAddress, deploymentVersion)

		// enable tls for cluster if necessary
		if msAddress.GetProto() == "https" || endpoint.Port == 443 {
			result.ClusterTlsConfig[cluster.Name] = regRequest.Cluster + "-tls"
		}

		// build routes
		endpointAddr := fmt.Sprintf("%s:%v", endpoint.Address, endpoint.Port)
		for _, routeItem := range regRequest.Routes {
			logger.InfoC(ctx, "Processing Route{prefix=%s,prefixRewrite=%s,timeout=%v,headerMatchers=%v", routeItem.Prefix, routeItem.PrefixRewrite, routeItem.Timeout, routeItem.HeaderMatchers)
			domainHeaderMatchers := dto.HeaderMatchersToDomain(routeItem.HeaderMatchers)
			routeEntry := creator.NewRouteEntry(routeItem.Prefix, routeItem.PrefixRewrite, regRequest.Namespace, creator.GetInt64Timeout(routeItem.Timeout), creator.DefaultIdleTimeoutSpec, domainHeaderMatchers)
			isValidRoute := routeEntry.IsValidRoute()
			if !isValidRoute {
				logger.Warnf("The route hasn't been registered. Detected a bad route in the request. microserviceUrl = %s, Route entry = %v", regRequest.Endpoint, routeEntry)
				continue
			}
			route := routeEntry.CreateRoute(0, routeEntry.GetFrom(), endpointAddr, clusterName, routeEntry.GetTimeout(), routeEntry.GetIdleTimeout(), deploymentVersion, initialDeploymentVersion, routeEntry.GetHeaderMatchers(), []domain.Header{}, []string{})
			if isAllowed {
				routeEntry.ConfigureAllowedRoute(route)
			} else {
				routeEntry.ConfigureProhibitedRoute(route, false)
			}
			virtualHost.Routes = append(virtualHost.Routes, route)
			// save route to grouped map for usage by RoutesAutoGenerator
			result.GroupedRoutes.PutRoute(regRequest.Namespace, clusterName, deploymentVersion, route)
		}
	}

	// set everything to resulting struct

	result.Clusters = make([]domain.Cluster, 0, len(clusters))
	for _, cluster := range clusters {
		result.Clusters = append(result.Clusters, *cluster)
		result.ClusterNodeGroups[cluster.Name] = []string{nodeGroupName}
	}

	routeConfiguration.VirtualHosts = append(routeConfiguration.VirtualHosts, &virtualHost)
	result.RouteConfigurations = []domain.RouteConfiguration{routeConfiguration}

	result.DeploymentVersions = make([]string, 0, len(deploymentVersionsSet)+1)
	for version := range deploymentVersionsSet {
		result.DeploymentVersions = append(result.DeploymentVersions, version)
	}

	return result, nil
}
