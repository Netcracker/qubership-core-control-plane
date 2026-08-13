package websocket

import (
	"bytes"

	"github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/events"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/active"
)

type ActiveActiveController struct {
	data     dao.Dao
	watchers *WatcherSet
}

func (c *ActiveActiveController) getWatchers() *WatcherSet {
	return c.watchers
}

func (c *ActiveActiveController) getData() dao.Dao {
	return c.data
}

func NewActiveActiveController(eventBus bus.BusSubscriber, data dao.Dao) *ActiveActiveController {
	vController := &ActiveActiveController{
		watchers: NewWatcherSet(),
		data:     data,
	}
	eventBus.Subscribe(bus.TopicChanges, vController.handleActiveActiveChange)
	return vController
}

func (c *ActiveActiveController) NotifyWatchers(changes []memdb.Change) error {
	msg := NewActiveActiveMessageFromClusters(convertActiveActiveClusterChanges(changes))
	if msg != nil {
		c.watchers.Iter(func(w *watcher) {
			w.source <- msg
		})
	}
	return nil
}

func (c *ActiveActiveController) handleActiveActiveChange(data interface{}) {
	event := data.(*events.ChangeEvent)
	if event.NodeGroup != active.PublicGwName {
		return
	}
	if changes, ok := event.Changes[domain.ClusterTable]; ok {
		if err := c.NotifyWatchers(changes); err != nil {
			log.Errorf("Can't notify active-active watchers: %v", err)
		}
	}
}

// HandleActiveActiveWatch godoc
// @Id HandleActiveActiveWatch
// @Summary Active Watch
// @Description Active Watch
// @Tags control-plane-v3
// @Produce json
// @Param Connection header string true "Connection"
// @Security ApiKeyAuth
// @Success 200
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 405 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v3/active-active/watch [get]
func (c *ActiveActiveController) HandleActiveActiveWatch(fiberCtx fiber.Ctx) error {
	ctx := fiberCtx.Context()
	connHeader := fiberCtx.Request().Header.Peek("Connection")
	if bytes.EqualFold(connHeader, []byte("upgrade")) {
		fiberCtx.Request().Header.Set("Connection", "Upgrade")
	}
	err := UpgradeWsSocket(ctx, fiberCtx, c)
	if err != nil {
		log.ErrorC(ctx, "Error upgrade websocket for Active Active Controller: %v", err)
		return nil
	}
	return nil
}

func (c *ActiveActiveController) doOnUpgradeWsSocket(conn *websocket.Conn, dao dao.Repository) error {
	balancingClusterName := active.GetActiveActiveClusterName(active.PublicGwName)
	activeCluster, err := dao.FindClusterByName(balancingClusterName)
	if err != nil {
		return err
	}
	var message interface{}
	if activeCluster == nil {
		message = NewActiveActiveMessage([]string{}, "")
	} else {
		// find clusters for external active-active public gws
		activeActiveClusters := []*domain.Cluster{activeCluster}
		for _, endpoint := range activeCluster.Endpoints {
			externalGwClusterName := active.GetClusterNameForExternalGw(active.PublicGwName, endpoint.Hostname)
			externalGwCluster, err := dao.FindClusterByName(externalGwClusterName)
			if err != nil {
				return err
			}
			if externalGwCluster != nil {
				activeActiveClusters = append(activeActiveClusters, externalGwCluster)
			}
		}
		message = NewActiveActiveMessageFromClusters(activeActiveClusters, []*domain.Cluster{})
	}
	err = conn.WriteJSON(message)
	if err != nil {
		return err
	}
	return nil
}

type ActiveActiveConfigMessage struct {
	Aliases      []string `json:"aliases"`
	CurrentAlias string   `json:"currentAlias"`
}

func NewActiveActiveMessage(aliases []string, currentAlias string) *ActiveActiveConfigMessage {
	if aliases != nil {
		return &ActiveActiveConfigMessage{
			Aliases:      aliases,
			CurrentAlias: currentAlias,
		}
	} else {
		return &ActiveActiveConfigMessage{
			Aliases:      []string{},
			CurrentAlias: currentAlias,
		}
	}
}

func NewActiveActiveMessageFromClusters(created, deleted []*domain.Cluster) *ActiveActiveConfigMessage {
	var currentAlias string
	if len(created) == 0 && len(deleted) > 0 {
		// delete scenario
		clustersMap := ClusterSliceToMap(deleted)
		activeActiveCluster := clustersMap[active.GetActiveActiveClusterName(active.PublicGwName)]
		if activeActiveCluster == nil {
			return nil
		} else {
			// active-active cluster was deleted, send empty message
			return NewActiveActiveMessage([]string{}, "")
		}
	} else if len(created) > 0 && len(deleted) == 0 {
		// create scenario
		clustersMap := ClusterSliceToMap(created)
		activeActiveCluster := clustersMap[active.GetActiveActiveClusterName(active.PublicGwName)]
		if activeActiveCluster == nil {
			return nil
		}
		aliases := make([]string, len(activeActiveCluster.Endpoints))
		for _, endpoint := range activeActiveCluster.Endpoints {
			alias := endpoint.Hostname
			aliases[endpoint.OrderId] = alias
			// find current alias. For current alias we should not have external gw cluster
			clusterNameForExternalGw := active.GetClusterNameForExternalGw(active.PublicGwName, alias)
			if _, ok := clustersMap[clusterNameForExternalGw]; !ok {
				currentAlias = alias
			}
		}
		return NewActiveActiveMessage(aliases, currentAlias)
	}
	return nil
}

func ClusterSliceToMap(clusters []*domain.Cluster) map[string]*domain.Cluster {
	result := make(map[string]*domain.Cluster, len(clusters))
	for _, cluster := range clusters {
		result[cluster.Name] = cluster
	}
	return result
}

func convertActiveActiveClusterChanges(changes []memdb.Change) (created []*domain.Cluster, deleted []*domain.Cluster) {
	for _, change := range changes {
		if change.Before != nil && change.After == nil {
			// delete scenario
			deleted = append(deleted, getClusterPtr(change.Before))
		} else if change.Before == nil && change.After != nil {
			// create scenario
			created = append(created, getClusterPtr(change.After))
		} else if change.Before != nil && change.After != nil {
			// update scenario?
			created = append(created, getClusterPtr(change.After))
		}
	}
	return
}

func getClusterPtr(cluster interface{}) *domain.Cluster {
	var result *domain.Cluster
	if val, ok := cluster.(*domain.Cluster); ok {
		result = val
	}
	if val, ok := cluster.(domain.Cluster); ok {
		result = &val
	}
	return result
}
