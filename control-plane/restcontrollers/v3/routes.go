package v3

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	gerrors "github.com/go-errors/errors"
	"github.com/gofiber/fiber/v3"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dr"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/errorcodes"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/restutils"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/pkg/errors"
)

var log = logging.GetLogger("rest-controllers/v3")

//go:generate mockgen -source=routes.go -destination=../../test/mock/restcontrollers/v3/stub_routes.go -package=mock_v3
type RouteService interface {
	RegisterRoutingConfig(ctx context.Context, request dto.RoutingConfigRequestV3) error
	DeleteRoutes(ctx context.Context, request []dto.RouteDeleteRequestV3) ([]*domain.Route, error)
	DeleteDomains(ctx context.Context, request []dto.DomainDeleteRequestV3) ([]*domain.VirtualHostDomain, error)
	GetVirtualService(nodeGroup, virtualService string) (dto.VirtualServiceResponse, error)
	DeleteVirtualService(ctx context.Context, nodeGroup, virtualService string) error
	UpdateVirtualService(ctx context.Context, nodeGroup, virtualServiceName string, virtualService dto.VirtualService) error
	CreateVirtualService(ctx context.Context, nodeGroup string, request dto.VirtualService) error
	DeleteVirtualServiceRoutes(ctx context.Context, rawPrefixes []string, nodeGroup, virtualService, namespace, version string) ([]*domain.Route, error)
	DeleteEndpoints(ctx context.Context, endpointsToDelete []domain.Endpoint, version string) ([]*domain.Endpoint, error)
	DeleteRouteByUUID(ctx context.Context, routeUUID string) (*domain.Route, error)
}

type RequestValidator interface {
	Validate(request dto.RoutingConfigRequestV3) (bool, string)
	ValidateVirtualService(req dto.VirtualService, gateways []string) (bool, string)
	ValidateVirtualServiceUpdate(req dto.VirtualService, nodeGroup string) (bool, string)
	ValidateDomainDeleteRequestV3(req []dto.DomainDeleteRequestV3) (bool, string)
	ValidateStatefulSession(req dto.StatefulSession) (bool, string)
	ValidateRouteStatefulSession(request dto.StatefulSession) (bool, string)
}

type RoutingConfigController struct {
	service   RouteService
	validator RequestValidator
}

func NewRoutingConfigController(service RouteService, validator RequestValidator) *RoutingConfigController {
	return &RoutingConfigController{
		service:   service,
		validator: validator,
	}
}

type stackTracer interface {
	StackTrace() errors.StackTrace
}

type unwrapper interface {
	Unwrap() error
}

func printStack(err error) {
	if err == nil {
		return
	}

	if ster, ok := err.(stackTracer); ok {
		fmt.Printf("%+v", ster)
	}

	if wrapped, ok := err.(unwrapper); ok {
		printStack(wrapped.Unwrap())
	}
}

// HandlePostRoutingConfig godoc
// @Id PostRoutingConfig
// @Summary Post Routing Config
// @Description Post Routing Config
// @Tags control-plane-v3
// @Produce json
// @Param request body dto.RoutingConfigRequestV3 true "RoutingConfigRequestV3"
// @Security ApiKeyAuth
// @Success 201
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v3/routes [post]
func (c *RoutingConfigController) HandlePostRoutingConfig(fiberCtx fiber.Ctx) error {
	ctx := fiberCtx.Context()

	log.DebugC(ctx, "Received request body: \n\t%s", fiberCtx.Body())
	var routingReq dto.RoutingConfigRequestV3
	if err := json.Unmarshal(fiberCtx.Body(), &routingReq); err != nil {
		return errorcodes.NewCpError(errorcodes.UnmarshalRequestError, fmt.Sprintf("Could not unmarshal routes registration v3 request body: %v", err), err)
	}

	if valid, msg := c.validator.Validate(routingReq); !valid {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, fmt.Sprintf("Routes registration v3 is invalid. Cause: %s", msg), nil)
	}

	if dr.GetMode() == dr.Standby {
		return restutils.RespondWithJson(fiberCtx, http.StatusCreated, nil)
	}

	if err := c.service.RegisterRoutingConfig(ctx, routingReq); err != nil {
		log.ErrorC(ctx, "Failed to register routes via v3 api: %v", err)
		if gerrors.Is(err, services.BadRouteRegistrationRequest) {
			return errorcodes.NewCpError(errorcodes.ValidationRequestError, fmt.Sprintf("Failed to register routes via v3 api. Cause: %s", err.Error()), err)
		}
		return err
	}
	return restutils.RespondWithJson(fiberCtx, http.StatusCreated, nil)
}

