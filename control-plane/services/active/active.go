package active

import (
	"context"
	"errors"
	"fmt"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/events"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/envoy"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
)

var log logging.Logger

func init() {
	log = logging.GetLogger("active-active")
}

const (
	DefaultSuffix            = "core-active-active"
	PublicGwName             = "public-gateway-service"
	PrivateGwName            = "private-gateway-service"
	BalanceTrackHeaderName   = "x-balanced-by-anchor"
	AnchorHeaderName         = "x-anchor"
	DefaultGwHealthPath      = "/health"
	DefaultDeploymentVersion = "v1" //todo active-active configuration should not be affected by blue green

	ProtocolHttp  = "http"
	ProtocolHttps = "https"
)

type ActiveDCsService interface {
	ApplyActiveDCsConfig(ctx context.Context, activeDCsConfig *dto.ActiveDCsV3) error
	DeleteActiveDCsConfig(ctx context.Context) error
}

type ActiveDCsServiceImpl struct {
	dao                dao.Dao
	entityService      *entity.Service
	eventBus           *bus.EventBusAggregator
	localPublicGwHost  string
	localPrivateGwHost string
}

type DisabledActiveDCsService struct {
	reason string
}

type AliasesConfig struct {
	GW             string
	AliasesOrder   []string
	AliasToHostMap map[string]string
	HostToAliasMap map[string]string
	Protocol       string
	HttpPort       int32
	HttpsPort      int32
}

func NewActiveDCsService(dao dao.Dao, entityService *entity.Service, eventBus *bus.EventBusAggregator, localPublicGwHost, localPrivateGwHost string) ActiveDCsService {
	svc := &ActiveDCsServiceImpl{
		dao:                dao,
		entityService:      entityService,
		eventBus:           eventBus,
		localPublicGwHost:  localPublicGwHost,
		localPrivateGwHost: localPrivateGwHost,
	}
	return svc
}

func NewDisabledActiveDCsService(reason string) ActiveDCsService {
	log.Warn("Using DisabledActiveDCsService implementation for ActiveDCsService. Reason: %s", reason)
	return &DisabledActiveDCsService{reason: reason}
}

func (s *DisabledActiveDCsService) ApplyActiveDCsConfig(ctx context.Context, activeDCsConfig *dto.ActiveDCsV3) error {
	return fmt.Errorf("ActiveDCsService disabled. Reason: %s", s.reason)
}

func (s *DisabledActiveDCsService) DeleteActiveDCsConfig(ctx context.Context) error {
	return fmt.Errorf("ActiveDCsService disabled. Reason: %s", s.reason)
}

func (s *ActiveDCsServiceImpl) ApplyActiveDCsConfig(ctx context.Context, activeDCsConfig *dto.ActiveDCsV3) error {
	routeConfigPublic, err := s.buildRouteConfig(activeDCsConfig, PublicGwName)
	if err != nil {
		return err
	}
	routeConfigPrivate, err := s.buildRouteConfig(activeDCsConfig, PrivateGwName)
	if err != nil {
		return err
	}
	return s.applyActiveRouteConfigs(ctx, []*domain.RouteConfiguration{routeConfigPublic, routeConfigPrivate})
}

func (s *ActiveDCsServiceImpl) DeleteActiveDCsConfig(ctx context.Context) error {
	var configs []*domain.RouteConfiguration
	routeConfigs, err := s.entityService.GetRouteConfigurationsWithRelations(s.dao)
	if err != nil {
		return err
	}
	for _, routeConfig := range routeConfigs {
		if routeConfig.Name == GetRouteConfigName(PublicGwName) ||
			routeConfig.Name == GetRouteConfigName(PrivateGwName) {
			configs = append(configs, routeConfig)
		}
	}
	return s.DeleteActiveDCs(ctx, configs)
}

func GetDcAlias(i int) string {
	return "dc-" + strconv.Itoa(i+1) + ".local"
}

