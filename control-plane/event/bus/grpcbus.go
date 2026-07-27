package bus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"sync"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/clustering"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/data"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/tlsmode"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util"
	"github.com/netcracker/qubership-core-lib-go/v3/utils"
	errors2 "github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// EventBusServerImpl provides implementation of EventBus gRPC service server stub.
//
// HOW-TO GENERATE GRPC STUBS:
// execute in control-plane-service folder (protoc must be installed):
//
// $ protoc -I event/bus/ event/bus/bus.proto --go_out=plugins=grpc:event/bus
type EventBusServerImpl struct {
	// subs holds subscriptions represented as channels for gRPCChannelMsg messages mapped on topics.
	// These channels are used to pass message (event) to thread that is communicating with specific subscriber,
	// so there is one channel for each subscriber on each his subscription.
	subs *SubscribersCache
	// chanBuffer specifies buffer size for every channel that represents subscription in subscribers cache.
	chanBuffer       int
	storage          data.RestorableStorage
	deferredRegistry DeferredMessagesRegistry
}

// gRPCChannelMsg is message that is passed to a subscription channel to be processed
// by thread that is communicating with specific subscriber.
type gRPCChannelMsg struct {
	// terminal indicates whether this is a message to terminate connection. If true, then event field is ignored.
	terminal bool
	// event holds event that needs to be sent to subscriber, serialized in protobuf.
	event *Event
}

var ErrConnGracefullyClosed = errors.New("bus: gRPC server gracefully closed connection")
var ErrSubscriberIsShutDown = errors.New("bus: gRPC subscriber is already shut down")

var getNodeMetadataFunc = GetMetadataWithNodeId

func NewEventBusServerImpl(chanBuffer int, storage data.RestorableStorage) *EventBusServerImpl {
	masterImpl := EventBusServerImpl{
		chanBuffer:       chanBuffer,
		subs:             NewSubscribersCache(),
		storage:          storage,
		deferredRegistry: *NewDeferredEventMessagesRegistry(util.DefaultRetryProvider{}.DeferredMessagesTTL()),
	}
	return &masterImpl
}

// Subscribe is server-side implementation of EventBus#Subscribe rpc defined in bus.proto.
func (s *EventBusServerImpl) Subscribe(topic *Topic, stream EventBus_SubscribeServer) error {
	clustering.CurrentNodeState.WaitMasterReady()
	log.Debugf("Received gRPC event bus subscription on topic %v", topic)
	// channel will be used by other threads to pass us messages so we can read them from channel and stream to the client
	ch := make(chan gRPCChannelMsg, s.chanBuffer)
	s.subs.Add(topic.Name, ch)
	defer s.subs.Remove(topic.Name, ch)

	if err := s.sendDeferredIfExists(topic, stream); err != nil {
		log.Warnf("Error at sending deferred messages: %v", err)
	}

	for {
		// we need to block forever until terminal event comes,
		// because if we exit from Subscribe function, gRPC connection will be closed
		event := <-ch
		if event.terminal {
			log.Info("Shutting down gRPC server stream")
			return nil
		}
		if err := stream.Send(event.event); err != nil {
			log.Errorf("Failed grpc event %v sending on topic %v: %v", event, topic, err)
			if deferErr := s.deferMessage(stream.Context(), event, topic); deferErr != nil {
				log.Warnf("Cannot defer message: %v", deferErr)
			}
			return err
		}
		log.Debugf("Successfully sent gRPC event %v on %s topic", event.event.EventType, topic)
	}
}

func (s *EventBusServerImpl) deferMessage(ctx context.Context, msg gRPCChannelMsg, topic *Topic) error {
	return s.deferredRegistry.Visit(func(r *DeferredMessagesRegistry) error {
		meta, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return errors.New("no client id was received")
		}
		return r.PushMessageForClient(ExtractNodeId(meta), topic.Name, msg)
	})
}

