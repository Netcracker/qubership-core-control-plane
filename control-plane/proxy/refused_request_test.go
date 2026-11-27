package proxy

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/clustering"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testPath = "/api/v2/control-plane/routes/node"
	// The headers alone: Fiber refuses the request on the declared length, over the app's BodyLimit of 1024.
	bodyOverTheLimit = "DELETE " + testPath + " HTTP/1.1\r\nHost: test\r\nContent-Length: 2048\r\nConnection: close\r\n\r\n"
	bodyWithinLimit  = "DELETE " + testPath + " HTTP/1.1\r\nHost: test\r\nContent-Length: 2\r\nConnection: close\r\n\r\n{}"
)

// A request Fiber v3 refused while reading it runs the group middleware but no route; a proxied copy, without the
// unread body, used to reach the master, which executed the DELETE while the client received 413.
func TestProxyRequestsToMaster_doesNotProxyARequestRefusedWhileReading(t *testing.T) {
	calls, upstream := startRecordingUpstream(t)
	setNodeState(t, upstream, clustering.Slave)
	addr, _ := serveGroup(t, ProxyRequestsToMaster())

	tests := []struct {
		name       string
		request    string
		wantStatus int
		wantCalls  int32
	}{
		{name: "body over the limit", request: bodyOverTheLimit, wantStatus: http.StatusRequestEntityTooLarge, wantCalls: 0},
		{name: "body within the limit", request: bodyWithinLimit, wantStatus: http.StatusNoContent, wantCalls: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls.Store(0)

			status := sendRaw(t, addr, tt.request)

			assert.Equal(t, tt.wantStatus, status, "status")
			assert.Equal(t, tt.wantCalls, calls.Load(), "requests the master received")
		})
	}
}

func TestProxyRequestsToMaster_answersARefusedRequestWithoutWaitingForTheMaster(t *testing.T) {
	_, upstream := startRecordingUpstream(t)
	setNodeState(t, upstream, clustering.Master)
	addr, laterCalls := serveGroup(t, ProxyRequestsToMaster())
	// Let a request stuck in WaitMasterReady finish, or the app's shutdown waits for it.
	t.Cleanup(clustering.CurrentNodeState.SetMasterReady)

	status := sendRaw(t, addr, bodyOverTheLimit)

	assert.Equal(t, http.StatusRequestEntityTooLarge, status, "status")
	assert.Zero(t, laterCalls.Load(), "calls of the group middleware after the proxy")
}

func TestServiceProxyHandler_doesNotProxyARequestRefusedWhileReading(t *testing.T) {
	calls, upstream := startRecordingUpstream(t)
	never := func(fiber.Ctx) bool { return false }
	always := func(fiber.Ctx) bool { return true }
	fallback := func(c fiber.Ctx) error { return c.SendStatus(http.StatusTeapot) }
	addr, _ := serveGroup(t, NewService(upstream.address(), never, always, fallback).ProxyHandler())

	tests := []struct {
		name       string
		request    string
		wantStatus int
		wantCalls  int32
	}{
		{name: "body over the limit", request: bodyOverTheLimit, wantStatus: http.StatusRequestEntityTooLarge, wantCalls: 0},
		{name: "body within the limit", request: bodyWithinLimit, wantStatus: http.StatusNoContent, wantCalls: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls.Store(0)

			status := sendRaw(t, addr, tt.request)

			assert.Equal(t, tt.wantStatus, status, "status")
			assert.Equal(t, tt.wantCalls, calls.Load(), "requests the upstream received")
		})
	}
}

// The group middleware after the proxy would parse the unread body of a refused request, and a route validator that
// fails resets the response headers, X-Request-Id among them. It used to run for such a request.
func TestServiceProxyHandler_endsTheChainForARefusedRequest(t *testing.T) {
	always := func(fiber.Ctx) bool { return true }
	fallback := func(c fiber.Ctx) error { return c.SendStatus(http.StatusTeapot) }
	addr, laterCalls := serveGroup(t, NewService("", always, always, fallback).ProxyHandler())

	tests := []struct {
		name           string
		request        string
		wantStatus     int
		wantLaterCalls int32
	}{
		{name: "body over the limit", request: bodyOverTheLimit, wantStatus: http.StatusRequestEntityTooLarge, wantLaterCalls: 0},
		{name: "body within the limit", request: bodyWithinLimit, wantStatus: http.StatusNoContent, wantLaterCalls: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			laterCalls.Store(0)

			status := sendRaw(t, addr, tt.request)

			assert.Equal(t, tt.wantStatus, status, "status")
			assert.Equal(t, tt.wantLaterCalls, laterCalls.Load(), "calls of the group middleware after the proxy")
		})
	}
}

type upstreamNode struct {
	ip   string
	port uint16
}

func (n upstreamNode) address() string {
	return net.JoinHostPort(n.ip, strconv.Itoa(int(n.port)))
}

func startRecordingUpstream(t *testing.T) (*atomic.Int32, upstreamNode) {
	t.Helper()
	calls := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)
	return calls, upstreamNode{ip: u.Hostname(), port: uint16(port)}
}

func setNodeState(t *testing.T, master upstreamNode, role clustering.Role) {
	t.Helper()
	clustering.CurrentNodeState.ChangeNodeState(clustering.NodeInfo{IP: master.ip, HttpPort: master.port}, role)
	t.Cleanup(func() { clustering.CurrentNodeState.ChangeNodeState(clustering.NodeInfo{}, clustering.Initial) })
}

// serveGroup serves the DELETE route under an /api/v2 group with middleware, the way the service registers its API,
// on a real listener: app.Test cannot send a request whose body Fiber refuses while reading it. A second group
// middleware, counted in the returned counter, stands for the route validators registered after the proxy.
func serveGroup(t *testing.T, middleware fiber.Handler) (string, *atomic.Int32) {
	t.Helper()
	laterCalls := &atomic.Int32{}
	later := func(c fiber.Ctx) error {
		laterCalls.Add(1)
		return c.Next()
	}
	app := fiber.New(fiber.Config{BodyLimit: 1024})
	app.Group("/api/v2", middleware, later).Delete("/control-plane/routes/node", func(c fiber.Ctx) error {
		return c.SendStatus(http.StatusNoContent)
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = app.Listener(listener, fiber.ListenConfig{DisableStartupMessage: true}) }()
	t.Cleanup(func() { _ = app.Shutdown() })
	return listener.Addr().String(), laterCalls
}

func sendRaw(t *testing.T, addr, request string) int {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
	_, err = io.WriteString(conn, request)
	require.NoError(t, err)
	response, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err, "response to %q", request)
	defer func() { _ = response.Body.Close() }()
	return response.StatusCode
}
