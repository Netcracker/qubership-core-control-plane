package bus

import (
	"bytes"
	"encoding/gob"
	"github.com/hashicorp/go-memdb"
	uuid3 "github.com/hashicorp/go-uuid"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/events"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/ram"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
)

var (
	nodeGroup = "private-gateway-service"
)

func TestTLSConfigSerializable(t *testing.T) {
	storage := ram.NewStorage()
	tls := []*domain.TlsConfig{
		{
			Id:   1,
			Name: "A",
		},
		{
			Id:   2,
			Name: "B",
		},
	}
	tx := storage.WriteTx()
	err := storage.Save(tx, domain.TlsConfigTable, tls)
	if err != nil {
		t.Fatal(err)
	}
	tx.Commit()

	changeEvent := events.NewChangeEventByNodeGroup(nodeGroup, tx.Changes())
	builtEvent, err := buildProtobufEvent(changeEvent)
	assert.Nil(t, err)
	assert.NotNil(t, builtEvent)
	assert.NotNil(t, builtEvent.Data)
	assert.True(t, len(builtEvent.Data.Value) > 0)

	readChangeEvent, err := readChangeEventFromProtoEvent(builtEvent)

	assert.Nil(t, err)
	assert.Equal(t, changeEvent, readChangeEvent)
}

func TestTLSConfigSerializable_InReloadEvent(t *testing.T) {
	storage := ram.NewStorage()
	tls := []*domain.TlsConfig{
		{
			Id:   1,
			Name: "A",
		},
		{
			Id:   2,
			Name: "B",
		},
	}
	tx := storage.WriteTx()
	err := storage.Save(tx, domain.TlsConfigTable, tls)
	if err != nil {
		t.Fatal(err)
	}
	tx.Commit()

	changeEvent := events.NewReloadEvent(tx.Changes())
	builtEvent, err := buildProtobufEvent(changeEvent)
	assert.Nil(t, err)
	assert.NotNil(t, builtEvent)
	assert.NotNil(t, builtEvent.Data)
	assert.True(t, len(builtEvent.Data.Value) > 0)

	readChangeEvent, err := readReloadEventFromProtoEvent(builtEvent)

	assert.Nil(t, err)
	assert.Equal(t, changeEvent, readChangeEvent)
}

func TestReadProtobufEvent(t *testing.T) {
	changeEvent := &events.ReloadEvent{
		Changes: map[string][]memdb.Change{
			"changes-1": {memdb.Change{Table: "table-1"}},
		},
	}
	event, err := buildProtobufEvent(changeEvent)
	assert.Nil(t, err)

	afterRead, err := readProtobufEvent(event)
	assert.Nil(t, err)
	assert.NotNil(t, afterRead)
	assert.Equal(t, changeEvent, afterRead)
}

func TestRouteSerializable(t *testing.T) {
	storage := ram.NewStorage()
	uuid, _ := uuid3.GenerateUUID()
	routes := []*domain.Route{
		{
			Id:                1,
			Uuid:              uuid,
			VirtualHostId:     1,
			RouteKey:          "/api/v1",
			Prefix:            "/api/v1",
			ClusterName:       "cluster",
			DeploymentVersion: "v1",
		},
	}
	retry := &domain.RetryPolicy{
		Id:    1,
		Route: routes[0],
	}
	routes[0].RetryPolicy = retry

	tx := storage.WriteTx()
	err := storage.Save(tx, domain.RouteTable, routes)
	if err != nil {
		t.Fatal(err)
	}
	tx.Commit()

	changeEvent := events.NewChangeEventByNodeGroup(nodeGroup, tx.Changes())
	builtEvent, err := buildProtobufEvent(changeEvent)
	assert.Nil(t, err)
	assert.NotNil(t, builtEvent)
	assert.NotNil(t, builtEvent.Data)
	assert.True(t, len(builtEvent.Data.Value) > 0)

	readChangeEvent, err := readChangeEventFromProtoEvent(builtEvent)

	assert.Nil(t, err)
	assert.Equal(t, changeEvent, readChangeEvent)
}