func (s *EventBusServerImpl) sendDeferredIfExists(topic *Topic, stream EventBus_SubscribeServer) error {
	var clientID string
	if meta, ok := metadata.FromIncomingContext(stream.Context()); ok {
		clientID = ExtractNodeId(meta)
	}
	if len(clientID) == 0 {
		log.Info("Client id is absent - cannot send possible deferred messages")
		return nil
	}
	return s.deferredRegistry.Visit(func(registry *DeferredMessagesRegistry) error {
		msgsForSent, err := registry.PopMessages(clientID, topic.Name)
		if err != nil {
			return err
		}
		for i, msg := range msgsForSent {
			if sendErr := stream.Send(msg.event); sendErr != nil {
				log.Errorf("Cannot send deferred message to `%s` client: %v", clientID, err)
				remainDeferred := msgsForSent[i:]
				if err := registry.PushMessagesForClient(clientID, topic.Name, remainDeferred); err != nil {
					return fmt.Errorf("%s: %w", "cannot push messages to registry for client `%s`", err)
				}
				return sendErr
			}
		}
		return nil
	})
}

func (s *EventBusServerImpl) GetLastSnapshot(ctx context.Context, empty *Empty) (*Event, error) {
	clustering.CurrentNodeState.WaitMasterReady()
	snapshot, err := s.storage.Backup()
	if err != nil {
		return nil, err
	}
	return buildProtobufEventFromSnapshot(snapshot)
}

func (s *EventBusServerImpl) mustEmbedUnimplementedEventBusServer() {
}

// GRPCBusPublisher is a gRPC-based implementation of the BusPublisher interface.
type GRPCBusPublisher struct {
	serviceImpl *EventBusServerImpl
	gRPCServer  *grpc.Server
	storage     data.RestorableStorage
	active      bool
}

func NewGRPCBusPublisher(storage data.RestorableStorage) *GRPCBusPublisher {
	publisher := &GRPCBusPublisher{storage: storage, active: false}
	return publisher
}

func (p *GRPCBusPublisher) Activate(address string) error {
	if !p.active {
		p.serviceImpl = NewEventBusServerImpl(10, p.storage)
		log.Infof("Starting gRPC server on addr %v", address)

		lis, err := net.Listen("tcp", address)
		if err != nil {
			return errors2.Wrap(err, fmt.Sprintf("Failed to listen on gRPC addr %v", address))
		}

		if tlsmode.GetMode() == tlsmode.Disabled {
			p.gRPCServer = grpc.NewServer()
		} else {
			p.gRPCServer = grpc.NewServer(grpc.Creds(credentials.NewTLS(utils.GetTlsConfig())))
		}
		RegisterEventBusServer(p.gRPCServer, p.serviceImpl)
		go func() {
			log.Infof("gRPC server shutdown: %v", p.gRPCServer.Serve(lis))
		}()
		p.active = true
	} else {
		return fmt.Errorf("attempt to activate gRPC publisher already activated with addr: %v", address)
	}
	return nil
}

func (p *GRPCBusPublisher) HasSubscribers() bool {
	return p.serviceImpl.subs.size > 0
}

func (p *GRPCBusPublisher) Publish(topic string, data interface{}) error {
	if p.active {
		log.Debugf("Publishing gRPC event bus message on topic %v", topic)
		marshalledData, err := buildProtobufEvent(data)
		if err != nil && err == ErrUnsupportedEvent {
			log.Debugf("Skip publishing on topic %s. There is unsupported event: %v", topic, data)
			return nil
		} else if err != nil {
			log.Errorf("Failed to marshall data %v to protobuf: %v", data, err)
			return err
		}
		p.serviceImpl.subs.ForEachSubInTopic(topic, func(sub interface{}) bool {
			log.Debugf("Notifying sub on topic %v", topic)
			sub.(chan gRPCChannelMsg) <- gRPCChannelMsg{
				terminal: false,
				event:    marshalledData,
			}
			log.Debugf("Sub on topic %v was notified", topic)
			return true
		})
	} else {
		log.Debugf("Attempt to publish data to topic '%v' when gRPC publisher inactive", topic)
	}
	return nil
}

