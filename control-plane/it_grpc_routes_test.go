package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/lib"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	asrt "github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoimpl"
	"google.golang.org/protobuf/types/known/anypb"
	"io"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"
)

func Test_IT_Grpc_Routing(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	config := `apiVersion: nc.core.mesh/v3
kind: RouteConfiguration
metadata:
 name: trace-service-with-grpc-routes
 namespace: ''
spec:
 gateways:
   - internal-gateway-service
 virtualServices:
 - name: internal-gateway-service
   hosts: ["*"]
   routeConfiguration:
     routes:
     - destination:
         cluster: test-service
         endpoint: test-service-v1:8080
       rules:
       - match:
           prefix: /api/v1/test-service
         prefixRewrite: /api/v1
     - destination:
         cluster: test-service
         endpoint: test-service-v1:8888
         httpVersion: 2
       rules:
       - match:
           prefix: /org.qubership.mesh.v3.test.bus`

	const cluster1 = "test-service"
	traceSrvContainer1 := createTraceServiceContainer(cluster1, "v1", true)
	defer traceSrvContainer1.Purge()

	internalGateway.ApplyConfigAndWait(assert, 60*time.Second, config)

	const TestTopic = "it_grpc_routes_test"
	receivedEvent := false
	receivedEventRef := &receivedEvent

	// test Subscribe()

	grpcSub := NewStartedGRPCBusSubscriber(internalGateway.HostAndPort)
	grpcSub.Subscribe(TestTopic, func(data interface{}) {
		var msg TraceResponse
		assert.Nil(json.Unmarshal(data.([]byte), &msg))
		log.InfoC(ctx, "Got gRPC event: %+v", msg)
		assert.Equal(cluster1, msg.FamilyName)
		assert.Equal("v1", msg.Version)
		// set shared variable receivedEventRef so the main thread can check whether the event was received
		*receivedEventRef = true
	})

	req, err := http.NewRequest(http.MethodPost, internalGateway.Url+"/api/v1/test-service/bus/topics/"+TestTopic, nil)
	assert.Nil(err)
	_, statusCode := SendToTraceSrvWithRetry503(assert, req)
	assert.Equal(200, statusCode)

	deadline := time.Now().Add(60 * time.Second)
	for !*receivedEventRef && time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)

		// tell test-service to publish event on topic
		req, err := http.NewRequest(http.MethodPost, internalGateway.Url+"/api/v1/test-service/bus/topics/"+TestTopic, nil)
		assert.Nil(err)
		_, statusCode := SendToTraceSrvWithRetry503(assert, req)
		assert.Equal(200, statusCode)
	}
	assert.True(*receivedEventRef)

	// test GetLastSnapshot()

	client := NewTestEventBusClient(grpcSub.clientConn)
	event, err := client.GetLastSnapshot(ctx, &Empty{})
	assert.Nil(err)
	testSnapshot, err := readRawBytesDataFromProto(event)
	assert.Nil(err)
	assert.NotNil(testSnapshot)
	log.InfoC(ctx, "Received event %v on gRPC GetLastSnapshot() call", string(testSnapshot))

	grpcSub.Shutdown()

	// cleanup routes
	internalGateway.DeleteRoutesAndWait(assert, 60*time.Second, dto.RouteDeleteRequestV3{
		Gateways:       []string{"internal-gateway-service"},
		VirtualService: "internal-gateway-service",
		RouteDeleteRequest: dto.RouteDeleteRequest{
			Routes: []dto.RouteDeleteItem{
				{Prefix: "/api/v1/test-service"},
				{Prefix: "/org.qubership.mesh.v3.test.bus"},
			},
			Version: "v1",
		},
	})
}

