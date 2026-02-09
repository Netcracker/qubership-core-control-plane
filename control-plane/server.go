package main

import (
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/lib"
	fiberSec "github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v2/security"
	"github.com/netcracker/qubership-core-lib-go/v3/security"
	"github.com/netcracker/qubership-core-lib-go/v3/serviceloader"

	// memlimit sets memory limit = 0.9 of cgroup memory limit
	_ "github.com/netcracker/qubership-core-lib-go/v3/memlimit"
)

func init() {
	serviceloader.Register(1, &security.DummyToken{})
	serviceloader.Register(1, &fiberSec.DummyFiberServerSecurityMiddleware{})
}

// @title 						Control Plane API
// @version 					2.0
// @description 				Control-plane is a central microservice of Service Mesh which is responsible for all the gateways configuration. For more information, visit our Documentation(https://github.com/Netcracker/qubership-core-control-plane/tree/main/README.md).
// @tag.name routes-controller-v1
// @tag.description routing operations for v1 apis
// @tag.name control-plane-v2
// @tag.description routing operations for v2 apis
// @tag.name control-plane-v3
// @tag.description routing operations for v3 apis
// @Produce 					json
// @securityDefinitions.apikey 	ApiKeyAuth
// @in 							header
// @name 						Authorization
// @externalDocs.description 	OpenAPI
// @externalDocs.url       		https://swagger.io/resources/open-api/
//
//go:generate go run github.com/swaggo/swag/cmd/swag init --generalInfo /restcontrollers/v1/routes.go --parseDependency  --parseGoList=false --parseDepth 2
func main() {
	lib.RunServer()
}