func GetActiveActiveClusterName(gw string) string {
	return gw + "||" + DefaultSuffix
}

func GetClusterNameForExternalGw(gw, dcName string) string {
	return gw + "||" + DefaultSuffix + "||" + dcName
}

func GetRouteConfigName(gw string) string {
	return gw + "-routes"
}

func GetExternalVirtualHostName(gw string, alias string) string {
	return gw + "||" + DefaultSuffix + "||" + alias
}

func IsActiveActiveCluster(clusterName string) bool {
	return strings.Contains(clusterName, DefaultSuffix)
}

func (s *ActiveDCsServiceImpl) buildRouteConfig(activeDCsConfig *dto.ActiveDCsV3, gw string) (*domain.RouteConfiguration, error) {
	routeConfiguration := &domain.RouteConfiguration{
		Name:        GetRouteConfigName(gw),
		Version:     1,
		NodeGroupId: gw,
	}
	aliasesConfig, err := s.getAliasesConfig(activeDCsConfig, gw)
	if err != nil {
		return nil, err
	}
	nodeGroup, err := s.buildNodeGroup(activeDCsConfig, aliasesConfig)
	if err != nil {
		return nil, err
	}
	routeConfiguration.NodeGroup = nodeGroup
	virtualHosts, err := s.buildVirtualHosts(activeDCsConfig, aliasesConfig)
	if err != nil {
		return nil, err
	}
	routeConfiguration.VirtualHosts = virtualHosts
	return routeConfiguration, nil
}

func (s *ActiveDCsServiceImpl) buildNodeGroup(activeDCsConfig *dto.ActiveDCsV3, aliasesConfig *AliasesConfig) (*domain.NodeGroup, error) {
	nodeGroup := &domain.NodeGroup{
		Name: aliasesConfig.GW,
	}
	clusters, err := s.buildClusters(activeDCsConfig, aliasesConfig)
	if err != nil {
		return nil, err
	}
	nodeGroup.Clusters = clusters
	return nodeGroup, nil
}

func (s *ActiveDCsServiceImpl) buildVirtualHosts(activeDCsConfig *dto.ActiveDCsV3, aliasesConfig *AliasesConfig) ([]*domain.VirtualHost, error) {
	var virtualHosts []*domain.VirtualHost
	var gwName string
	var localGwHost string
	if aliasesConfig.GW == PublicGwName {
		gwName = PublicGwName
		localGwHost = s.localPublicGwHost
	} else if aliasesConfig.GW == PrivateGwName {
		gwName = PrivateGwName
		localGwHost = s.localPrivateGwHost
	} else {
		return nil, errors.New(fmt.Sprintf("Unsupported gateway name: %s", aliasesConfig.GW))
	}
	defaultVH := &domain.VirtualHost{
		Name:    gwName,
		Version: 1,
		Routes:  nil,
		Domains: []*domain.VirtualHostDomain{{
			Domain:  "*",
			Version: 1,
		}},
	}
	balancingRoute, err := s.buildBalancingRoute(activeDCsConfig, aliasesConfig)
	if err != nil {
		return nil, err
	}
	defaultVH.Routes = []*domain.Route{balancingRoute}
	virtualHosts = append(virtualHosts, defaultVH)

	for alias, host := range aliasesConfig.AliasToHostMap {
		// build route only for external dc gws, local dc request will be processed by default virtualHost with '*' domain
		if host != localGwHost {
			// create virtual host for requests coming via loopback interface with domain equal to external dc alias
			externalDcGwVH := &domain.VirtualHost{
				Name:    GetExternalVirtualHostName(gwName, alias),
				Version: 1,
				Domains: []*domain.VirtualHostDomain{{
					Domain:  alias,
					Version: 1,
				}},
			}
			externalDcGwRoute, err := s.buildRouteToExternalDcGw(activeDCsConfig, aliasesConfig, alias, host)
			if err != nil {
				return nil, err
			}
			externalDcGwVH.Routes = []*domain.Route{externalDcGwRoute}
			virtualHosts = append(virtualHosts, externalDcGwVH)
		}
	}
	return virtualHosts, nil
}

