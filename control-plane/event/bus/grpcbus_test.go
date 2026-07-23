package bus

import (
	"context"
	"errors"
	"fmt"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/clustering"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/data"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/events"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"sync"
	"testing"
	"time"
)

func TestIsExpectedTransientSubscribeError(t *testing.T) {
	// expected transient errors during master pod restart -> not ERROR
	assert.True(t, isExpectedTransientSubscribeError(context.Canceled))
	assert.True(t, isExpectedTransientSubscribeError(context.DeadlineExceeded))
	assert.True(t, isExpectedTransientSubscribeError(status.Error(codes.Canceled, "context canceled while waiting for connections to become ready")))
	assert.True(t, isExpectedTransientSubscribeError(status.Error(codes.Unavailable, "connection refused")))
	assert.True(t, isExpectedTransientSubscribeError(status.Error(codes.DeadlineExceeded, "deadline")))

	// genuine errors stay ERROR
	assert.False(t, isExpectedTransientSubscribeError(status.Error(codes.Internal, "boom")))
	assert.False(t, isExpectedTransientSubscribeError(status.Error(codes.Unauthenticated, "no creds")))
	assert.False(t, isExpectedTransientSubscribeError(errors.New("plain error")))
}

func TestALotOfGrpcSubs(t *testing.T) {
	getNodeMetadataFunc = func() (metadata.MD, error) {
		return nil, errors.New("test without node id")
	}

	masterBusPort := findFreePort()
	log.Infof("Event bus test will be using free port %v", masterBusPort)
	clustering.CurrentNodeState.SetMasterReady()
	pub := NewGRPCBusPublisher(nil)
	if err := pub.Activate(fmt.Sprintf("0.0.0.0:%d", masterBusPort)); err != nil {
		t.Errorf("Cannot create and start gRPC bus publisher: %v", err)
	}
	defer pub.Shutdown()

	const topic = "test-topic"
	const totalSubs = 20
	const totalPublishedEvents = 20

	log.Infof("TestALotOfGrpcSubs phase 1")
	testLoadOnGrpcBus(t, pub, topic, masterBusPort, totalSubs, totalPublishedEvents)

	// repeat
	log.Infof("TestALotOfGrpcSubs phase 2")
	testLoadOnGrpcBus(t, pub, topic, masterBusPort, totalSubs, totalPublishedEvents)
}

func Test_GivenTwoTopicInSubscriber_OnShutdownSubscriber_ConnectionsAreCancelledAndClosed(t *testing.T) {
	getNodeMetadataFunc = func() (metadata.MD, error) {
		return nil, errors.New("test without node id")
	}

	masterBusPort := findFreePort()
	log.Infof("Event bus test will be using free port %v", masterBusPort)
	clustering.CurrentNodeState.SetMasterReady()
	pub := NewGRPCBusPublisher(nil)
	if err := pub.Activate(fmt.Sprintf("0.0.0.0:%d", masterBusPort)); err != nil {
		t.Errorf("Cannot create and start gRPC bus publisher: %v", err)
	}
	defer pub.Shutdown()

	topics := []string{"test-topic-1", "test-topic-2"}

	sub := newTestSubWithMultipleTopics(fmt.Sprintf("127.0.0.1:%d", masterBusPort), topics)
	log.Infof("Waiting for everybody to subscribe...")
	waitUntilDesiredNumberOfSubs(t, pub, len(topics)) // wait for everybody to subscribe

	waitUntilTimeout(t,
		func() { sub.sub.Shutdown() },
		10*time.Second)
}

func TestSendDeferredMessages_whenExists_thenTerminate(t *testing.T) {
	eventBusServer := NewEventBusServerImpl(1, nil)
	pub := NewGRPCBusPublisher(nil)
	pub.active = true
	pub.serviceImpl = eventBusServer

	clustering.CurrentNodeState.SetMasterReady()

	meta, err := GetMetadataWithNodeId()
	assert.Nil(t, err)
	assert.NotNil(t, meta)

	clientNodeID := ExtractNodeId(meta)
	assert.True(t, len(clientNodeID) > 0)

	topic := &Topic{Name: TopicChanges}
	deferredMsg := gRPCChannelMsg{event: &Event{EventType: EventType_CHANGE, Data: nil}}
	err = eventBusServer.deferredRegistry.PushMessageForClient(clientNodeID, topic.Name, deferredMsg)
	assert.Nil(t, err)

	// Incoming context instead of OutgoingContext because of internal implementation
	ctx := metadata.NewIncomingContext(context.Background(), meta)
	stream := newMockEventBus_SubscribeServer(false, ctx)

	subscribeIsOver := make(chan struct{})

	go func() {
		err := eventBusServer.Subscribe(topic, &stream)
		assert.Nil(t, err)
		subscribeIsOver <- struct{}{}
	}()

	waitUntilDesiredNumberOfSubs(t, pub, 1)
	pub.sendTerminalEvent()
	<-subscribeIsOver

	assert.Equal(t, deferredMsg.event, stream.sentMessages[0])
	remainedMessages, err := eventBusServer.deferredRegistry.PopMessages(clientNodeID, topic.Name)

	assert.Nil(t, err)
	assert.Empty(t, remainedMessages)
}

