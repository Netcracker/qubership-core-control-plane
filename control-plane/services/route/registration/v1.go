package registration

import (
	"context"
	"fmt"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/cluster/clusterkey"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/creator"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
)

//var V1RequestProcessor = v1RequestProcessor{}

type V1RequestProcessor struct {
	dao dao.Dao
}

func NewV1RequestProcessor(dao dao.Dao) *V1RequestProcessor {
	return &V1RequestProcessor{
		dao: dao,
	}
}

func (proc V1RequestProcessor) ProcessRequestV1(ctx context.Context, nodeGroupName string, regRequest creator.RouteEntityRequest, activeVersion string) (ProcessedRequest, error) {
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
	microserviceUrl := regRequest.GetMicroserviceUrl()
	isAllowed := regRequest.IsAllowed()
	logger.InfoC(ctx, "Processing %v", regRequest)

	for _, routeEntry := range regRequest.GetRoutes() {

		logger.InfoC(ctx, "Processing %v", routeEntry)
		msAddress := msaddr.NewMicroserviceAddress(microserviceUrl, routeEntry.GetNamespace())
		clusterName := clusterkey.DefaultClusterKeyGenerator.GenerateKey("", msAddress)
		initialDeploymentVersion, deploymentVersion, err := ResolveVersions(proc.dao, clusterName, "", activeVersion)
		if err != nil {
			return ProcessedRequest{}, err
		}
		logger.InfoC(ctx, "Resolved initialDeploymentVersion as '%s' and DeploymentVersion as '%s'", initialDeploymentVersion, deploymentVersion)

		// build cluster
		var cluster *domain.Cluster
		alreadyPresent := false
		if cluster, alreadyPresent = clusters[clusterName]; !alreadyPresent {
			cluster = domain.NewCluster(clusterName, false)
			cluster.Endpoints = make([]*domain.Endpoint, 0)
			clusters[clusterName] = cluster
		}

		// build endpoint
		endpoint := resolveEndpoint(cluster, msAddress, activeVersion)

		// enable tls for cluster if necessary
		if msAddress.GetProto() == "https" || endpoint.Port == 443 {
			result.ClusterTlsConfig[cluster.Name] = clusterkey.DefaultClusterKeyGenerator.ExtractFamilyName(clusterName) + "-tls"
		}

		// build route
		isValidRoute := routeEntry.IsValidRoute()
		if !isValidRoute {
			logger.WarnC(ctx, "The route hasn't been registered. Detected a bad route in the request. microserviceUrl = %s, Route entry = %v", microserviceUrl, routeEntry)
			continue
		}
		endpointAddr := fmt.Sprintf("%s:%v", endpoint.Address, endpoint.Port)
		var routeHeaderMatchers []*domain.HeaderMatcher
		route := routeEntry.CreateRoute(0, routeEntry.GetFrom(), endpointAddr, clusterName,
			routeEntry.GetTimeout(), creator.DefaultIdleTimeoutSpec, deploymentVersion, initialDeploymentVersion,
			routeHeaderMatchers, []domain.Header{}, []string{})
		if isAllowed {
			routeEntry.ConfigureAllowedRoute(route)
		} else {
			routeEntry.ConfigureProhibitedRoute(route, false)
		}
		virtualHost.Routes = append(virtualHost.Routes, route)
	}

	// set everything to resulting struct

	result.Clusters = make([]domain.Cluster, 0, len(clusters))
	for _, cluster := range clusters {
		result.Clusters = append(result.Clusters, *cluster)
		result.ClusterNodeGroups[cluster.Name] = []string{nodeGroupName}
	}

	routeConfiguration.VirtualHosts = append(routeConfiguration.VirtualHosts, &virtualHost)
	result.RouteConfigurations = []domain.RouteConfiguration{routeConfiguration}

	result.DeploymentVersions = []string{}

	return result, nil
}