func (s *ActiveDCsServiceImpl) buildClusters(activeDCsConfig *dto.ActiveDCsV3, aliasesConfig *AliasesConfig) ([]*domain.Cluster, error) {
	var clusters []*domain.Cluster
	balancingCluster, err := s.buildBalancingClusterToAliasEndpoints(activeDCsConfig, aliasesConfig)
	if err != nil {
		return nil, err
	}
	clusters = append(clusters, balancingCluster)
	var localGwHost string
	if aliasesConfig.GW == PublicGwName {
		localGwHost = s.localPublicGwHost
	} else if aliasesConfig.GW == PrivateGwName {
		localGwHost = s.localPrivateGwHost
	} else {
		return nil, fmt.Errorf("invalid gateway name: %s", aliasesConfig.GW)
	}
	// build clusters for external data centers gws
	for gwHost, gwAlias := range aliasesConfig.HostToAliasMap {
		if gwHost != localGwHost {
			// endpoint to external dc, create cluster
			externalCluster, err := s.buildClusterForExternalDC(activeDCsConfig, aliasesConfig, gwAlias, gwHost)
			if err != nil {
				return nil, err
			}
			clusters = append(clusters, externalCluster)
		}
	}
	return clusters, nil
}

func (s *ActiveDCsServiceImpl) getAliasesConfig(activeDCsConfig *dto.ActiveDCsV3, gw string) (*AliasesConfig, error) {
	var aliasesConfig *AliasesConfig
	var hosts []string
	var localHost string
	if gw == PublicGwName {
		hosts = activeDCsConfig.PublicGwHosts
		localHost = s.localPublicGwHost
	} else if gw == PrivateGwName {
		hosts = activeDCsConfig.PrivateGwHosts
		localHost = s.localPrivateGwHost
	} else {
		return nil, fmt.Errorf("Unsupported gateway name: %s", gw)
	}
	hosts = NormalizeHostSlice(hosts)
	// check that hosts contain localGwHost configured in control-plane during deploy
	localGwHostFound := false
	for _, host := range hosts {
		if host == localHost {
			localGwHostFound = true
			break
		}
	}
	if !localGwHostFound {
		return nil, fmt.Errorf("hosts for '%s' does not contain local gw host: '%s'", gw, localHost)
	}
	httpPort := int32(80)
	if activeDCsConfig.HttpPort != nil {
		httpPort = *activeDCsConfig.HttpPort
	}
	httpsPort := int32(443)
	if activeDCsConfig.HttpsPort != nil {
		httpPort = *activeDCsConfig.HttpsPort
	}
	aliasesConfig, err := generateAliasesConfig(gw, hosts, activeDCsConfig.Protocol, httpPort, httpsPort)
	if err != nil {
		return nil, err
	}
	return aliasesConfig, nil
}

func generateAliasesConfig(gw string, hosts []string, protocol string, httpPort, httpsPort int32) (*AliasesConfig, error) {
	var aliasesConfig *AliasesConfig
	if len(hosts) == 0 {
		return nil, fmt.Errorf("hosts for '%s' cannot be empty", gw)
	}
	protocol = strings.TrimSpace(protocol)
	if protocol == "" {
		return nil, errors.New("'protocol' cannot be empty")
	}
	aliasToHostMap, hostToAliasMap, err := generateMappings(hosts)
	if err != nil {
		return nil, err
	}
	aliases := make([]string, len(hosts))
	for i, host := range hosts {
		aliases[i] = hostToAliasMap[host]
	}
	aliasesConfig = &AliasesConfig{
		GW:             gw,
		Protocol:       protocol,
		AliasesOrder:   aliases,
		AliasToHostMap: aliasToHostMap,
		HostToAliasMap: hostToAliasMap,
		HttpPort:       httpPort,
		HttpsPort:      httpsPort,
	}
	return aliasesConfig, nil
}

