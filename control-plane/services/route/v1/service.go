package v1

import (
	"context"
	gerrors "github.com/go-errors/errors"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/events"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/envoy"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/creator"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/registration"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/routingmode"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/pkg/errors"
)

var logger logging.Logger
var ErrNoClusterExist = errors.New("Cluster does not exist for passed clusterId")

func init() {
	logger = logging.GetLogger("route-services/v1")
}

type Service struct {
	storage            dao.Dao
	eventBus           bus.BusPublisher
	entityService      *entity.Service
	regService         *route.RegistrationService
	routModeService    *routingmode.Service
	v1RequestProcessor *registration.V1RequestProcessor
}

func NewV1Service(entityService *entity.Service, storage dao.Dao, eventBus bus.BusPublisher, routingModeService *routingmode.Service, regService *route.RegistrationService) *Service {
	this := &Service{}
	this.storage = storage
	this.eventBus = eventBus
	this.entityService = entityService
	this.routModeService = routingModeService
	this.regService = regService
	this.v1RequestProcessor = registration.NewV1RequestProcessor(storage)
	return this
}

func (s *Service) RegisterRoutes(ctx context.Context, nodeGroup string, routeRequest *creator.RouteEntityRequest) error {
	changes, err := s.storage.WithWTx(func(dao dao.Repository) error {
		activeVersion, err := s.entityService.GetActiveDeploymentVersion(dao)
		if err != nil {
			logger.ErrorC(ctx, "Failed to get active deployment version: %v", err)
			return err
		}
		//processedRequest := registration.V1RequestProcessor.ProcessRequestV1(ctx, nodeGroup, *routeRequest, activeVersion.Version)
		processedRequest, err := s.v1RequestProcessor.ProcessRequestV1(ctx, nodeGroup, *routeRequest, activeVersion.Version)
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

		namespace := routeRequest.GetRoutes()[0].GetNamespace()
		s.routModeService.UpdateRoutingMode("", msaddr.NewNamespace(namespace))

		if err := envoy.UpdateAllResourceVersions(dao, nodeGroup); err != nil {
			logger.ErrorC(ctx, "Failed to update envoy resource versions: %v", err)
			return err
		}
		return nil
	})

	if err != nil {
		logger.ErrorC(ctx, "Error in route registration transaction for node group %s and request %v: \n %v", nodeGroup, *routeRequest, err)
		return err
	}

	logger.InfoC(ctx, "Publish changes for %s", nodeGroup)
	event := events.NewChangeEventByNodeGroup(nodeGroup, changes)
	err = s.eventBus.Publish(bus.TopicChanges, event)
	if err != nil {
		logger.ErrorC(ctx, "Can't publish changes to eventBus for node group %s: \n %v", nodeGroup, err)
		return err
	}
	return nil
}

func (s *Service) GetClusters() ([]*dto.ClusterResponse, error) {
	clusters, err := s.entityService.GetClustersWithRelations(s.storage)
	if err != nil {
		logger.Errorf("Failed to find clusters %v", err)
		return nil, err
	}
	return dto.DefaultResponseConverter.ConvertClustersToResponse(clusters), nil
}

func (s *Service) DeleteCluster(clusterId int32) error {
	cluster, err := s.storage.FindClusterById(clusterId)
	if err != nil {
		logger.Errorf("Failed to find cluster to delete: %v", err)
		return err
	}
	if cluster == nil {
		logger.Errorf("Cluster does not exist for passed clusterId %v", clusterId)
		return ErrNoClusterExist
	}
	nodeGroups, err := s.storage.FindNodeGroupsByCluster(cluster)
	if err != nil {
		logger.Errorf("Failed to find node groups for cluster %v: %v", cluster.Name, err)
		return err
	}
	changes, err := s.storage.WithWTx(func(repo dao.Repository) error {
		if err := s.entityService.DeleteClusterCascade(repo, cluster); err != nil {
			logger.Errorf("Error during cluster deletion: %v", err)
			return err
		}

		for _, nodeGroup := range nodeGroups {
			if err := envoy.UpdateAllResourceVersions(repo, nodeGroup.Name); err != nil {
				logger.Errorf("Cluster deletion have failed due to error while updating envoy node group %v version: %v", nodeGroup.Name, err)
				return err
			}
		}
		return nil
	})
	for _, nodeGroup := range nodeGroups {
		if err := s.eventBus.Publish(bus.TopicChanges, events.NewChangeEventByNodeGroup(nodeGroup.Name, changes)); err != nil {
			logger.Errorf("Failed to publish changes for node group %v: %v", nodeGroup, err)
			return err
		}
	}
	logger.Info("Cluster has been deleted successfully")
	return nil
}

func (s *Service) GetRouteConfigurations() ([]*dto.RouteConfigurationResponse, error) {
	routeConfigs, err := s.entityService.GetRouteConfigurationsWithRelations(s.storage)
	if err != nil {
		logger.Errorf("Failed to find route configurations %v", err)
		return nil, err
	}
	return dto.DefaultResponseConverter.ConvertRouteConfigurationsToResponse(routeConfigs), nil
}

func (s *Service) GetNodeGroups() ([]*domain.NodeGroup, error) {
	return s.storage.FindAllNodeGroups()
}

func (s *Service) GetListeners() ([]*domain.Listener, error) {
	return s.storage.FindAllListeners()
}

func (s *Service) DeleteRoutes(ctx context.Context, nodeGroup, fromPrefix, namespace string) ([]*domain.Route, error) {
	deletedRoutes, changes, err := s.storage.WithWTxVal(func(storage dao.Repository) (interface{}, error) {
		dVersion, err := s.entityService.GetActiveDeploymentVersion(storage)
		if err != nil {
			return nil, err
		}

		regSrvCtx := s.regService.WithContext(ctx, storage)
		var deletedRoutes []*domain.Route
		if fromPrefix == "" {
			deletedRoutes, err = regSrvCtx.DeleteRoutes(nodeGroup, namespace, dVersion.Version)
		} else {
			deletedRoutes, err = regSrvCtx.DeleteRoutes(nodeGroup, namespace, dVersion.Version, fromPrefix)
		}
		if err != nil {
			return nil, errors.Wrapf(err, "removing routes for node-group: %s, namespace: %s, version: %s caused error", nodeGroup, namespace, dVersion.Version)
		}
		err = envoy.UpdateAllResourceVersions(storage, nodeGroup)
		if err != nil {
			return nil, errors.Wrap(err, "updating envoy resources versions caused error")
		}
		return deletedRoutes, nil
	})

	if err != nil {
		return nil, err
	}

	if deletedRoutes == nil || len(deletedRoutes.([]*domain.Route)) == 0 {
		return nil, nil
	}

	s.routModeService.UpdateRouteModeDetails()
	event := events.NewChangeEventByNodeGroup(nodeGroup, changes)
	err = s.eventBus.Publish(bus.TopicChanges, event)
	if err != nil {
		return nil, err
	}
	return deletedRoutes.([]*domain.Route), nil
}
