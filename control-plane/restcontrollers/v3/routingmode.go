package v3

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/restutils"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/routingmode"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
)

type RoutingModeController struct {
	service *routingmode.Service
}

func NewRoutingModeController(service *routingmode.Service) *RoutingModeController {
	return &RoutingModeController{service: service}
}

// HandleGetRoutingModeDetails godoc
// @Id GetRoutingModeDetails
// @Summary Get Routing Mode Details
// @Description Get Routing Mode Details
// @Tags control-plane-v3
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {array} byte
// @Failure 500 {object} map[string]string
// @Router /api/v3/routing/details [get]
func (c *RoutingModeController) HandleGetRoutingModeDetails(fiberCtx fiber.Ctx) error {
	routingModeDetails := c.service.UpdateRouteModeDetails()
	log.Debugf("Updated route mode to %+v", routingModeDetails.RoutingMode)
	return fiberCtx.Status(http.StatusOK).JSON(routingModeDetails)
}

func (c *RoutingModeController) AllowedRoutesMiddlewareV1() fiber.Handler {
	return func(ctx fiber.Ctx) error {
		var routeEntityReq dto.RouteEntityRequest

		if err := json.Unmarshal(ctx.Body(), &routeEntityReq); err != nil && err != io.EOF {
			ctx.RequestCtx().Error(fmt.Sprintf("invalid request payload: %s", err), http.StatusBadRequest)
			return restutils.RespondWithError(ctx, http.StatusBadRequest, fmt.Sprintf("invalid request payload: %s", err))
		}

		hasForbiddenRoutingMode := false
		for _, re := range *routeEntityReq.Routes {
			if c.service.IsForbiddenRoutingMode("", re.Namespace) {
				hasForbiddenRoutingMode = true
				break
			}
		}

		if hasForbiddenRoutingMode {
			log.Warnf("Routes for microservice with URL %s are not registered", routeEntityReq.MicroserviceUrl)
			ctx.RequestCtx().Error(
				fmt.Sprintf(
					"Routes for microservice with URL %s are not registered, because routes with routing mode %v are registered on the Control-Plane",
					routeEntityReq.MicroserviceUrl,
					c.service.GetRoutingMode(),
				),
				http.StatusBadRequest)

			return restutils.RespondWithError(ctx, http.StatusBadRequest, fmt.Sprintf(
				"Routes for microservice with URL %s are not registered, because routes with routing mode %v are registered on the Control-Plane",
				routeEntityReq.MicroserviceUrl,
				c.service.GetRoutingMode(),
			))
		}
		return ctx.Next()
	}
}

func (c *RoutingModeController) ValidateRoutesApplicabilityToCurrentRoutingMode() fiber.Handler {
	return func(ctx fiber.Ctx) error {
		var routeRegistrationReq []dto.RouteRegistrationRequest
		if err := json.Unmarshal(ctx.Body(), &routeRegistrationReq); err != nil && err != io.EOF {
			ctx.RequestCtx().Error(fmt.Sprintf("invalid request payload: %s", err), http.StatusBadRequest)
			return restutils.RespondWithError(ctx, http.StatusBadRequest, fmt.Sprintf("invalid request payload: %s", err))
		}

		for _, request := range routeRegistrationReq {
			if err := c.validateSingleRouteRegistrationRequest(ctx, &request); err != nil {
				log.Debugf("ValidateRoutesApplicabilityToCurrentRoutingMode request validation error: %v", err)
				return restutils.RespondWithError(ctx, http.StatusBadRequest, fmt.Sprintf("ValidateRoutesApplicabilityToCurrentRoutingMode request validation error: %v", err))
			}
		}
		return ctx.Next()
	}
}

func (c *RoutingModeController) validateSingleRouteRegistrationRequest(ctx fiber.Ctx, routeRegistrationReq *dto.RouteRegistrationRequest) error {
	version := routeRegistrationReq.Version
	ns := routeRegistrationReq.Namespace
	namespace := msaddr.NewNamespace(ns)

	if version != "" && !strings.EqualFold(version, c.service.GetDefaultDeployVersion()) && !namespace.IsCurrentNamespace() {
		ctx.RequestCtx().Error("Request must not contain both 'version' and 'namespace' fields", http.StatusBadRequest)
		return errors.New("ValidateRoutesApplicabilityToCurrentRoutingMode: request contains both 'version' and 'namespace' fields")
	}

	if c.service.IsForbiddenRoutingMode(version, ns) {
		routingMode := c.service.GetRoutingMode()
		log.Warnf("Routes for cluster %s are not registered", routeRegistrationReq.Cluster)
		errMsg := fmt.Sprintf(
			"Routes for cluster %s are not registered, because routes with routing mode %v are registered on the Control-Plane",
			routeRegistrationReq.Cluster,
			routingMode,
		)
		ctx.RequestCtx().Error(errMsg, http.StatusBadRequest)
		return errors.New(fmt.Sprintf("ValidateRoutesApplicabilityToCurrentRoutingMode: %s", errMsg))
	}
	return nil
}