func TestMarshalPrepareDoesNotMutateLiveMemdbObject(t *testing.T) {
	storage := ram.NewStorage()
	uuid, _ := uuid3.GenerateUUID()
	route := &domain.Route{
		Id:                1,
		Uuid:              uuid,
		VirtualHostId:     1,
		RouteKey:          "/api/v1",
		Prefix:            "/api/v1",
		ClusterName:       "cluster",
		DeploymentVersion: "v1",
	}
	retry := &domain.RetryPolicy{Id: 1, Route: route}
	route.RetryPolicy = retry

	tx := storage.WriteTx()
	tx.TrackChanges()
	if err := storage.Save(tx, domain.RouteTable, []*domain.Route{route}); err != nil {
		t.Fatal(err)
	}
	changes := tx.Changes()
	tx.Commit()

	stored, err := storage.FindById(storage.ReadTx(), domain.RouteTable, int32(1))
	if err != nil {
		t.Fatal(err)
	}
	storedRoute := stored.(*domain.Route)
	assert.NotNil(t, storedRoute.RetryPolicy, "route in memdb must have relations before publish")

	eventFromFirstRequest := events.NewChangeEventByNodeGroup(nodeGroup, changes)
	eventFromSecondRequest := events.NewChangeEventByNodeGroup(nodeGroup, changes)

	_, err = buildProtobufEvent(eventFromFirstRequest)
	assert.NoError(t, err)

	storedAfterPublish, err := storage.FindById(storage.ReadTx(), domain.RouteTable, int32(1))
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, storedAfterPublish.(*domain.Route).RetryPolicy,
		"MarshalPrepare during publish must not mutate live memdb object")

	changeAfter := eventFromSecondRequest.Changes[domain.RouteTable][0].After.(*domain.Route)
	assert.NotNil(t, changeAfter.RetryPolicy,
		"ChangeEvent still references live route; relations must remain until memdb update")
}

func TestCloneChangeEntityShallowCopyIsolatesMarshalPrepare(t *testing.T) {
	live := &domain.Route{
		Id:          1,
		RetryPolicy: &domain.RetryPolicy{Id: 1},
	}
	copied := cloneChangeEntity(live).(*domain.Route)
	assert.NotSame(t, live, copied)
	assert.Equal(t, live.Id, copied.Id)

	assert.NoError(t, copied.MarshalPrepare())
	assert.Nil(t, copied.RetryPolicy)
	assert.NotNil(t, live.RetryPolicy)
}

func TestGobEncodeRouteCycleSucceedsAfterMarshalPrepare(t *testing.T) {
	vh := &domain.VirtualHost{Id: 1, Name: "vh"}
	route := &domain.Route{Id: 1, VirtualHostId: 1, VirtualHost: vh, DeploymentVersion: "v1", Uuid: "u"}
	vh.Routes = []*domain.Route{route}

	assert.NoError(t, route.MarshalPrepare())
	assert.NotPanics(t, func() {
		err := gob.NewEncoder(&bytes.Buffer{}).Encode(route)
		assert.NoError(t, err)
	})
}

func Test_EncodingAndDecodingZeroValueOfCookieTTLInHashPolicy(t *testing.T) {
	var hashPolicy domain.HashPolicy
	hashPolicy.CookieName = "name"
	expectedValue := domain.NewNullInt(int64(0))
	hashPolicy.CookieTTL = expectedValue

	b := bytes.Buffer{}
	e := gob.NewEncoder(&b)
	err := e.Encode(hashPolicy)
	assert.Nil(t, err)

	bytesBuffer := bytes.NewBuffer(b.Bytes())
	decoder := gob.NewDecoder(bytesBuffer)
	restoredHashPolicy := &domain.HashPolicy{}
	err = decoder.Decode(restoredHashPolicy)
	assert.Nil(t, err)

	assert.Equal(t, "name", restoredHashPolicy.CookieName)
	assert.NotNil(t, restoredHashPolicy.CookieTTL)
	assert.True(t, hashPolicy.Equals(restoredHashPolicy))
}

// Reproduces "panic: reflect: slice index out of range": concurrent publishes share the same
// live *domain.Route, so pre-fix in-place MarshalPrepare raced with gob.Encode. Run with -race.
func TestBuildProtobufEvent_ConcurrentPublishDoesNotRaceOnSharedMemdbObjects(t *testing.T) {
	storage := ram.NewStorage()

	uuid, _ := uuid3.GenerateUUID()
	route := &domain.Route{
		Id:                1,
		Uuid:              uuid,
		VirtualHostId:     1,
		RouteKey:          "/api/v1",
		Prefix:            "/api/v1",
		ClusterName:       "cluster",
		DeploymentVersion: "v1",
		HeaderMatchers: []*domain.HeaderMatcher{
			{Id: 1, Name: "h1"},
			{Id: 2, Name: "h2"},
		},
	}
	route.RetryPolicy = &domain.RetryPolicy{Id: 1, Route: route}
	route.HeaderMatchers[0].Route = route
	route.HeaderMatchers[1].Route = route

	tx := storage.WriteTx()
	tx.TrackChanges()
	if err := storage.Save(tx, domain.RouteTable, []*domain.Route{route}); err != nil {
		t.Fatal(err)
	}
	changes := tx.Changes()
	tx.Commit()

	const (
		goroutines = 32
		iterations = 200
	)
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				event := events.NewChangeEventByNodeGroup(nodeGroup, changes)
				if _, err := buildProtobufEvent(event); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent publish must not fail: %v", err)
	}
}
