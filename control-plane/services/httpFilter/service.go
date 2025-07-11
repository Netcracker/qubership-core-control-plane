package httpFilter

import (
	"context"
	"fmt"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/events"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/cluster"
	cfgres "github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/configresources"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/httpFilter/extAuthz"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/pkg/errors"
	"net/url"
)

var logger logging.Logger

func init() {
	logger = logging.GetLogger("listener-service")
}

type Service struct {
	dao             dao.Dao
	bus             bus.BusPublisher
	clusterService  *cluster.Service
	entityService   *entity.Service
	extAuthzService extAuthz.Service
}

func NewWasmFilterService(dao dao.Dao, bus bus.BusPublisher, clusterService *cluster.Service, entityService *entity.Service, extAuthzService extAuthz.Service) *Service {
	return &Service{
		dao:             dao,
		bus:             bus,
		clusterService:  clusterService,
		entityService:   entityService,
		extAuthzService: extAuthzService,
	}
}

func (s *Service) GetGatewayFilters(ctx context.Context, nodeGroup string) (dto.HttpFiltersConfigRequestV3, error) {
	listeners, err := s.dao.FindListenersByNodeGroupId(nodeGroup)
	if err != nil {
		logger.ErrorC(ctx, "Failed to load listeners for node group %s while getting http filters:\n %v", nodeGroup, err)
		return dto.HttpFiltersConfigRequestV3{}, err
	}

	wasmFilterDtos := make([]dto.WasmFilter, 0)
	for _, listener := range listeners {
		wasmFilters, err := s.dao.FindWasmFilterByListenerId(listener.Id)
		if err != nil {
			logger.ErrorC(ctx, "Failed to load %s wasm filters while getting http filters:\n %v", nodeGroup, err)
			return dto.HttpFiltersConfigRequestV3{}, err
		}
		for _, wasm := range wasmFilters {
			wasmFilterDtos = append(wasmFilterDtos, dto.ConvertWasmDomainToFilter(wasm))
		}
	}

	luaFilterDtos := make([]dto.LuaFilter, 0)
	for _, listener := range listeners {
		luaFilters, err := s.dao.FindLuaFilterByListenerId(listener.Id)
		if err != nil {
			logger.ErrorC(ctx, "Failed to load %s lua filters while getting http filters:\n %v", nodeGroup, err)
			return dto.HttpFiltersConfigRequestV3{}, err
		}
		for _, lua := range luaFilters {
			luaFilterDtos = append(luaFilterDtos, dto.ConvertLuaDomainToFilter(lua))
		}
	}

	extAuthzFilter, err := s.extAuthzService.Get(ctx, nodeGroup)
	if err != nil {
		logger.ErrorC(ctx, "Failed to load %s extAuthz filter while getting http filters:\n %v", nodeGroup, err)
		return dto.HttpFiltersConfigRequestV3{}, err
	}

	return dto.HttpFiltersConfigRequestV3{
		Gateways:       []string{nodeGroup},
		WasmFilters:    wasmFilterDtos,
		ExtAuthzFilter: extAuthzFilter,
		LuaFilters:     luaFilterDtos,
	}, nil
}