func generateMappings(hosts []string) (aliasToHostMap, hostToAliasMap map[string]string, err error) {
	dcAmount := len(hosts)
	aliasToHostMap = make(map[string]string, dcAmount)
	hostToAliasMap = make(map[string]string, dcAmount)
	for i, host := range hosts {
		alias := GetDcAlias(i)
		aliasToHostMap[alias] = host
		hostToAliasMap[host] = alias
	}
	if len(aliasToHostMap) != len(hostToAliasMap) {
		err = fmt.Errorf("invalid 'hosts' %v. Each host must be unique", hosts)
		return
	}
	return
}

func NormalizeHostSlice(hostsSlice []string) []string {
	var hostsSliceNormalized []string
	for _, host := range hostsSlice {
		hostTrimmed := strings.TrimSpace(host)
		if hostTrimmed == "" {
			continue
		}
		hostsSliceNormalized = append(hostsSliceNormalized, hostTrimmed)
	}
	return hostsSliceNormalized
}

func (s *ActiveDCsServiceImpl) buildBalancingClusterToAliasEndpoints(activeDCsConfig *dto.ActiveDCsV3, aliasesConfig *AliasesConfig) (*domain.Cluster, error) {
	var httpVersion int32 = 1
	cluster := &domain.Cluster{
		Name:             GetActiveActiveClusterName(aliasesConfig.GW),
		LbPolicy:         domain.LbPolicyMaglev,
		DiscoveryType:    domain.DISCOVERY_TYPE_STRICT_DNS,
		DiscoveryTypeOld: domain.DISCOVERY_TYPE_STRICT_DNS,
		HttpVersion:      &httpVersion,
		Version:          1,
	}
	commonLbConfig := &domain.CommonLbConfig{
		HealthyPanicThreshold:           50,
		UpdateMergeWindow:               nil,
		IgnoreNewHostsUntilFirstHc:      true,
		CloseConnectionsOnHostSetChange: false,
		ConsistentHashingLbConfig: &domain.ConsistentHashingLbConfig{
			UseHostnameForHashing: true,
		},
	}
	commonLbConfigDto := activeDCsConfig.CommonLbConfig
	if commonLbConfigDto != nil {
		commonLbConfig.HealthyPanicThreshold = commonLbConfigDto.HealthyPanicThreshold
	}
	cluster.CommonLbConfig = commonLbConfig
	healthCheckConfig := activeDCsConfig.HealthCheck
	var healthCheck *domain.HealthCheck
	if healthCheckConfig == nil {
		healthCheckConfig = &dto.ActiveDCsHealthCheckV3{
			Timeout:  1000,
			Interval: 1000,
		}
	}
	healthCheck = s.buildHealthCheckFromDto(healthCheckConfig)
	cluster.HealthChecks = []*domain.HealthCheck{healthCheck}
	// build dns_resolver
	dnsResolver := domain.DnsResolver{
		SocketAddress: &domain.SocketAddress{
			Address:     "127.0.0.1",
			Port:        1053,
			Protocol:    "UDP",
			IPv4_compat: true,
		},
	}
	cluster.DnsResolvers = []domain.DnsResolver{dnsResolver}
	// build endpoints
	var endpoints []*domain.Endpoint
	for id, gwAlias := range aliasesConfig.AliasesOrder {
		// endpoint will listen on loopback interface, so the traffic will be redirected to envoy for further routing
		endpoint := &domain.Endpoint{
			Address:                  gwAlias,
			Port:                     8080,
			DeploymentVersion:        DefaultDeploymentVersion,
			InitialDeploymentVersion: DefaultDeploymentVersion,
			Hostname:                 gwAlias,
			OrderId:                  int32(id),
		}
		endpoints = append(endpoints, endpoint)
	}
	cluster.Endpoints = endpoints
	return cluster, nil
}

