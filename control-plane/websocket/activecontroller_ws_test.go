package websocket

import (
	"net"
	"testing"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	mock_dao "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/dao"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActiveActiveWatchOverWebSocket(t *testing.T) {
	ctrl := gomock.NewController(t)
	data := mock_dao.NewMockDao(ctrl)
	repository := mock_dao.NewMockRepository(ctrl)
	data.EXPECT().WithRTx(gomock.Any()).DoAndReturn(func(f func(dao.Repository) error) error { return f(repository) })
	repository.EXPECT().FindClusterByName(gomock.Any()).Return(nil, nil)
	controller := &ActiveActiveController{data: data, watchers: NewWatcherSet()}
	app := fiber.New()
	app.Get("/watch", controller.HandleActiveActiveWatch)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = app.Listener(listener) }()
	t.Cleanup(func() { _ = app.Shutdown() })

	conn, _, err := websocket.DefaultDialer.Dial("ws://"+listener.Addr().String()+"/watch", nil)

	require.NoError(t, err, "upgrade")
	defer conn.Close()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, message, err := conn.ReadMessage()
	require.NoError(t, err, "read the first message")
	assert.JSONEq(t, `{"aliases":[],"currentAlias":""}`, string(message), "first message")
}
