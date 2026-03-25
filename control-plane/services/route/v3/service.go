package v3

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/events"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/envoy"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/business/format"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/creator"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/registration"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/routingmode"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"strings"
)

var logger logging.Logger

func init() {
	logger = logging.GetLogger("route-service/v3")
}

type Service struct {
	dao                dao.Dao
	bus                bus.BusPublisher
	routingModeService *routingmode.Service
	regService         *route.RegistrationService
	entityService      *entity.Service
	v3RequestProcessor registration.V3RequestProcessor
}

func NewV3Service(dao dao.Dao, bus bus.BusPublisher, routingModeService *routingmode.Service, regService *route.RegistrationService, entityService *entity.Service, v3RequestProcessor registration.V3RequestProcessor) *Service {
	return &Service{
		dao:                dao,
		bus:                bus,
		routingModeService: routingModeService,
		regService:         regService,
		entityService:      entityService,
		v3RequestProcessor: v3RequestProcessor,
	}
}

func (s *Service) RegisterRoutingConfig(ctx context.Context, regRequest dto.RoutingConfigRequestV3) error {
	logger.InfoC(ctx, "Registering routes configuration for namespace %s and gateways %v", regRequest.Namespace, regRequest.Gateways)
	logger.DebugC(ctx, "Request: %+v", regRequest)
	var activeVersion *domain.DeploymentVersion
	activeVersionRaw, _, err := s.dao.WithWTxVal(func(dao dao.Repository) (interface{}, error) {
		return s.entityService.GetActiveDeploymentVersion(dao)
	})
	if err != nil {
		logger.ErrorC(ctx, "Failed to get active deployment version: %v", err)
		return err
	}
	activeVersion = activeVersionRaw.(*domain.DeploymentVersion)
	logger.InfoC(ctx, "Active version defined as '%s'", activeVersion.Version)
	var processedRequests []registration.ProcessedRequest
	processedRequests, err = s.v3RequestProcessor.ProcessRequestV3(ctx, regRequest, activeVersion.Version)
	if err != nil {
		return err
	}
	changesResultMap := make(map[string][]memdb.Change)
	logger.DebugC(ctx, "Registering cast structures.")
	for _, processedRequest := range processedRequests {
		nodeGroupName := processedRequest.NodeGroups[0].Name
		changes, err := s.dao.WithWTx(func(dao dao.Repository) error {

			if valid, msg, err := route.Validate(ctx, dao, processedRequest); !valid {
				if err != nil {
					return errors.Wrap(err, 0)
				}
				return errors.WrapPrefix(services.BadRouteRegistrationRequest, msg, 0)
			}

			logger.InfoC(ctx, "Registering cast structures for '%s' node-group", nodeGroupName)
			logger.DebugC(ctx, "Processed request %v", processedRequest)
			if err := s.regService.RegisterRoutes(ctx, dao, processedRequest); err != nil {
				logger.ErrorC(ctx, "Failed to save processed routes to in-memory storage: %v", err)
				return err
			}

			if err := envoy.UpdateAllResourceVersions(dao, nodeGroupName); err != nil {
				logger.ErrorC(ctx, "Failed to update envoy resource versions: %v", err)
				return err
			}
			return nil
		})

		if err != nil {
			logger.ErrorC(ctx, "Error in route registration transaction for request %+v: \n %v", regRequest, err)
			return err
		}
		changesResultMap[nodeGroupName] = append(changesResultMap[nodeGroupName], changes...)
	}

	logger.DebugC(ctx, "Making versions map to update routing mode")
	versionsMap := make(map[string]bool)
	for _, vs := range regRequest.VirtualServices {
		version := vs.RouteConfiguration.Version
		if version != "" {
			version = activeVersion.Version
		}
		versionsMap[version] = true
	}
	logger.DebugC(ctx, "Versions map: %v", versionsMap)
	namespace := msaddr.NewNamespace(regRequest.Namespace)
	for version, _ := range versionsMap {
		s.routingModeService.UpdateRoutingMode(version, namespace)
	}

	for gateway, changes := range changesResultMap {
		logger.InfoC(ctx, "Publishing changes for %s", gateway)
		event := events.NewChangeEventByNodeGroup(gateway, changes)
		err = s.bus.Publish(bus.TopicChanges, event)
		if err != nil {
			logger.ErrorC(ctx, "Can't publish changes to eventBus for node group %s: \n %v", gateway, err)
			return err
		}
	}
	logger.InfoC(ctx, "Registering routes configuration has been finished successfully")
	return nil
}

