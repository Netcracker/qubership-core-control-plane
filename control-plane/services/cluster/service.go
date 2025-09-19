package cluster

import (
	"context"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/events"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	cfgres "github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/configresources"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/registration"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
)

var logger logging.Logger

func init() {
	logger = logging.GetLogger("cluster-service")
}

type Service struct {
	dao           dao.Dao
	bus           bus.BusPublisher
	entityService *entity.Service
}

func NewClusterService(entityService *entity.Service, dao dao.Dao, bus bus.BusPublisher) *Service {
	return &Service{
		dao:           dao,
		bus:           bus,
		entityService: entityService,
	}
}

func (s *Service) AddClusterDaoProvided(ctx context.Context, dao dao.Repository, nodeGroupId string, cluster *dto.ClusterConfigRequestV3) error {
	clusterToAdd := domain.NewCluster(cluster.Name, false)
	tlsConfig, err := dao.FindTlsConfigByName(cluster.TLS)
	if err != nil {
		return err
	}
	if tlsConfig != nil {
		clusterToAdd.TLSId = tlsConfig.Id
	}

	err = entity.RebaseCluster(dao, clusterToAdd)
	if err != nil {
		logger.Errorf("Error while rebasing cluster with name %v: %v", cluster.Name, err)
		return err
	}

	if cluster.TcpKeepalive != nil {
		clusterToAdd.TcpKeepalive = &domain.TcpKeepalive{
			Probes:   int32(cluster.TcpKeepalive.Probes),
			Time:     int32(cluster.TcpKeepalive.Time),
			Interval: int32(cluster.TcpKeepalive.Interval),
		}
	}
	if err = s.entityService.UpdateClusterTcpKeepalive(dao, clusterToAdd); err != nil {
		logger.Errorf("Error while actualizing tcp keepalive for cluster with name %v: %v", clusterToAdd.Name, err)
		return err
	}

	//Change if add new CircuitBreakers or thresholds
	if cluster.CircuitBreaker.Threshold.MaxConnections != 0 {

		var circuitBreaker *domain.CircuitBreaker
		if clusterToAdd.CircuitBreakerId == 0 {
			circuitBreaker = &domain.CircuitBreaker{}
		} else {
			circuitBreaker, err = dao.FindCircuitBreakerById(clusterToAdd.CircuitBreakerId)
			if err != nil {
				logger.Errorf("Error while searching for existing CircuitBreaker with id %v: %v", clusterToAdd.CircuitBreakerId, err)
				return err
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

		}

		threshold.MaxConnections = int32(cluster.CircuitBreaker.Threshold.MaxConnections)

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
		clusterToAdd.CircuitBreakerId = circuitBreaker.Id

	} else {
		if clusterToAdd.CircuitBreakerId != 0 {
			if err := s.entityService.DeleteCircuitBreakerCascadeById(dao, clusterToAdd.CircuitBreakerId); err != nil {
				logger.Errorf("Error during cascade CircuitBreaker deletion: %v", err)
				return err
			}
			clusterToAdd.CircuitBreakerId = 0
		}
	}

	if cluster.MaxRequestsPerConnection != 0 {
		clusterToAdd.MaxRequestsPerConnection = int32(cluster.MaxRequestsPerConnection)
	} else {
		logger.Debug("cluster.MaxRequestsPerConnection is empty")
	}

	if cluster.ConnectionIdleTimeout != nil {
		clusterToAdd.ConnectionIdleTimeout = domain.NewNullInt(*cluster.ConnectionIdleTimeout)
	} else {
		logger.Debug("cluster.ConnectionIdleTimeout is empty")
	}

	err = dao.SaveCluster(clusterToAdd)
	if err != nil {
		logger.Errorf("Error while saving cluster with name %v: %v", cluster.Name, err)
		return err
	}

	clusterToAdd.NodeGroups = append(clusterToAdd.NodeGroups, &domain.NodeGroup{Name: nodeGroupId})
	err = s.entityService.PutClustersNodeGroupIfAbsent(dao, domain.NewClusterNodeGroups(clusterToAdd.Id, nodeGroupId))
	if err != nil {
		return err
	}

	endpointsToAdd := make([]*domain.Endpoint, len(cluster.Endpoints))
	for i, e := range cluster.Endpoints {
		host, port, err := e.HostPort()
		if err != nil {
			logger.ErrorC(ctx, "can not parse endpoint url=%s: %v", e, err)
			return err
		}

		version, err := s.entityService.GetActiveDeploymentVersion(dao)
		if err != nil {
			logger.ErrorC(ctx, "can not get active deployment version: %v", e, err)
			return err
		}
		initVersion, deployVersion, err := registration.ResolveVersions(s.dao, cluster.Name, "", version.Version)
		if err != nil {
			return err
		}
		endpointsToAdd[i] = domain.NewEndpoint(host, int32(port), deployVersion, initVersion, clusterToAdd.Id)
	}
	err = s.entityService.PutEndpoints(dao, clusterToAdd.Id, endpointsToAdd)
	if err != nil {
		return err
	}

	if err := dao.SaveEnvoyConfigVersion(domain.NewEnvoyConfigVersion(nodeGroupId, domain.ClusterTable)); err != nil {
		logger.ErrorC(ctx, "add cluster failed due to error in envoy config version saving for clusters: %v", err)
		return err
	}
	return nil

}

func (s *Service) AddCluster(ctx context.Context, nodeGroupId string, cluster *dto.ClusterConfigRequestV3) error {
	changes, err := s.dao.WithWTx(func(dao dao.Repository) error {
		return s.AddClusterDaoProvided(ctx, dao, nodeGroupId, cluster)
	})

	event := events.NewChangeEventByNodeGroup(nodeGroupId, changes)
	err = s.bus.Publish(bus.TopicChanges, event)
	if err != nil {
		logger.Errorf("can not publish changes for listener with nodeGroupId=%s, %v", nodeGroupId, err)
		return err
	}

	return err
}

func (s *Service) GetClusterResource() cfgres.Resource {
	return &clusterResource{service: s}
}