func (s *ActiveDCsServiceImpl) buildClusterForExternalDC(activeDCsConfig *dto.ActiveDCsV3, aliasesConfig *AliasesConfig, dcName, host string) (*domain.Cluster, error) {
	var httpVersion int32 = 1
	cluster := &domain.Cluster{
		Name:             GetClusterNameForExternalGw(aliasesConfig.GW, dcName),
		LbPolicy:         domain.LbPolicyLeastRequest,
		DiscoveryType:    domain.DISCOVERY_TYPE_STRICT_DNS,
		DiscoveryTypeOld: domain.DISCOVERY_TYPE_STRICT_DNS,
		HttpVersion:      &httpVersion,
		Version:          1,
	}
	var port int32
	if aliasesConfig.Protocol == ProtocolHttp {
		port = aliasesConfig.HttpPort
	} else if aliasesConfig.Protocol == ProtocolHttps {
		port = aliasesConfig.HttpsPort
	} else {
		return nil, fmt.Errorf("invalid protocol: '%s'", aliasesConfig.Protocol)
	}
	endpoint := &domain.Endpoint{
		Address:                  host,
		Port:                     port,
		DeploymentVersion:        DefaultDeploymentVersion,
		InitialDeploymentVersion: DefaultDeploymentVersion,
		Hostname:                 host,
	}
	cluster.Endpoints = []*domain.Endpoint{endpoint}
	return cluster, nil
}

func (s *ActiveDCsServiceImpl) buildHealthCheckFromDto(healthCheckDto *dto.ActiveDCsHealthCheckV3) *domain.HealthCheck {
	healthCheck := &domain.HealthCheck{
		Timeout:                      healthCheckDto.Timeout,
		Interval:                     healthCheckDto.Interval,
		HealthyThreshold:             10,
		InitialJitter:                0,
		IntervalJitter:               0,
		IntervalJitterPercent:        0,
		ReuseConnection:              true,
		NoTrafficInterval:            1000,
		UnhealthyInterval:            healthCheckDto.Interval,
		UnhealthyThreshold:           10,
		UnhealthyEdgeInterval:        healthCheckDto.Interval,
		HealthyEdgeInterval:          healthCheckDto.Interval,
		EventLogPath:                 "",
		AlwaysLogHealthCheckFailures: false,
		TlsOptions:                   nil,
	}
	if healthCheckDto.HealthyThreshold != nil {
		healthCheck.HealthyThreshold = *healthCheckDto.HealthyThreshold
	}
	if healthCheckDto.UnhealthyThreshold != nil {
		healthCheck.UnhealthyThreshold = *healthCheckDto.UnhealthyThreshold
	}
	if healthCheckDto.NoTrafficInterval != nil {
		healthCheck.NoTrafficInterval = *healthCheckDto.NoTrafficInterval
	}
	if healthCheckDto.UnhealthyInterval != nil {
		healthCheck.UnhealthyInterval = *healthCheckDto.UnhealthyInterval
		healthCheck.UnhealthyEdgeInterval = *healthCheckDto.UnhealthyInterval
	}
	healthCheck.HttpHealthCheck = &domain.HttpHealthCheck{
		Path: DefaultGwHealthPath,
		ExpectedStatuses: []domain.RangeMatch{
			{
				Start: domain.NewNullInt(200),
				End:   domain.NewNullInt(202),
			},
		},
	}
	return healthCheck
}

