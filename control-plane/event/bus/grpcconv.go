package bus

import (
	"bytes"
	"encoding/gob"
	"errors"
	"github.com/golang/protobuf/ptypes"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/data"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/events"
)

var ErrUnsupportedEvent = errors.New("bus: unsupported event type")

func init() {
	gob.Register(domain.HeaderMatcher{})
	gob.Register(domain.NodeGroup{})
	gob.Register(domain.VirtualHost{})
	gob.Register(domain.VirtualHostDomain{})
	gob.Register(domain.DeploymentVersion{})
	gob.Register(domain.HashPolicy{})
	gob.Register(domain.Route{})
	gob.Register(domain.ClustersNodeGroup{})
	gob.Register(domain.RouteConfiguration{})
	gob.Register(domain.Listener{})
	gob.Register(domain.Cluster{})
	gob.Register(domain.EnvoyConfigVersion{})
	gob.Register(domain.Endpoint{})
	gob.Register(domain.TlsConfig{})
	gob.Register(domain.ClustersNodeGroup{})
	gob.Register(domain.WasmFilter{})
	gob.Register(domain.ListenersWasmFilter{})
	gob.Register(domain.RetryPolicy{})
	gob.Register(domain.RetryBackOff{})
	gob.Register(domain.HealthCheck{})
	gob.Register(domain.StatefulSession{})
	gob.Register(domain.TlsConfigsNodeGroups{})
	gob.Register(domain.RateLimit{})
	gob.Register(domain.ExtAuthzFilter{})
	gob.Register(domain.CircuitBreaker{})
	gob.Register(domain.Threshold{})
	gob.Register(domain.TcpKeepalive{})
	gob.Register(domain.MicroserviceVersion{})
	gob.Register(domain.LuaFilter{})
	gob.Register(domain.ListenersLuaFilter{})
}

//
// Functions to convert domain structures to gRPC protobuf events
//

func buildProtobufEvent(any interface{}) (*Event, error) {
	switch t := any.(type) {
	case *data.Snapshot:
		return buildProtobufEventFromSnapshot(any.(*data.Snapshot))
	case *events.ChangeEvent:
		return buildProtoEvent(EventType_CHANGE, any.(*events.ChangeEvent))
	case *events.ReloadEvent:
		return buildProtoEvent(EventType_RELOAD, any.(*events.ReloadEvent))
	default:
		log.Debugf("Event bus doesn't support events of type %T!", t)
		return nil, ErrUnsupportedEvent
	}
}

func buildProtoEvent(evType EventType, source domain.MarshalPreparer) (*Event, error) {
	log.Debugf("buildProtoChangeEvent %s %+v", evType, source)
	buf := &bytes.Buffer{}
	if err := source.MarshalPrepare(); err != nil {
		return nil, err
	}
	if err := gob.NewEncoder(buf).Encode(source); err != nil {
		return nil, err
	}
	marshalledEvent, err := ptypes.MarshalAny(&RawBytesData{Data: buf.Bytes()})
	if err != nil {
		log.Errorf("Failed to marshall ReloadEvent to protobuf: %v", err)
		return nil, err
	}
	return &Event{EventType: evType, Data: marshalledEvent}, nil
}

func buildProtobufEventFromSnapshot(snapshot *data.Snapshot) (*Event, error) {
	// recursive links here is removed already because snapshot is taken via Backup method of RAM storage
	// so we don't need to call domain.MarshalPreparer
	binaryData, err := convertSnapshotToBytes(snapshot)
	if err != nil {
		return nil, err
	}
	return buildProtobufRawDataEvent(binaryData)
}

func buildProtobufRawDataEvent(source []byte) (*Event, error) {
	marshalledEvent, err := ptypes.MarshalAny(&RawBytesData{Data: source})
	if err != nil {
		log.Errorf("Failed to marshal byte slice to protobuf event: %v", err)
		return nil, err
	}
	log.Debugf("gRPC binary message has been built. size: %d bytes", len(source))
	return &Event{EventType: EventType_RAW_DATA, Data: marshalledEvent}, nil
}

//
// Functions to parse domain structures from gRPC protobuf events
//

func readProtobufEvent(event *Event) (interface{}, error) {
	switch event.EventType {
	case EventType_RAW_DATA:
		return readSnapshotFromProtoEvent(event)
	case EventType_CHANGE:
		return readChangeEventFromProtoEvent(event)
	case EventType_MULTIPLE_CHANGE:
		panic("Multiple changes event sending is not supported on gRPC - implement on demand")
	case EventType_RELOAD:
		return readReloadEventFromProtoEvent(event)
	default:
		return nil, ErrUnsupportedEvent
	}
}

func readChangeEventFromProtoEvent(event *Event) (*events.ChangeEvent, error) {
	rawBytes, err := readRawBytesDataFromProto(event)
	if err != nil {
		return nil, err
	}
	var changeEvent *events.ChangeEvent
	if err := gob.NewDecoder(bytes.NewReader(rawBytes)).Decode(&changeEvent); err != nil {
		return nil, err
	}
	return changeEvent, nil
}

func readReloadEventFromProtoEvent(event *Event) (*events.ReloadEvent, error) {
	rawBytes, err := readRawBytesDataFromProto(event)
	if err != nil {
		return nil, err
	}
	var changeEvent *events.ReloadEvent
	if err := gob.NewDecoder(bytes.NewReader(rawBytes)).Decode(&changeEvent); err != nil {
		return nil, err
	}
	return changeEvent, nil
}

func readRawBytesDataFromProto(source *Event) ([]byte, error) {
	var data RawBytesData
	if err := ptypes.UnmarshalAny(source.Data, &data); err != nil {
		log.Errorf("Failed to unmarshal event %v data: %v", source.EventType, err)
		return nil, err
	}
	return data.Data, nil
}

func readSnapshotFromProtoEvent(source *Event) (*data.Snapshot, error) {
	binaryData, err := readRawBytesDataFromProto(source)
	if err != nil {
		return nil, err
	}
	snapshot, err := convertBytesToSnapshot(&binaryData)
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

func convertSnapshotToBytes(snapshot *data.Snapshot) ([]byte, error) {
	b := bytes.Buffer{}
	e := gob.NewEncoder(&b)
	err := e.Encode(snapshot)
	/*data, err := json.MarshalIndent(snapshot, "", "  ")*/
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func convertBytesToSnapshot(binData *[]byte) (*data.Snapshot, error) {
	bytesBuffer := bytes.NewBuffer(*binData)
	decoder := gob.NewDecoder(bytesBuffer)
	restoredSnapshot := &data.Snapshot{}
	err := decoder.Decode(restoredSnapshot)
	/*restoredSnapshot := &data.Snapshot{}
	err := json.Unmarshal(*binData, restoredSnapshot)*/
	if err != nil {
		return nil, err
	}
	return restoredSnapshot, nil
}
