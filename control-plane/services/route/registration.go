package route

import (
	"context"
	"fmt"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/business/format"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/factory"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/registration"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/routingmode"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/queue"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"strconv"
)

var logger logging.Logger

func init() {
	logger = logging.GetLogger("route-service")
}

type ClusterRegistrationService interface {
	SaveCluster(ctx context.Context, dao dao.Repository, clusterToSave domain.Cluster, tlsConfigName string, clusterNodeGroups ...string) error
}

type RegistrationService struct {
	groupQueue         *queue.GroupTaskExecutor
	routeCompFactory   *factory.ComponentsFactory
	entityService      *entity.Service
	dao                dao.Dao
	bus                bus.BusPublisher
	routingModeService *routingmode.Service
}

type RegistrationServiceContext struct {
	srv *RegistrationService
	ctx context.Context
	rep dao.Repository
}

func NewRegistrationService(routeCompFactory *factory.ComponentsFactory, entityService *entity.Service,
	dao dao.Dao, bus bus.BusPublisher, routingModeService *routingmode.Service) *RegistrationService {
	return &RegistrationService{
		groupQueue:         queue.NewGroupTaskExecutor(),
		routeCompFactory:   routeCompFactory,
		entityService:      entityService,
		dao:                dao,
		bus:                bus,
		routingModeService: routingModeService,
	}
}

func (s *RegistrationService) WithContext(ctx context.Context, rep dao.Repository) RegistrationServiceContext {
	return RegistrationServiceContext{
		srv: s,
		ctx: ctx,
		rep: rep,
	}
}

func (s *RegistrationService) RegisterRoutes(ctx context.Context, dao dao.Repository, request registration.ProcessedRequest) error {
	// save NodeGroup
	logger.DebugC(ctx, "Saving NodeGroups...")
	for _, newNodeGroup := range request.NodeGroups {
		if _, err := s.entityService.CreateOrUpdateNodeGroup(dao, newNodeGroup); err != nil {
			logger.Errorf("Failed to load or save nodeGroup %v to in memory storage: %v", newNodeGroup.Name, err)
			return err
		} else {
			logger.DebugC(ctx, "Saved %v", newNodeGroup)
		}
	}

	// save DeploymentVersion
	logger.DebugC(ctx, "Saving DeploymentVersions...")
	for _, version := range request.DeploymentVersions {
		if version != "" {
			if _, err := s.entityService.GetOrCreateDeploymentVersion(dao, version); err != nil {
				logger.Errorf("Failed to load or save deployment version %v: %v", version, err)
				return err
			} else {
				logger.DebugC(ctx, "Saved %s", version)
			}
		} else {
			logger.WarnC(ctx, "Found version with empty name")
		}
	}

	// save Cluster
	logger.DebugC(ctx, "Saving Clusters...")
	for _, newCluster := range request.Clusters {
		clusterToSave := newCluster

		if err := s.SaveCluster(ctx, dao, clusterToSave, request.ClusterTlsConfig[clusterToSave.Name], request.ClusterNodeGroups[clusterToSave.Name]...); err != nil {
			return err
		}
	}

	// save Listener
	logger.DebugC(ctx, "Saving listeners...")
	for _, newListener := range request.Listeners {
		listenerToSave := newListener
		err := s.entityService.PutListener(dao, &listenerToSave)
		if err != nil {
			logger.Errorf("Failed to load or save listener %v to in memory storage: %v", listenerToSave.Name, err)
			return err
		}
		logger.DebugC(ctx, "Saved %s", listenerToSave)
	}

	// save RouteConfig
	logger.DebugC(ctx, "Saving route configurations...")
	for _, newRouteConfig := range request.RouteConfigurations {
		routeConfigToSave := newRouteConfig
		err := s.entityService.PutRouteConfig(dao, &routeConfigToSave)
		if err != nil {
			logger.Errorf("Failed to load or save route configuration %v to in memory storage: %v", routeConfigToSave.Name, err)
			return err
		}
		logger.DebugC(ctx, "Saved %s", routeConfigToSave)
		// save VirtualHost
		logger.DebugC(ctx, "Saving virtual hosts...")
		for _, newVirtualHost := range newRouteConfig.VirtualHosts {
			newVirtualHost.RouteConfigurationId = routeConfigToSave.Id
			err := s.entityService.PutVirtualHost(dao, newVirtualHost)
			if err != nil {
				logger.Errorf("Failed to create or update virtual host %v to in memory storage: %v", newVirtualHost.Name, err)
				return err
			}
			logger.DebugC(ctx, "Saved %s", newVirtualHost)

			// save Routes
			logger.DebugC(ctx, "Saving routes...")
			for _, newRoute := range newVirtualHost.Routes {
				err := s.entityService.PutRoute(dao, newVirtualHost.Id, newRoute)
				if err != nil {
					logger.Errorf("Failed to create or update route %v to in memory storage: %v", newRoute.RouteKey, err)
					return err
				}
				logger.DebugC(ctx, "Saved %+v", newRoute)
			}
		}
	}
	// autogenerate routes
	logger.DebugC(ctx, "Generating prohibit routes...")
	if err := request.GroupedRoutes.ForEachGroup(func(namespace, clusterKey, deploymentVersion string, routes []*domain.Route) error {
		routesAutoGenerator := s.routeCompFactory.GetRoutesAutoGenerator(dao)
		generatedRoutes := routesAutoGenerator.GenerateRoutes(ctx, clusterKey, namespace, deploymentVersion, routes)
		for _, route := range generatedRoutes {
			if err := s.entityService.PutRoute(dao, route.VirtualHostId, route); err != nil {
				logger.Errorf("Failed to put newly generated prohibit route in storage: %v", err)
				return err
			}
			logger.DebugC(ctx, "Saved: %v", route)
		}
		logger.DebugC(ctx, "Generated %d prohibit routes", len(generatedRoutes))
		return nil
	}); err != nil {
		logger.Errorf("Failed to generate prohibit routes: %v", err)
		return err
	}
	return nil
}