func Test_IT_Grpc_HttpVersionOverwrite(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	const clusterName = "test-cluster-4httpver"
	traceSrvContainer1 := createTraceServiceContainer(clusterName, "v1", true)
	defer traceSrvContainer1.Purge()

	// register route without specifying http version for this cluster
	config := `apiVersion: nc.core.mesh/v3
kind: RouteConfiguration
metadata:
 name: trace-service-http-routes
 namespace: ''
spec:
 gateways:
   - internal-gateway-service
 virtualServices:
 - name: internal-gateway-service
   hosts: ["*"]
   routeConfiguration:
     routes:
     - destination:
         cluster: test-cluster-4httpver
         endpoint: test-cluster-4httpver-v1:8080
       rules:
       - match:
           prefix: /api/v1/test-cluster-4httpver
         prefixRewrite: /api/v1
     - destination:
         cluster: test-cluster-4httpver
         endpoint: test-cluster-4httpver-v1:8888
       rules:
       - match:
           prefix: /org.qubership.mesh.v3.test.bus`
	internalGateway.ApplyConfigAndWait(assert, 60*time.Second, config)

	// verify default HTTP version is 1
	cluster, err := lib.GenericDao.FindClusterByName("test-cluster-4httpver||test-cluster-4httpver||8888")
	assert.Nil(err)
	assert.NotNil(*cluster.HttpVersion)
	assert.Equal(int32(1), *cluster.HttpVersion)

	// register route for HTTP2
	config = `apiVersion: nc.core.mesh/v3
kind: RouteConfiguration
metadata:
 name: trace-service-with-grpc-routes
 namespace: ''
spec:
 gateways:
   - internal-gateway-service
 virtualServices:
 - name: internal-gateway-service
   hosts: ["*"]
   routeConfiguration:
     routes:
     - destination:
         cluster: test-cluster-4httpver
         endpoint: test-cluster-4httpver-v1:8888
         httpVersion: 2
       rules:
       - match:
           prefix: /org.qubership.mesh.v3.test.bus.TestEventBus`
	internalGateway.ApplyConfigAndWait(assert, 60*time.Second, config)

	// verify HTTP version is overwritten with HTTP2
	cluster, err = lib.GenericDao.FindClusterByName("test-cluster-4httpver||test-cluster-4httpver||8888")
	assert.Nil(err)
	assert.NotNil(*cluster.HttpVersion)
	assert.Equal(int32(2), *cluster.HttpVersion)

	// register another route without specifying http version for this cluster
	config = `apiVersion: nc.core.mesh/v3
kind: RouteConfiguration
metadata:
 name: trace-service-http-routes
 namespace: ''
spec:
 gateways:
   - internal-gateway-service
 virtualServices:
 - name: internal-gateway-service
   hosts: ["*"]
   routeConfiguration:
     routes:
     - destination:
         cluster: test-cluster-4httpver
         endpoint: test-cluster-4httpver-v1:8888
       rules:
       - match:
           prefix: /org.qubership.mesh.v3.test.bus.AnotherService`
	internalGateway.ApplyConfigAndWait(assert, 60*time.Second, config)

	// verify HTTP version did not change
	cluster, err = lib.GenericDao.FindClusterByName("test-cluster-4httpver||test-cluster-4httpver||8888")
	assert.Nil(err)
	assert.NotNil(*cluster.HttpVersion)
	assert.Equal(int32(2), *cluster.HttpVersion)

	// test gRPC routing for this cluster
	const TestTopic = "it_grpc_http_ver_test"
	receivedEvent := false
	receivedEventRef := &receivedEvent

	grpcSub := NewStartedGRPCBusSubscriber(internalGateway.HostAndPort)

	grpcSub.Subscribe(TestTopic, func(data interface{}) {
		var msg TraceResponse
		assert.Nil(json.Unmarshal(data.([]byte), &msg))
		log.InfoC(ctx, "Got gRPC event: %+v", msg)
		assert.Equal(clusterName, msg.FamilyName)
		assert.Equal("v1", msg.Version)
		*receivedEventRef = true
	})

	req, err := http.NewRequest(http.MethodPost, internalGateway.Url+"/api/v1/test-cluster-4httpver/bus/topics/"+TestTopic, nil)
	assert.Nil(err)
	_, statusCode := SendToTraceSrvWithRetry503(assert, req)
	assert.Equal(200, statusCode)

	deadline := time.Now().Add(60 * time.Second)
	for !*receivedEventRef && time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)

		req, err := http.NewRequest(http.MethodPost, internalGateway.Url+"/api/v1/test-cluster-4httpver/bus/topics/"+TestTopic, nil)
		assert.Nil(err)
		_, statusCode := SendToTraceSrvWithRetry503(assert, req)
		assert.Equal(200, statusCode)
	}
	assert.True(*receivedEventRef)

	grpcSub.Shutdown()

	// cleanup routes
	internalGateway.DeleteRoutesAndWait(assert, 60*time.Second, dto.RouteDeleteRequestV3{
		Gateways:       []string{"internal-gateway-service"},
		VirtualService: "internal-gateway-service",
		RouteDeleteRequest: dto.RouteDeleteRequest{
			Routes: []dto.RouteDeleteItem{
				{Prefix: "/api/v1/test-cluster-4httpver"},
				{Prefix: "/org.qubership.mesh.v3.test.bus"},
				{Prefix: "/org.qubership.mesh.v3.test.bus.TestEventBus"},
				{Prefix: "/org.qubership.mesh.v3.test.bus.AnotherService"},
			},
			Version: "v1",
		},
	})
}

