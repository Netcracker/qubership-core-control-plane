package proxy

import (
	"bytes"
	"github.com/gofiber/fiber/v2"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/clustering"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/errorcodes"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/valyala/fasthttp"
	"net/http"
	"strings"
)

var (
	logger      logging.Logger
	 // Load configuration with defaults
          maxConnsPerHost, _ := strconv.Atoi(
              configloader.GetOrDefaultString("http.proxy.client.maxConnsPerHost", "2048"))


	proxyClient = &fasthttp.Client{TLSConfig: util.GetTlsConfigWithoutHostNameValidation(), DialDualStack: true}
)

func init() {
	logger = logging.GetLogger("reverse-proxy")
}

type Service struct {
	address          string
	conditionToServe func(c *fiber.Ctx) bool
	conditionToProxy func(c *fiber.Ctx) bool
	fallbackFunc     func(c *fiber.Ctx) error
}

func NewService(address string, conditionToServe func(c *fiber.Ctx) bool, conditionToProxy func(c *fiber.Ctx) bool, fallbackFunc func(c *fiber.Ctx) error) *Service {
	return &Service{address: address, conditionToServe: conditionToServe, conditionToProxy: conditionToProxy, fallbackFunc: fallbackFunc}
}

func (srv *Service) ProxyHandler() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		if srv.conditionToServe(ctx) {
			logger.Debugf("Serving request with method %s on %s", ctx.Method(), ctx.Path())
			return ctx.Next()
		} else if srv.conditionToProxy(ctx) {
			logger.Debugf("Proxying request with method %s on %s to %s", ctx.Method(), ctx.Path(), srv.address)
			proxyRequest(srv.address, ctx)
		} else {
			logger.Debugf("Executing fallback function for request with method %s on %s", ctx.Method(), ctx.Path())
			return srv.fallbackFunc(ctx)
		}
		return nil
	}
}

func proxyRequest(addressToRedirect string, fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	logger.InfoC(ctx, "Redirect incoming request %s to upstream node %s", fiberCtx.Path(), addressToRedirect)
	req := fiberCtx.Request()
	resp := fiberCtx.Response()
	prepareRequest(req)
	req.SetHost(addressToRedirect)
	req.Header.SetHost(addressToRedirect)
	logger.DebugC(ctx, "Request URI from prepared request: %s", req.URI().String())
	var err error
	if err = proxyClient.Do(req, resp); err != nil {
		logger.Errorf("error when proxying request %s to master: %s", req.URI().String(), err)
	}
	postprocessResponse(resp)
	return err
}

func prepareRequest(req *fasthttp.Request) {
	webSocketConnection := getWebSocketUpgradeHeaderRequest(req)

	// do not proxy "Connection" header.
	req.Header.Del("Connection")
	if webSocketConnection != "" {
		// After stripping all the hop-by-hop connection headers above, add back any
		// necessary for protocol upgrades, such as for websockets.
		req.Header.Set("Connection", "Upgrade")
		req.Header.Set("Upgrade", webSocketConnection)
	} else {
		req.SetConnectionClose()
	}

}

func postprocessResponse(resp *fasthttp.Response) {
	// do not proxy "Connection" header
	resp.Header.Del("Connection")
}

func getWebSocketUpgradeHeaderRequest(req *fasthttp.Request) string {
	connHeader := req.Header.Peek("Connection")
	if bytes.EqualFold(connHeader, []byte("upgrade")) {
		return string(req.Header.Peek("Upgrade"))
	}
	return ""
}

func ProxyRequestsToMaster() fiber.Handler {
	return func(ctx *fiber.Ctx) error {

		switch clustering.CurrentNodeState.GetRole() {
		case clustering.Master:
			clustering.CurrentNodeState.WaitMasterReady()
			return ctx.Next()
		case clustering.Slave:
			if urlPathBelongsToApi(ctx.Path()) {
				if ctx.Method() == http.MethodGet {
					return ctx.Next()
				} else {
					return proxyRequest(clustering.CurrentNodeState.GetHttpAddress(), ctx)
				}
			} else {
				return ctx.Next()
			}
		case clustering.Phantom:
			if ctx.Method() == http.MethodGet {
				return ctx.Next()
			} else {
				return errorcodes.NewCpError(errorcodes.PhantomModeError, "The database is currently unavailable", nil)
			}
		default:
			return ctx.SendStatus(http.StatusInternalServerError)
		}
		return nil
	}
}

func urlPathBelongsToApi(path string) bool {
	return strings.HasPrefix(path, "/api/")
}
