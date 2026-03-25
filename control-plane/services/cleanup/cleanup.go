package cleanup

import (
	"fmt"
	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/clustering"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/events"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/envoy"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"strings"
	"sync"
	"time"
)

const (
	localDevNamespacePostfix = ".nip.io"
	namespaceHeader          = "namespace"
	cleanupHourTime          = 2 // 2 AM
)

var logger logging.Logger

func init() {
	logger = logging.GetLogger("cleanup")
}

type RoutesCleanupService interface {
	CleanupRoutes() error
}

type localDevCleanupService struct {
	dao           dao.Dao
	eventBus      bus.BusPublisher
	entityService *entity.Service
}

func NewRoutesCleanupService(dao dao.Dao, eventBus bus.BusPublisher, entityService *entity.Service) RoutesCleanupService {
	return &localDevCleanupService{
		dao:           dao,
		eventBus:      eventBus,
		entityService: entityService,
	}
}

type RoutesCleanupWorker interface {
	Start()
	ChangeCleanupNecessary(clustering.NodeInfo, clustering.Role) error
	Stop()
}

type routesCleanupWorkerImpl struct {
	stop                       chan struct{}
	cleanupService             RoutesCleanupService
	nowFunc                    func() time.Time
	mustPerformCleanup         bool
	cleanupNecessaryStateMutex *sync.RWMutex
}

func NewRoutesCleanupWorker(cleanupService RoutesCleanupService) RoutesCleanupWorker {
	return &routesCleanupWorkerImpl{
		stop:                       make(chan struct{}),
		cleanupService:             cleanupService,
		nowFunc:                    time.Now,
		cleanupNecessaryStateMutex: &sync.RWMutex{},
	}
}

func (r *routesCleanupWorkerImpl) performCleanupIfNeeds() {
	r.cleanupNecessaryStateMutex.RLock()
	defer r.cleanupNecessaryStateMutex.RUnlock()

	if r.mustPerformCleanup {
		if err := r.cleanupService.CleanupRoutes(); err != nil {
			logger.Errorf("Error at localDev routes cleanup. Err: %v", err)
		}
	}
}

// ChangeCleanupNecessary performs cleanup only if this pod is a master pod
func (r *routesCleanupWorkerImpl) ChangeCleanupNecessary(_ clustering.NodeInfo, state clustering.Role) error {
	r.cleanupNecessaryStateMutex.Lock()
	defer r.cleanupNecessaryStateMutex.Unlock()

	r.mustPerformCleanup = state == clustering.Master

	return nil
}

func (r *routesCleanupWorkerImpl) Start() {
	go func() {
		cleanupStartupTime := r.computeCleanupStartupTime()
		ticker := time.NewTicker(cleanupStartupTime.Sub(r.nowFunc()))

		for {
			select {
			case <-ticker.C:
				ticker.Reset(time.Hour * 24)
				r.performCleanupIfNeeds()
			case <-r.stop:
				return
			}
		}
	}()
}

func (r *routesCleanupWorkerImpl) Stop() {
	r.stop <- struct{}{}
}

func (r *routesCleanupWorkerImpl) computeCleanupStartupTime() time.Time {
	now := r.nowFunc()
	cleanupDay := now.Day()
	if now.Hour() >= cleanupHourTime {
		cleanupDay += 1
	}

	return time.Date(now.Year(), now.Month(), cleanupDay, cleanupHourTime, 0, 0, 0, time.Local)
}

