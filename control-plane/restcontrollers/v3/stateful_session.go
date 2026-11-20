package v3

import (
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v3"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dr"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/errorcodes"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/restutils"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/statefulsession"
	"net/http"
)

type StatefulSessionController struct {
	service   statefulsession.Service
	validator RequestValidator
}

func NewStatefulSessionController(service statefulsession.Service, validator RequestValidator) *StatefulSessionController {
	return &StatefulSessionController{service: service, validator: validator}
}

// HandlePostStatefulSession godoc
// @Id PostStatefulSession
// @Summary Post Stateful Session
// @Description Post Stateful Session
// @Tags control-plane-v3
// @Produce json
// @Param request body dto.StatefulSession true "StatefulSession"
// @Security ApiKeyAuth
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/v3/load-balance/stateful-session [post]
func (c *StatefulSessionController) HandlePostStatefulSession(fiberCtx fiber.Ctx) error {
	request, err := c.readRequestBody(fiberCtx)
	if err != nil {
		return err
	}
	return c.applyStatefulSession(fiberCtx, request)
}

// HandlePutStatefulSession godoc
// @Id PutStatefulSession
// @Summary Put Stateful Session
// @Description Put Stateful Session
// @Tags control-plane-v3
// @Produce json
// @Param request body dto.StatefulSession true "StatefulSession"
// @Security ApiKeyAuth
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/v3/load-balance/stateful-session [put]
func (c *StatefulSessionController) HandlePutStatefulSession(fiberCtx fiber.Ctx) error {
	return c.HandlePostStatefulSession(fiberCtx)
}

// HandleDeleteStatefulSession godoc
// @Id DeleteStatefulSession
// @Summary Delete Stateful Session
// @Description Delete Stateful Session
// @Tags control-plane-v3
// @Produce json
// @Param request body dto.StatefulSession true "StatefulSession"
// @Security ApiKeyAuth
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/v3/load-balance/stateful-session [delete]
func (c *StatefulSessionController) HandleDeleteStatefulSession(fiberCtx fiber.Ctx) error {
	request, err := c.readRequestBody(fiberCtx)
	if err != nil {
		return err
	}
	// delete == apply config with empty stateful session spec
	request.Cookie = nil
	request.Enabled = nil
	return c.applyStatefulSession(fiberCtx, request)
}

// HandleGetStatefulSessions godoc
// @Id GetStatefulSessions
// @Summary Get Stateful Sessions
// @Description Get Stateful Sessions
// @Tags control-plane-v3
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {array} dto.StatefulSession
// @Failure 500 {object} map[string]string
// @Router /api/v3/load-balance/stateful-session [get]
func (c *StatefulSessionController) HandleGetStatefulSessions(fiberCtx fiber.Ctx) error {
	ctx := fiberCtx.Context()
	configs, err := c.service.FindAll(ctx)
	if err != nil {
		log.ErrorC(ctx, "Failed to to get stateful session: %v", err)
		return err
	}
	return restutils.ResponseOk(fiberCtx, configs)
}

func (c *StatefulSessionController) readRequestBody(fiberCtx fiber.Ctx) (*dto.StatefulSession, error) {
	var request dto.StatefulSession
	if err := json.Unmarshal(fiberCtx.Body(), &request); err != nil {
		return nil, errorcodes.NewCpError(errorcodes.UnmarshalRequestError, fmt.Sprintf("Failed to unmarshal StatefulSession apply request body: %v", err), err)
	}

	if valid, msg := c.validator.ValidateStatefulSession(request); !valid {
		return nil, errorcodes.NewCpError(errorcodes.ValidationRequestError, fmt.Sprintf("Request for StatefulSession apply is invalid. Cause: %s", msg), nil)
	}
	return &request, nil
}

func (c *StatefulSessionController) applyStatefulSession(fiberCtx fiber.Ctx, request *dto.StatefulSession) error {
	ctx := fiberCtx.Context()

	if dr.GetMode() == dr.Standby {
		return restutils.RespondWithJson(fiberCtx, http.StatusOK, nil)
	}

	if err := c.service.ApplyStatefulSession(ctx, request); err != nil {
		log.ErrorC(ctx, "Failed to to apply stateful session: %v", err)
		return err
	}
	return restutils.RespondWithJson(fiberCtx, http.StatusOK, map[string]string{"message": "StatefulSession configuration applied successfully"})
}
