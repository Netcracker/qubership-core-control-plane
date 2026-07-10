package v3

import (
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v3"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dr"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/errorcodes"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/restutils"
	cfgres "github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/configresources"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/gateway"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util"
	"net/http"
)

type GatewaySpecController struct {
	srv cfgres.ResourceService[dto.GatewayDeclaration]
}

func NewGatewaySpecController(srv cfgres.ResourceService[dto.GatewayDeclaration]) *GatewaySpecController {
	return &GatewaySpecController{srv: srv}
}

// HandlePostGatewaySpecs godoc
// @Id PostGatewaySpecs
// @Summary Post Gateway Specs
// @Description Post Gateway Specs
// @Tags control-plane-v3
// @Param request body dto.GatewayDeclaration true "GatewayDeclaration"
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/v3/gateways/specs [post]
func (c *GatewaySpecController) HandlePostGatewaySpecs(fiberCtx fiber.Ctx) error {
	return c.handleGatewaySpecApply(fiberCtx)
}

// HandleDeleteGatewaySpecs godoc
// @Id DeleteGatewaySpecs
// @Summary Delete Gateway Specs
// @Description Delete Gateway Specs
// @Tags control-plane-v3
// @Param request body dto.GatewayDeclaration true "GatewayDeclaration"
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/v3/gateways/specs [delete]
func (c *GatewaySpecController) HandleDeleteGatewaySpecs(fiberCtx fiber.Ctx) error {
	return c.handleGatewaySpecApply(fiberCtx)
}

func (c *GatewaySpecController) handleGatewaySpecApply(fiberCtx fiber.Ctx) error {
	ctx := fiberCtx.Context()
	method := fiberCtx.Method()
	log.DebugC(ctx, "Received %s gateway spec request", method)

	if dr.GetMode() == dr.Standby {
		return restutils.ResponseOk(fiberCtx, map[string]string{"message": "gateway declaration applied successfully"})
	}

	var payload dto.GatewayDeclaration
	if err := json.Unmarshal(fiberCtx.Body(), &payload); err != nil {
		return errorcodes.NewCpError(errorcodes.UnmarshalRequestError, fmt.Sprintf("Failed to unmarshall gateway spec modification request: %v", err), err)
	}

	if method == http.MethodDelete {
		payload.Exists = util.WrapValue(false)
	}

	if isValid, errMsg := c.srv.Validate(ctx, payload); !isValid {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, fmt.Sprintf("Gateway spec modification request validation failed: %s", errMsg), nil)
	}

	result, err := c.srv.Apply(ctx, payload)
	if err != nil {
		if gateway.IsInputError(err) {
			return errorcodes.NewCpError(errorcodes.ValidationRequestError, fmt.Sprintf("Gateway spec modification request validation failed: %s", err.Error()), err)
		}
		log.ErrorC(ctx, "Failed to apply gateway declaration:\n %v", err)
		return err
	}

	return restutils.ResponseOk(fiberCtx, result)
}

// HandleGetGatewaySpecs godoc
// @Id GetGatewaySpecs
// @Summary Get Gateway Specs
// @Description Get Gateway Specs
// @Tags control-plane-v3
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {array} []dto.GatewayDeclaration
// @Failure 500 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/v3/gateways/specs [get]
func (c *GatewaySpecController) HandleGetGatewaySpecs(fiberCtx fiber.Ctx) error {
	ctx := fiberCtx.Context()
	log.DebugC(ctx, "Received GET gateway specs request")

	responseData, err := c.srv.GetAll(ctx)
	if err != nil {
		log.ErrorC(ctx, "Failed to get gateway specs:\n %v", err)
		return err
	}
	return restutils.ResponseOk(fiberCtx, responseData)
}
