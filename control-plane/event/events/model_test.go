package events

import (
	"control-plane/domain"
    "encoding/gob"
    "runtime"
    "sync"
    "sync/atomic"
	"testing"
)

func TestChangeEventMarshalPrepare(t *testing.T) {
	pr := &mockPreparer{}
	changes := map[string][]memdb.Change{
		"A": {{Before: pr}},
	}
	event := &ChangeEvent{Changes: changes}
	assert.False(t, pr.called)
    clone := event.Changes["A"][0].Before.(*mockPreparer)
    assert.NotSame(t, pr, clone)
    assert.True(t, clone.called)
}

func TestChangeEventToString(t *testing.T) {
	pr := &mockPreparer{}
	changes := map[string][]memdb.Change{
		"A": {{Before: pr}},
	}
	event := &ChangeEvent{Changes: changes}
	assert.Contains(t, event.ToString(), "{ChangeEvent: nodeGroup='', changes=[\nA: [&{false}, nil, ], \n]}")
}

func TestMultipleChangeEventMarshalPrepare(t *testing.T) {
	pr := &mockPreparer{}
	changes := map[string][]memdb.Change{
		"A": {{Before: pr}},
	}
	event := &MultipleChangeEvent{Changes: changes}
	assert.Nil(t, event.MarshalPrepare())
	assert.False(t, pr.called)
    clone := event.Changes["A"][0].Before.(*mockPreparer)
    assert.NotSame(t, pr, clone)
    assert.True(t, clone.called)
}

func TestReloadEventMarshalPrepare(t *testing.T) {
	pr := &mockPreparer{}
	changes := map[string][]memdb.Change{
		"A": {{Before: pr}},
	}
	event := &ReloadEvent{Changes: changes}
	assert.Nil(t, event.MarshalPrepare())
	assert.False(t, pr.called)
    clone := event.Changes["A"][0].Before.(*mockPreparer)
    assert.NotSame(t, pr, clone)
    assert.True(t, clone.called)
}
func TestChangeEventMarshalPrepare_ClonesBeforeAndAfter(t *testing.T) {
	before := &mockPreparer{}
	after := &mockPreparer{}
	event := &ChangeEvent{
		Changes: map[string][]memdb.Change{
			"A": {{Before: before, After: after}},
		},
	}
	assert.Nil(t, event.MarshalPrepare())

	assert.False(t, before.called)
	assert.False(t, after.called)

	beforeClone := event.Changes["A"][0].Before.(*mockPreparer)
	afterClone := event.Changes["A"][0].After.(*mockPreparer)
	assert.NotSame(t, before, beforeClone)
	assert.NotSame(t, after, afterClone)
	assert.True(t, beforeClone.called)
	assert.True(t, afterClone.called)
}

func TestChangeEventMarshalPrepare_ErrorWhenNotMarshalPreparer(t *testing.T) {
	event := &ChangeEvent{
		Changes: map[string][]memdb.Change{
			"A": {{Before: "not a domain entity"}},
		},
	}
	err := event.MarshalPrepare()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "MarshalPreparer")
}

func TestChangeEventMarshalPrepare_DoesNotMutateSharedRouteConfiguration(t *testing.T) {
	rc := &domain.RouteConfiguration{
		Id:           32,
		Name:         "egress-gateway-routes",
		VirtualHosts: []*domain.VirtualHost{{Id: 61, Name: "vh"}},
		NodeGroup:    &domain.NodeGroup{Name: "egress-gateway"},
	}

	event := &ChangeEvent{
		Changes: map[string][]memdb.Change{
			"route_configurations": {{After: rc}},
		},
	}
	assert.Nil(t, event.MarshalPrepare())

	assert.NotNil(t, rc.VirtualHosts)
	assert.NotNil(t, rc.NodeGroup)

	clone := event.Changes["route_configurations"][0].After.(*domain.RouteConfiguration)
	assert.NotSame(t, rc, clone)
	assert.Nil(t, clone.VirtualHosts)
	assert.Nil(t, clone.NodeGroup)

	enc := gob.NewEncoder(discardWriter{})
	assert.Nil(t, enc.Encode(clone))
}

// Concurrent reader must see a stable memdb-shared rc while MarshalPrepare runs on clones.
// firstRead avoids flaky scheduling on single-CPU CI agents.
func TestChangeEventMarshalPrepare_SafeUnderConcurrentReader(t *testing.T) {
	rc := &domain.RouteConfiguration{
		Id:           1,
		Name:         "rc",
		VirtualHosts: []*domain.VirtualHost{{Id: 1, Name: "vh"}},
		NodeGroup:    &domain.NodeGroup{Name: "ng"},
	}
	event := &ChangeEvent{
		Changes: map[string][]memdb.Change{
			"route_configurations": {{After: rc}},
		},
	}

	var (
		reads     atomic.Int64
		stop      atomic.Bool
		firstRead = make(chan struct{})
		once      sync.Once
		readerWg  sync.WaitGroup
	)
	readerWg.Add(1)
	go func() {
		defer readerWg.Done()
		for !stop.Load() {
			if rc.VirtualHosts != nil && rc.NodeGroup != nil {
				reads.Add(1)
				once.Do(func() { close(firstRead) })
			}
			runtime.Gosched()
		}
	}()

	<-firstRead

	for i := 0; i < 1000; i++ {
		assert.Nil(t, event.MarshalPrepare())
		if i%50 == 0 {
			runtime.Gosched()
		}
	}
	stop.Store(true)
	readerWg.Wait()

	assert.Greater(t, reads.Load(), int64(0))
	assert.NotNil(t, rc.VirtualHosts)
	assert.NotNil(t, rc.NodeGroup)
}

type mockPreparer struct {
	called bool
}
type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

func (m *mockPreparer) MarshalPrepare() error {
	m.called = true
	return nil
}
