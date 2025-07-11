package dao

import (
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
)

func (d *InMemRepo) FindAllLuaFilters() ([]*domain.LuaFilter, error) {
	return FindAll[domain.LuaFilter](d, domain.LuaFilterTable)
}

func (d *InMemRepo) FindLuaFilterByName(filterName string) (*domain.LuaFilter, error) {
	return FindFirstByIndex[domain.LuaFilter](d, domain.LuaFilterTable, "name", filterName)
}

func (d *InMemRepo) FindLuaFilterByListenerId(listenerId int32) ([]*domain.LuaFilter, error) {
	txCtx := d.getTxCtx(false)
	defer txCtx.closeIfLocal()
	if found, err := d.storage.FindByIndex(txCtx.tx, domain.ListenersLuaFilterTable, "listenerId", listenerId); err == nil {
		listenerToLuaFilters := found.([]*domain.ListenersLuaFilter)
		luaFilters := make([]*domain.LuaFilter, len(listenerToLuaFilters))
		for i, listenerToLuaFilter := range listenerToLuaFilters {
			if wf, err := d.storage.FindById(txCtx.tx, domain.LuaFilterTable, listenerToLuaFilter.LuaFilterId); err == nil {
				luaFilters[i] = wf.(*domain.LuaFilter)
			} else {
				return nil, err
			}
		}
		return luaFilters, nil
	} else {
		return nil, err
	}
}

func (d *InMemRepo) SaveLuaFilter(luaFilter *domain.LuaFilter) error {
	return d.SaveUnique(domain.LuaFilterTable, luaFilter)
}

func (d *InMemRepo) DeleteLuaFilterByName(filterName string) (int32, error) {
	txCtx := d.getTxCtx(true)
	defer txCtx.closeIfLocal()
	filterToDelete, err := d.FindLuaFilterByName(filterName)
	if err != nil {
		return 0, err
	}
	_, err = txCtx.tx.DeleteAll(domain.LuaFilterTable, "id", filterToDelete.Id)
	if err != nil {
		return 0, err
	}
	return filterToDelete.Id, nil
}

func (d *InMemRepo) DeleteLuaFilterById(id int32) error {
	return d.DeleteById(domain.LuaFilterTable, id)
}
