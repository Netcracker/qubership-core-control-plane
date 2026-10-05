package bus

import (
	"reflect"

	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/events"
)

// MarshalPrepare mutates entities in place, so publishing runs over private copies, not the
// pointers shared with memdb.
func copyChangeEventForPublish(ce *events.ChangeEvent) *events.ChangeEvent {
	if ce == nil {
		return nil
	}
	return &events.ChangeEvent{
		NodeGroup: ce.NodeGroup,
		Changes:   copyChangesForPublish(ce.Changes),
	}
}

func copyReloadEventForPublish(re *events.ReloadEvent) *events.ReloadEvent {
	if re == nil {
		return nil
	}
	return &events.ReloadEvent{
		Changes: copyChangesForPublish(re.Changes),
	}
}

func copyChangesForPublish(src map[string][]memdb.Change) map[string][]memdb.Change {
	out := make(map[string][]memdb.Change, len(src))
	for table, changes := range src {
		copied := make([]memdb.Change, len(changes))
		for i, ch := range changes {
			copied[i] = memdb.Change{
				Table:  ch.Table,
				Before: cloneChangeEntity(ch.Before),
				After:  cloneChangeEntity(ch.After),
			}
		}
		out[table] = copied
	}
	return out
}

// Shallow copy: MarshalPrepare only nil-s top-level fields. Make it deep if that changes.
func cloneChangeEntity(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return v
	}
	elem := rv.Elem()
	if elem.Kind() != reflect.Struct {
		return v
	}
	clone := reflect.New(elem.Type())
	clone.Elem().Set(elem)
	return clone.Interface()
}
