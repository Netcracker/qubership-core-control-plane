package dao

import (
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
)

func (d *InMemRepo) FindAllListeners() ([]*domain.Listener, error) {
	return FindAll[domain.Listener](d, domain.ListenerTable)
}

func (d *InMemRepo) FindListenerByNodeGroupIdAndName(nodeGroupId, name string) (*domain.Listener, error) {
	return FindFirstByIndex[domain.Listener](d, domain.ListenerTable, "nodeGroupAndName", nodeGroupId, name)
}

func (d *InMemRepo) FindListenersByNodeGroupId(nodeGroupId string) ([]*domain.Listener, error) {
	return FindByIndex[domain.Listener](d, domain.ListenerTable, "nodeGroup", nodeGroupId)
}

func (d *InMemRepo) HasWasmFilterWithId(listenerId, wasmFilterId int32) (bool, error) {
	txCtx := d.getTxCtx(false)
	defer txCtx.closeIfLocal()
	result, err := d.storage.FindById(txCtx.tx, domain.ListenersWasmFilterTable, listenerId, wasmFilterId)
	if err == nil {
		if result != nil {
			return true, nil
		}
		return false, nil
	} else {
		return false, err
	}
}

func (d *InMemRepo) FindListenerIdsByWasmFilterId(wasmFilterId int32) ([]int32, error) {
	txCtx := d.getTxCtx(false)
	defer txCtx.closeIfLocal()
	result, err := d.storage.FindByIndex(txCtx.tx, domain.ListenersWasmFilterTable, "wasmFilterId", wasmFilterId)
	if err == nil {
		if result != nil {
			listenersWasmFilters := result.([]*domain.ListenersWasmFilter)
			listenerIds := make([]int32, len(listenersWasmFilters))
			for i, lw := range listenersWasmFilters {
				listenerIds[i] = lw.ListenerId
			}
			return listenerIds, nil
		}
		return nil, nil
	} else {
		return nil, err
	}
}

func (d *InMemRepo) SaveListener(listener *domain.Listener) error {
	return d.SaveUnique(domain.ListenerTable, listener)
}

func (d *InMemRepo) FindListenerById(id int32) (*domain.Listener, error) {
	return FindById[domain.Listener](d, domain.ListenerTable, id)
}

func (d *InMemRepo) DeleteListenerByNodeGroupName(nodeGroupId string) error {
	txCtx := d.getTxCtx(true)
	defer txCtx.closeIfLocal()
	_, err := txCtx.tx.DeleteAll(domain.ListenerTable, "nodeGroup", nodeGroupId)
	return err
}

func (d *InMemRepo) DeleteListenerById(id int32) error {
	return d.DeleteById(domain.ListenerTable, id)
}

func (d *InMemRepo) SaveListenerWasmFilter(relation *domain.ListenersWasmFilter) error {
	return d.SaveEntity(domain.ListenersWasmFilterTable, relation)
}

func (d *InMemRepo) DeleteListenerWasmFilter(relation *domain.ListenersWasmFilter) error {
	return d.DeleteById(domain.ListenersWasmFilterTable, relation.ListenerId, relation.WasmFilterId)
}

func (d *InMemRepo) FindAllListenerWasmFilter() ([]*domain.ListenersWasmFilter, error) {
	return FindAll[domain.ListenersWasmFilter](d, domain.ListenersWasmFilterTable)
}


func (d *InMemRepo) HasLuaFilterWithId(listenerId, luaFilterId int32) (bool, error) {
	txCtx := d.getTxCtx(false)
	defer txCtx.closeIfLocal()
	result, err := d.storage.FindById(txCtx.tx, domain.ListenersLuaFilterTable, listenerId, luaFilterId)
	if err == nil {
		if result != nil {
			return true, nil
		}
		return false, nil
	} else {
		return false, err
	}
}

func (d *InMemRepo) FindListenerIdsByLuaFilterId(luaFilterId int32) ([]int32, error) {
	txCtx := d.getTxCtx(false)
	defer txCtx.closeIfLocal()
	result, err := d.storage.FindByIndex(txCtx.tx, domain.ListenersLuaFilterTable, "luaFilterId", luaFilterId)
	if err == nil {
		if result != nil {
			listenersLuaFilters := result.([]*domain.ListenersLuaFilter)
			listenerIds := make([]int32, len(listenersLuaFilters))
			for i, lw := range listenersLuaFilters {
				listenerIds[i] = lw.ListenerId
			}
			return listenerIds, nil
		}
		return nil, nil
	} else {
		return nil, err
	}
}

func (d *InMemRepo) SaveListenerLuaFilter(relation *domain.ListenersLuaFilter) error {
	return d.SaveEntity(domain.ListenersLuaFilterTable, relation)
}

func (d *InMemRepo) DeleteListenerLuaFilter(relation *domain.ListenersLuaFilter) error {
	return d.DeleteById(domain.ListenersLuaFilterTable, relation.ListenerId, relation.LuaFilterId)
}

func (d *InMemRepo) FindAllListenerLuaFilter() ([]*domain.ListenersLuaFilter, error) {
	return FindAll[domain.ListenersLuaFilter](d, domain.ListenersLuaFilterTable)
}