// HandleGetVirtualService godoc
// @Id GetVirtualService
// @Summary Get Virtual Service
// @Description Get Virtual Service
// @Tags control-plane-v3
// @Produce json
// @Param nodeGroup path string true "nodeGroup"
// @Param virtualServiceName path string true "virtualServiceName"
// @Security ApiKeyAuth
// @Success 200 {object} dto.VirtualServiceResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v3/routes/{nodeGroup}/{virtualServiceName} [get]
func (c *RoutingConfigController) HandleGetVirtualService(fiberCtx fiber.Ctx) error {
	nodeGroup := restutils.GetFiberParam(fiberCtx, "nodeGroup")
	if strings.TrimSpace(nodeGroup) == "" {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, "Path variable 'nodeGroup' must not be empty.", nil)
	}
	virtualServiceName := restutils.GetFiberParam(fiberCtx, "virtualServiceName")
	if strings.TrimSpace(virtualServiceName) == "" {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, "Path variable 'virtualServiceName' must not be empty.", nil)
	}
	virtualService, err := c.service.GetVirtualService(nodeGroup, virtualServiceName)
	if err != nil {
		log.ErrorC(fiberCtx.Context(), "Failed to get virtual service via v3 api: %v", err)
		return err
	}

	return restutils.RespondWithJson(fiberCtx, http.StatusOK, virtualService)
}

// HandleDeleteVirtualService godoc
// @Id DeleteVirtualService
// @Summary Delete Virtual Service
// @Description Delete Virtual Service
// @Tags control-plane-v3
// @Produce json
// @Param nodeGroup path string true "nodeGroup"
// @Param virtualServiceName path string true "virtualServiceName"
// @Security ApiKeyAuth
// @Success 200
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v3/routes/{nodeGroup}/{virtualServiceName} [delete]
func (c *RoutingConfigController) HandleDeleteVirtualService(fiberCtx fiber.Ctx) error {
	ctx := fiberCtx.Context()
	nodeGroup := restutils.GetFiberParam(fiberCtx, "nodeGroup")
	if strings.TrimSpace(nodeGroup) == "" {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, "Path variable 'nodeGroup' must not be empty.", nil)
	}
	virtualService := restutils.GetFiberParam(fiberCtx, "virtualServiceName")
	if strings.TrimSpace(virtualService) == "" {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, "Path variable 'virtualServiceName' must not be empty.", nil)
	}

	if dr.GetMode() == dr.Standby {
		return restutils.RespondWithJson(fiberCtx, http.StatusOK, nil)
	}

	if err := c.service.DeleteVirtualService(ctx, nodeGroup, virtualService); err != nil {
		log.ErrorC(ctx, "Failed to delete routes via v3 api: %v", err)
		return err
	}
	return restutils.RespondWithJson(fiberCtx, http.StatusOK, nil)
}

