package websocket

import (
	"bytes"
	"github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/clustering"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/events"
)

type VersionController struct {
	data     dao.Dao
	watchers *WatcherSet
}

func (c *VersionController) getWatchers() *WatcherSet {
	return c.watchers
}

func (c *VersionController) getData() dao.Dao {
	return c.data
}

func NewVersionController(eventBus bus.BusSubscriber, data dao.Dao) *VersionController {
	vController := &VersionController{
		watchers: NewWatcherSet(),
		data:     data,
	}
	eventBus.Subscribe(bus.TopicChanges, vController.handleVersionChange)
	eventBus.Subscribe(bus.TopicBgRegistry, vController.handleVersionChange)
	eventBus.Subscribe(bus.TopicReload, vController.handleReload)
	return vController
}

func (c *VersionController) NotifyWatchers(changes []memdb.Change) error {
	return c.notifyWatchers(convertChanges(changes))
}

func (c *VersionController) notifyWatchers(changes []Change) error {
	msg, err := NewMessage(nil, changes)
	log.Debugf("notifyWatchers msg %v", msg)
	if err != nil {
		log.Errorf("Can't convert DeploymentVersions to websocket Message: %v", err)
		return err
	}
	c.watchers.Iter(func(w *watcher) {
		w.source <- msg
	})
	return nil
}

func (c *VersionController) handleReload(data interface{}) {
	event := data.(*events.ReloadEvent)
	if changes, ok := event.Changes[domain.DeploymentVersionTable]; ok {
		if err := c.NotifyWatchers(changes); err != nil {
			log.Errorf("Can't notify watchers: %v", err)
		}
	}
}

func (c *VersionController) handleVersionChange(data interface{}) {
	event := data.(*events.ChangeEvent)
	if changes, ok := event.Changes[domain.DeploymentVersionTable]; ok {
		if err := c.NotifyWatchers(changes); err != nil {
			log.Errorf("Can't notify watchers: %v", err)
		}
	}
}

func (c *VersionController) ResetConnections(_ clustering.NodeInfo, _ clustering.Role) {
	log.Info("Abort all ws sessions for version watch API due to node role change")
	c.watchers.Iter(func(w *watcher) {
		if !w.stopped {
			w.quit <- struct{}{}
		}
	})
	log.Info("Ws session abort for version watch API is done")
}

func convertChanges(changes []memdb.Change) []Change {
	newChanges := make([]Change, len(changes))
	for i, change := range changes {
		newChanges[i] = NewChange(change)
	}
	return unique(newChanges)
}

func unique(changes []Change) []Change {
	chMap := make(map[Change]bool)
	result := make([]Change, 0)
	for _, change := range changes {
		if _, ok := chMap[change]; !ok {
			chMap[change] = true
			result = append(result, change)
		}
	}
	return result
}

// HandleVersionsWatchV3 godoc
// @Id HandleVersionsWatch
// @Summary Handle Versions V3 Watch
// @Description Handle Versions V3 Watch
// @Tags control-plane-v3
// @Produce json
// @Param Connection header string true "Connection"
// @Security ApiKeyAuth
// @Success 200
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 405 {object} map[string]string
// @Router /api/v3/versions/watch [get]
func (c *VersionController) HandleVersionsWatch(fiberCtx fiber.Ctx) error {
	ctx := fiberCtx.Context()
	connHeader := fiberCtx.Request().Header.Peek("Connection")
	if bytes.EqualFold(connHeader, []byte("upgrade")) {
		fiberCtx.Request().Header.Set("Connection", "Upgrade")
	}
	err := UpgradeWsSocket(ctx, fiberCtx, c)
	if err != nil {
		// Upgrader already fills in response with error, so there is no reason to do it here
		log.ErrorC(ctx, "Error upgrade websocket for Version Controller: %v", err)
		return nil
	}
	return nil
}

// HandleVersionsWatchV2 godoc
// @Id HandleVersionsWatchv2
// @Summary Handle Versions V2 Watch
// @Description Handle Versions V2 Watch
// @Tags control-plane-v2
// @Produce json
// @Param Connection header string true "Connection"
// @Security ApiKeyAuth
// @Success 200
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 405 {object} map[string]string
// @Router /api/v2/control-plane/versions/watch [get]
func (c *VersionController) HandleVersionsWatchv2(fiberCtx fiber.Ctx) error {
	return c.HandleVersionsWatch(fiberCtx)
}

func (c *VersionController) doOnUpgradeWsSocket(conn *websocket.Conn, dao dao.Repository) error {
	dVersions, err := dao.FindAllDeploymentVersions()
	if err != nil {
		return err
	}
	message, err := NewMessage(dVersions, nil)
	if err != nil {
		return err
	}
	err = conn.WriteJSON(message)
	if err != nil {
		return err
	}
	return nil
}