func (s *Service) Apply(ctx context.Context, req *dto.HttpFiltersConfigRequestV3) error {
	if len(req.WasmFilters) > 0 {
		for _, nodeGroupId := range req.Gateways {
			err := s.AddWasmFilter(ctx, nodeGroupId, req.WasmFilters)
			if err != nil {
				return err
			}
		}
	}
	if len(req.LuaFilters) > 0 {
		for _, nodeGroupId := range req.Gateways {
			err := s.AddLuaFilter(ctx, nodeGroupId, req.LuaFilters)
			if err != nil {
				return err
			}
		}
	}
	if req.ExtAuthzFilter != nil {
		return s.extAuthzService.Apply(ctx, *req.ExtAuthzFilter, req.Gateways...)
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, req *dto.HttpFiltersDropConfigRequestV3) error {
	if len(req.WasmFilters) > 0 {
		for _, nodeGroupId := range req.Gateways {
			err := s.DeleteWasmFilter(ctx, nodeGroupId, asSliceByName(req.WasmFilters))
			if err != nil {
				return err
			}
		}
	}
	if len(req.LuaFilters) > 0 {
		for _, nodeGroupId := range req.Gateways {
			err := s.DeleteLuaFilter(ctx, nodeGroupId, asSliceByName(req.LuaFilters))
			if err != nil {
				return err
			}
		}
	}
	if req.ExtAuthzFilter != nil {
		return s.extAuthzService.Delete(ctx, *req.ExtAuthzFilter, req.Gateways...)
	}
	return nil
}

func (s *Service) ValidateApply(ctx context.Context, req *dto.HttpFiltersConfigRequestV3) (bool, string) {
	if len(req.Gateways) == 0 {
		return false, "spec.gateways is mandatory"
	}
	if req.ExtAuthzFilter != nil {
		return s.extAuthzService.ValidateApply(ctx, *req.ExtAuthzFilter, req.Gateways...)
	}
	return true, ""
}

func (s *Service) ValidateDelete(ctx context.Context, req *dto.HttpFiltersDropConfigRequestV3) (bool, string) {
	if len(req.Gateways) == 0 {
		return false, "spec.gateways is mandatory"
	}
	if req.ExtAuthzFilter != nil {
		return s.extAuthzService.ValidateDelete(ctx, *req.ExtAuthzFilter, req.Gateways...)
	}
	return true, ""
}

func (s *Service) AddWasmFilter(ctx context.Context, nodeGroupId string, filters []dto.WasmFilter) error {
	filtersToAdd := make([]domain.WasmFilter, len(filters))
	changes, err := s.dao.WithWTx(func(dao dao.Repository) error {
		for i, f := range filters {
			wasmFilter := dto.ConvertFilterToDomain(&f)
			filtersToAdd[i] = *wasmFilter
			clusterName, err := wasmFilter.Cluster()
			if err != nil {
				return err
			}
			foundCluster, err := dao.FindClusterByName(clusterName)
			if err != nil {
				logger.ErrorC(ctx, "can not check if cluster with name=%s exists, %v", clusterName, err)
				return err
			}
			if foundCluster == nil {
				clusterConfig := &dto.ClusterConfigRequestV3{Name: clusterName, Gateways: []string{nodeGroupId}, TLS: f.TlsConfigName}
				clusterUrl, err := url.Parse(f.URL)
				if err != nil {
					return err
				}
				clusterConfig.Endpoints = []dto.RawEndpoint{dto.RawEndpoint(clusterUrl.Scheme + "://" + clusterUrl.Host)}
				if clusterUrl.Scheme == "https" {
					tlsConfigName := clusterConfig.Name + "-tls"
					err := dao.SaveTlsConfig(&domain.TlsConfig{Name: tlsConfigName, Enabled: true})
					if err != nil {
						return err
					}
					clusterConfig.TLS = tlsConfigName
				}
				err = s.clusterService.AddClusterDaoProvided(ctx, dao, nodeGroupId, clusterConfig)
				if err != nil {
					return err
				}
			}
		}

		listeners, err := dao.FindListenersByNodeGroupId(nodeGroupId)
		if err != nil || len(listeners) == 0 {
			errMsg := fmt.Sprintf("can not find listener with nodeGroupId=%s", nodeGroupId)
			logger.ErrorC(ctx, errMsg)
			if err == nil {
				err = errors.New(errMsg)
			}
			return err
		}

		for _, listener := range listeners {
			for _, filterToAdd := range filtersToAdd {
				err := s.entityService.PutWasmFilter(dao, &filterToAdd)
				if err != nil {
					logger.ErrorC(ctx, "can not save wasm filter with nodeGroupId=%s, %v", nodeGroupId, err)
					return err
				}
				err = s.entityService.PutListenerWasmFilterIfAbsent(dao, &domain.ListenersWasmFilter{
					ListenerId:   listener.Id,
					WasmFilterId: filterToAdd.Id,
				})
				if err != nil {
					logger.ErrorC(ctx, "can not save listener to wasm filter relation with nodeGroupId=%s, %v", nodeGroupId, err)
					return err
				}
			}
		}
		err = updateRelatedVersions(ctx, nodeGroupId, dao)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		logger.Errorf("can not add WASM filter for listener with nodeGroupId=%s, %v", nodeGroupId, err)
		return err
	}

	event := events.NewChangeEventByNodeGroup(nodeGroupId, changes)
	err = s.bus.Publish(bus.TopicChanges, event)
	if err != nil {
		logger.Errorf("can not publish changes for wasm filters with nodeGroupId=%s, %v", nodeGroupId, err)
		return err
	}
	return nil
}

func (s *Service) DeleteWasmFilter(ctx context.Context, nodeGroupId string, filters []string) error {
	changes, err := s.dao.WithWTx(func(dao dao.Repository) error {
		for _, f := range filters {
			foundFilter, err := dao.FindWasmFilterByName(f)
			if err != nil {
				return err
			}
			if foundFilter == nil {
				return nil
			}
			foundListeners, err := dao.FindListenersByNodeGroupId(nodeGroupId)
			if err != nil {
				return err
			}
			for _, fl := range foundListeners {
				err := dao.DeleteListenerWasmFilter(&domain.ListenersWasmFilter{
					ListenerId:   fl.Id,
					WasmFilterId: foundFilter.Id,
				})
				if err != nil {
					return err
				}
			}
			wasmFilterRelations, err := dao.FindAllListenerWasmFilter()
			if err != nil {
				return err
			}
			if len(wasmFilterRelations) == 0 { // no relations. can be deleted
				_, err := dao.DeleteWasmFilterByName(f)
				if err != nil {
					return err
				}
			}
		}

		err := updateRelatedVersions(ctx, nodeGroupId, dao)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil || len(changes) == 0 {
		return err
	}
	event := events.NewChangeEventByNodeGroup(nodeGroupId, changes)
	err = s.bus.Publish(bus.TopicChanges, event)
	if err != nil {
		logger.Errorf("can not publish changes for wasm filters with nodeGroupId=%s, %v", nodeGroupId, err)
		return err
	}
	return nil
}

func (s *Service) AddLuaFilter(ctx context.Context, nodeGroupId string, filters []dto.LuaFilter) error {
	filtersToAdd := make([]domain.LuaFilter, len(filters))
	changes, err := s.dao.WithWTx(func(dao dao.Repository) error {
		for i, f := range filters {
			luaFilter := dto.ConvertLuaFilterToDomain(&f)
			filtersToAdd[i] = *luaFilter
			clusterName, err := luaFilter.Cluster()
			if err != nil {
				return err
			}
			foundCluster, err := dao.FindClusterByName(clusterName)
			if err != nil {
				logger.ErrorC(ctx, "can not check if cluster with name=%s exists, %v", clusterName, err)
				return err
			}
			if foundCluster == nil {
				clusterConfig := &dto.ClusterConfigRequestV3{Name: clusterName, Gateways: []string{nodeGroupId}}
				clusterUrl, err := url.Parse(f.URL)
				if err != nil {
					return err
				}
				clusterConfig.Endpoints = []dto.RawEndpoint{dto.RawEndpoint(clusterUrl.Scheme + "://" + clusterUrl.Host)}
				if clusterUrl.Scheme == "https" {
					tlsConfigName := clusterConfig.Name + "-tls"
					err := dao.SaveTlsConfig(&domain.TlsConfig{Name: tlsConfigName, Enabled: true})
					if err != nil {
						return err
					}
					clusterConfig.TLS = tlsConfigName
				}
				err = s.clusterService.AddClusterDaoProvided(ctx, dao, nodeGroupId, clusterConfig)
				if err != nil {
					return err
				}
			}
		}

		listeners, err := dao.FindListenersByNodeGroupId(nodeGroupId)
		if err != nil || len(listeners) == 0 {
			errMsg := fmt.Sprintf("can not find listener with nodeGroupId=%s", nodeGroupId)
			logger.ErrorC(ctx, errMsg)
			if err == nil {
				err = errors.New(errMsg)
			}
			return err
		}

		for _, listener := range listeners {
			for _, filterToAdd := range filtersToAdd {
				err := s.entityService.PutLuaFilter(dao, &filterToAdd)
				if err != nil {
					logger.ErrorC(ctx, "can not save lua filter with nodeGroupId=%s, %v", nodeGroupId, err)
					return err
				}
				err = s.entityService.PutListenerLuaFilterIfAbsent(dao, &domain.ListenersLuaFilter{
					ListenerId:   listener.Id,
					LuaFilterId:  filterToAdd.Id,
				})
				if err != nil {
					logger.ErrorC(ctx, "can not save listener to lua filter relation with nodeGroupId=%s, %v", nodeGroupId, err)
					return err
				}
			}
		}
		err = updateRelatedVersions(ctx, nodeGroupId, dao)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		logger.Errorf("can not add Lua filter for listener with nodeGroupId=%s, %v", nodeGroupId, err)
		return err
	}

	event := events.NewChangeEventByNodeGroup(nodeGroupId, changes)
	err = s.bus.Publish(bus.TopicChanges, event)
	if err != nil {
		logger.Errorf("can not publish changes for lua filters with nodeGroupId=%s, %v", nodeGroupId, err)
		return err
	}
	return nil
}

func (s *Service) DeleteLuaFilter(ctx context.Context, nodeGroupId string, filters []string) error {
	changes, err := s.dao.WithWTx(func(dao dao.Repository) error {
		for _, f := range filters {
			foundFilter, err := dao.FindLuaFilterByName(f)
			if err != nil {
				return err
			}
			if foundFilter == nil {
				return nil
			}
			foundListeners, err := dao.FindListenersByNodeGroupId(nodeGroupId)
			if err != nil {
				return err
			}
			for _, fl := range foundListeners {
				err := dao.DeleteListenerLuaFilter(&domain.ListenersLuaFilter{
					ListenerId:   fl.Id,
					LuaFilterId: foundFilter.Id,
				})
				if err != nil {
					return err
				}
			}
			luaFilterRelations, err := dao.FindAllListenerLuaFilter()
			if err != nil {
				return err
			}
			if len(luaFilterRelations) == 0 { // no relations. can be deleted
				_, err := dao.DeleteLuaFilterByName(f)
				if err != nil {
					return err
				}
			}
		}

		err := updateRelatedVersions(ctx, nodeGroupId, dao)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil || len(changes) == 0 {
		return err
	}
	event := events.NewChangeEventByNodeGroup(nodeGroupId, changes)
	err = s.bus.Publish(bus.TopicChanges, event)
	if err != nil {
		logger.Errorf("can not publish changes for lua filters with nodeGroupId=%s, %v", nodeGroupId, err)
		return err
	}
	return nil
}

func updateRelatedVersions(ctx context.Context, nodeGroupId string, dao dao.Repository) error {
	if err := dao.SaveEnvoyConfigVersion(domain.NewEnvoyConfigVersion(nodeGroupId, domain.ListenerTable)); err != nil {
		logger.ErrorC(ctx, "add http filter failed due to error in envoy config version saving for clusters: %v", err)
		return err
	}

	return nil
}

func (s *Service) GetHttpFiltersResourceAdd() cfgres.Resource {
	return &httpFiltersResourceAdd{service: s}
}

func (s *Service) GetHttpFiltersResourceDrop() cfgres.Resource {
	return &httpFiltersDropResourceDrop{service: s}
}