// HandlePutVirtualService godoc
// @Id PutVirtualService
// @Summary Put Virtual Service
// @Description Put Virtual Service
// @Tags control-plane-v3
// @Produce json
// @Param nodeGroup path string true "nodeGroup"
// @Param virtualServiceName path string true "virtualServiceName"
// @Param request body dto.VirtualService true "VirtualService"
// @Security ApiKeyAuth
// @Success 200
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v3/routes/{nodeGroup}/{virtualServiceName} [put]
func (c *RoutingConfigController) HandlePutVirtualService(fiberCtx fiber.Ctx) error {
	ctx := fiberCtx.Context()
	nodeGroup := restutils.GetFiberParam(fiberCtx, "nodeGroup")
	if strings.TrimSpace(nodeGroup) == "" {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, "Path variable 'nodeGroup' must not be empty.", nil)
	}
	virtualService := restutils.GetFiberParam(fiberCtx, "virtualServiceName")
	if strings.TrimSpace(virtualService) == "" {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, "Path variable 'virtualService' must not be empty.", nil)
	}
	var virtualServiceReq dto.VirtualService
	if err := json.Unmarshal(fiberCtx.Body(), &virtualServiceReq); err != nil {
		return errorcodes.NewCpError(errorcodes.UnmarshalRequestError, fmt.Sprintf("Failed to unmarshal virtual service v3 request body %v", err), err)
	}

	if valid, msg := c.validator.ValidateVirtualServiceUpdate(virtualServiceReq, nodeGroup); !valid {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, fmt.Sprintf("Update virtual service request v3 is invalid. Cause: %s", msg), nil)
	}

	if dr.GetMode() == dr.Standby {
		return restutils.RespondWithJson(fiberCtx, http.StatusOK, nil)
	}

	if err := c.service.UpdateVirtualService(ctx, nodeGroup, virtualService, virtualServiceReq); err != nil {
		log.ErrorC(ctx, "Failed to update routes via v3 api: %v", err)
		if errors.Is(err, entity.LegacyRouteDisallowed) {
			return errorcodes.NewCpError(errorcodes.BlueGreenConflictError, err.Error(), err)
		}
		return err
	}
	return restutils.RespondWithJson(fiberCtx, http.StatusOK, nil)
}

// HandleCreateVirtualService godoc
// @Id CreateVirtualService
// @Summary Create Virtual Service
// @Description Create Virtual Service
// @Tags control-plane-v3
// @Produce json
// @Param nodeGroup path string true "nodeGroup"
// @Param virtualServiceName path string true "virtualServiceName"
// @Param request body dto.VirtualService true "VirtualService"
// @Security ApiKeyAuth
// @Success 201
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v3/routes/{nodeGroup}/{virtualServiceName} [post]
func (c *RoutingConfigController) HandleCreateVirtualService(fiberCtx fiber.Ctx) error {
	ctx := fiberCtx.Context()
	nodeGroup := restutils.GetFiberParam(fiberCtx, "nodeGroup")
	if strings.TrimSpace(nodeGroup) == "" {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, "Path variable 'nodeGroup' must not be empty.", nil)
	}
	virtualServiceName := restutils.GetFiberParam(fiberCtx, "virtualServiceName")
	if strings.TrimSpace(virtualServiceName) == "" {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, "Path variable 'virtualServiceName' must not be empty.", nil)
	}
	var virtualServiceRequest dto.VirtualService
	if err := json.Unmarshal(fiberCtx.Body(), &virtualServiceRequest); err != nil {
		return errorcodes.NewCpError(errorcodes.UnmarshalRequestError, fmt.Sprintf("Failed to unmarshal virtual service creation request body JSON: %v", err), err)
	}
	virtualServiceRequest.Name = virtualServiceName
	if valid, msg := c.validator.ValidateVirtualService(virtualServiceRequest, []string{nodeGroup}); !valid {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, fmt.Sprintf("Request for creation virtual service is invalid. Cause: %s", msg), nil)
	}

	if dr.GetMode() == dr.Standby {
		return restutils.RespondWithJson(fiberCtx, http.StatusCreated, nil)
	}

	if err := c.service.CreateVirtualService(ctx, nodeGroup, virtualServiceRequest); err != nil {
		log.ErrorC(ctx, "Failed to create virtual service via v3 api: %v", err)
		return err
	}
	return restutils.RespondWithJson(fiberCtx, http.StatusCreated, nil)
}

// HandleDeleteVirtualServiceRoutes godoc
// @Id DeleteVirtualServiceRoutes
// @Summary Delete Virtual Service Routes
// @Description Delete Virtual Service Routes
// @Tags control-plane-v3
// @Produce json
// @Param request body []dto.RouteDeleteRequestV3 true "RouteDeleteRequestV3"
// @Security ApiKeyAuth
// @Success 200 {array} domain.Route
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v3/routes [delete]
func (c *RoutingConfigController) HandleDeleteVirtualServiceRoutes(fiberCtx fiber.Ctx) error {
	ctx := fiberCtx.Context()
	var request []dto.RouteDeleteRequestV3
	if err := json.Unmarshal(fiberCtx.Body(), &request); err != nil {
		return errorcodes.NewCpError(errorcodes.UnmarshalRequestError, fmt.Sprintf("Failed to unmarshal routes deletion v3 request body: %v", err), err)
	}

	if dr.GetMode() == dr.Standby {
		return restutils.RespondWithJson(fiberCtx, http.StatusOK, nil)
	}

	deletedRoutes, err := c.service.DeleteRoutes(ctx, request)
	if err != nil {
		log.ErrorC(ctx, "Failed to delete routes via v3 api: %v", err)
		return err
	}
	log.InfoC(ctx, "deleted routes: %s", deletedRoutes)
	return restutils.ResponseOk(fiberCtx, deletedRoutes)
}

