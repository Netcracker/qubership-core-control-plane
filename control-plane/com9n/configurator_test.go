package com9n

import (
	"fmt"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/clustering"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/data"
	bus2 "github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/stretchr/testify/assert"
	"testing"
)

type StorageMock struct {
}

func (s *StorageMock) Backup() (*data.Snapshot, error) {
	return &data.Snapshot{}, nil
}

func (s *StorageMock) Restore(snapshot data.Snapshot) error {
	return nil
}

type ConfigUpdaterMock struct {
}

func (s *ConfigUpdaterMock) InitConfigWithRetry() {

}

func TestConfigurator_SetUpNodesCommunicationIntegrationTest(t *testing.T) {
	configloader.Init(configloader.EnvPropertySource())

	storage := &StorageMock{}
	internalBus := bus2.GetInternalBusInstance()
	grpcSub := bus2.NewGRPCBusSubscriber()
	grpcPub := bus2.NewGRPCBusPublisher(storage)
	bus := bus2.NewEventBusAggregator(storage, internalBus, internalBus, grpcSub, grpcPub)
	cfg := NewConfigurator(storage, bus, grpcPub, grpcSub, grpcSub, &ConfigUpdaterMock{})

	//emptyNodeInfo := clustering.NodeInfo{}
	master1NodeInfo := clustering.NodeInfo{
		IP:       "127.0.0.1",
		SWIMPort: 12345,
		BusPort:  54321,
		HttpPort: 8080,
	}
	/*master2NodeInfo := clustering.NodeInfo{
	    IP:       "1.1.1.1",
	    SWIMPort: 12345,
	    BusPort:  54321,
	    HttpPort: 8080,
	}*/
	err := cfg.sender.StartSending(master1NodeInfo)
	assert.Nil(t, err)
	clustering.CurrentNodeState.SetMasterReady()
	err = cfg.receiver.StartReceiving(master1NodeInfo.BusAddress())
	assert.Nil(t, err)
	assert.True(t, cfg.receiver.IsStarted())
	err = cfg.receiver.StopReceiving()
	assert.Nil(t, err)
	assert.False(t, cfg.receiver.IsStarted())

	err = cfg.receiver.StartReceiving(master1NodeInfo.BusAddress())
	assert.Nil(t, err)
	assert.True(t, cfg.receiver.IsStarted())
	err = cfg.receiver.StopReceiving()
	assert.Nil(t, err)
	assert.False(t, cfg.receiver.IsStarted())

	err = cfg.receiver.StartReceiving(master1NodeInfo.BusAddress())
	assert.Nil(t, err)
	assert.True(t, cfg.receiver.IsStarted())
	err = cfg.receiver.StopReceiving()
	assert.Nil(t, err)
	assert.False(t, cfg.receiver.IsStarted())
}

func TestConfigurator_SetUpNodesCommunication(t *testing.T) {
	receiver := &ReceiverMock{startRcvArgs: make(map[int]string)}
	sender := &SenderMock{startSendArgs: make(map[int]clustering.NodeInfo)}
	cfg := Configurator{
		sender:   sender,
		receiver: receiver,
	}
	emptyNodeInfo := clustering.NodeInfo{}
	masterNodeInfo := clustering.NodeInfo{
		IP:       "0.0.0.0",
		SWIMPort: 12345,
		BusPort:  54321,
		HttpPort: 8080,
	}
	sender.isStarted = false
	receiver.isStarted = true
	cfg.SetUpNodesCommunication(masterNodeInfo, clustering.Slave)
	assert.Equal(t, 0, sender.stopSendCount)
	assert.Equal(t, 0, sender.startSendCount)
	assert.Equal(t, 1, receiver.stopRcvCount)
	assert.Equal(t, 1, receiver.startRcvCount)
	assert.Equal(t, masterNodeInfo.BusAddress(), receiver.startRcvArgs[1])

	cfg.SetUpNodesCommunication(emptyNodeInfo, clustering.Phantom)
	assert.Equal(t, 0, sender.stopSendCount)
	assert.Equal(t, 0, sender.startSendCount)
	assert.Equal(t, 1, receiver.stopRcvCount)
	assert.Equal(t, 1, receiver.startRcvCount)
	assert.True(t, receiver.isStarted)

	cfg.SetUpNodesCommunication(masterNodeInfo, clustering.Slave)
	assert.Equal(t, 0, sender.stopSendCount)
	assert.Equal(t, 0, sender.startSendCount)
	assert.Equal(t, 2, receiver.stopRcvCount)
	assert.Equal(t, 2, receiver.startRcvCount)
	assert.Equal(t, masterNodeInfo.BusAddress(), receiver.startRcvArgs[2])

	cfg.SetUpNodesCommunication(masterNodeInfo, clustering.Master)
	assert.Equal(t, 0, sender.stopSendCount)
	assert.Equal(t, 1, sender.startSendCount)
	assert.Equal(t, 3, receiver.stopRcvCount)
	assert.Equal(t, 2, receiver.startRcvCount)
	assert.False(t, cfg.IsReceiverStarted())
	assert.Equal(t, masterNodeInfo.BusAddress(), receiver.startRcvArgs[2])
	assert.Equal(t, masterNodeInfo, sender.startSendArgs[1])

	sender.isStarted = true
	receiver.isStarted = true
	cfg.SetUpNodesCommunication(masterNodeInfo, clustering.Slave)
	assert.Equal(t, 1, sender.stopSendCount)
	assert.Equal(t, 1, sender.startSendCount)
	assert.Equal(t, 4, receiver.stopRcvCount)
	assert.Equal(t, 3, receiver.startRcvCount)
	assert.Equal(t, masterNodeInfo.BusAddress(), receiver.startRcvArgs[3])
	assert.True(t, cfg.IsReceiverStarted())
}

type SenderMock struct {
	isStarted      bool
	stopSendCount  int
	startSendCount int
	startSendArgs  map[int]clustering.NodeInfo
}

func (s *SenderMock) StartSending(info clustering.NodeInfo) error {
	s.startSendCount++
	s.startSendArgs[s.startSendCount] = info
	return nil
}

func (s *SenderMock) StopSending() error {
	s.stopSendCount++
	return nil
}

func (s *SenderMock) Clear() {
	s.isStarted = false
	s.startSendCount = 0
	s.stopSendCount = 0
	s.startSendArgs = make(map[int]clustering.NodeInfo)
}

func (s *SenderMock) IsStarted() bool {
	return s.isStarted
}

type ReceiverMock struct {
	isStarted            bool
	stopRcvCount         int
	startRcvCount        int
	startRcvArgs         map[int]string
	failOnStartReceiving bool
}

func (r *ReceiverMock) StartReceiving(addr string) error {
	if r.failOnStartReceiving {
		return fmt.Errorf("rrror on start receiving")
	}
	r.startRcvCount++
	r.startRcvArgs[r.startRcvCount] = addr
	r.isStarted = true
	return nil
}

func (r *ReceiverMock) StopReceiving() error {
	r.stopRcvCount++
	r.isStarted = false
	return nil
}

func (r *ReceiverMock) IsStarted() bool {
	return r.isStarted
}

func (r *ReceiverMock) Clear() {
	r.isStarted = false
	r.startRcvCount = 0
	r.stopRcvCount = 0
	r.startRcvArgs = make(map[int]string)
}
