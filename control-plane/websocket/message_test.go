package websocket

import (
	"context"
	"fmt"
	"github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/golang/mock/gomock"
	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/clustering"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	mock_dao "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/dao"
	"github.com/netcracker/qubership-core-lib-go/v3/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"
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

const websocketTestTimeout = 5 * time.Second

func controller_WebsocketIsClosed_OnPreparingDeploymentVersionFails(t *testing.T, handler fiber.Handler, watcherSet *WatcherSet) {
	url := startTestServer(t, "/versions/watch", handler)
	conn := DialWebsocket(t, "/versions/watch", url)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(websocketTestTimeout)))
	_, _, err := conn.ReadMessage()
	var closeError *websocket.CloseError
	require.ErrorAs(t, err, &closeError, "server must close the failed WebSocket, not time out")
	waitForWatcherCount(t, watcherSet, 0)
}

func controller_WebsocketIsClosed_OnClosingConnection(t *testing.T, handler fiber.Handler, watcherSet *WatcherSet) {
	url := startTestServer(t, "/versions/watch", handler)
	conn := DialWebsocket(t, "/versions/watch", url)
	waitForWatcherCount(t, watcherSet, 1)

	// A pong proves the connection is usable without timing out a read goroutine.
	pong := make(chan struct{}, 1)
	readDone := make(chan error, 1)
	conn.SetPongHandler(func(string) error {
		select {
		case pong <- struct{}{}:
		default:
		}
		return nil
	})
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				readDone <- err
				return
			}
		}
	}()
	require.NoError(t, conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(websocketTestTimeout)))
	select {
	case <-pong:
	case err := <-readDone:
		t.Fatalf("connection closed before pong: %v", err)
	case <-time.After(websocketTestTimeout):
		t.Fatal("WebSocket did not respond to ping")
	}
	require.NoError(t, conn.Close())
	select {
	case <-readDone:
	case <-time.After(websocketTestTimeout):
		t.Fatal("WebSocket reader did not stop")
	}
	waitForWatcherCount(t, watcherSet, 0)
}

func Test_VersionController_ResetConnections_ClosesActiveWebsocket(t *testing.T) {
	controller := NewMockVersionController(t, false)
	url := startTestServer(t, "/versions/watch", controller.HandleVersionsWatch)
	conn := DialWebsocket(t, "/versions/watch", url)
	waitForWatcherCount(t, controller.watchers, 1)

	done := make(chan struct{})
	go func() {
		controller.ResetConnections(clustering.NodeInfo{}, clustering.Role(1))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(websocketTestTimeout):
		t.Fatal("ResetConnections did not finish")
	}
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(websocketTestTimeout)))
	_, _, err := conn.ReadMessage()
	var closeError *websocket.CloseError
	require.ErrorAs(t, err, &closeError, "reset must close the active WebSocket")
	waitForWatcherCount(t, controller.watchers, 0)
}

func waitForWatcherCount(t *testing.T, watchers *WatcherSet, want int) {
	t.Helper()
	require.Eventually(t, func() bool {
		watchers.Lock()
		defer watchers.Unlock()
		return len(watchers.watchersMap) == want
	}, websocketTestTimeout, time.Millisecond, "unexpected number of WebSocket watchers")
}

func Test_VersionController_WebsocketIsClosedAndResetConnectionsIsRun_ResetConnectionsDoesNotStickOnWritingInChannel(t *testing.T) {
	ctrl := gomock.NewController(t)
	dao := mock_dao.NewMockDao(ctrl)
	dao.EXPECT().WithRTx(gomock.Any()).Return(nil)

	locker := &controlledLocker{requests: make(chan chan struct{}, 3), unlocked: make(chan struct{}, 3), done: make(chan struct{})}
	t.Cleanup(func() { close(locker.done) })
	controller := &VersionController{
		watchers: &WatcherSet{Locker: locker, watchersMap: make(map[*watcher]bool)},
		data:     dao,
	}
	url := startTestServer(t, "/versions/watch", controller.HandleVersionsWatch)
	conn := DialWebsocket(t, "/versions/watch", url)

	// Push waits at the first lock. Let registration finish before closing.
	first := locker.next(t)
	first()
	require.NoError(t, conn.Close())
	// Remove reaches the second lock only after the watcher has stopped.
	second := locker.next(t)

	done := make(chan struct{})
	go func() {
		controller.ResetConnections(clustering.NodeInfo{}, clustering.Role(1))
		close(done)
	}()
	// Keep Remove blocked while ResetConnections inspects the stopped watcher.
	third := locker.next(t)
	third()
	select {
	case <-done:
	case <-time.After(websocketTestTimeout):
		t.Fatal("ResetConnections blocked on the stopped watcher's quit channel")
	}
	second()
	for i := 0; i < 3; i++ {
		select {
		case <-locker.unlocked:
		case <-time.After(websocketTestTimeout):
			t.Fatal("watcher operation did not release its lock")
		}
	}
}

// Each lock acquisition is handed to the test explicitly, without polling.
// Remove is deliberately held while ResetConnections acquires the next gate.
type controlledLocker struct {
	requests chan chan struct{}
	unlocked chan struct{}
	done     chan struct{}
}

func (m *controlledLocker) Lock() {
	gate := make(chan struct{})
	select {
	case m.requests <- gate:
	case <-m.done:
		return
	}
	select {
	case <-gate:
	case <-m.done:
	}
}

func (m *controlledLocker) Unlock() {
	select {
	case m.unlocked <- struct{}{}:
	case <-m.done:
	}
}

func (m *controlledLocker) next(t *testing.T) func() {
	t.Helper()
	select {
	case gate := <-m.requests:
		release := sync.OnceFunc(func() { close(gate) })
		t.Cleanup(release)
		return release
	case <-time.After(websocketTestTimeout):
		t.Fatal("expected watcher lock acquisition did not occur")
		return nil
	}
}

func startTestServer(t *testing.T, path string, handler fiber.Handler) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get(path, handler)
	done := make(chan error, 1)
	go func() { done <- app.Listener(listener) }()
	t.Cleanup(func() {
		require.NoError(t, app.ShutdownWithTimeout(websocketTestTimeout))
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(websocketTestTimeout):
			t.Error("test HTTP server did not stop")
		}
	})
	return "http://" + listener.Addr().String()
}

func DialWebsocket(t *testing.T, path string, rawUrl string) *websocket.Conn {
	t.Helper()
	dialer := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: websocketTestTimeout,
		TLSClientConfig:  utils.GetTlsConfig(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), websocketTestTimeout)
	defer cancel()
	webSocketURL, err := url.ParseRequestURI(rawUrl)
	require.NoError(t, err)
	webSocketURL.Scheme = "ws"
	webSocketURL.Path = path
	requestHeaders := http.Header{}
	requestHeaders.Add("Host", webSocketURL.Host)
	conn, response, err := dialer.DialContext(ctx, webSocketURL.String(), requestHeaders)
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
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