func TestSendDeferredMessages_WhenExists_DoNotClashesWithDifferentTopic(t *testing.T) {
	eventBusServer := NewEventBusServerImpl(1, nil)
	pub := NewGRPCBusPublisher(nil)
	pub.active = true
	pub.serviceImpl = eventBusServer

	clustering.CurrentNodeState.SetMasterReady()

	meta, err := GetMetadataWithNodeId()
	assert.Nil(t, err)
	assert.NotNil(t, meta)

	clientNodeID := ExtractNodeId(meta)
	assert.True(t, len(clientNodeID) > 0)

	topicWithDeferredMessage := &Topic{Name: TopicChanges}
	deferredMsg := gRPCChannelMsg{event: &Event{EventType: EventType_CHANGE, Data: nil}}
	err = eventBusServer.deferredRegistry.PushMessageForClient(clientNodeID, topicWithDeferredMessage.Name, deferredMsg)
	assert.Nil(t, err)

	// Incoming context instead of OutgoingContext because of internal implementation
	ctx := metadata.NewIncomingContext(context.Background(), meta)
	stream := newMockEventBus_SubscribeServer(false, ctx)

	topicWithoutDeferredMessages := &Topic{Name: "another-topic-without-deferred-message"}

	subscribeIsOver := make(chan struct{})
	go func() {
		err := eventBusServer.Subscribe(topicWithoutDeferredMessages, &stream)
		assert.Nil(t, err)
		subscribeIsOver <- struct{}{}
	}()

	waitUntilDesiredNumberOfSubs(t, pub, 1)
	pub.sendTerminalEvent()
	<-subscribeIsOver

	assert.Empty(t, stream.sentMessages)

	pub.PurgeAllDeferredMessages(clustering.NodeInfo{}, clustering.Master)
}

func TestDeferredMessageMustBeSent_whenFirstSendOnSubscriptionFails(t *testing.T) {
	eventBusServer := NewEventBusServerImpl(1, nil)
	pub := NewGRPCBusPublisher(nil)
	pub.active = true
	pub.serviceImpl = eventBusServer

	clustering.CurrentNodeState.SetMasterReady()

	meta, err := GetMetadataWithNodeId()
	assert.Nil(t, err)
	assert.NotNil(t, meta)

	clientNodeID := ExtractNodeId(meta)
	assert.True(t, len(clientNodeID) > 0)
	ctx := metadata.NewIncomingContext(context.Background(), meta)

	badStream := newMockEventBus_SubscribeServer(true, ctx)
	topic := &Topic{Name: TopicChanges}
	subscribeIsOver := make(chan struct{})

	deferredMsgEvent := &events.ChangeEvent{NodeGroup: "test-node-group"}
	assert.Len(t, eventBusServer.deferredRegistry.MessagesCache.Keys(), 0)

	go func() {
		err := eventBusServer.Subscribe(topic, &badStream)
		assert.NotNil(t, err)
		subscribeIsOver <- struct{}{}
	}()

	waitUntilDesiredNumberOfSubs(t, pub, 1)
	err = pub.Publish(topic.Name, deferredMsgEvent)
	assert.Nil(t, err)
	<-subscribeIsOver
	assert.Len(t, badStream.sentMessages, 1)
	assert.Len(t, eventBusServer.deferredRegistry.MessagesCache.Keys(), 1)

	goodStream := newMockEventBus_SubscribeServer(false, ctx)
	go func() {
		err := eventBusServer.Subscribe(topic, &goodStream)
		assert.Nil(t, err)
		subscribeIsOver <- struct{}{}
	}()

	waitUntilDesiredNumberOfSubs(t, pub, 1)
	pub.sendTerminalEvent()
	<-subscribeIsOver

	assert.Len(t, goodStream.sentMessages, 1)
	sentEvent, err := readProtobufEvent(goodStream.sentMessages[0])
	assert.Nil(t, err)
	assert.Equal(t, sentEvent.(*events.ChangeEvent).NodeGroup, deferredMsgEvent.NodeGroup)

	remainedMessages, err := eventBusServer.deferredRegistry.PopMessages(clientNodeID, topic.Name)

	assert.Nil(t, err)
	assert.Empty(t, remainedMessages)
}