// BELOW IS THE GRPC CLIENT IMPLEMENTATION WITH STUBS GENERATED BY PROTOC.
// ACTUAL PROTO FILE CAN VE FOUND IN control-plane-test-service/bus PACKAGE.

// HOW-TO GENERATE GRPC STUBS:
// execute in control-plane-test-service/bus folder (protoc must be installed):
//
// $ protoc -I . test_bus.proto --go-grpc_out=. --go_out=.

var ErrConnGracefullyClosed = errors.New("bus: gRPC server gracefully closed connection")
var ErrSubscriberIsShutDown = errors.New("bus: gRPC subscriber is already shut down")

// GRPCBusSubscriber is a gRPC-based implementation of the BusSubscriber interface.
type GRPCBusSubscriber struct {
	targetAddress string
	// Delay before retry of the client gRPC call
	retryDelay time.Duration
	mutex      *sync.Mutex
	clientConn *grpc.ClientConn
	// subscriberIsActive indicates that subscriber was not yet shut down
	// and needs to retry client connection in case of failure
	subscriberIsActive bool
	wg                 *sync.WaitGroup
}

func NewGRPCBusSubscriber() *GRPCBusSubscriber {
	return &GRPCBusSubscriber{
		retryDelay:         100 * time.Millisecond,
		mutex:              &sync.Mutex{},
		wg:                 &sync.WaitGroup{},
		subscriberIsActive: false}
}

func NewStartedGRPCBusSubscriber(addr string) *GRPCBusSubscriber {
	subscriber := NewGRPCBusSubscriber()
	_ = subscriber.Activate(addr)
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

func (s *GRPCBusSubscriber) subscribeWithRetry(topic string, handler func(data interface{})) {
	for s.subscriberIsActive {
		s.wg.Add(1)
		s.subscribeWithRecovery(topic, handler)
		time.Sleep(s.retryDelay)
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

	conn, err := grpc.Dial(s.targetAddress, grpc.WithInsecure(), grpc.WithDefaultServiceConfig(loadRetryConfig()))
	if err != nil {
		log.Errorf("Failed to dial client gRPC conn: %v", err)
		panic(err)
	}
	s.clientConn = conn
}

func (s *GRPCBusSubscriber) closeConnection() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.clientConn != nil {
		_ = s.clientConn.Close()
	}
}

func (s *GRPCBusSubscriber) subscribeInternal(topic string, handler func(data interface{})) {
	log.Infof("Subscribe on server gRPC events on topic %v from %s", topic, s.targetAddress)
	s.openConnection()
	defer s.closeConnection()

	client := NewTestEventBusClient(s.clientConn)
	stream, err := client.Subscribe(context.Background(), &Topic{Name: topic})
	if err != nil {
		log.Errorf("gRPC client failed to subscribe: %v", err)
		panic(err)
	}
	log.Infof("gRPC client successfully subscribed on topic %v", topic)

	for {
		event, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				log.Infof("gRPC client received EOF: %v", err)
				panic(ErrConnGracefullyClosed)
			}
			log.Errorf("gRPC client failed to receive event: %v", err)
			panic(err)
		}
		data, err := readProtobufEvent(event)
		if err != nil {
			panic(err)
		}
		handler(data)
	}
}

