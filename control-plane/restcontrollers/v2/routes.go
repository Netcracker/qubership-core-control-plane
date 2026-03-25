package v2

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dr"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/errorcodes"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/restutils"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/configresources"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
	errs "github.com/pkg/errors"
	"io"
	"net/http"
)

//go:generate mockgen -source=routes.go -destination=../../test/mock/restcontrollers/v2/stub_routes.go -package=mock_v2
type RouteService interface {
	RegisterRoutes(ctx context.Context, requests []dto.RouteRegistrationRequest, nodeGroup string) error
	DeleteRoutes(ctx context.Context, nodeGroup, namespace, version string, prefixes ...string) ([]*domain.Route, error)
	DeleteRouteByUUID(ctx context.Context, routeUUID string) (*domain.Route, error)
	GetNodeGroups() ([]*domain.NodeGroup, error)
	DeleteEndpoints(ctx context.Context, endpointsToDelete []domain.Endpoint, version string) ([]*domain.Endpoint, error)
	GetRegisterRoutesResource() configresources.Resource
}

type RoutesController struct {
	service   RouteService
	validator RequestValidator
}

type RequestValidator interface {
	Validate(requests []dto.RouteRegistrationRequest, nodeGroup string) (bool, string)
}

func NewRoutesController(service RouteService, validator RequestValidator) *RoutesController {
	return &RoutesController{
		service:   service,
		validator: validator,
	}
}

func ValidateRouteDeleteRequest() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		var reqBody []dto.RouteDeleteRequest
		if err := json.Unmarshal(ctx.Body(), &reqBody); err != nil && err != io.EOF {
			return restutils.RespondWithError(ctx, http.StatusBadRequest, fmt.Sprintf("Error during validation. Invalid request body: %v", err))
		}

		for _, delRequest := range reqBody {
			if delRequest.Version == "" && len(delRequest.Routes) == 0 && delRequest.Namespace == "" {
				ctx.Context().Error("at least on of 'namespace', 'routes', 'versions' should be present in request body.", http.StatusBadRequest)
				return restutils.RespondWithError(ctx, http.StatusBadRequest, "at least on of 'namespace', 'routes', 'versions' should be present in request body.")
			}

			ns := &msaddr.Namespace{Namespace: delRequest.Namespace}
			if !ns.IsCurrentNamespace() && delRequest.Version != "" {
				ctx.Context().Error("namespace should be current or version should be absent", http.StatusBadRequest)
				return restutils.RespondWithError(ctx, http.StatusBadRequest, "namespace should be current or version should be absent")
			}
		}
		return ctx.Next()
	}
}

func (c *RoutesController) DeleteRouteUnsecure(fiberCtx *fiber.Ctx) error {
	return c.HandleDeleteRoutesWithNodeGroup(fiberCtx)
}

// HandlePostRoutesWithNodeGroup godoc
// @Id PostRoutesWithNodeGroup
// @Summary Post Routes With Node Group
// @Description Post Routes With Node Group
// @Tags control-plane-v2
// @Produce json
// @Param nodeGroup path string true "nodeGroup"
// @Param request body []dto.RouteRegistrationRequest true "RouteRegistrationRequest"
// @Security ApiKeyAuth
// @Success 201
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v2/control-plane/routes/{nodeGroup} [post]
func (c *RoutesController) HandlePostRoutesWithNodeGroup(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	nodeGroup := utils.CopyString(fiberCtx.Params("nodeGroup"))
	if len(nodeGroup) == 0 {
		return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, "Path variable 'nodeGroup' must not be empty.")
	}
	logger.InfoC(ctx, "Request to create routes for %v", nodeGroup)

	var data []dto.RouteRegistrationRequest
	if err := json.Unmarshal(fiberCtx.Body(), &data); err != nil {
		logger.ErrorC(ctx, "Failed to unmarshal routes registration v2 request body: %v", err)
		return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, fmt.Sprintf("Could not unmarshal request body JSON: %v", err))
	}
	isDataValid, msg := c.validator.Validate(data, nodeGroup)
	if !isDataValid {
		logger.ErrorC(ctx, "%s", msg)
		return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, msg)
	}

	if dr.GetMode() == dr.Standby {
		return restutils.RespondWithJson(fiberCtx, http.StatusCreated, nil)
	}

	if err := c.service.RegisterRoutes(ctx, data, nodeGroup); err != nil {
		logger.ErrorC(ctx, "Failed to register routes via v2 api: %v", err)
		if errors.Is(err, entity.LegacyRouteDisallowed) || errors.Is(err, services.BadRouteRegistrationRequest) {
			return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, err.Error())
		} else if errr := errorcodes.GetCpErrCodeErrorOrNil(err); errr != nil {
			return restutils.RespondWithError(fiberCtx, errr.GetHttpCode(), errr.Error())
		}
		return restutils.RespondWithError(fiberCtx, http.StatusInternalServerError, "Routes registration has failed.")
	}
	return restutils.RespondWithJson(fiberCtx, http.StatusCreated, nil)
}

