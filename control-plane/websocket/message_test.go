package websocket

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/golang/mock/gomock"
	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/clustering"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	mock_dao "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/dao"
	"github.com/netcracker/qubership-core-lib-go/v3/utils"
	"github.com/stretchr/testify/assert"
)

func TestNewChange(t *testing.T) {
	dVersion1 := &domain.DeploymentVersion{}
	memdbChange := memdb.Change{Before: dVersion1}
	ch := NewChange(memdbChange)
	assert.Equal(t, &dVersion1, &ch.Old)

	dVersion2 := domain.DeploymentVersion{}
	memdbChange = memdb.Change{After: dVersion2}
	ch = NewChange(memdbChange)
	assert.Equal(t, &dVersion2, ch.New)
}

func Test_ActiveActiveController_WebsocketIsClosed_OnPreparingDeploymentVersionFails(t *testing.T) {
	wsActiveActiveController := NewMockActiveActiveController(t, true) // set up controller without mocking error

	controller_WebsocketIsClosed_OnPreparingDeploymentVersionFails(t, wsActiveActiveController.HandleActiveActiveWatch, wsActiveActiveController.watchers)
}

func Test_VersionController_WebsocketIsClosed_OnPreparingDeploymentVersionFails(t *testing.T) {
	wsVersionController := NewMockVersionController(t, true) // set up controller with mocking error

	controller_WebsocketIsClosed_OnPreparingDeploymentVersionFails(t, wsVersionController.HandleVersionsWatch, wsVersionController.watchers)
}

func Test_VersionController_WebsocketIsClosed_OnClosingConnection(t *testing.T) {
	wsVersionController := NewMockVersionController(t, false) // set up controller with mocking error

	controller_WebsocketIsClosed_OnClosingConnection(t, wsVersionController.HandleVersionsWatch, wsVersionController.watchers)
}

func Test_ActiveActiveController_WebsocketIsClosed_OnClosingConnection(t *testing.T) {
	wsActiveActiveController := NewMockActiveActiveController(t, false) // set up controller without mocking error

	controller_WebsocketIsClosed_OnClosingConnection(t, wsActiveActiveController.HandleActiveActiveWatch, wsActiveActiveController.watchers)
}

func controller_WebsocketIsClosed_OnPreparingDeploymentVersionFails(t *testing.T, handler fiber.Handler, watcherSet *WatcherSet) {
	app := startTestServer("/versions/watch", handler)
	defer stopTestServer(app)

	conn := DialWebsocket(t, "/versions/watch", "http://127.0.0.1:10801")

	time.Sleep(10 * time.Second) // wait some background actions on handling request

	assert.Equal(t, 0, len(watcherSet.watchersMap), "Watchers is not cleaned")
	assert.Equal(t, true, isClosed(conn), "Connection is not closed")
}

func controller_WebsocketIsClosed_OnClosingConnection(t *testing.T, handler fiber.Handler, watcherSet *WatcherSet) {
	app := startTestServer("/versions/watch", handler)
	defer stopTestServer(app)

	conn := DialWebsocket(t, "/versions/watch", "http://127.0.0.1:10801")

	time.Sleep(10 * time.Second) // wait some background actions on handling request

	assert.Equal(t, 1, len(watcherSet.watchersMap), "Watchers is cleaned")
	assert.Equal(t, false, isClosed(conn), "Connection is closed")

	conn.Close()

	time.Sleep(10 * time.Second) // wait some background actions on closing

	assert.Equal(t, 0, len(watcherSet.watchersMap), "Watchers is not cleaned")
	assert.Equal(t, true, isClosed(conn), "Connection is not closed")
}

func Test_VersionController_WebsocketIsClosedAndResetConnectionsIsRun_ResetConnectionsDoesNotStickOnWritingInChannel(t *testing.T) {
	ctrl := gomock.NewController(t)
	dao := mock_dao.NewMockDao(ctrl)

	dao.EXPECT().WithRTx(gomock.Any()).Return(nil)

	controller := &VersionController{
		watchers: &WatcherSet{
			Locker:      &MockLocker{},
			watchersMap: make(map[*watcher]bool),
		},
		data: dao,
	}

	app := startTestServer("/versions/watch", controller.HandleVersionsWatch)
	defer stopTestServer(app)

	conn := DialWebsocket(t, "/versions/watch", "http://127.0.0.1:10801")

	// establishing connection run into Lock. This is first Lock. We need to release it to gp further
	controller.watchers.Locker.(*MockLocker).UnlockByNumber(1)

	conn.Close() // closing connection. Close run into Lock as well. This is second Lock. We have to keep it

	waitWatcherStopped(controller) // wait when the watcher is marked as stopped after closing

	done := make(chan error)
	go func(done chan error) {
		// run action where we can stick on writing in channel. ResetConnections run into third Lock
		controller.ResetConnections(clustering.NodeInfo{}, clustering.Role(1))
		done <- nil
	}(done)
	time.Sleep(time.Millisecond)

	controller.watchers.Locker.(*MockLocker).UnlockByNumber(3) // release Lock from ResetConnections

	isDone, _ := waitDoneWithTimeout(done, 5*time.Second)
	assert.True(t, isDone, "Operation has not completed within 5 seconds")
}

