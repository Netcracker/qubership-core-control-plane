package v2

import (
	"context"
	"fmt"
	gerrors "github.com/go-errors/errors"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/events"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	v2 "github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/v2"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/configresources"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/envoy"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/factory"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/registration"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/routingmode"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/queue"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"reflect"
)

var logger logging.Logger

func init() {
	logger = logging.GetLogger("route-service/v2")
}

type Service struct {
	groupQueue         *queue.GroupTaskExecutor
	routeCompFactory   *factory.ComponentsFactory
	entityService      *entity.Service
	dao                dao.Dao
	bus                bus.BusPublisher
	routingModeService *routingmode.Service
	regService         *route.RegistrationService
	v2RequestProcessor registration.V2RequestProcessor
}

func NewV2Service(routeCompFactory *factory.ComponentsFactory, entityService *entity.Service, dao dao.Dao,
	bus bus.BusPublisher, routingModeService *routingmode.Service, regService *route.RegistrationService) *Service {
	return &Service{
		groupQueue:         queue.NewGroupTaskExecutor(),
		routeCompFactory:   routeCompFactory,
		entityService:      entityService,
		dao:                dao,
		bus:                bus,
		routingModeService: routingModeService,
		regService:         regService,
		v2RequestProcessor: registration.NewV2RequestProcessor(dao),
	}
}

func (s *Service) RegisterRoutes(ctx context.Context, requests []dto.RouteRegistrationRequest, nodeGroup string) error {
	logger.InfoC(ctx, "Creating/Updating routes for NodeGroup '%s' has been started.", nodeGroup)
	changes, err := s.dao.WithWTx(func(dao dao.Repository) error {
		activeVersion, err := s.entityService.GetActiveDeploymentVersion(dao)
		if err != nil {
			logger.ErrorC(ctx, "Failed to get active deployment version: %v", err)
			return err
		}
		processedRequest, err := s.v2RequestProcessor.ProcessRequestV2(ctx, nodeGroup, requests, activeVersion.Version)
		if err != nil {
			return err
		}

		if valid, msg, err := route.Validate(ctx, dao, processedRequest); !valid {
			if err != nil {
				return gerrors.Wrap(err, 0)
			}
			return gerrors.WrapPrefix(services.BadRouteRegistrationRequest, msg, 0)
		}
		if err := s.regService.RegisterRoutes(ctx, dao, processedRequest); err != nil {
			logger.ErrorC(ctx, "Failed to save processed routes to in-memory storage: %v", err)
			return err
		}

		for _, request := range requests {
			s.routingModeService.UpdateRoutingMode(request.Version, msaddr.NewNamespace(request.Namespace))
		}

		if err := envoy.UpdateAllResourceVersions(dao, nodeGroup); err != nil {
			logger.ErrorC(ctx, "Failed to update envoy resource versions: %v", err)
			return err
		}
		return nil
	})

	if err != nil {
		logger.ErrorC(ctx, "Error in route registration transaction for node group %s and request %+v: \n %v", nodeGroup, requests, err)
		return err
	}

	logger.InfoC(ctx, "Publishing changes for NodeGroup '%s'", nodeGroup)
	event := events.NewChangeEventByNodeGroup(nodeGroup, changes)
	err = s.bus.Publish(bus.TopicChanges, event)
	if err != nil {
		logger.ErrorC(ctx, "Couldn't publish changes to eventBus for NodeGroup '%s': %v", nodeGroup, err)
		return err
	}
	logger.InfoC(ctx, "Creating/Updating routes for NodeGroup '%s' has been finished successfully.", nodeGroup)
	return nil
}

