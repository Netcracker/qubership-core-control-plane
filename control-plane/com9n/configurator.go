package com9n

import (
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/clustering"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/data"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/pkg/errors"
	"time"
)

var log logging.Logger

const Topic = "SnapshotChanges"

func init() {
	log = logging.GetLogger("com9n")
}

type IConfigurator interface {
	SetUpNodesCommunication(info clustering.NodeInfo, role clustering.Role) error
	IsReceiverStarted() bool
}

type Configurator struct {
	sender   Sender
	receiver Receiver
}

func NewConfigurator(storage data.RestorableStorage, internalBus InternalBus,
	grpcPublisher ActivatablePublisher, grpcSubscriber ActivatableSubscriber,
	snapshotProvider RemoteSnapshotProvider, configUpdater ConfigUpdater) *Configurator {
	sender := NewDataSender(storage, grpcPublisher)
	slaveDataReceiver := NewSlaveNodeDataReceiver(storage, grpcSubscriber, snapshotProvider, configUpdater)
	internalBus.Subscribe(bus.TopicChanges, sender.HandleChangeEvent)
	internalBus.Subscribe(bus.TopicMultipleChanges, sender.HandleChangeEvent)
	internalBus.Subscribe(bus.TopicReload, sender.HandleChangeEvent)
	return &Configurator{
		sender:   sender,
		receiver: slaveDataReceiver,
	}
}

func (c *Configurator) SetUpNodesCommunication(info clustering.NodeInfo, role clustering.Role) error {
	switch role {
	case clustering.Slave:
		if c.sender.IsStarted() {
			err := c.sender.StopSending()
			if err != nil {
				log.Errorf("Can't stop sender. Cause: %v", err)
			}
		}
		if c.receiver.IsStarted() {
			err := c.receiver.StopReceiving()
			if err != nil {
				log.Errorf("Can't stop receiver. Cause: %v", err)
			}
		}
		err := c.receiver.StartReceiving(info.BusAddress())
		if err != nil {
			log.Errorf("Can't start receiver. Nodes communication is broken: %v", err)
			return errors.Wrapf(err, "Can't start receiver. Nodes communication is broken")
		}
	case clustering.Master:
		if c.receiver.IsStarted() {
			err := c.receiver.StopReceiving()
			if err != nil {
				log.Errorf("Can't stop receiver. Cause: %v", err)
			}
		}
		if !c.sender.IsStarted() {
			err := c.sender.StartSending(info)
			if err != nil {
				log.Errorf("Can't start sender. Nodes communication is broken: %v", err)
				return errors.Wrapf(err, "Can't start sender. Nodes communication is broken")
			}
		}
	case clustering.Phantom:
	default:
		log.Error("Setting up nodes communication called with unknown cluster role %v", role)
	}

	return nil
}

func (c *Configurator) IsReceiverStarted() bool {
	return c.receiver.IsStarted()
}

type Receiver interface {
	StartReceiving(address string) error
	StopReceiving() error
	IsStarted() bool
}

type Sender interface {
	StartSending(info clustering.NodeInfo) error
	StopSending() error
	IsStarted() bool
}

type ActivatablePublisher interface {
	bus.BusPublisher
	Activate(address string) error
	HasSubscribers() bool
}

type ActivatableSubscriber interface {
	bus.BusSubscriber
	Activate(address string) error
}

type RemoteSnapshotProvider interface {
	ActivatableSubscriber
	GetSnapshot() (*data.Snapshot, error)
}

type InternalBus interface {
	Subscribe(topic string, handler func(data interface{}))
}

type ConfigUpdater interface {
	InitConfigWithRetry()
}

type RetryProvider interface {
	SleepPeriodOnSnapshotSend() time.Duration
}