// HandleDeleteRoutesWithNodeGroup godoc
// @Id DeleteRoutesWithNodeGroupV2
// @Summary Delete Routes With Node Group V2
// @Description Delete Routes With Node Group V2
// @Tags control-plane-v2
// @Produce json
// @Param nodeGroup path string true "nodeGroup"
// @Param request body []dto.RouteDeleteRequest true "RouteDeleteRequest"
// @Security ApiKeyAuth
// @Success 200 {array} domain.Route
// @Failure 500 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/v2/control-plane/routes/{nodeGroup} [delete]
func (c *RoutesController) HandleDeleteRoutesWithNodeGroup(fiberCtx *fiber.Ctx) error {
	logger.Debugf("Starting delete routes with node group")
	ctx := fiberCtx.UserContext()
	nodeGroup := utils.CopyString(fiberCtx.Params("nodeGroup"))

	logger.InfoC(ctx, "Request to delete route nodeGroup=%s", nodeGroup)

	if nodeGroup == "" {
		return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, "Empty nodeGroup path param")
	}

	var reqBody []dto.RouteDeleteRequest
	err := json.Unmarshal(fiberCtx.Body(), &reqBody)
	if err != nil {
		logger.Errorf("Error occurred during unmarshalling request: %v", err)
		return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, fmt.Sprintf("Invalid request payload: %v", err))
	}
	var deletedRoutes []*domain.Route
	logger.Debugf("dr mode = %s", dr.GetMode())
	if dr.GetMode() == dr.Standby {
		return restutils.ResponseOk(fiberCtx, deletedRoutes)
	}
	for _, delReq := range reqBody {
		logger.DebugC(ctx, "processing request for deleting %+v", delReq)
		deleteCriteria := make([]string, len(delReq.Routes))
		for i, r := range delReq.Routes {
			deleteCriteria[i] = r.Prefix
		}
		deleted, err := c.service.DeleteRoutes(ctx, nodeGroup, delReq.Namespace, delReq.Version, deleteCriteria...)
		deletedRoutes = append(deletedRoutes, deleted...)
		if err != nil {
			logger.Errorf("Error occurred during deleting routes: %v", err)
			return restutils.RespondWithError(fiberCtx, http.StatusInternalServerError, err.Error())
		}
	}
	return restutils.ResponseOk(fiberCtx, deletedRoutes)
}

// HandleDeleteRouteWithUUID godoc
// @Id DeleteRouteWithUUID
// @Summary Delete Route With UUID
// @Description Delete Route With UUID
// @Tags control-plane-v2
// @Param uuid path string true "uuid"
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} dto.Route
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v2/control-plane/routes/uuid/{uuid} [delete]
func (c *RoutesController) HandleDeleteRouteWithUUID(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	routeUUID := utils.CopyString(fiberCtx.Params("uuid"))
	logger.InfoC(ctx, "Request to delete route by uuid=%s", routeUUID)
	if routeUUID == "" {
		logger.ErrorC(ctx, "Invalid request data. UUID must be set.")
		return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, "Empty UUID path param")
	}

	if dr.GetMode() == dr.Standby {
		return restutils.RespondWithJson(fiberCtx, http.StatusOK, nil)
	}
	deletedRoute, err := c.service.DeleteRouteByUUID(ctx, routeUUID)

	if _, ok := errs.Cause(err).(*services.RouteUUIDMatchError); err != nil && ok {
		logger.ErrorC(ctx, "Couldn't match route by UUID: %s cause: %v", routeUUID, err)
		return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, err.Error())
	}

	if err != nil {
		logger.ErrorC(ctx, "Can't delete route cause: %v", err)
		return restutils.RespondWithError(fiberCtx, http.StatusInternalServerError, err.Error())
	}

	return restutils.RespondWithJson(fiberCtx, http.StatusOK, deletedRoute)
}