func (s *ActiveDCsServiceImpl) buildBalancingRoute(activeDCsConfig *dto.ActiveDCsV3, aliasesConfig *AliasesConfig) (*domain.Route, error) {
	// this route is only used for 'anchored' request (requests with 'activeDCsConfig.BalancingHeaderName' header and yet no 'BALANCE_TRACK_HEADER_NAME' header)
	balancingRoute := &domain.Route{
		Uuid:                     uuid.New().String(),
		Prefix:                   "/",
		HostAutoRewrite:          domain.NewNullBool(true),
		Version:                  1,
		DeploymentVersion:        DefaultDeploymentVersion,
		InitialDeploymentVersion: DefaultDeploymentVersion,
		HeaderMatchers: []*domain.HeaderMatcher{
			{
				Name:         AnchorHeaderName,
				Version:      1,
				PresentMatch: domain.NewNullBool(true),
			},
			{
				Name:         BalanceTrackHeaderName,
				Version:      1,
				PresentMatch: domain.NewNullBool(true),
				InvertMatch:  true,
			},
		},
		RequestHeadersToAdd: []domain.Header{
			{
				Name:  BalanceTrackHeaderName,
				Value: "-",
			},
		},
		ClusterName: GetActiveActiveClusterName(aliasesConfig.GW),
		HashPolicies: []*domain.HashPolicy{
			{
				HeaderName: AnchorHeaderName,
			},
		},
		RetryPolicy: &domain.RetryPolicy{
			RetryOn:              "gateway-error,reset,connect-failure",
			NumRetries:           30,
			PerTryTimeout:        2000,
			RetriableStatusCodes: []uint32{200},
			RetryBackOff: &domain.RetryBackOff{
				BaseInterval: 2000,
				MaxInterval:  2000,
			},
		},
	}
	retryPolicyConfig := activeDCsConfig.RetryPolicy
	if retryPolicyConfig != nil {
		balancingRoute.RetryPolicy.RetryOn = retryPolicyConfig.RetryOn
		balancingRoute.RetryPolicy.NumRetries = retryPolicyConfig.NumRetries
		if retryPolicyConfig.PerTryTimeout != nil {
			balancingRoute.RetryPolicy.PerTryTimeout = *retryPolicyConfig.PerTryTimeout
		}
		if retryPolicyConfig.RetryBackOff != nil {
			balancingRoute.RetryPolicy.RetryBackOff = &domain.RetryBackOff{
				BaseInterval: retryPolicyConfig.RetryBackOff.BaseInterval,
				MaxInterval:  retryPolicyConfig.RetryBackOff.MaxInterval,
			}
		}
	}
	return balancingRoute, nil
}

func (s *ActiveDCsServiceImpl) buildRouteToExternalDcGw(activeDCsConfig *dto.ActiveDCsV3, aliasesConfig *AliasesConfig, externalDcName, host string) (*domain.Route, error) {
	routeToExternalDcGw := &domain.Route{
		Uuid:   uuid.New().String(),
		Prefix: "/",
		//HostRewrite:              host,
		HostAutoRewrite:          domain.NewNullBool(true),
		Version:                  1,
		DeploymentVersion:        DefaultDeploymentVersion,
		InitialDeploymentVersion: DefaultDeploymentVersion,
		ClusterName:              GetClusterNameForExternalGw(aliasesConfig.GW, externalDcName),
	}
	return routeToExternalDcGw, nil
}

func (s *ActiveDCsServiceImpl) applyActiveRouteConfigs(ctx context.Context, routeConfigs []*domain.RouteConfiguration) error {
	for _, routeConfig := range routeConfigs {
		changes, err := s.dao.WithWTx(func(dao dao.Repository) error {
			return s.persistActiveDCsConfig(ctx, routeConfig, dao)
		})
		if err != nil {
			return err
		}
		log.InfoC(ctx, "Active-Active config successfully applied. RouteConfig: %+v", routeConfig)
		if err = s.publishChanges(ctx, routeConfig.NodeGroup.Name, changes); err != nil {
			return err
		}
	}
	return nil
}