func (s *Service) GetVirtualService(nodeGroup, virtualServiceName string) (dto.VirtualServiceResponse, error) {
	virtualService, err := s.dao.WithRTxVal(func(dao dao.Repository) (interface{}, error) {
		virtualHost, err := s.entityService.LoadVirtualHostRelationByNameAndNodeGroup(dao, nodeGroup, virtualServiceName)
		if err != nil {
			return nil, err
		}
		clustersMap := make(map[string]bool)
		for _, route := range virtualHost.Routes {
			clustersMap[route.ClusterName] = true
		}
		var clusters []*domain.Cluster
		for clusterName, _ := range clustersMap {
			cluster, err := s.entityService.GetClusterWithRelations(dao, clusterName)
			if err != nil {
				return nil, err
			}
			clusters = append(clusters, cluster)
		}
		dtoVirtualService := dto.VirtualServiceResponse{}
		dtoVirtualService.Clusters = dto.DefaultResponseConverter.ConvertClustersToResponse(clusters)
		dtoVirtualService.VirtualHost = dto.DefaultResponseConverter.ConvertVirtualHost(virtualHost)
		return dtoVirtualService, nil
	})
	if err != nil {
		logger.Errorf("Can't get virtual service %s for node group %s", virtualService, nodeGroup)
		return dto.VirtualServiceResponse{}, err
	}
	return virtualService.(dto.VirtualServiceResponse), nil
}

func (s *Service) DeleteVirtualService(ctx context.Context, nodeGroup, virtualService string) error {
	changes, err := s.dao.WithWTx(func(dao dao.Repository) error {
		err := s.entityService.DeleteVirtualServiceByNodeGroupAndName(dao, nodeGroup, virtualService)
		if err != nil {
			logger.ErrorC(ctx, "Failed to delete virtual service %s for node group %s with error: %v", virtualService, nodeGroup, err)
			return err
		}
		if err := envoy.UpdateAllResourceVersions(dao, nodeGroup); err != nil {
			logger.ErrorC(ctx, "Failed to update envoy resource versions: %v", err)
			return err
		}
		return nil
	})

	if err != nil {
		logger.ErrorC(ctx, "Error deleting virtual service %s for node group %s: %+v", virtualService, nodeGroup, err)
		return err
	}

	logger.InfoC(ctx, "Publish changes for %s", nodeGroup)
	event := events.NewChangeEventByNodeGroup(nodeGroup, changes)
	err = s.bus.Publish(bus.TopicChanges, event)
	if err != nil {
		logger.ErrorC(ctx, "Can't publish changes to eventBus for node group %s: \n %v", nodeGroup, err)
		return err
	}
	return nil
}

