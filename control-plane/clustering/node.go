package clustering

import (
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/serf"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/pkg/errors"
	"strings"
	"time"
)

type Node struct {
	config    *NodeConfig
	serf      *serf.Serf
	eventCh   chan serf.Event
	callbacks []func(string)
}

type NodeConfig struct {
	Name             string
	IP               string
	Port             uint16
	MemberListConfig *memberlist.Config
	SerfConfig       *serf.Config
}

func NewNode(config NodeConfig) (*Node, error) {
	if config.MemberListConfig == nil {
		config.MemberListConfig = memberlist.DefaultLANConfig()
		config.MemberListConfig.BindAddr = config.IP
		config.MemberListConfig.BindPort = int(config.Port)
		config.MemberListConfig.LogOutput = logFilterWriter{logger: log}
	}
	if config.SerfConfig == nil {
		config.SerfConfig = serf.DefaultConfig()
		config.SerfConfig.NodeName = config.Name
		config.SerfConfig.ReconnectTimeout = 10 * time.Minute
		config.SerfConfig.LogOutput = logFilterWriter{logger: log}
	}

	eventCh := make(chan serf.Event, 16)
	config.SerfConfig.EventCh = eventCh
	config.SerfConfig.MemberlistConfig = config.MemberListConfig

	serfSrv, err := serf.Create(config.SerfConfig)
	if err != nil {
		return nil, err
	}

	newNode := &Node{
		config:    &config,
		serf:      serfSrv,
		eventCh:   eventCh, // TODO check how it works with instance and channels
		callbacks: nil,
	}
	go func() {
		for {
			ev := <-eventCh
			if memberEvent, ok := ev.(serf.MemberEvent); ok {
				log.Debugf("Handle member event: %v", memberEvent.Type)
				newNode.HandleMemberDroppingOut(memberEvent)
			}
		}
	}()
	return newNode, nil
}

func (n *Node) Leave() error {
	return n.serf.Leave()
}

func (n *Node) AddOnMemberDropped(callback func(nodeName string)) {
	n.callbacks = append(n.callbacks, callback)
}

func (n *Node) JoinMembersNetwork(record NodeInfo, role Role) error {
	if role == Slave && len(n.serf.Members()) <= 1 {
		_, err := n.serf.Join([]string{record.SWIMAddress()}, false)
		if err != nil {
			log.Errorf("Join members network failed: %v", err)
			return errors.Wrapf(err, "Join members network failed")
		}
	}
	return nil
}

func (n *Node) HandleMemberDroppingOut(event serf.MemberEvent) {
	if isMemberDroppingOutEvent(&event) {
		n.notifyMemberDropped(&event)
	}
}

func (n *Node) notifyMemberDropped(s *serf.MemberEvent) {
	for _, callback := range n.callbacks {
		for _, member := range s.Members {
			callback(member.Name)
		}
	}
}

func isMemberDroppingOutEvent(event *serf.MemberEvent) bool {
	eventType := event.EventType()
	return eventType == serf.EventMemberLeave || eventType == serf.EventMemberFailed
}

type logFilterWriter struct {
	logger logging.Logger
}

func (l logFilterWriter) Write(p []byte) (n int, err error) {
	msg := string(p)
	if strings.Contains(msg, "[ERR]") {
		l.logger.Error("%s", msg)
		return -1, nil
	} else if strings.Contains(msg, "[WARN]") {
		l.logger.Warn("%s", msg)
		return -1, nil
	} else if strings.Contains(msg, "[DEBUG]") {
		l.logger.Debug("%s", msg)
		return -1, nil
	} else {
		l.logger.Info("%s", msg)
		return -1, nil
	}
}