func readProtobufEvent(event *Event) (interface{}, error) {
	switch event.EventType {
	case EventType_RAW_DATA:
		return readRawBytesDataFromProto(event)
	default:
		log.Errorf("Event bus gRPC client doesn't support events of type %v!", event.EventType)
		return nil, errors.New("it_grpc_routes_test: unsupported gRPC event type: " + event.EventType.String())
	}
}

func readRawBytesDataFromProto(source *Event) ([]byte, error) {
	var data RawBytesData
	if err := ptypes.UnmarshalAny(source.Data, &data); err != nil {
		log.Errorf("Failed to unmarshal event %v data: %v", source.EventType, err)
		return nil, err
	}
	return data.Data, nil
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
		if s.clientConn != nil {
			log.Infof("Closed gRPC client conn: %v", s.clientConn.Close())
		}
		s.wg.Wait()
	} else {
		log.Warnf("Attempt to shutdown inactive gRPC subscriber")
	}
}

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0
// 	protoc        v3.17.3
// source: test_bus.proto

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// This is a compile-time assertion that a sufficiently up-to-date version
// of the legacy proto package is being used.
const _ = proto.ProtoPackageIsVersion4

type EventType int32

const (
	EventType_RAW_DATA EventType = 0
)

// Enum value maps for EventType.
var (
	EventType_name = map[int32]string{
		0: "RAW_DATA",
	}
	EventType_value = map[string]int32{
		"RAW_DATA": 0,
	}
)

func (x EventType) Enum() *EventType {
	p := new(EventType)
	*p = x
	return p
}