func (s *Service) UpdateVirtualService(ctx context.Context, nodeGroup, virtualServiceName string, virtualService dto.VirtualService) error {
	logger.InfoC(ctx, "Updating VirtualHost %s for NodeGroup %s with %v", virtualServiceName, nodeGroup, virtualService)
	changes, err := s.dao.WithWTx(func(dao dao.Repository) error {
		virtualHost, err := s.entityService.LoadVirtualHostRelationByNameAndNodeGroup(dao, nodeGroup, virtualServiceName)
		if err != nil {
			logger.ErrorC(ctx, "Failed to find virtual service %s for node group %s with error: %v", virtualServiceName, nodeGroup, err)
			return err
		}

		virtualHost.RequestHeadersToRemove = util.MergeStringSlices(virtualHost.RequestHeadersToRemove, virtualService.RemoveHeaders)
		virtualHost.RequestHeadersToAdd = util.MergeHeaderSlices(virtualHost.RequestHeadersToAdd, s.v3RequestProcessor.ConvertRequestHeadersToDomain(virtualService.AddHeaders))
		virtualHost.Domains = util.MergeVirtualHostDomainsSlices(virtualHost.Domains, s.v3RequestProcessor.GenerateVirtualHostDomainsWithVirtualHostId(virtualService.Hosts, virtualHost.Id))

		gatewayDeclaration, err := dao.FindNodeGroupByName(nodeGroup)
		if err != nil {
			logger.ErrorC(ctx, "UpdateVirtualService failed to find node group %s using DAO with error:\n %v", nodeGroup, err)
			return err
		}

		if gatewayDeclaration.ForbidVirtualHosts {
			if len(virtualHost.Domains) > 1 || (len(virtualHost.Domains) == 1 && !virtualHost.HasGenericDomain()) {
				return errors.New(fmt.Sprintf("gateway '%s' declaration forbids to register virtual service hosts not starting with *", nodeGroup))
			}
		}

		deploymentVersion := virtualService.RouteConfiguration.Version
		if deploymentVersion == "" {
			activeDeploymentVersion, err := s.entityService.GetActiveDeploymentVersion(dao)
			if err != nil {
				logger.ErrorC(ctx, "Failed to get active deployment version: %v", err)
				return err
			}
			deploymentVersion = activeDeploymentVersion.Version
		}

		for _, v3Route := range virtualService.RouteConfiguration.Routes {
			for _, v3RouteRule := range v3Route.Rules {
				err := s.processRoutes(dao, virtualHost.Routes, nodeGroup, deploymentVersion, v3RouteRule)
				if err != nil {
					logger.ErrorC(ctx, "Failed to process v3 route rule %v for virtual service %s with node group %s with error: %v", v3RouteRule, virtualServiceName, nodeGroup, err)
					return nil
				}
			}
		}
		err = s.entityService.PutVirtualHost(dao, virtualHost)
		if err != nil {
			logger.ErrorC(ctx, "Failed to update virtual service %s for node group %s with error: %v", virtualServiceName, nodeGroup, err)
			return err
		}
		if err := envoy.UpdateAllResourceVersions(dao, nodeGroup); err != nil {
			logger.ErrorC(ctx, "Failed to update envoy resource versions: %v", err)
			return err
		}
		return nil
	})

	if err != nil {
		logger.ErrorC(ctx, "Error updating virtual service %s for node group %s: %+v", virtualServiceName, nodeGroup, err)
		return err
	}

	logger.InfoC(ctx, "Publish changes for %s", nodeGroup)
	event := events.NewChangeEventByNodeGroup(nodeGroup, changes)
	err = s.bus.Publish(bus.TopicChanges, event)
	if err != nil {
		logger.ErrorC(ctx, "Can't publish changes to eventBus for node group %s: \n %v", nodeGroup, err)
		return err
	}
	return nil
}