func (l localDevCleanupService) CleanupRoutes() error {
	logger.Infof("LocalDev routes cleanup process start")

	var nodeGroups []*domain.NodeGroup

	dbChanges, err := l.dao.WithWTx(func(repo dao.Repository) error {
		if err := l.cleanupLocalDevRoutes(repo); err != nil {
			return fmt.Errorf("cannot perform cleanup localDev routes: %w", err)
		}

		clusters, err := l.dao.FindAllClusters()
		if err != nil {
			return fmt.Errorf("error at finding all clusters while performing localDev route cleanup: %w", err)
		}
		if err := l.cleanupLocalDevClusters(repo, clusters); err != nil {
			return fmt.Errorf("cannot cleanup localDev clusters: %w", err)
		}

		nodeGroups, err = repo.FindAllNodeGroups()
		if err != nil {
			return fmt.Errorf("error at getting node groups while localDev routes cleanup for updating envoy: %w", err)
		}

		if err := l.updateEnvoyVersions(repo, nodeGroups); err != nil {
			return fmt.Errorf("cannot update envoy with removed localDev routes: %w", err)
		}

		return nil
	})

	if err != nil {
		logger.Errorf("Cannot proceed localDev routes cleanup process")
		return err
	}

	logger.Infof("Updating envoy config at localDev cleanup")
	err = l.updateEnvoyConfig(nodeGroups, dbChanges)
	if err != nil {
		return fmt.Errorf("cannot perform envoy config update at localDev cleanup: %w", err)
	} else {
		logger.Infof("LocalDev routes cleanup finished successfully")
	}

	return nil
}

func (l localDevCleanupService) cleanupLocalDevRoutes(repo dao.Repository) error {
	routes, err := repo.FindAllRoutes()
	if err != nil {
		return fmt.Errorf("error at getting all routes while localDev routes cleanup: %w", err)
	}
	for _, route := range routes {
		if route.HeaderMatchers, err = repo.FindHeaderMatcherByRouteId(route.Id); err != nil {
			return fmt.Errorf("error at filling localDev route %v while routes cleanup: %w", *route, err)
		}
		if l.hasLocalDevHeaderMatcher(route.HeaderMatchers) {
			if err = l.entityService.DeleteRouteCascade(repo, route); err != nil {
				return fmt.Errorf("error at deleting localDev route %v: %w", *route, err)
			}
		}
	}
	return err
}

func (l localDevCleanupService) updateEnvoyVersions(repo dao.Repository, nodeGroups []*domain.NodeGroup) error {
	for _, nodeGroup := range nodeGroups {
		if err := envoy.UpdateAllResourceVersions(repo, nodeGroup.Name); err != nil {
			return fmt.Errorf("failed to publish changes for node group %v: %w", *nodeGroup, err)
		}
	}
	return nil
}

func (l localDevCleanupService) updateEnvoyConfig(nodeGroups []*domain.NodeGroup, dbChanges []memdb.Change) error {
	for _, nodeGroup := range nodeGroups {
		event := events.NewChangeEventByNodeGroup(nodeGroup.Name, dbChanges)
		if err := l.eventBus.Publish(bus.TopicChanges, event); err != nil {
			logger.Errorf("Cannot update %s nodeGroup at localDev cleanup. Do not update other nodeGroups", nodeGroup.Name)
			return fmt.Errorf("failed to publish changes for %s nodeGroup after localDev cleanup: %w", nodeGroup.Name, err)
		}
	}
	return nil
}

func (l localDevCleanupService) cleanupLocalDevClusters(repo dao.Repository, clusters []*domain.Cluster) error {
	for _, cluster := range clusters {
		if l.isLocalDevCluster(cluster) {
			if err := l.entityService.DeleteClusterCascade(repo, cluster); err != nil {
				return fmt.Errorf("error at deleting localDev cluster %v: %w", *cluster, err)
			}
		}
	}
	return nil
}

func (l localDevCleanupService) isLocalDevCluster(cluster *domain.Cluster) bool {
	clusterNameParts := strings.Split(cluster.Name, "||")
	return len(clusterNameParts) > 2 && strings.Contains(clusterNameParts[1], localDevNamespacePostfix)
}

func (l localDevCleanupService) hasLocalDevHeaderMatcher(matchers []*domain.HeaderMatcher) bool {
	for _, matcher := range matchers {
		if strings.Contains(matcher.ExactMatch, localDevNamespacePostfix) && matcher.Name == namespaceHeader {
			return true
		}
	}
	return false
}
