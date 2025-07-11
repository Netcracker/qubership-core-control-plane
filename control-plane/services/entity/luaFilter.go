package entity

import (
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
)

func (srv *Service) PutLuaFilter(dao dao.Repository, filter *domain.LuaFilter) error {
	if filter.Id == 0 {
		existing, err := dao.FindLuaFilterByName(filter.Name)
		if err != nil {
			logger.Errorf("Error while trying to find existing lua filter by name %s: %s", filter.Name, err.Error())
			return err
		}
		if existing != nil {
			filter.Id = existing.Id
		}
	}
	return dao.SaveLuaFilter(filter)
}

func (srv *Service) PutListenerLuaFilterIfAbsent(dao dao.Repository, relation *domain.ListenersLuaFilter) error {
	alreadyHasFilter, err := dao.HasLuaFilterWithId(relation.ListenerId, relation.LuaFilterId)
	if err != nil {
		logger.Errorf("Error while check relation by listenerId=%d and luaFilterId=%d: %s", relation.ListenerId, relation.LuaFilterId, err.Error())
		return err
	}
	if alreadyHasFilter {
		logger.Infof("Lua filter with id=%d is already connected to listener with id=%d", relation.LuaFilterId, relation.ListenerId)
		return nil
	}
	err = dao.SaveListenerLuaFilter(relation)
	if err != nil {
		logger.Errorf("Error while saving listener lua filter relation %v: %s", relation, err.Error())
		return err
	}
	return nil
}
