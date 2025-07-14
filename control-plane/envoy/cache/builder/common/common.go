package common

import (
	"os"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	luav3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/lua/v3"
	stateful_sessionv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/stateful_session/v3"
	cookiev3 "github.com/envoyproxy/go-control-plane/envoy/extensions/http/stateful_session/cookie/v3"
	httpv3 "github.com/envoyproxy/go-control-plane/envoy/type/http/v3"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

var (
	logger     logging.Logger
	ipV4Compat = false
)

func init() {
	logger = logging.GetLogger("EnvoyConfigBuilder#common")
	resolveIPVersion()
}

func IsReservedGatewayName(name string) bool {
	return name == domain.PublicGateway || name == domain.PrivateGateway || name == domain.InternalGateway
}

func resolveIPVersion() {
	if ipStack := os.Getenv("IP_STACK"); ipStack != "" {
		ipV4Compat = ipStack == "v6"
	}
}

func BuildSocketAddr(address string, port uint32) *core.Address {
	return &core.Address{Address: &core.Address_SocketAddress{
		SocketAddress: &core.SocketAddress{
			Protocol: core.SocketAddress_TCP,
			Address:  address,
			PortSpecifier: &core.SocketAddress_PortValue{
				PortValue: port,
			},
			Ipv4Compat: ipV4Compat,
		},
	}}
}

func BuildFilenameDataSource(filename string) *core.DataSource {
	return &core.DataSource{Specifier: &core.DataSource_Filename{Filename: filename}}
}

func BuildInlineStringDataSource(inlineString string) *core.DataSource {
	return &core.DataSource{Specifier: &core.DataSource_InlineString{InlineString: inlineString}}
}

func BuildCookieBasedSessionFilterForListener() (*any.Any, error) {
	statefulSession := &stateful_sessionv3.StatefulSession{}
	return ptypes.MarshalAny(statefulSession)
}

func BuildStatefulSessionPerRoute(statefulSession *domain.StatefulSession) (*any.Any, error) {
	var statefulSessionPerRoute *stateful_sessionv3.StatefulSessionPerRoute
	if statefulSession.Enabled {
		cookieSpec := &httpv3.Cookie{
			Name: statefulSession.CookieName,
			Path: statefulSession.CookiePath,
		}
		if statefulSession.CookieTtl != nil {
			cookieSpec.Ttl = util.MillisToDuration(*statefulSession.CookieTtl)
		}
		cookieBasedFilter := &cookiev3.CookieBasedSessionState{Cookie: cookieSpec}
		marshalledCookieBasedFilter, err := ptypes.MarshalAny(cookieBasedFilter)
		if err != nil {
			return nil, err
		}
		statefulSessionPerRoute = &stateful_sessionv3.StatefulSessionPerRoute{
			Override: &stateful_sessionv3.StatefulSessionPerRoute_StatefulSession{
				StatefulSession: &stateful_sessionv3.StatefulSession{
					SessionState: &core.TypedExtensionConfig{
						Name:        "envoy.http.stateful_session.cookie",
						TypedConfig: marshalledCookieBasedFilter,
					},
				},
			},
		}
	} else {
		statefulSessionPerRoute = &stateful_sessionv3.StatefulSessionPerRoute{
			Override: &stateful_sessionv3.StatefulSessionPerRoute_Disabled{
				Disabled: true,
			},
		}
	}
	return ptypes.MarshalAny(statefulSessionPerRoute)
}


func BuildLuaFilterPerRoute(luaFilter *domain.LuaFilter) (*any.Any, error) {
	luaFilterPerRoute := &luav3.LuaPerRoute{
		Override: &luav3.LuaPerRoute_SourceCode{
			SourceCode: &core.DataSource{
				Specifier: &core.DataSource_InlineString{
					InlineString: luaFilter.LuaScript,
				},
			},
		},
	}

	return ptypes.MarshalAny(luaFilterPerRoute)
}