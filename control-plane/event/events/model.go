package events

import (
	"fmt"
	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
)

type ChangeEvent struct {
	NodeGroup string
	Changes   map[string][]memdb.Change
}

func (ce *ChangeEvent) ToString() string {
	str := "{ChangeEvent: nodeGroup='" + ce.NodeGroup + "', changes=[\n"
	for key, changes := range ce.Changes {
		str += key + ": ["
		for _, change := range changes {
			if change.Before == nil {
				str += "nil, "
			} else {
				str += fmt.Sprintf("%v, ", change.Before)
			}
			if change.After == nil {
				str += "nil, "
			} else {
				str += fmt.Sprintf("%v, ", change.After)
			}
		}
		str += "], \n"
	}
	return str + "]}"
}

// MultipleChangeEvent supports changes in multiple nodeGroups.
type MultipleChangeEvent struct {
	Changes map[string][]memdb.Change
}

type ReloadEvent struct {
	Changes map[string][]memdb.Change
}

func (ce *ChangeEvent) MarshalPrepare() error {
	for _, changes := range ce.Changes {
		if err := marshalPrepareForChanges(changes); err != nil {
			return err
		}
	}
	return nil
}

func (mce *MultipleChangeEvent) MarshalPrepare() error {
	for _, changes := range mce.Changes {
		if err := marshalPrepareForChanges(changes); err != nil {
			return err
		}
	}
	return nil
}

func (re *ReloadEvent) MarshalPrepare() error {
	for _, changes := range re.Changes {
		if err := marshalPrepareForChanges(changes); err != nil {
			return err
		}
	}
	return nil
}

func marshalPrepareForChanges(changes []memdb.Change) error {
	for i := range changes {
    		if changes[i].Before != nil {
    			cloned, err := cloneAndPrepare(changes[i].Before)
    			if err != nil {
    				return err
    			}
    			changes[i].Before = cloned
    		}
		if changes[i].After != nil {
        			cloned, err := cloneAndPrepare(changes[i].After)
        			if err != nil {
        				return err
        			}
        			changes[i].After = cloned
        		}
        	}
	}
	return nil
}

func cloneAndPrepare(entity interface{}) (interface{}, error) {
	clone := shallowClonePtr(entity)

	preparer, ok := clone.(domain.MarshalPreparer)
	if !ok {
		return nil, fmt.Errorf("event source does not implement domain.MarshalPreparer: %T", clone)
	}

	if err := preparer.MarshalPrepare(); err != nil {
		return nil, err
	}

	return clone, nil
}

func shallowClonePtr(entity interface{}) interface{} {
	v := reflect.ValueOf(entity)
	if v.Kind() != reflect.Ptr {
		return entity
	}
	clone := reflect.New(v.Elem().Type())
	clone.Elem().Set(v.Elem())
	return clone.Interface()
}


// PartialReloadEvent signals that all envoy cache entries for the specified nodeGroup:entityType pairs
// must be reloaded from in-memory storage.
type PartialReloadEvent struct {
	EnvoyVersions []*domain.EnvoyConfigVersion
}
