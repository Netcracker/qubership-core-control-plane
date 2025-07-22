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