func (s *Service) DeleteRoutes(ctx context.Context, nodeGroup, namespace, version string, prefixes ...string) ([]*domain.Route, error) {
	logger.DebugC(ctx, "start DeleteRoutes nodeGroup=%s, namespace=%s, version=%s, prefixes=%s", nodeGroup, namespace, version, prefixes)
	deletedRoutes, changes, err := s.dao.WithWTxVal(func(storage dao.Repository) (interface{}, error) {
		regSrvCtx := s.regService.WithContext(ctx, storage)
		deletedRoutes, err := regSrvCtx.DeleteRoutes(nodeGroup, namespace, version, prefixes...)
		err = envoy.UpdateAllResourceVersions(storage, nodeGroup)
		if err != nil {
			return nil, err
		}

		return deletedRoutes, nil
	})

	logger.DebugC(ctx, "deletedRoutes %v, changes %v", deletedRoutes, changes)

	if err != nil {
		return nil, err
	}

	if deletedRoutes == nil || len(deletedRoutes.([]*domain.Route)) == 0 {
		return nil, nil
	}

	s.routingModeService.UpdateRouteModeDetails()
	event := events.NewChangeEventByNodeGroup(nodeGroup, changes)
	err = s.bus.Publish(bus.TopicChanges, event)
	if err != nil {
		return nil, err
	}
	return deletedRoutes.([]*domain.Route), nil
}

func (s *Service) DeleteRouteByUUID(ctx context.Context, routeUUID string) (*domain.Route, error) {
	deletedRoute, changes, err := s.dao.WithWTxVal(func(storage dao.Repository) (interface{}, error) {

		routesToDelete, err := storage.FindRoutesByUUIDPrefix(routeUUID)
		if err != nil {
			logger.ErrorC(ctx, "Can not delete route by UUID=%s, %v", routeUUID, err)
			return nil, err
		}
		if routesToDelete == nil || len(routesToDelete) != 1 {
			err = &services.RouteUUIDMatchError{Err: fmt.Errorf("route does not exist or more than one route matches uuid: %s", routeUUID)}
			logger.ErrorC(ctx, "%s", err.Error())
			return nil, err
		}
		routeToDelete := routesToDelete[0]
		if err := s.entityService.DeleteRouteByUUID(storage, routeToDelete); err != nil {
			logger.ErrorC(ctx, "Can not delete route by UUID=%s, %v", routeUUID, err)
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

func (s *Service) GetNodeGroups() ([]*domain.NodeGroup, error) {
	return s.dao.FindAllNodeGroups()
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

func (s *Service) GetRegisterRoutesResource() configresources.Resource {
	return routingRequestResource{
		service:   s,
		validator: dto.RoutingV2RequestValidator{},
	}
}

type routingRequestResource struct {
	service   *Service
	validator v2.RequestValidator
}

func (r routingRequestResource) GetKey() configresources.ResourceKey {
	return configresources.ResourceKey{
		APIVersion: "",
		Kind:       "",
	}
}

func (r routingRequestResource) GetDefinition() configresources.ResourceDef {
	return configresources.ResourceDef{
		Type: reflect.TypeOf([]dto.RouteRegistrationRequest{}),
		Validate: func(ctx context.Context, md configresources.Metadata, entity interface{}) (bool, string) {
			if ok, message := route.ValidateMetadataStringField(md, "nodeGroup"); !ok {
				return false, message
			}
			req := *entity.(*[]dto.RouteRegistrationRequest)
			return r.validator.Validate(req, md["nodeGroup"].(string))
		},
		Handler: func(ctx context.Context, metadata configresources.Metadata, entity interface{}) (interface{}, error) {
			req := *entity.(*[]dto.RouteRegistrationRequest)
			nodeGroup := metadata["nodeGroup"].(string)
			if err := r.service.RegisterRoutes(ctx, req, nodeGroup); err != nil {
				return nil, err
			}
			return nil, nil
		},
		IsOverriddenByCR: func(ctx context.Context, metadata configresources.Metadata, entity interface{}) bool {
			requests := *entity.(*[]dto.RouteRegistrationRequest)
			if len(requests) == 0 {
				return false
			}
			overridden := requests[0].Overridden
			for _, request := range requests {
				if overridden != request.Overridden {
					return false
				}
				overridden = request.Overridden
			}
			return overridden
		},
	}
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