func (p *GRPCBusPublisher) Shutdown() {
	if p.active {
		p.sendTerminalEvent()
		p.gRPCServer.GracefulStop()
		p.active = false
	} else {
		log.Warnf("Attempt to shutdown inactive gRPC publisher")
	}
}

func (p *GRPCBusPublisher) sendTerminalEvent() {
	p.serviceImpl.subs.ForEach(func(topic string, msgChanSlice []interface{}) bool {
		for _, msgChan := range msgChanSlice {
			// send terminal event to each subscription channel
			msgChan.(chan gRPCChannelMsg) <- gRPCChannelMsg{terminal: true}
		}
		return true
	})
}

func (p *GRPCBusPublisher) PurgeAllDeferredMessages(clustering.NodeInfo, clustering.Role) {
	if p.serviceImpl == nil { // not activated
		return
	}
	if err := p.serviceImpl.deferredRegistry.Visit(func(registry *DeferredMessagesRegistry) error {
		log.Infof("Clear all deferred messages because of node role change")
		registry.ClearAllMessages()
		return nil
	}); err != nil {
		log.Errorf("Clear process of deferred messages is failed with following error: %v", err)
		return
	}
	log.Infof("Deferred messages cleared")
}

// GRPCBusSubscriber is a gRPC-based implementation of the BusSubscriber interface.
type GRPCBusSubscriber struct {
	targetAddress string
	mutex         *sync.Mutex
	clientConn    *grpc.ClientConn
	// subscriberIsActive indicates that subscriber was not yet shut down
	// and needs to retry client connection in case of failure
	subscriberIsActive bool
	wg                 *sync.WaitGroup
	retryProvider      retryParamProvider
	cancelCtxFun       context.CancelFunc
	ctxWithCancel      context.Context
}

func NewGRPCBusSubscriber() *GRPCBusSubscriber {
	return &GRPCBusSubscriber{
		mutex:              &sync.Mutex{},
		wg:                 &sync.WaitGroup{},
		subscriberIsActive: false,
		retryProvider:      util.DefaultRetryProvider{}}
}

func NewStartedGRPCBusSubscriber(addr string) *GRPCBusSubscriber {
	subscriber := NewGRPCBusSubscriber()
	if err := subscriber.Activate(addr); err != nil {
		log.Errorf("Cannot create and start gRPC bus subscriber: %v", err)
	}
	return subscriber
}

func (s *GRPCBusSubscriber) Subscribe(topic string, handler func(data interface{})) {
	if s.subscriberIsActive {
		go s.subscribeWithRetry(topic, handler)
	} else {
		log.Warnf("Attempt to subscribe to '%s' topic when gRPC subscriber is not active.", topic)
	}
}

func (s *GRPCBusSubscriber) Activate(address string) error {
	if !s.subscriberIsActive {
		s.targetAddress = address
		s.subscriberIsActive = true
	} else {
		return fmt.Errorf("attempt to activate gRPC suscriber already activated with addr: %v", address)
	}
	return nil
}

func (s *GRPCBusSubscriber) GetSnapshot() (*data.Snapshot, error) {
	if s.subscriberIsActive {
		attempts := s.retryProvider.AttemptAmountGetSnapshot()
		for i := 0; s.subscriberIsActive && i < attempts; i++ {
			if snapshot, err := s.tryGetSnapshot(); err != nil {
				log.Errorf("%v", err)
				time.Sleep(s.retryProvider.SleepPeriodGetSnapshot())
				continue
			} else {
				return snapshot, nil
			}
		}
		return nil, fmt.Errorf("getting snapshot attempt limit exceeded")
	} else {
		return nil, fmt.Errorf("attempt to get snapshot when gRPC subscriber is not active")
	}
}

