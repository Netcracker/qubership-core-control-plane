package routingmode

import (
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"strings"
	"sync"
)

var (
	logger logging.Logger
)

func init() {
	logger = logging.GetLogger("routing-mode")
}

type Summary struct {
	RoutingMode RoutingMode `json:"routingMode"`
	RouteKeys   []string    `json:"routeKeys"`
}

type Service struct {
	mode           RoutingMode
	modeLock       sync.Mutex
	storage        dao.Dao
	defaultVersion string
}

func NewService(dao dao.Dao, defaultVersion string) *Service {
	service := &Service{
		mode:           SIMPLE,
		modeLock:       sync.Mutex{},
		storage:        dao,
		defaultVersion: defaultVersion,
	}
	service.UpdateRouteModeDetails()
	return service
}

func (s *Service) GetDefaultDeployVersion() string {
	return s.defaultVersion
}

func (s *Service) UpdateRouteModeDetails() *Summary {
	logger.Debugf("start UpdateRouteModeDetails")
	namespacedRoutes := s.getNamespacedRouteKeys()
	versionedRoutes := s.getVersionedRouteKeys()
	logger.Debugf("namespacedRoutes=%+v, versionedRoutes=%+v", namespacedRoutes, versionedRoutes)

	namespacedCount := len(namespacedRoutes)
	versionedCount := len(versionedRoutes)

	summary := &Summary{RouteKeys: make([]string, 0)}

	logger.Debugf("namespacedCount=%d, versionedCount=%d", namespacedCount, versionedCount)
	if namespacedCount+versionedCount == 0 {
		summary.RoutingMode = SIMPLE
	} else if versionedCount > 0 && namespacedCount > 0 {
		summary.RoutingMode = MIXED
		summary.RouteKeys = append(namespacedRoutes, versionedRoutes...)
	} else if versionedCount > 0 {
		summary.RoutingMode = VERSIONED
		summary.RouteKeys = versionedRoutes
	} else if namespacedCount > 0 {
		summary.RoutingMode = NAMESPACED
		summary.RouteKeys = namespacedRoutes
	}

	s.SetRoutingMode(summary.RoutingMode)
	return summary
}

func (s *Service) UpdateRoutingMode(version string, ns *msaddr.Namespace) bool {
	logger.Debugf("start UpdateRoutingMode: %s, %s", version, ns)
	routingMode := s.GetRoutingMode()
	logger.Debugf("current routingMode = %s", routingMode)
	if routingMode == SIMPLE {
		if version != "" && !strings.EqualFold(version, s.defaultVersion) {
			logger.Debugf("updating routing mode to %s", VERSIONED)
			s.SetRoutingMode(VERSIONED)
			return true
		}
		if !ns.IsCurrentNamespace() {
			logger.Debugf("updating routing mode to %s", NAMESPACED)
			s.SetRoutingMode(NAMESPACED)
			return true
		}
	}
	return false
}

func (s *Service) SetRoutingMode(mode RoutingMode) {
	s.modeLock.Lock()
	s.mode = mode
	logger.Debugf("Control plane set routing mode to '%v'", mode)
	s.modeLock.Unlock()
}

func (s *Service) GetRoutingMode() RoutingMode {
	s.modeLock.Lock()
	defer s.modeLock.Unlock()
	return s.mode
}

func (s *Service) IsForbiddenRoutingMode(version, namespaceName string) bool {
	routingMode := s.GetRoutingMode()
	switch routingMode {
	case SIMPLE:
		return false
	case NAMESPACED:
		if version != "" && !strings.EqualFold(version, s.defaultVersion) {
			logger.Warnf(
				"Control-Plane's routing mode is '%v', but incoming request try to register routes with version '%s'",
				routingMode,
				version,
			)
			return true
		}
	case VERSIONED:
		ns := &msaddr.Namespace{Namespace: namespaceName}
		if !ns.IsCurrentNamespace() {
			logger.Warnf(
				"Control-Plane's routing mode is '%v', but incoming request try to register routes with namespace '%s'",
				routingMode,
				ns.Namespace,
			)
			return true
		}
	}
	return false
}

func (s *Service) getNamespacedRouteKeys() []string {
	routes, err := s.storage.WithRTxVal(func(dao dao.Repository) (interface{}, error) {
		routes, err := dao.FindRoutesByNamespaceHeaderIsNot(msaddr.CurrentNamespaceAsString())
		if err != nil {
			return nil, err
		}
		return routes, nil
	})
	if err != nil {
		logger.Errorf("can not get routes by namespace header: %s", err)
		return nil
	}

	return toRouteKeys(routes.([]*domain.Route))
}

func (s *Service) getVersionedRouteKeys() []string {
	dv, err := s.storage.FindAllDeploymentVersions()
	if err != nil {
		logger.Errorf("can not get deployment versions: %s", err)
	}

	hasTheOnlyVersion := len(dv) == 1

	if !hasTheOnlyVersion {
		foundRoutes, err := s.storage.WithRTxVal(func(dao dao.Repository) (interface{}, error) {
			routes, err := dao.FindRoutesByDeploymentVersionStageIn(domain.CandidateStage, domain.LegacyStage)
			if err != nil {
				return nil, err
			}
			return routes, nil
		})
		if err != nil {
			logger.Errorf("can not get not active routes")
			return nil
		}

		return toRouteKeys(foundRoutes.([]*domain.Route))
	}
	return nil
}

func toRouteKeys(routes []*domain.Route) []string {
	routeKeys := make([]string, len(routes))
	for i, r := range routes {
		routeKeys[i] = r.RouteKey
	}

	return routeKeys
}