// HandleDeleteVirtualServiceDomains godoc
// @Id DeleteVirtualServiceDomains
// @Summary Delete Virtual Service Domains
// @Description Delete Virtual Service Domains
// @Tags control-plane-v3
// @Produce json
// @Param request body []dto.DomainDeleteRequestV3 true "DomainDeleteRequestV3"
// @Security ApiKeyAuth
// @Success 200
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v3/domains [delete]
func (c *RoutingConfigController) HandleDeleteVirtualServiceDomains(fiberCtx fiber.Ctx) error {
	ctx := fiberCtx.Context()
	log.DebugC(ctx, "Request to delete virtual service domains")

	var request []dto.DomainDeleteRequestV3
	if err := json.Unmarshal(fiberCtx.Body(), &request); err != nil {
		return errorcodes.NewCpError(errorcodes.UnmarshalRequestError, fmt.Sprintf("Failed to unmarshal domains deletion v3 request body: %v", err), err)
	}
	if valid, msg := c.validator.ValidateDomainDeleteRequestV3(request); !valid {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, fmt.Sprintf("Request for deleting virtual service domains is invalid. Cause: %s", msg), nil)
	}

	if dr.GetMode() == dr.Standby {
		return restutils.ResponseOk(fiberCtx, nil)
	}

	deletedDomains, err := c.service.DeleteDomains(ctx, request)
	if err != nil {
		log.ErrorC(ctx, "Failed to delete virtual service domains via v3 api: %v", err)
		return err
	}
	log.InfoC(ctx, "Deleted domains: %s", deletedDomains)
	return restutils.ResponseOk(fiberCtx, nil)
}

// HandleDeleteEndpoints godoc
// @Id HandleDeleteEndpointsV3
// @Summary Delete Endpoints V3
// @Description Delete Endpoints V3
// @Tags control-plane-v3
// @Produce json
// @Param request body []dto.EndpointDeleteRequest true "EndpointDeleteRequest"
// @Security ApiKeyAuth
// @Success 200 {array} dto.Endpoint
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v3/endpoints [delete]
func (c *RoutingConfigController) HandleDeleteEndpoints(fiberCtx fiber.Ctx) error {
	ctx := fiberCtx.Context()
	log.InfoC(ctx, "Request to delete endpoints")

	var deleteRequests []dto.EndpointDeleteRequest
	err := json.Unmarshal(fiberCtx.Body(), &deleteRequests)
	if err != nil {
		return errorcodes.NewCpError(errorcodes.UnmarshalRequestError, fmt.Sprintf("Failed to unmarshal delete endpoints request body JSON: %v", err), err)
	}

	deletedEndpoints := make([]*domain.Endpoint, 0)
	if dr.GetMode() == dr.Standby {
		return restutils.ResponseOk(fiberCtx, deletedEndpoints)
	}
	for _, deleteRequest := range deleteRequests {

		endpointsToDelete := make([]domain.Endpoint, len(deleteRequest.Endpoints))
		for i, r := range deleteRequest.Endpoints {
			endpointsToDelete[i] = domain.Endpoint{Address: r.Address, Port: r.Port, DeploymentVersion: deleteRequest.Version}
		}
		log.InfoC(ctx, "Deleting endpoints:%v", endpointsToDelete)
		endpoints, err := c.service.DeleteEndpoints(ctx, endpointsToDelete, deleteRequest.Version)
		if err != nil {
			log.ErrorC(ctx, "Failed to delete endpoints: %v", err)
			return err
		}
		if endpoints != nil && len(endpoints) > 0 {
			deletedEndpoints = append(deletedEndpoints, endpoints...)
		}
	}
	log.InfoC(ctx, "Deleted endpoints: %s", deletedEndpoints)
	return restutils.ResponseOk(fiberCtx, deletedEndpoints)
}