func (s *GRPCBusSubscriber) dial() (*grpc.ClientConn, error) {
	if tlsmode.GetMode() == tlsmode.Disabled {
		return grpc.Dial(
			s.targetAddress,
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(math.MaxInt32)),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultServiceConfig(loadRetryConfig()))
	} else {
		tlsConfig := util.GetTlsConfigWithoutHostNameValidation()
		return grpc.Dial(
			s.targetAddress,
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(math.MaxInt32)),
			grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
			grpc.WithDefaultServiceConfig(loadRetryConfig()))
	}
}

func (s *GRPCBusSubscriber) tryGetSnapshot() (*data.Snapshot, error) {
	conn, err := s.dial()
	defer func() {
		if conn != nil {
			if err := conn.Close(); err != nil {
				log.Errorf("Can't close connection. Cause: %v", err)
			}
		}
	}()
	if err != nil {
		return nil, errors2.Wrap(err, "failed to dial client gRPC conn")
	}
	client := NewEventBusClient(conn)
	event, err := client.GetLastSnapshot(context.TODO(), &Empty{})
	if err != nil {
		return nil, err
	}
	snapshot, err := readSnapshotFromProtoEvent(event)
	if err != nil {
		return nil, err
	}
	log.Debugf("Getting snapshot has been done successfully")
	return snapshot, nil
}

func (s *GRPCBusSubscriber) subscribeWithRetry(topic string, handler func(data interface{})) {
	for s.subscriberIsActive {
		s.wg.Add(1)
		s.subscribeWithRecovery(topic, handler)
		time.Sleep(s.retryProvider.SleepPeriodSubscribe())
		log.Infof("completing subscribeWithRetry %v", topic)
		s.wg.Done()
	}
}

func (s *GRPCBusSubscriber) subscribeWithRecovery(topic string, handler func(data interface{})) {
	defer s.recoverSubscriberPanic()
	s.subscribeInternal(topic, handler)
}

func (s *GRPCBusSubscriber) recoverSubscriberPanic() {
	if recoveryMessage := recover(); recoveryMessage != nil {
		log.Warnf("Recovered gRPC event s subscriber panic: %v", recoveryMessage)
	}
	log.Debug("Executed gRPC event s subscriber recovery function")
}

func (s *GRPCBusSubscriber) openConnection() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.subscriberIsActive {
		log.Debug("Tried to open gRPC connection for already shut down subscriber")
		panic(ErrSubscriberIsShutDown)
	}

	conn, err := s.dial()
	if err != nil {
		log.Errorf("Failed to dial client gRPC conn: %v", err)
		panic(err)
	}
	s.clientConn = conn
}

func (s *GRPCBusSubscriber) closeConnection() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.cancelContext()
	if s.clientConn != nil {
		_ = s.clientConn.Close()
	}
}

func (s *GRPCBusSubscriber) subscribeInternal(topic string, handler func(data interface{})) {
	log.Infof("Subscribe on server gRPC events on topic %v from %s", topic, s.targetAddress)
	s.openConnection()
	defer s.closeConnection()

	ctx := s.getCancelableCtx()
	client := NewEventBusClient(s.clientConn)

	if meta, err := getNodeMetadataFunc(); err == nil {
		ctx = metadata.NewOutgoingContext(ctx, meta)
		log.Infof("Subscription will be sent with node id metadata, value: %s", ExtractNodeId(meta))
	} else {
		log.Warnf("Cannot get metadata with node id: %v", err)
	}

	stream, err := client.Subscribe(ctx, &Topic{Name: topic})
	if err != nil {
		logSubscribeError("gRPC client failed to subscribe", err)
		panic(err)
	}
	log.Infof("gRPC client successfully subscribed on topic %v", topic)

	for {
		event, err := stream.Recv()
		log.Debugf("event=%v", event)
		if err != nil {
			if err == io.EOF {
				log.Infof("gRPC client received EOF: %v", err)
				panic(ErrConnGracefullyClosed)
			}
			logSubscribeError("gRPC client failed to receive event", err)
			panic(err)
		}
		if data, err := readProtobufEvent(event); err == nil && data != nil {
			log.Debugf("handling data: %v", data)
			handler(data)
		} else if err == ErrUnsupportedEvent {
			log.Warnf("Event bus gRPC client doesn't support events of type %v, skip handling", event.EventType)
			continue
		} else if err != nil {
			panic(err)
		}
	}
}

