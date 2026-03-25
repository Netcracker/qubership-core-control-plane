package websocket

import (
	"context"
	"github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/valyala/fasthttp"
	"time"
)

const (
	writeWait  = time.Second
	pingPeriod = time.Minute
)

type Controller interface {
	getWatchers() *WatcherSet
	getData() dao.Dao
	doOnUpgradeWsSocket(conn *websocket.Conn, repository dao.Repository) error
}

var (
	wsSocketUpgrader = &websocket.FastHTTPUpgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(ctx *fasthttp.RequestCtx) bool {
			return true
		},
	}
	log = logging.GetLogger("websocket")
)

type JSONSupportConnection interface {
	WriteJSON(v interface{}) error
}

func validateConnection(ctx context.Context, conn *websocket.Conn, watcher *watcher) {
	defer func() {
		if r := recover(); r != nil {
			log.ErrorC(ctx, "Validate Connection has been recovered after panic: %v", r)
		}
		if !watcher.stopped {
			watcher.quit <- struct{}{}
		}
		_ = conn.Close()
	}()
	for {
		if _, _, err := conn.NextReader(); err != nil {
			log.ErrorC(ctx, "Error in reading websocket connection: %v", err)
			break
		}
	}
}

func runWatcherWork(ctx context.Context, watcher *watcher, conn *websocket.Conn) {
	for {
		select {
		case data := <-watcher.source:
			log.DebugC(ctx, "Sending message to client: %+v", data)
			if err := conn.WriteJSON(data); err != nil {
				log.ErrorC(ctx, "Error send message to client: %v", err)
				return
			}
		case <-watcher.quit:
			log.DebugC(ctx, "Websocket is closing")
			watcher.Stop()
			return
		}
	}
}

func tuneConnAndReturnCancelFunc(ctx context.Context, conn *websocket.Conn, watcher *watcher) func() {
	// setup ping process, close handler etc
	ctx, cancelContext := context.WithCancel(ctx)
	conn.SetCloseHandler(func(code int, text string) error {
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseMessage, ""), time.Now().Add(writeWait))
		return nil
	})
	go validateConnection(ctx, conn, watcher)
	go pingRecipient(ctx, conn)
	// return func that must be called when websocket handler is over
	return func() {
		log.DebugC(ctx, "Returning Websocket close function")
		cancelContext()
		err := conn.Close()
		if err != nil {
			log.ErrorC(ctx, "Error In closing resource: %v", err)
		}
	}
}

func pingRecipient(ctx context.Context, conn *websocket.Conn) {
	for {
		select {
		case <-time.After(pingPeriod):
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				log.Warnf("Cannot send ping message: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func UpgradeWsSocket(ctx context.Context, fiberCtx *fiber.Ctx, c Controller) interface{} {
	return wsSocketUpgrader.Upgrade(fiberCtx.Context(), func(conn *websocket.Conn) {
		log.DebugC(ctx, "Websocket connection for config watch has opened.")
		watcher := newWatcher()
		closeConnGracefullyFunc := tuneConnAndReturnCancelFunc(ctx, conn, watcher)
		defer closeConnGracefullyFunc()
		c.getWatchers().Push(watcher)
		defer c.getWatchers().Remove(watcher)
		if err := c.getData().WithRTx(func(dao dao.Repository) error {
			return c.doOnUpgradeWsSocket(conn, dao)
		}); err != nil {
			log.ErrorC(ctx, "Preparing backends for send caused error: %v", err)
			watcher.Stop()
			return // runWatcherWork will not start
		}
		runWatcherWork(ctx, watcher, conn)
	})
}