func waitWatcherStopped(controller *VersionController) bool {
	for len(controller.watchers.watchersMap) == 0 { // wait when watcher appears here
		time.Sleep(time.Second)
	}
	for w := range controller.watchers.watchersMap {
		for !w.stopped { // wait when watcher is stopped
			time.Sleep(time.Second)
		}
		return true
	}

	return false
}

type MockLocker struct {
	chans []chan struct{}
}

func NewMyLocker() MockLocker {
	return MockLocker{chans: []chan struct{}{}}
}

func (m *MockLocker) Lock() {
	ch := make(chan struct{})
	m.chans = append(m.chans, ch)
	<-ch
}

func (m *MockLocker) Unlock() {

}

func (m *MockLocker) UnlockByNumber(j int) {
	for {
		for i := 0; i < len(m.chans); i++ {
			if i == j-1 {
				m.chans[i] <- struct{}{}
				return
			}
		}
		time.Sleep(time.Second)
	}
}

func startTestServer(path string, handler fiber.Handler) *fiber.App {
	app := fiber.New()
	app.Get(path, handler)
	go app.Listen(":10801", fiber.ListenConfig{ListenerNetwork: fiber.NetworkTCP})
	time.Sleep(time.Second)
	return app
}

func stopTestServer(app *fiber.App) {
	app.Shutdown()
	time.Sleep(time.Second)
}

func DialWebsocket(t *testing.T, path string, rawUrl string) *websocket.Conn {
	dialer := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
		TLSClientConfig:  utils.GetTlsConfig(),
	}
	ctx := context.Background()
	cpAddr, _ := url.ParseRequestURI(rawUrl)
	webSocketURL := cpAddr
	webSocketURL.Scheme = "ws"
	webSocketURL.Path = path
	requestHeaders := http.Header{}
	requestHeaders.Add("Host", webSocketURL.Host)
	conn, response, err := dialer.DialContext(ctx, webSocketURL.String(), requestHeaders)

	assert.Equal(t, 101, response.StatusCode)
	assert.Nil(t, err)

	return conn
}

func isClosed(conn *websocket.Conn) bool {
	done := make(chan error)

	go func() {
		_, _, err := conn.ReadMessage()
		done <- err
	}()

	isDone, err := waitDoneWithTimeout(done, 3*time.Second)
	if isDone {
		if _, ok := err.(*websocket.CloseError); ok {
			return true
		}
		if _, ok := err.(*net.OpError); ok {
			return true
		}
	}

	return false
}

func waitDoneWithTimeout(done chan error, duration time.Duration) (bool, error) {
	select {
	case <-time.After(duration):
		return false, nil
	case err := <-done:
		close(done)
		return true, err
	}
}

func NewMockVersionController(t *testing.T, withError bool) *VersionController {
	ctrl := gomock.NewController(t)
	dao := mock_dao.NewMockDao(ctrl)

	if withError {
		dao.EXPECT().WithRTx(gomock.Any()).Return(fmt.Errorf("some error rised during preparing"))
	} else {
		dao.EXPECT().WithRTx(gomock.Any()).Return(nil)
	}

	vController := &VersionController{
		watchers: NewWatcherSet(),
		data:     dao,
	}
	return vController
}

func NewMockActiveActiveController(t *testing.T, withError bool) *ActiveActiveController {
	ctrl := gomock.NewController(t)
	dao := mock_dao.NewMockDao(ctrl)

	if withError {
		dao.EXPECT().WithRTx(gomock.Any()).Return(fmt.Errorf("some error rised during preparing"))
	} else {
		dao.EXPECT().WithRTx(gomock.Any()).Return(nil)
	}
	aController := &ActiveActiveController{
		watchers: NewWatcherSet(),
		data:     dao,
	}
	return aController
}