func (x EventType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (EventType) Descriptor() protoreflect.EnumDescriptor {
	return file_test_bus_proto_enumTypes[0].Descriptor()
}

func (EventType) Type() protoreflect.EnumType {
	return &file_test_bus_proto_enumTypes[0]
}

func (x EventType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use EventType.Descriptor instead.
func (EventType) EnumDescriptor() ([]byte, []int) {
	return file_test_bus_proto_rawDescGZIP(), []int{0}
}

type Topic struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
}

func (x *Topic) Reset() {
	*x = Topic{}
	if protoimpl.UnsafeEnabled {
		mi := &file_test_bus_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Topic) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Topic) ProtoMessage() {}

func (x *Topic) ProtoReflect() protoreflect.Message {
	mi := &file_test_bus_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Topic.ProtoReflect.Descriptor instead.
func (*Topic) Descriptor() ([]byte, []int) {
	return file_test_bus_proto_rawDescGZIP(), []int{0}
}

func (x *Topic) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

type Event struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	EventType EventType  `protobuf:"varint,1,opt,name=eventType,proto3,enum=org.qubership.mesh.v3.test.bus.EventType" json:"eventType,omitempty"`
	Data      *anypb.Any `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *Event) Reset() {
	*x = Event{}
	if protoimpl.UnsafeEnabled {
		mi := &file_test_bus_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Event) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Event) ProtoMessage() {}

func (x *Event) ProtoReflect() protoreflect.Message {
	mi := &file_test_bus_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Event.ProtoReflect.Descriptor instead.
func (*Event) Descriptor() ([]byte, []int) {
	return file_test_bus_proto_rawDescGZIP(), []int{1}
}

func (x *Event) GetEventType() EventType {
	if x != nil {
		return x.EventType
	}
	return EventType_RAW_DATA
}

func (x *Event) GetData() *anypb.Any {
	if x != nil {
		return x.Data
	}
	return nil
}

type RawBytesData struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Data []byte `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *RawBytesData) Reset() {
	*x = RawBytesData{}
	if protoimpl.UnsafeEnabled {
		mi := &file_test_bus_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RawBytesData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RawBytesData) ProtoMessage() {}

func (x *RawBytesData) ProtoReflect() protoreflect.Message {
	mi := &file_test_bus_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RawBytesData.ProtoReflect.Descriptor instead.
func (*RawBytesData) Descriptor() ([]byte, []int) {
	return file_test_bus_proto_rawDescGZIP(), []int{2}
}

func (x *RawBytesData) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

type Empty struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *Empty) Reset() {
	*x = Empty{}
	if protoimpl.UnsafeEnabled {
		mi := &file_test_bus_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Empty) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Empty) ProtoMessage() {}

func (x *Empty) ProtoReflect() protoreflect.Message {
	mi := &file_test_bus_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Empty.ProtoReflect.Descriptor instead.
func (*Empty) Descriptor() ([]byte, []int) {
	return file_test_bus_proto_rawDescGZIP(), []int{3}
}

var File_test_bus_proto protoreflect.FileDescriptor

var file_test_bus_proto_rawDesc = []byte{
	0x0a, 0x0e, 0x74, 0x65, 0x73, 0x74, 0x5f, 0x62, 0x75, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x1f, 0x63, 0x6f, 0x6d, 0x2e, 0x6e, 0x65, 0x74, 0x63, 0x72, 0x61, 0x63, 0x6b, 0x65, 0x72,
	0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x33, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x62, 0x75,
	0x73, 0x1a, 0x19, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x75, 0x66, 0x2f, 0x61, 0x6e, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x1b, 0x0a, 0x05,
	0x54, 0x6f, 0x70, 0x69, 0x63, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x22, 0x7b, 0x0a, 0x05, 0x45, 0x76, 0x65,
	0x6e, 0x74, 0x12, 0x48, 0x0a, 0x09, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x54, 0x79, 0x70, 0x65, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x2a, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x6e, 0x65, 0x74, 0x63,
	0x72, 0x61, 0x63, 0x6b, 0x65, 0x72, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x33, 0x2e, 0x74,
	0x65, 0x73, 0x74, 0x2e, 0x62, 0x75, 0x73, 0x2e, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x54, 0x79, 0x70,
	0x65, 0x52, 0x09, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x54, 0x79, 0x70, 0x65, 0x12, 0x28, 0x0a, 0x04,
	0x64, 0x61, 0x74, 0x61, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x14, 0x2e, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x41, 0x6e, 0x79,
	0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x22, 0x22, 0x0a, 0x0c, 0x52, 0x61, 0x77, 0x42, 0x79, 0x74,
	0x65, 0x73, 0x44, 0x61, 0x74, 0x61, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x22, 0x07, 0x0a, 0x05, 0x45, 0x6d,
	0x70, 0x74, 0x79, 0x2a, 0x19, 0x0a, 0x09, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x54, 0x79, 0x70, 0x65,
	0x12, 0x0c, 0x0a, 0x08, 0x52, 0x41, 0x57, 0x5f, 0x44, 0x41, 0x54, 0x41, 0x10, 0x00, 0x32, 0xd4,
	0x01, 0x0a, 0x0c, 0x54, 0x65, 0x73, 0x74, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x42, 0x75, 0x73, 0x12,
	0x5f, 0x0a, 0x09, 0x53, 0x75, 0x62, 0x73, 0x63, 0x72, 0x69, 0x62, 0x65, 0x12, 0x26, 0x2e, 0x63,
	0x6f, 0x6d, 0x2e, 0x6e, 0x65, 0x74, 0x63, 0x72, 0x61, 0x63, 0x6b, 0x65, 0x72, 0x2e, 0x6d, 0x65,
	0x73, 0x68, 0x2e, 0x76, 0x33, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x62, 0x75, 0x73, 0x2e, 0x54,
	0x6f, 0x70, 0x69, 0x63, 0x1a, 0x26, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x6e, 0x65, 0x74, 0x63, 0x72,
	0x61, 0x63, 0x6b, 0x65, 0x72, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x33, 0x2e, 0x74, 0x65,
	0x73, 0x74, 0x2e, 0x62, 0x75, 0x73, 0x2e, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x22, 0x00, 0x30, 0x01,
	0x12, 0x63, 0x0a, 0x0f, 0x47, 0x65, 0x74, 0x4c, 0x61, 0x73, 0x74, 0x53, 0x6e, 0x61, 0x70, 0x73,
	0x68, 0x6f, 0x74, 0x12, 0x26, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x6e, 0x65, 0x74, 0x63, 0x72, 0x61,
	0x63, 0x6b, 0x65, 0x72, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x33, 0x2e, 0x74, 0x65, 0x73,
	0x74, 0x2e, 0x62, 0x75, 0x73, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x1a, 0x26, 0x2e, 0x63, 0x6f,
	0x6d, 0x2e, 0x6e, 0x65, 0x74, 0x63, 0x72, 0x61, 0x63, 0x6b, 0x65, 0x72, 0x2e, 0x6d, 0x65, 0x73,
	0x68, 0x2e, 0x76, 0x33, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x62, 0x75, 0x73, 0x2e, 0x45, 0x76,
	0x65, 0x6e, 0x74, 0x22, 0x00, 0x42, 0x07, 0x5a, 0x05, 0x2e, 0x3b, 0x62, 0x75, 0x73, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_test_bus_proto_rawDescOnce sync.Once
	file_test_bus_proto_rawDescData = file_test_bus_proto_rawDesc
)

func file_test_bus_proto_rawDescGZIP() []byte {
	file_test_bus_proto_rawDescOnce.Do(func() {
		file_test_bus_proto_rawDescData = protoimpl.X.CompressGZIP(file_test_bus_proto_rawDescData)
	})
	return file_test_bus_proto_rawDescData
}

var file_test_bus_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_test_bus_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_test_bus_proto_goTypes = []interface{}{
	(EventType)(0),       // 0: org.qubership.mesh.v3.test.bus.EventType
	(*Topic)(nil),        // 1: org.qubership.mesh.v3.test.bus.Topic
	(*Event)(nil),        // 2: org.qubership.mesh.v3.test.bus.Event
	(*RawBytesData)(nil), // 3: org.qubership.mesh.v3.test.bus.RawBytesData
	(*Empty)(nil),        // 4: org.qubership.mesh.v3.test.bus.Empty
	(*anypb.Any)(nil),    // 5: google.protobuf.Any
}
var file_test_bus_proto_depIdxs = []int32{
	0, // 0: org.qubership.mesh.v3.test.bus.Event.eventType:type_name -> org.qubership.mesh.v3.test.bus.EventType
	5, // 1: org.qubership.mesh.v3.test.bus.Event.data:type_name -> google.protobuf.Any
	1, // 2: org.qubership.mesh.v3.test.bus.TestEventBus.Subscribe:input_type -> org.qubership.mesh.v3.test.bus.Topic
	4, // 3: org.qubership.mesh.v3.test.bus.TestEventBus.GetLastSnapshot:input_type -> org.qubership.mesh.v3.test.bus.Empty
	2, // 4: org.qubership.mesh.v3.test.bus.TestEventBus.Subscribe:output_type -> org.qubership.mesh.v3.test.bus.Event
	2, // 5: org.qubership.mesh.v3.test.bus.TestEventBus.GetLastSnapshot:output_type -> org.qubership.mesh.v3.test.bus.Event
	4, // [4:6] is the sub-list for method output_type
	2, // [2:4] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_test_bus_proto_init() }
func file_test_bus_proto_init() {
	if File_test_bus_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_test_bus_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Topic); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_test_bus_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Event); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_test_bus_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RawBytesData); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_test_bus_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Empty); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_test_bus_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_test_bus_proto_goTypes,
		DependencyIndexes: file_test_bus_proto_depIdxs,
		EnumInfos:         file_test_bus_proto_enumTypes,
		MessageInfos:      file_test_bus_proto_msgTypes,
	}.Build()
	File_test_bus_proto = out.File
	file_test_bus_proto_rawDesc = nil
	file_test_bus_proto_goTypes = nil
	file_test_bus_proto_depIdxs = nil
}

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// TestEventBusClient is the client API for TestEventBus service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type TestEventBusClient interface {
	Subscribe(ctx context.Context, in *Topic, opts ...grpc.CallOption) (TestEventBus_SubscribeClient, error)
	GetLastSnapshot(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*Event, error)
}