func (s *Service) CreateVirtualService(ctx context.Context, nodeGroup string, virtualServiceReq dto.VirtualService) error {
	var activeVersion *domain.DeploymentVersion
	activeVersionRaw, _, err := s.dao.WithWTxVal(func(dao dao.Repository) (interface{}, error) {
		return s.entityService.GetActiveDeploymentVersion(dao)
	})
	if err != nil {
		logger.ErrorC(ctx, "Failed to get active deployment version: %v", err)
		return err
	}
	activeVersion = activeVersionRaw.(*domain.DeploymentVersion)
	processedRequest, err := s.v3RequestProcessor.ProcessVirtualServiceRequestV3(virtualServiceReq, nodeGroup, activeVersion.Version)
	if err != nil {
		return err
	}
	changes, err := s.dao.WithWTx(func(dao dao.Repository) error {
		logger.InfoC(ctx, "Creating routes %v", processedRequest)

		isValid, errMsg, serverErr := route.ValidateGatewayDeclarationConflicts(ctx, dao, processedRequest)
		if serverErr != nil {
			logger.ErrorC(ctx, "CreateVirtualService failed with internal error:\n %v", serverErr)
			return serverErr
		}
		if !isValid {
			return errors.New("v3: create virtual service request validation failed: " + errMsg)
		}

		if err := s.regService.RegisterRoutes(ctx, dao, processedRequest); err != nil {
			logger.ErrorC(ctx, "Failed to save processed routes to in-memory storage: %v", err)
			return err
		}

		if err := envoy.UpdateAllResourceVersions(dao, nodeGroup); err != nil {
			logger.ErrorC(ctx, "Failed to update envoy resource versions: %v", err)
			return err
		}
		return nil
	})

	if err != nil {
		logger.ErrorC(ctx, "Error in transaction for virtual service creation %+v: \n %v", virtualServiceReq, err)
		return err
	}

	namespace := msaddr.NewNamespace("")
	version := virtualServiceReq.RouteConfiguration.Version
	if version == "" {
		version = activeVersion.Version
	}
	s.routingModeService.UpdateRoutingMode(version, namespace)

	logger.InfoC(ctx, "Publish changes for %s", nodeGroup)
	event := events.NewChangeEventByNodeGroup(nodeGroup, changes)
	err = s.bus.Publish(bus.TopicChanges, event)
	if err != nil {
		logger.ErrorC(ctx, "Can't publish changes to eventBus for node group %s: \n %v", nodeGroup, err)
		return err
	}
	return nil
}

func (s *Service) DeleteVirtualServiceRoutes(ctx context.Context, rawPrefixes []string, nodeGroup, virtualService, namespace, version string) ([]*domain.Route, error) {
	deletedRoutes, changes, err := s.dao.WithWTxVal(func(storage dao.Repository) (interface{}, error) {
		virtualHost, err := s.entityService.FindVirtualHostByNameAndNodeGroup(storage, nodeGroup, virtualService)
		if err != nil {
			logger.Errorf("Failed to find virtual host %s by node group %s: %v", virtualService, nodeGroup, err)
			return nil, err
		}
		regSrvCtx := s.regService.WithContext(ctx, storage)
		deletedRoutes, err := regSrvCtx.DeleteRoutesByRawPrefixNamespaceVersion(virtualHost.Id, namespace, version, rawPrefixes...)
		err = envoy.UpdateAllResourceVersions(storage, nodeGroup)
		if err != nil {
			logger.Infof("Failed to update all resources: %v", err)
			return nil, err
		}

		return deletedRoutes, nil
	})

	if err != nil {
		logger.Errorf("Failed to delete routes for virtual service %s and node group %s: %v", virtualService, nodeGroup, err)
		return nil, err
	}

	if deletedRoutes == nil || len(deletedRoutes.([]*domain.Route)) == 0 {
		return nil, nil
	}

	s.routingModeService.UpdateRouteModeDetails()
	event := events.NewChangeEventByNodeGroup(nodeGroup, changes)
	err = s.bus.Publish(bus.TopicChanges, event)
	if err != nil {
		logger.Errorf("Failed to publish changes to bus: %v", err)
		return nil, err
	}
	return deletedRoutes.([]*domain.Route), nil
}