func (s *RegistrationService) SaveCluster(ctx context.Context, dao dao.Repository, clusterToSave domain.Cluster, tlsConfigName string, clusterNodeGroups ...string) error {
	// add TLS
	if tlsConfigName != "" {
		clusterToUpdate, err := dao.FindClusterByName(clusterToSave.Name)
		logger.DebugC(ctx, "Cluster has TLS configuration, trying to apply.")
		tlsConfigToApply, err := dao.FindTlsConfigByName(tlsConfigName)
		if err != nil {
			logger.Error("can not find tls config by name=%s: %v", tlsConfigName, err)
			return err
		}
		if tlsConfigToApply != nil {
			clusterToSave.TLSId = tlsConfigToApply.Id
		} else if clusterToUpdate == nil {
			tlsConfigToApply = &domain.TlsConfig{
				Name:    tlsConfigName,
				Enabled: true,
			}
			err := dao.SaveTlsConfig(tlsConfigToApply)
			if err != nil {
				logger.Error("can not save new tls config: %v", err)
				return err
			}
			clusterToSave.TLSId = tlsConfigToApply.Id
		} else {
			clusterToSave.TLSId = clusterToUpdate.TLSId
			logger.DebugC(ctx, "Trying to apply changes for cluster %s.", clusterToUpdate.Name)
		}
		logger.DebugC(ctx, "Applied TLS configuration: %s", tlsConfigToApply)
	}

	err := entity.RebaseCluster(dao, &clusterToSave)
	if err != nil {
		logger.Errorf("Error while rebasing cluster with name %v: %v", clusterToSave.Name, err)
		return err
	}

	if err = s.entityService.UpdateClusterTcpKeepalive(dao, &clusterToSave); err != nil {
		logger.Errorf("Error while actualizing tcp keepalive for cluster with name %v: %v", clusterToSave.Name, err)
		return err
	}

	//Change if add new CircuitBreaker or thresholds
	if clusterToSave.CircuitBreaker != nil && clusterToSave.CircuitBreaker.Threshold != nil && clusterToSave.CircuitBreaker.Threshold.MaxConnections != 0 {
		var circuitBreaker *domain.CircuitBreaker
		if clusterToSave.CircuitBreakerId == 0 {
			circuitBreaker = &domain.CircuitBreaker{}
		} else {
			circuitBreaker, err = dao.FindCircuitBreakerById(clusterToSave.CircuitBreakerId)
			if err != nil {
				logger.Errorf("Error while searching for existing CircuitBreaker with id %v: %v", clusterToSave.CircuitBreakerId, err)
				return err
			}
			if circuitBreaker == nil {
				circuitBreaker = &domain.CircuitBreaker{}
			}
		}

		var threshold *domain.Threshold
		if circuitBreaker.ThresholdId == 0 {
			threshold = &domain.Threshold{}
		} else {
			threshold, err = dao.FindThresholdById(circuitBreaker.ThresholdId)
			if err != nil {
				logger.Errorf("Error while searching for existing Threshold with id %v: %v", circuitBreaker.ThresholdId, err)
				return err
			}
			if threshold == nil {
				threshold = &domain.Threshold{}
			}
		}

		threshold.MaxConnections = clusterToSave.CircuitBreaker.Threshold.MaxConnections

		err = dao.SaveThreshold(threshold)
		if err != nil {
			logger.Errorf("Error while saving Threshold with id %v: %v", threshold.Id, err)
			return err
		}
		circuitBreaker.ThresholdId = threshold.Id

		err = dao.SaveCircuitBreaker(circuitBreaker)
		if err != nil {
			logger.Errorf("Error while saving CircuitBreaker with id %v: %v", circuitBreaker.Id, err)
			return err
		}
		clusterToSave.CircuitBreakerId = circuitBreaker.Id
	} else {
		if clusterToSave.CircuitBreakerId != 0 {
			if err := s.entityService.DeleteCircuitBreakerCascadeById(dao, clusterToSave.CircuitBreakerId); err != nil {
				logger.Errorf("Error during cascade CircuitBreaker deletion: %v", err)
				return err
			}
			clusterToSave.CircuitBreakerId = 0
		}
	}

	err = dao.SaveCluster(&clusterToSave)
	if err != nil {
		logger.Errorf("Error while saving cluster with name %v: %v", clusterToSave.Name, err)
		return err
	} else {
		logger.DebugC(ctx, "Saved %s", clusterToSave)
	}

	// save ClusterNodeGroup
	for _, nodeGroup := range clusterNodeGroups {
		clusterNodeGroup := domain.NewClusterNodeGroups(clusterToSave.Id, nodeGroup)
		err = s.entityService.PutClustersNodeGroupIfAbsent(dao, clusterNodeGroup)
		if err != nil {
			logger.Errorf("Failed to bind cluster %v to node group %v: %v", clusterToSave.Name, nodeGroup, err)
			return err
		}
	}

	// save Endpoint
	logger.DebugC(ctx, "Saving endpoints...")
	for _, newEndpoint := range clusterToSave.Endpoints {
		newEndpoint.ClusterId = clusterToSave.Id
		if err = s.entityService.PutEndpoint(dao, newEndpoint); err != nil {
			logger.Errorf("Failed to load or save endpoint %v to in memory storage: %v", newEndpoint, err)
			return err
		}
		logger.DebugC(ctx, "Saved %s", newEndpoint)
	}
	return nil
}