// HandleDeleteRoutes godoc
// @Id DeleteRoutes
// @Summary DeleteRoutes
// @Description DeleteRoutes
// @Tags control-plane-v2
// @Produce json
// @Param request body []dto.RouteDeleteRequest true "RouteDeleteRequest"
// @Security ApiKeyAuth
// @Success 200 {array} domain.Route
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v2/control-plane/routes [delete]
func (c *RoutesController) HandleDeleteRoutes(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	logger.InfoC(ctx, "Request to delete routes")

	var deleteRequests []dto.RouteDeleteRequest
	err := json.Unmarshal(fiberCtx.Body(), &deleteRequests)
	if err != nil {
		return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, fmt.Sprintf("Invalid request payload: %s", err))
	}

	deletedRoutes := make([]*domain.Route, 0)
	if dr.GetMode() == dr.Standby {
		return restutils.ResponseOk(fiberCtx, deletedRoutes)
	}
	for _, deleteRequest := range deleteRequests {

		deleteCriteria := make([]string, len(deleteRequest.Routes))
		for i, r := range deleteRequest.Routes {
			deleteCriteria[i] = r.Prefix
		}
		nodeGroups, err := c.service.GetNodeGroups()
		if err != nil {
			logger.ErrorC(ctx, "Can't find node groups %+v", err)
			return restutils.RespondWithError(fiberCtx, http.StatusInternalServerError, fmt.Sprintf("Can't find node groups %v", err))
		}
		for _, nodeGroup := range nodeGroups {
			logger.InfoC(ctx, "Deleting route for node group %+v", nodeGroup)
			routes, err := c.service.DeleteRoutes(ctx, nodeGroup.Name, deleteRequest.Namespace, deleteRequest.Version, deleteCriteria...)
			if err != nil {
				return restutils.RespondWithError(fiberCtx, http.StatusInternalServerError, err.Error())
			}
			if routes != nil && len(routes) > 0 {
				deletedRoutes = append(deletedRoutes, routes...)
			}
		}
	}
	logger.InfoC(ctx, "Deleted routes: %s", deletedRoutes)
	return restutils.ResponseOk(fiberCtx, deletedRoutes)
}

// HandleDeleteEndpoints godoc
// @Id DeleteEndpoints
// @Summary Delete Endpoints V2
// @Description Delete Endpoints V2
// @Tags control-plane-v2
// @Produce json
// @Param request body []dto.EndpointDeleteRequest true "EndpointDeleteRequest"
// @Security ApiKeyAuth
// @Success 200 {array} dto.Endpoint
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v2/control-plane/endpoints [delete]
func (c *RoutesController) HandleDeleteEndpoints(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	logger.InfoC(ctx, "Request to delete endpoints")

	var deleteRequests []dto.EndpointDeleteRequest
	err := json.Unmarshal(fiberCtx.Body(), &deleteRequests)
	if err != nil {
		return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, fmt.Sprintf("Invalid request payload: %s", err))
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
		logger.InfoC(ctx, "Deleting endpoints:%v", endpointsToDelete)
		endpoints, err := c.service.DeleteEndpoints(nil, endpointsToDelete, deleteRequest.Version)
		if err != nil {
			return restutils.RespondWithError(fiberCtx, http.StatusInternalServerError, err.Error())
		}
		if endpoints != nil && len(endpoints) > 0 {
			deletedEndpoints = append(deletedEndpoints, endpoints...)
		}
	}
	logger.InfoC(ctx, "Deleted endpoints: %s", deletedEndpoints)
	return restutils.ResponseOk(fiberCtx, deletedEndpoints)
}

func (c *RoutesController) ValidateHeaderMatcher() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		var routeRegistrationReq []dto.RouteRegistrationRequest
		if err := json.Unmarshal(ctx.Body(), &routeRegistrationReq); err != nil && err != io.EOF {
			ctx.Context().Error(fmt.Sprintf("invalid request payload: %s", err), http.StatusBadRequest)
			return restutils.RespondWithError(ctx, http.StatusBadRequest, fmt.Sprintf("invalid request payload: %s", err))
		}

		for _, request := range routeRegistrationReq {
			if err := c.ValidateHeaderMatcherInSingleRouteRequest(ctx, &request); err != nil {
				logger.Debugf("ValidateHeaderMatcher request validation error: %v", err)
				return restutils.RespondWithError(ctx, http.StatusBadRequest, fmt.Sprintf("ValidateHeaderMatcher request validation error: %s", err))
			}
		}
		return ctx.Next()
	}
}

func (c *RoutesController) ValidateHeaderMatcherInSingleRouteRequest(ctx *fiber.Ctx, routeRegistrationReq *dto.RouteRegistrationRequest) error {
	for _, route := range routeRegistrationReq.Routes {
		for _, headerMatcher := range route.HeaderMatchers {
			logger.Debugf("Validating header matcher %v", headerMatcher)
			if headerMatcher.Name == "" {
				errMsg := "Header name have to be set"
				ctx.Context().Error(errMsg, http.StatusBadRequest)
				return errors.New(errMsg)
			}

			numberOfAssignedVariables := util.BoolToInt(headerMatcher.ExactMatch != "") + util.BoolToInt(headerMatcher.PrefixMatch != "") +
				util.BoolToInt(headerMatcher.PresentMatch.Valid) + util.BoolToInt(headerMatcher.RangeMatch.Start.Valid || headerMatcher.RangeMatch.End.Valid) +
				util.BoolToInt(headerMatcher.SafeRegexMatch != "") + util.BoolToInt(headerMatcher.SuffixMatch != "")
			logger.Debugf("Number of non empty fields: %d", numberOfAssignedVariables)
			if numberOfAssignedVariables > 1 {
				errMsg := "Only one header matcher fields have to be filled!"
				ctx.Context().Error(errMsg, http.StatusBadRequest)
				return errors.New(errMsg)
			}
		}
	}
	return nil
}