func (s *Service) DeleteVirtualServiceDomains(ctx context.Context, domains []string, nodeGroup, virtualService string) ([]*domain.VirtualHostDomain, error) {
	deletedDomains, changes, err := s.dao.WithWTxVal(func(storage dao.Repository) (interface{}, error) {
		virtualHost, err := s.entityService.FindVirtualHostByNameAndNodeGroup(storage, nodeGroup, virtualService)
		if err != nil {
			logger.Errorf("Failed to find virtual host %s by node group %s: %v", virtualService, nodeGroup, err)
			return nil, err
		}
		regSrvCtx := s.regService.WithContext(ctx, storage)
		deletedDomains, err := regSrvCtx.DeleteDomains(virtualHost.Id, domains...)
		err = storage.SaveEnvoyConfigVersion(domain.NewEnvoyConfigVersion(nodeGroup, domain.RouteConfigurationTable))
		if err != nil {
			logger.Infof("Domains deletion failed due to error in envoy config version saving for clusters: %v", err)
			return nil, err
		}

		return deletedDomains, nil
	})

	if err != nil {
		logger.Errorf("Failed to delete domains for virtual service %s and node group %s: %v", virtualService, nodeGroup, err)
		return nil, err
	}

	if deletedDomains == nil || len(deletedDomains.([]*domain.VirtualHostDomain)) == 0 {
		return nil, nil
	}

	event := events.NewChangeEventByNodeGroup(nodeGroup, changes)
	err = s.bus.Publish(bus.TopicChanges, event)
	if err != nil {
		logger.Errorf("Failed to publish changes to bus: %v", err)
		return nil, err
	}
	return deletedDomains.([]*domain.VirtualHostDomain), nil
}

func (s *Service) DeleteRoutes(ctx context.Context, requests []dto.RouteDeleteRequestV3) ([]*domain.Route, error) {
	deletedRoutes := make([]*domain.Route, 0)
	for _, routeDeletionReq := range requests {
		routesToDelete := make([]string, len(routeDeletionReq.Routes))
		for i, r := range routeDeletionReq.Routes {
			routesToDelete[i] = r.Prefix
		}
		for _, nodeGroup := range routeDeletionReq.Gateways {
			logger.InfoC(ctx, "Deleting route for node group %v", nodeGroup)
			routes, err := s.DeleteVirtualServiceRoutes(ctx, routesToDelete, nodeGroup, routeDeletionReq.VirtualService, routeDeletionReq.Namespace, routeDeletionReq.Version)
			if err != nil {
				return nil, err
			}
			if routes != nil && len(routes) > 0 {
				deletedRoutes = append(deletedRoutes, routes...)
			}
		}
	}
	return deletedRoutes, nil
}

func (s *Service) DeleteDomains(ctx context.Context, requests []dto.DomainDeleteRequestV3) ([]*domain.VirtualHostDomain, error) {
	deletedDomains := make([]*domain.VirtualHostDomain, 0)
	for _, domainDeletionReq := range requests {
		logger.InfoC(ctx, "Deleting domains for virtual service %s for node group %s", domainDeletionReq.VirtualService, domainDeletionReq.Gateway)
		domains, err := s.DeleteVirtualServiceDomains(ctx, domainDeletionReq.Domains, domainDeletionReq.Gateway, domainDeletionReq.VirtualService)
		if err != nil {
			return nil, err
		}
		if domains != nil && len(domains) > 0 {
			deletedDomains = append(deletedDomains, domains...)
		}
	}
	return deletedDomains, nil
}

func (s *Service) processRoutes(dao dao.Repository, routes []*domain.Route, nodeGroup, dVersion string, v3RouteRule dto.Rule) error {
	routeEntry := creator.NewRouteEntry(
		v3RouteRule.Match.Prefix,
		v3RouteRule.PrefixRewrite,
		"",
		0,
		0,
		[]*domain.HeaderMatcher{})
	routeFromAddr := format.NewRouteFromAddress(routeEntry.GetFrom())
	routeToFind := &domain.Route{
		Prefix: routeFromAddr.RouteFromPrefix,
		Regexp: routeFromAddr.RouteFromRegex,
	}
	routeEntry.ConfigureAllowedRoute(routeToFind)
	for _, route := range routes {
		if route.DeploymentVersion == dVersion && route.Prefix == routeToFind.Prefix && route.PrefixRewrite == routeToFind.PrefixRewrite &&
			route.Regexp == routeToFind.Regexp && route.RegexpRewrite == routeToFind.RegexpRewrite {
			return s.updateRoute(dao, nodeGroup, route, v3RouteRule)
		}
	}
	return nil
}