func Validate(ctx context.Context, repository dao.Repository, request registration.ProcessedRequest) (bool, string, error) {
	isValid, errMsg, serverErr := validateWithDomainProducer(ctx, makeVirtualHostDomainsExtractor(repository, request), request)
	if !isValid || serverErr != nil {
		return isValid, errMsg, serverErr
	}
	return ValidateGatewayDeclarationConflicts(ctx, repository, request)
}

func makeVirtualHostDomainsExtractor(repository dao.Repository, request registration.ProcessedRequest) func() ([]*domain.VirtualHostDomain, error) {
	return func() ([]*domain.VirtualHostDomain, error) {
		domains := make([]*domain.VirtualHostDomain, 0)
		for _, nodeGroup := range request.NodeGroups {
			routeConfigurations, err := repository.FindRouteConfigsByNodeGroupId(nodeGroup.Name)
			if err != nil {
				return nil, err
			}
			for _, routeConfiguration := range routeConfigurations {
				virtualHosts, err := repository.FindVirtualHostsByRouteConfigurationId(routeConfiguration.Id)
				if err != nil {
					return nil, err
				}
				for _, virtualHost := range virtualHosts {
					virtualHostDomains, err := repository.FindVirtualHostDomainByVirtualHostId(virtualHost.Id)
					if err != nil {
						return nil, err
					}
					for _, virtualHostDomain := range virtualHostDomains {
						domains = append(domains, virtualHostDomain)
					}
				}
			}
		}
		return domains, nil
	}
}