type testEventBusClient struct {
	cc grpc.ClientConnInterface
}

func NewTestEventBusClient(cc grpc.ClientConnInterface) TestEventBusClient {
	return &testEventBusClient{cc}
}

func (c *testEventBusClient) Subscribe(ctx context.Context, in *Topic, opts ...grpc.CallOption) (TestEventBus_SubscribeClient, error) {
	stream, err := c.cc.NewStream(ctx, &TestEventBus_ServiceDesc.Streams[0], "/org.qubership.mesh.v3.test.bus.TestEventBus/Subscribe", opts...)
	if err != nil {
		return nil, err
	}
	x := &testEventBusSubscribeClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type TestEventBus_SubscribeClient interface {
	Recv() (*Event, error)
	grpc.ClientStream
}

type testEventBusSubscribeClient struct {
	grpc.ClientStream
}

func (x *testEventBusSubscribeClient) Recv() (*Event, error) {
	m := new(Event)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *testEventBusClient) GetLastSnapshot(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*Event, error) {
	out := new(Event)
	err := c.cc.Invoke(ctx, "/org.qubership.mesh.v3.test.bus.TestEventBus/GetLastSnapshot", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// TestEventBusServer is the server API for TestEventBus service.
// All implementations must embed UnimplementedTestEventBusServer
// for forward compatibility
type TestEventBusServer interface {
	Subscribe(*Topic, TestEventBus_SubscribeServer) error
	GetLastSnapshot(context.Context, *Empty) (*Event, error)
	mustEmbedUnimplementedTestEventBusServer()
}

// UnimplementedTestEventBusServer must be embedded to have forward compatible implementations.
type UnimplementedTestEventBusServer struct {
}

func (UnimplementedTestEventBusServer) Subscribe(*Topic, TestEventBus_SubscribeServer) error {
	return status.Errorf(codes.Unimplemented, "method Subscribe not implemented")
}
func (UnimplementedTestEventBusServer) GetLastSnapshot(context.Context, *Empty) (*Event, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetLastSnapshot not implemented")
}
func (UnimplementedTestEventBusServer) mustEmbedUnimplementedTestEventBusServer() {}

// UnsafeTestEventBusServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to TestEventBusServer will
// result in compilation errors.
//type UnsafeTestEventBusServer interface {
//	mustEmbedUnimplementedTestEventBusServer()
//}
//
//func RegisterTestEventBusServer(s grpc.ServiceRegistrar, srv TestEventBusServer) {
//	s.RegisterService(&TestEventBus_ServiceDesc, srv)
//}

func _TestEventBus_Subscribe_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(Topic)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(TestEventBusServer).Subscribe(m, &testEventBusSubscribeServer{stream})
}

type TestEventBus_SubscribeServer interface {
	Send(*Event) error
	grpc.ServerStream
}

type testEventBusSubscribeServer struct {
	grpc.ServerStream
}

func (x *testEventBusSubscribeServer) Send(m *Event) error {
	return x.ServerStream.SendMsg(m)
}

func _TestEventBus_GetLastSnapshot_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TestEventBusServer).GetLastSnapshot(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/org.qubership.mesh.v3.test.bus.TestEventBus/GetLastSnapshot",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TestEventBusServer).GetLastSnapshot(ctx, req.(*Empty))
	}
	return interceptor(ctx, in, info, handler)
}

// TestEventBus_ServiceDesc is the grpc.ServiceDesc for TestEventBus service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var TestEventBus_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "org.qubership.mesh.v3.test.bus.TestEventBus",
	HandlerType: (*TestEventBusServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetLastSnapshot",
			Handler:    _TestEventBus_GetLastSnapshot_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Subscribe",
			Handler:       _TestEventBus_Subscribe_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "test_bus.proto",
}