func testLoadOnGrpcBus(t *testing.T, pub *GRPCBusPublisher, topic string, busPort, totalSubs, totalPublishedEvents int) {
	subs := make([]*testSub, 0, totalSubs)
	for idx := 0; idx < totalSubs; idx++ {
		sub := newTestSub(fmt.Sprintf("127.0.0.1:%d", busPort), topic)
		subs = append(subs, sub)
	}

	log.Infof("Waiting for everybody to subscribe...")

	// wait for everybody to subscribe
	waitUntilDesiredNumberOfSubs(t, pub, totalSubs)

	log.Infof("Sending %v events to all subs...", totalPublishedEvents)
	for currentEvent := 0; currentEvent < totalPublishedEvents; currentEvent++ {
		log.Debugf("Publishing test event: %v", currentEvent)
		err := pub.Publish(topic, &data.Snapshot{})
		assert.Nil(t, err)
		time.Sleep(50 * time.Millisecond)
	}

	// wait for everybody to receive events
	log.Infof("Waiting for everybody to receive events...")

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && !checkAllSubsGotEvents(&subs, totalPublishedEvents) {
		time.Sleep(100 * time.Millisecond)
	}

	for _, sub := range subs {
		assert.Equal(t, totalPublishedEvents, sub.getReceivedEventsNum())
		log.Debugf("Shutting down sub...")
		sub.sub.Shutdown()
	}

	err := pub.Publish(topic, &data.Snapshot{}) // publish event to trigger cleanup of dead subs
	assert.Nil(t, err)
	waitUntilDesiredNumberOfSubs(t, pub, 0)
}

func waitUntilTimeout(t *testing.T, f func(), duration time.Duration) {
	quitCh := make(chan bool, 1)
	go func() {
		f()
		quitCh <- true
	}()
	select {
	case <-time.After(duration):
		assert.Fail(t, "Timeout reached.")
	case <-quitCh:
		break
	}
}

func waitUntilDesiredNumberOfSubs(t *testing.T, pub *GRPCBusPublisher, desiredSubsNum int) {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && int(pub.serviceImpl.subs.size) != desiredSubsNum {
		time.Sleep(100 * time.Millisecond)
	}
	assert.Equal(t, desiredSubsNum, int(pub.serviceImpl.subs.size))
}

func checkAllSubsGotEvents(subs *[]*testSub, expectedEventsNum int) bool {
	for _, sub := range *subs {
		if sub.getReceivedEventsNum() != expectedEventsNum {
			return false
		}
	}
	log.Infof("All sub got expected number of events (%d)", expectedEventsNum)
	return true
}

type testSub struct {
	sub            *GRPCBusSubscriber
	receivedEvents int
	mutex          *sync.Mutex
}

func (ts *testSub) getReceivedEventsNum() int {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()
	return ts.receivedEvents
}

func newTestSub(busAddr, topic string) *testSub {
	sub := NewStartedGRPCBusSubscriber(busAddr)
	this := testSub{
		sub:            sub,
		receivedEvents: 0,
		mutex:          &sync.Mutex{},
	}
	sub.Subscribe(topic, func(data interface{}) {
		log.Debug("Got into subs callback")
		log.Debugf("Got event on topic %s: %v", topic, data)
		this.mutex.Lock()
		defer this.mutex.Unlock()
		this.receivedEvents++
	})
	return &this
}

func newTestSubWithMultipleTopics(busAddr string, topics []string) *testSub {
	sub := NewStartedGRPCBusSubscriber(busAddr)
	this := testSub{
		sub:            sub,
		receivedEvents: 0,
		mutex:          &sync.Mutex{},
	}
	for _, topic := range topics {
		sub.Subscribe(topic, func(data interface{}) {
			log.Debug("Got into subs callback")
			log.Debugf("Got event on topic %s: %v", topic, data)
			this.mutex.Lock()
			defer this.mutex.Unlock()
			this.receivedEvents++
		})
	}
	return &this
}

type mockEventBus_SubscribeServer struct {
	sendMustFail bool
	ctx          context.Context
	sentMessages []*Event
}

func newMockEventBus_SubscribeServer(sendMustFail bool, ctx context.Context) mockEventBus_SubscribeServer {
	return mockEventBus_SubscribeServer{
		sentMessages: make([]*Event, 0),
		sendMustFail: sendMustFail,
		ctx:          ctx,
	}
}

func (m *mockEventBus_SubscribeServer) Send(event *Event) error {
	m.sentMessages = append(m.sentMessages, event)
	if m.sendMustFail {
		return errors.New("cannot send event")
	}
	return nil
}

func (m *mockEventBus_SubscribeServer) SetHeader(md metadata.MD) error {
	return nil
}

func (m *mockEventBus_SubscribeServer) SendHeader(md metadata.MD) error {
	return nil
}

func (m *mockEventBus_SubscribeServer) SetTrailer(md metadata.MD) {

}

func (m *mockEventBus_SubscribeServer) Context() context.Context {
	return m.ctx
}

func (m *mockEventBus_SubscribeServer) SendMsg(msg interface{}) error {
	return nil
}

func (m *mockEventBus_SubscribeServer) RecvMsg(msg interface{}) error {
	return nil
}
