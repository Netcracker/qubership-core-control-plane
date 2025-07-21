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