func (s *Service) updateRoute(dao dao.Repository, nodeGroup string, route *domain.Route, v3RouteRule dto.Rule) error {
	if v3RouteRule.Timeout != nil {
		route.Timeout = domain.NewNullInt(*v3RouteRule.Timeout)
	}
	if v3RouteRule.IdleTimeout != nil {
		route.IdleTimeout = domain.NewNullInt(*v3RouteRule.IdleTimeout)
	}
	route.RequestHeadersToAdd = util.MergeHeaderSlices(route.RequestHeadersToAdd, s.v3RequestProcessor.ConvertRequestHeadersToDomain(v3RouteRule.AddHeaders))
	route.RequestHeadersToRemove = util.MergeStringSlices(route.RequestHeadersToRemove, v3RouteRule.RemoveHeaders)
	route.HeaderMatchers = util.MergeHeaderMatchersSlices(route.HeaderMatchers, dto.HeaderMatchersToDomain(v3RouteRule.Match.HeaderMatchers))
	if v3RouteRule.StatefulSession != nil {
		route.StatefulSession = v3RouteRule.StatefulSession.ToRouteStatefulSession(nodeGroup)
	}
	return s.entityService.PutRoute(dao, route.VirtualHostId, route)
}

func (s *Service) DeleteEndpoints(ctx context.Context, endpointsToDelete []domain.Endpoint, version string) ([]*domain.Endpoint, error) {
	nodeGroups := map[string]bool{}
	deletedEndpoints, changes, err := s.dao.WithWTxVal(func(storage dao.Repository) (interface{}, error) {
		var dVersion *domain.DeploymentVersion
		if version == "" {
			var err error
			dVersion, err = s.entityService.GetActiveDeploymentVersion(storage)
			if err != nil {
				return nil, err
			}
		} else {
			var err error
			dVersion, err = storage.FindDeploymentVersion(version)
			if err != nil {
				return nil, err
			}
		}

		// TODO this must be checked in validation phase
		if dVersion == nil {
			return nil, fmt.Errorf("version '%s' is not exist", version)
		}

		var foundEndpoints []*domain.Endpoint

		if len(endpointsToDelete) == 0 {
			endpoints, _ := storage.FindEndpointsByDeploymentVersion(dVersion.Version)
			if endpoints != nil {
				foundEndpoints = append(foundEndpoints, endpoints...)
			}
		} else {
			for _, e := range endpointsToDelete {
				endpoints, _ := storage.FindEndpointsByAddressAndPortAndDeploymentVersion(e.Address, e.Port, dVersion.Version)
				if len(endpoints) != 0 {
					foundEndpoints = append(foundEndpoints, endpoints...)
				}
			}
		}

		if len(foundEndpoints) == 0 {
			return nil, nil
		}

		err := s.entityService.DeleteEndpointsCascade(storage, foundEndpoints)
		if err != nil {
			logger.ErrorC(ctx, "Can't delete endpoints %v", err)
			return nil, err
		}
		for _, endpoint := range foundEndpoints {

			clusterNodeGroups := []*domain.NodeGroup{}
			clusters, _ := storage.FindClusterByEndpointIn([]*domain.Endpoint{endpoint})

			for _, cluster := range clusters {
				foundNodeGroups, _ := storage.FindNodeGroupsByCluster(cluster)
				clusterNodeGroups = append(clusterNodeGroups, foundNodeGroups...)
			}

			for _, nodeGroup := range clusterNodeGroups {
				nodeGroups[nodeGroup.Name] = true
			}
		}

		for nodeGroupName, _ := range nodeGroups {
			if err := storage.SaveEnvoyConfigVersion(domain.NewEnvoyConfigVersion(nodeGroupName, domain.ClusterTable)); err != nil {
				logger.ErrorC(ctx, "Endpoint deletion failed due to error in envoy config version saving for clusters: %v", err)
				return nil, err
			}
		}

		return foundEndpoints, nil
	})

	if err != nil {
		return nil, err
	}

	if deletedEndpoints == nil || len(deletedEndpoints.([]*domain.Endpoint)) == 0 {
		return nil, nil
	}

	for nodeGroupName, _ := range nodeGroups {
		event := events.NewChangeEventByNodeGroup(nodeGroupName, changes)
		err = s.bus.Publish(bus.TopicChanges, event)
		if err != nil {
			return nil, err
		}
	}
	return deletedEndpoints.([]*domain.Endpoint), nil
}