func validateWithDomainProducer(ctx context.Context, existsDomainsProducer func() ([]*domain.VirtualHostDomain, error), request registration.ProcessedRequest) (bool, string, error) {
	endpoints := make(map[string]bool)
	domains := make(map[string]bool)
	for _, cluster := range request.Clusters {
		for _, endpoint := range cluster.Endpoints {
			endpoints[endpoint.Address+":"+strconv.FormatInt(int64(endpoint.Port), 10)] = true
		}
	}
	for _, routeConfiguration := range request.RouteConfigurations {
		for _, virtualHost := range routeConfiguration.VirtualHosts {
			for _, virtualHostDomain := range virtualHost.Domains {
				domains[virtualHostDomain.Domain] = true
			}
		}
	}
	for endpoint := range endpoints {
		if _, found := domains[endpoint]; found {
			msg := fmt.Sprintf("Found loop in request data. Virtual host handle requests with Host: %s and has route with destination: %s at the same time", endpoint, endpoint)
			logger.WarnC(ctx, "%s", msg)
			return false, msg, nil
		}
	}
	existsDomains, err := existsDomainsProducer()
	if err != nil {
		return false, "", err
	}
	for _, virtualHostDomain := range existsDomains {
		domains[virtualHostDomain.Domain] = true
	}
	for endpoint := range endpoints {
		if _, found := domains[endpoint]; found {
			msg := fmt.Sprintf("Found loop in configuration data. Virtual host handle requests with Host: %s and has route with destination: %s at the same time", endpoint, endpoint)
			logger.WarnC(ctx, "%s", msg)
			return false, msg, nil
		}
	}
	return true, "", nil
}

func (c RegistrationServiceContext) DeleteRoutesByCondition(vHostId int32, condition func(route domain.Route) bool) ([]*domain.Route, error) {
	routes, err := c.rep.FindRoutesByVirtualHostId(vHostId)
	if err != nil {
		logger.ErrorC(c.ctx, "Finding routes by virtual-host id '%d' caused error: %v", vHostId, err)
		return nil, err
	}
	for _, route := range routes {
		if routeMatchers, err := c.rep.FindHeaderMatcherByRouteId(route.Id); err == nil {
			route.HeaderMatchers = routeMatchers
		} else {
			return nil, err
		}
	}
	routesToDelete := make([]*domain.Route, 0)
	for _, route := range routes {
		if condition(*route) {
			routesToDelete = append(routesToDelete, route)
		}
	}
	logger.DebugC(c.ctx, "Routes list to delete has been made: %v", routesToDelete)
	err = c.srv.entityService.DeleteRoutesByUUID(c.rep, routesToDelete)
	return routesToDelete, err
}

func (c RegistrationServiceContext) DeleteRoutesByRawPrefixNamespaceVersion(vHostId int32, reqNamespace, reqVersion string, rawPrefixes ...string) ([]*domain.Route, error) {
	logger.Debug("Forming route delete condition")
	var version = ""
	if reqVersion == "" {
		dVersion, err := c.srv.entityService.GetActiveDeploymentVersion(c.rep)
		if err != nil {
			return nil, err
		}
		version = dVersion.Version
	} else {
		version = reqVersion
	}
	routeFroms := make([]*format.RouteFromAddress, len(rawPrefixes))
	for i, reqPrefix := range rawPrefixes {
		routeFroms[i] = format.NewRouteFromAddress(reqPrefix)
	}
	ns := &msaddr.Namespace{Namespace: reqNamespace}
	condition := func(route domain.Route) bool {
		logger.Debugf("Routes delete condition: check condition for %v", route)
		if route.DeploymentVersion != version {
			logger.Debugf("Routes delete condition: deployment version not equals, required version: `%s`, route version: `%s`, do not delete", version, route.DeploymentVersion)
			return false
		}
		if len(routeFroms) > 0 {
			found := false
			for _, routeFrom := range routeFroms {
				if route.Prefix != "" {
					found = found || route.Prefix == routeFrom.RouteFromPrefix
				} else {
					found = found || route.Regexp == routeFrom.RouteFromRegex
				}
			}
			if !found {
				return false
			}
		}
		if ns.Namespace != "" && !ns.IsCurrentNamespace() {
			if !IsRouteBelongsToNamespace(&route, ns) {
				logger.Debugf("Routes delete condition: required namespace `%s` not equals to route namespace, do not delete", ns.Namespace)
				return false
			}
		} else {
			if !IsDefaultNamespaceRoute(&route) {
				return false
			}
		}
		logger.Debugf("Routes delete condition: must delete")
		return true
	}
	return c.DeleteRoutesByCondition(vHostId, condition)
}