// logSubscribeError logs a subscriber connectivity error at the appropriate level.
// When the master pod restarts, the subscribeWithRetry loop loses its connection and
// keeps retrying until the master is back. The resulting errors (context canceled,
// connection refused, deadline exceeded) are expected and self-healed by the retry loop,
// so they are logged at WARN to avoid false-alarm ERRORs in log.
// Any other status code still indicates a real problem and is logged at ERROR.
func logSubscribeError(msg string, err error) {
	if isExpectedTransientSubscribeError(err) {
		log.Warnf("%s (will retry): %v", msg, err)
	} else {
		log.Errorf("%s: %v", msg, err)
	}
}

// isExpectedTransientSubscribeError reports whether err is an expected transient
// connectivity error that the subscribeWithRetry loop recovers from on its own,
// typically caused by the master pod restarting.
func isExpectedTransientSubscribeError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	switch status.Code(err) {
	case codes.Canceled, codes.Unavailable, codes.DeadlineExceeded:
		return true
	default:
		return false
	}
}

// loadRetryConfig provides retry configuration for gRPC service "EventBus",
// BUT THIS RETRY POLICY IS USED ONLY ON BUSINESS LAYER!
// It means that in most cases (connectivity and availability issues) this retryPolicy is not applied so it's almost useless.
func loadRetryConfig() string {
	return `{
            "methodConfig": [{
                "name": [{"service": "EventBus"}],
                "waitForReady": true,

                "retryPolicy": {
                    "MaxAttempts": 100,
                    "InitialBackoff": ".01s",
                    "MaxBackoff": ".01s",
                    "BackoffMultiplier": 1.0,
                    "RetryableStatusCodes": [ "UNAVAILABLE", "DEADLINE_EXCEEDED", "CANCELLED", "UNKNOWN", "INTERNAL", "RESOURCE_EXHAUSTED" ]
                }
            }]
        }`
}

func (s *GRPCBusSubscriber) Shutdown() {
	if s.subscriberIsActive {
		s.subscriberIsActive = false // prevent subscribers from retrying when they lose connection after we close it.
		log.Infof("Closing  gRPC connection for client: %v", s.clientConn)
		s.closeConnection()
		s.wg.Wait()
	} else {
		log.Warnf("Attempt to shutdown inactive gRPC subscriber")
	}
}

func (s *GRPCBusSubscriber) getCancelableCtx() context.Context {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.cancelCtxFun == nil {
		s.ctxWithCancel, s.cancelCtxFun = context.WithCancel(context.Background())
		s.runRoutine2CleanCancelCtxFunOnDoneFromContext()
	}
	return s.ctxWithCancel
}

func (s *GRPCBusSubscriber) runRoutine2CleanCancelCtxFunOnDoneFromContext() {
	go func() {
		select {
		case <-s.ctxWithCancel.Done():
			log.Debugf("context for bus subscriber %v is cancelled", s.targetAddress)
			s.cancelCtxFun = nil
		}
	}()
}

func (s *GRPCBusSubscriber) cancelContext() {
	log.Debugf("calling cancel context for bus subscriber %v: %v", s.targetAddress, s.cancelCtxFun)
	if s.cancelCtxFun != nil {
		s.cancelCtxFun()
	}
}

// DeferredMessagesRegistry is an expiring cache that can be used to store events for the client subscribers that failed
// to receive events previously
type DeferredMessagesRegistry struct {
	MessagesCache *ttlcache.Cache[string, []gRPCChannelMsg]
	sync.Mutex
}