func (s *Service) DeleteRouteByUUID(ctx context.Context, routeUUID string) (*domain.Route, error) {
	deletedRoute, changes, err := s.dao.WithWTxVal(func(storage dao.Repository) (interface{}, error) {
		logger.InfoC(ctx, "request to delete route by uuid=%s", routeUUID)
		routesToDelete, err := storage.FindRoutesByUUIDPrefix(routeUUID)
		if err != nil {
			logger.Errorf("Can not delete route by UUID=%s, %v", routeUUID, err)
			return nil, err
		}
		if routesToDelete == nil || len(routesToDelete) != 1 {
			err = &services.RouteUUIDMatchError{Err: fmt.Errorf("route does not exist or more than one route matches uuid: %s", routeUUID)}
			logger.Errorf("%s", err.Error())
			return nil, err
		}
		routeToDelete := routesToDelete[0]
		if err := s.entityService.DeleteRouteByUUID(storage, routeToDelete); err != nil {
			logger.Errorf("Can not delete route by UUID=%s, %v", routeUUID, err)
			return nil, err
		}
		nodeGroup, err := getNodeGroupIdFromVirtualHostId(routeToDelete.VirtualHostId, storage)
		if err != nil {
			return nil, err
		}

		err = envoy.UpdateAllResourceVersions(storage, nodeGroup)
		if err != nil {
			return nil, err
		}
		return routeToDelete, nil
	})
	if err != nil {
		return nil, err
	}

	s.routingModeService.UpdateRouteModeDetails()
	nodeGroup, err := getNodeGroupIdFromVirtualHostId(deletedRoute.(*domain.Route).VirtualHostId, s.dao)
	event := events.NewChangeEventByNodeGroup(nodeGroup, changes)
	err = s.bus.Publish(bus.TopicChanges, event)
	if err != nil {
		return nil, err
	}
	return deletedRoute.(*domain.Route), nil
}
func getNodeGroupIdFromVirtualHostId(virtualHostId int32, storage dao.Repository) (string, error) {
	vhost, err := storage.FindVirtualHostById(virtualHostId)
	if err != nil {
		return "", err
	}

	rConfig, err := storage.FindRouteConfigById(vhost.RouteConfigurationId)
	if err != nil {
		return "", err
	}

	return rConfig.NodeGroupId, nil
}

func (s *Service) GetAllDeploymentVersionsAliases() (map[string]string, error) {
	return getAllDeploymentVersionsAliases(s.dao.FindAllDeploymentVersions)
}

func (s *Service) GetVersionAliases() string {
	rawAliases, err := s.GetAllDeploymentVersionsAliases()
	if err != nil {
		logger.Errorf("Failed to get all DeploymentVersions aliases")
		return ""
	}
	jsonAliases, err := json.Marshal(rawAliases)
	if err != nil {
		logger.Errorf("Failed to marshal map of aliases %v. Reason %s", rawAliases, err)
		return ""
	}
	return string(jsonAliases)
}

func getAllDeploymentVersionsAliases(dVersionProvider func() ([]*domain.DeploymentVersion, error)) (map[string]string, error) {
	vAliases := make(map[string]string)
	dVersions, err := dVersionProvider()
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	for _, dVersion := range dVersions {
		stage := strings.ToLower(dVersion.Stage)
		if value, ok := vAliases[stage]; ok {
			savedNum, err := util.GetVersionNumber(value)
			if err != nil {
				return nil, errors.Wrap(err, 0)
			}
			newNum, err := util.GetVersionNumber(dVersion.Version)
			if err != nil {
				return nil, errors.Wrap(err, 0)
			}
			if newNum > savedNum {
				vAliases[stage] = dVersion.Version
			}
		} else {
			vAliases[stage] = dVersion.Version
		}
	}
	return vAliases, nil
}