func (c RegistrationServiceContext) DeleteRoutes(nodeGroup, reqNamespace, reqVersion string, rawPrefixes ...string) ([]*domain.Route, error) {
	routeConfigs, err := c.rep.FindRouteConfigsByNodeGroupId(nodeGroup)
	if err != nil {
		return nil, err
	}
	logger.Debugf("Route Configs for nodeGroup %s: %+v", nodeGroup, routeConfigs)
	delRoutesBunch := make([]*domain.Route, 0)
	for _, routeConfig := range routeConfigs {
		vHosts, err := c.rep.FindVirtualHostsByRouteConfigurationId(routeConfig.Id)
		if err != nil {
			return nil, err
		}
		logger.Debugf("v hosts for route config with id %d: %+v", routeConfig.Id, vHosts)
		for _, vHost := range vHosts {
			deletedRoutes, err := c.DeleteRoutesByRawPrefixNamespaceVersion(vHost.Id, reqNamespace, reqVersion, rawPrefixes...)
			if err != nil {
				return nil, err
			}
			delRoutesBunch = append(delRoutesBunch, deletedRoutes...)
		}
	}
	return delRoutesBunch, nil
}

func (c RegistrationServiceContext) DeleteDomains(vHostId int32, domains ...string) ([]*domain.VirtualHostDomain, error) {
	vHostDomains, err := c.rep.FindVirtualHostDomainByVirtualHostId(vHostId)
	if err != nil {
		return nil, err
	}
	delDomainsBunch := make([]*domain.VirtualHostDomain, 0)
	for _, rawDomain := range domains {
		for _, vHostDomain := range vHostDomains {
			if vHostDomain.Domain == rawDomain {
				delDomainsBunch = append(delDomainsBunch, vHostDomain)
			}
		}
	}
	err = c.srv.entityService.DeleteVirtualHostDomains(c.rep, delDomainsBunch)
	return delDomainsBunch, err
}

func ValidateGatewayDeclarationConflicts(ctx context.Context, repo dao.Repository, request registration.ProcessedRequest) (bool, string, error) {
	for _, nodeGroup := range request.NodeGroups {
		gatewayDeclaration, err := repo.FindNodeGroupByName(nodeGroup.Name)
		if err != nil {
			logger.ErrorC(ctx, "Routes registration request validation failed to load node group using DAO:\n %v", err)
			return false, "", err
		}

		if gatewayDeclaration == nil || (!gatewayDeclaration.ForbidVirtualHosts && gatewayDeclaration.GatewayType != domain.Ingress) {
			return true, "", nil
		}

		for _, routeConfig := range request.RouteConfigurations {
			for _, virtualHost := range routeConfig.VirtualHosts {

				if gatewayDeclaration.ForbidVirtualHosts {
					if len(virtualHost.Domains) > 1 || (len(virtualHost.Domains) == 1 && !virtualHost.HasGenericDomain()) {
						return false, fmt.Sprintf("invalid virtual service '%s' hosts: declaration of gateway %s forbids virtual hosts registration", virtualHost.Name, nodeGroup.Name), nil
					}
				}

				if gatewayDeclaration.GatewayType == domain.Ingress {
					for _, route := range virtualHost.Routes {
						if route.DirectResponseCode != 0 {
							continue
						}

						for _, cluster := range request.Clusters {
							if cluster.Name == route.ClusterName {
								for _, endpoint := range cluster.Endpoints {
									if endpoint.Address != domain.PublicGateway && endpoint.Address != domain.PrivateGateway {
										return false, fmt.Sprintf("invalid route destination in virtualService %s: gateway %s type is ingress, all routes should lead to public or private gateway", virtualHost.Name, nodeGroup.Name), nil
									}
								}
								break
							}
						}
					}
				}
			}
		}
	}
	return true, "", nil
}