func (s *ActiveDCsServiceImpl) DeleteActiveDCs(ctx context.Context, routeConfigs []*domain.RouteConfiguration) error {
	for _, routeConfig := range routeConfigs {
		changes, err := s.dao.WithWTx(func(dao dao.Repository) error {
			return s.deleteActiveDCsConfig(ctx, routeConfig, dao)
		})
		if err != nil {
			return err
		}
		if len(changes) > 0 {
			log.InfoC(ctx, "Active-Active config successfully deleted. RouteConfig: %+v", routeConfig)
			if err = s.publishChanges(ctx, routeConfig.NodeGroup.Name, changes); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *ActiveDCsServiceImpl) persistActiveDCsConfig(ctx context.Context, routeConfig *domain.RouteConfiguration, dao dao.Repository) error {
	// save NodeGroup
	nodeGroup := routeConfig.NodeGroup
	_, err := s.entityService.CreateOrUpdateNodeGroup(dao, *nodeGroup)
	if err != nil {
		log.ErrorC(ctx, "Failed to load or save nodeGroup %v to in memory storage: %v", nodeGroup.Name, err)
		return err
	}

	// save Cluster
	for i := range nodeGroup.Clusters {
		clusterToSave := nodeGroup.Clusters[i]
		err := s.entityService.PutCluster(dao, clusterToSave)
		if err != nil {
			log.Errorf("Failed to load or save cluster %v to in memory storage: %v", clusterToSave.Name, err)
			return err
		}

		// save ClusterNodeGroup
		clusterNodeGroup := domain.NewClusterNodeGroups(clusterToSave.Id, nodeGroup.Name)
		err = s.entityService.PutClustersNodeGroupIfAbsent(dao, clusterNodeGroup)
		if err != nil {
			log.Errorf("Failed to bind cluster %v to node group %v: %v", clusterToSave.Name, nodeGroup, err)
			return err
		}

		// save Endpoints
		for _, newEndpoint := range clusterToSave.Endpoints {
			newEndpoint.ClusterId = clusterToSave.Id
		}
		if err = s.entityService.PutEndpoints(dao, clusterToSave.Id, clusterToSave.Endpoints); err != nil {
			log.Errorf("Failed to load or save endpoints %v to in memory storage: %v", clusterToSave.Endpoints, err)
			return err
		}

		//save HealthChecks
		for _, newHealthCheck := range clusterToSave.HealthChecks {
			newHealthCheck.ClusterId = clusterToSave.Id
			if err = s.entityService.PutHealthCheck(dao, newHealthCheck); err != nil {
				log.Errorf("Failed to save HealthCheck %v to in memory storage: %v", newHealthCheck, err)
				return err
			}
		}
	}

	// save RouteConfig
	err = s.entityService.PutRouteConfig(dao, routeConfig)
	if err != nil {
		log.Errorf("Failed to load or save route configuration %v to in memory storage: %v", routeConfig.Name, err)
		return err
	}

	// save VirtualHost
	for _, newVirtualHost := range routeConfig.VirtualHosts {
		newVirtualHost.RouteConfigurationId = routeConfig.Id
		err := s.entityService.PutVirtualHost(dao, newVirtualHost)
		if err != nil {
			log.Errorf("Failed to create or update virtual host %v to in memory storage: %v", newVirtualHost.Name, err)
			return err
		}

		// save Routes
		for _, newRoute := range newVirtualHost.Routes {
			err := s.entityService.PutRoute(dao, newVirtualHost.Id, newRoute)
			if err != nil {
				log.Errorf("Failed to create or update route %v, err: %v", newRoute.RouteKey, err)
				return err
			}
		}
	}
	if err := envoy.UpdateAllResourceVersions(dao, nodeGroup.Name); err != nil {
		log.ErrorC(ctx, "Failed to update envoy resource versions: %v", err)
		return err
	}
	return nil
}

func (s *ActiveDCsServiceImpl) deleteActiveDCsConfig(ctx context.Context, routeConfig *domain.RouteConfiguration, dao dao.Repository) error {
	nodeGroup := routeConfig.NodeGroup
	if nodeGroup == nil {
		return nil
	}
	// delete active-active Clusters and all related entities
	clustersMap := clusterSliceToMap(nodeGroup.Clusters)
	if aaCluster, ok := clustersMap[GetActiveActiveClusterName(nodeGroup.Name)]; ok {
		// there is active-active cluster, need to delete
		for _, endpoint := range aaCluster.Endpoints {
			cluster := clustersMap[GetClusterNameForExternalGw(nodeGroup.Name, endpoint.Hostname)]
			if cluster == nil {
				// endpoint for current dc, skip
				continue
			}
			cluster, _ = s.dao.FindClusterById(cluster.Id)
			if cluster == nil {
				// already deleted
				continue
			}
			if err := s.entityService.DeleteClusterCascade(dao, cluster); err != nil {
				log.Errorf("Failed to delete cluster %s, err: %v", cluster.Name, err)
				return err
			}
		}
		aaCluster, _ = s.dao.FindClusterById(aaCluster.Id)
		if aaCluster != nil {
			if err := s.entityService.DeleteClusterCascade(dao, aaCluster); err != nil {
				log.Errorf("Failed to delete cluster %s, err: %v", aaCluster.Name, err)
				return err
			}
		}
	}
	// delete active-active VirtualHosts
	for _, virtualHost := range routeConfig.VirtualHosts {
		virtualHost, _ = s.dao.FindVirtualHostById(virtualHost.Id)
		if virtualHost == nil {
			// already deleted
			continue
		}
		if virtualHost.Name == GetExternalVirtualHostName(nodeGroup.Name, virtualHost.Domains[0].Domain) {
			// delete only active-active VH
			if err := s.entityService.DeleteVirtualHostDomainsByVirtualHost(dao, virtualHost); err != nil {
				log.Errorf("Failed to delete virtual host domains %s, err: %v", virtualHost.Name, err)
				return err
			}
			if err := dao.DeleteVirtualHost(virtualHost); err != nil {
				log.Errorf("Failed to delete virtual host %s, err: %v", virtualHost.Name, err)
				return err
			}
		} else if virtualHost.Name == nodeGroup.Name {
			// delete balancing route from default virtualHost
			balancingClusterName := GetActiveActiveClusterName(nodeGroup.Name)
			activeActiveRouteUuid := ""
			for _, route := range virtualHost.Routes {
				if route.ClusterName == balancingClusterName {
					activeActiveRouteUuid = route.Uuid
					break
				}
			}
			if activeActiveRouteUuid != "" {
				if err := dao.DeleteRouteByUUID(activeActiveRouteUuid); err != nil {
					log.Errorf("Failed to delete active-active balancing route with uuid %s, err: %v", activeActiveRouteUuid, err)
					return err
				}
			}
		}
	}

	if err := envoy.UpdateAllResourceVersions(dao, nodeGroup.Name); err != nil {
		log.ErrorC(ctx, "Failed to update envoy resource versions: %v", err)
		return err
	}
	return nil
}

func clusterSliceToMap(clusters []*domain.Cluster) map[string]*domain.Cluster {
	result := make(map[string]*domain.Cluster, len(clusters))
	for _, cluster := range clusters {
		result[cluster.Name] = cluster
	}
	return result
}

func (s *ActiveDCsServiceImpl) publishChanges(ctx context.Context, nodeGroupName string, changes []memdb.Change) error {
	event := events.NewChangeEventByNodeGroup(nodeGroupName, changes)
	if err := s.eventBus.Publish(bus.TopicChanges, event); err != nil {
		log.ErrorC(ctx, "Can't publish event to eventBus: topic=%s, event=%v, error: %s", bus.TopicChanges, event, err.Error())
		return err
	}
	return nil
}