var ErrClientIdAbsent = errors.New("cannot perform defer msg operations: client id is not set")

func NewDeferredEventMessagesRegistry(ttl time.Duration) *DeferredMessagesRegistry {
	cache := ttlcache.New[string, []gRPCChannelMsg](
		ttlcache.WithTTL[string, []gRPCChannelMsg](ttl),
	)
	registry := DeferredMessagesRegistry{
		MessagesCache: cache,
		Mutex:         sync.Mutex{},
	}
	cache.OnInsertion(registry.onNewItem)
	cache.OnEviction(registry.onExpiration)

	return &registry
}

// PushMessagesForClient pushes future messages for a particular client without TTL update
func (d *DeferredMessagesRegistry) PushMessagesForClient(clientID, topicName string, messages []gRPCChannelMsg) error {
	log.Infof("Defer %d messages for client %s at %s topic", len(messages), clientID, topicName)
	for _, msg := range messages {
		if err := d.PushMessageForClient(clientID, topicName, msg); err != nil {
			return err
		}
	}
	return nil
}

func (d *DeferredMessagesRegistry) constructCacheKey(clientID, topic string) string {
	return fmt.Sprintf("%s-%s", topic, clientID)
}

func (d *DeferredMessagesRegistry) PushMessageForClient(clientID, topicName string, message gRPCChannelMsg) error {
	if len(clientID) == 0 {
		return ErrClientIdAbsent
	}
	if message.terminal { // Do not send because we want to terminate connections only when they are alive, deferred closing behaviour is unacceptable
		return nil
	}
	log.Debugf("Defer message for client %s and %s topic", clientID, topicName)
	cacheKey := d.constructCacheKey(clientID, topicName)

	cacheValue := d.MessagesCache.Get(cacheKey, ttlcache.WithDisableTouchOnHit[string, []gRPCChannelMsg]())
	if cacheValue == nil {
		d.MessagesCache.Set(cacheKey, []gRPCChannelMsg{message}, ttlcache.DefaultTTL)
	} else {
		d.MessagesCache.Set(cacheKey, append(cacheValue.Value(), message), ttlcache.DefaultTTL)
	}
	return nil
}

// Visit for locking purposes
func (d *DeferredMessagesRegistry) Visit(operation func(registry *DeferredMessagesRegistry) error) error {
	d.Lock()
	defer d.Unlock()
	return operation(d)
}

func (d *DeferredMessagesRegistry) PopMessages(clientID, topicName string) ([]gRPCChannelMsg, error) {
	cacheKey := d.constructCacheKey(clientID, topicName)
	if len(cacheKey) == 0 {
		return []gRPCChannelMsg{}, ErrClientIdAbsent
	}
	messages := d.MessagesCache.Get(cacheKey)

	if messages == nil {
		return nil, nil
	}
	d.MessagesCache.Delete(cacheKey)
	log.Debugf("%d messages is pop out for client %s at %s topic", len(messages.Value()), cacheKey, topicName)

	return messages.Value(), nil
}

func (d *DeferredMessagesRegistry) ClearAllMessages() {
	d.MessagesCache.DeleteAll()
}

func (d *DeferredMessagesRegistry) onNewItem(_ context.Context, item *ttlcache.Item[string, []gRPCChannelMsg]) {
	log.Infof("Add message with `%s` key to deferred messages registry", item.Key())
}

func (d *DeferredMessagesRegistry) onExpiration(_ context.Context, evictionReason ttlcache.EvictionReason, item *ttlcache.Item[string, []gRPCChannelMsg]) {
	log.Infof("Cleanup deferred messages registry for `%s` key, discarded %d messages with reason %v",
		item.Key(), len(item.Value()), evictionReason)
}

type retryParamProvider interface {
	DeferredMessagesTTL() time.Duration
	SleepPeriodSubscribe() time.Duration
	SleepPeriodGetSnapshot() time.Duration
	AttemptAmountGetSnapshot() int
}
