package v3

import (
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/netcracker/qubership-core-control-plane/dr"
	"github.com/netcracker/qubership-core-control-plane/errorcodes"
	"github.com/netcracker/qubership-core-control-plane/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/restcontrollers/restutils"
	"github.com/netcracker/qubership-core-control-plane/services/ratelimit"
	"net/http"
)

type RateLimitController struct {
	rateLimitService *ratelimit.Service
}

func NewRateLimitController(rateLimitService *ratelimit.Service) *RateLimitController {
	return &RateLimitController{rateLimitService: rateLimitService}
}

// HandleGetRateLimit godoc
// @Id GetRateLimit
// @Summary Get Rate Limit
// @Description Get Rate Limit
// @Tags control-plane-v3
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {array} domain.RateLimit
// @Failure 500 {object} map[string]string
// @Router /api/v3/rate-limits [get]
func (c *RateLimitController) HandleGetRateLimit(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	log.DebugC(ctx, "Received GET rate limits request")
	rateLimits, err := c.rateLimitService.GetRateLimits(ctx)
	if err != nil {
		log.ErrorC(ctx, "Failed to get rate limit configurations: %v", err)
		return err
	}
	return restutils.ResponseOk(fiberCtx, rateLimits)
}

// HandlePostRateLimit godoc
// @Id PostRateLimit
// @Summary Post Rate Limit
// @Description Post Rate Limit
// @Tags control-plane-v3
// @Produce json
// @Param request body dto.RateLimit true "RateLimit"
// @Security ApiKeyAuth
// @Success 200
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v3/rate-limits [post]
func (c *RateLimitController) HandlePostRateLimit(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	log.DebugC(ctx, "Received POST rate limit request with body: \n\t%s", fiberCtx.Body())
	var rateLimitCreationRequest dto.RateLimit
	if err := json.Unmarshal(fiberCtx.Body(), &rateLimitCreationRequest); err != nil {
		return errorcodes.NewCpError(errorcodes.UnmarshalRequestError, fmt.Sprintf("Failed to unmarshal rate limit creation v3 request body: %v", err), err)
	}
	if err := c.rateLimitService.ValidateRequest(rateLimitCreationRequest); err != nil {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, fmt.Sprintf("Rate Limit request did not pass validation. Cause: %v", err), err)
	}

	if dr.GetMode() == dr.Standby {
		return restutils.RespondWithJson(fiberCtx, http.StatusOK, nil)
	}

	if err := c.rateLimitService.SaveRateLimit(ctx, &rateLimitCreationRequest); err != nil {
		log.ErrorC(ctx, "Failed to save rate limit configuration: %v", err)
		return err
	}
	return restutils.ResponseOk(fiberCtx, nil)
}

// HandleDeleteRateLimit godoc
// @Id DeleteRateLimit
// @Summary Delete Rate Limit
// @Description Delete Rate Limit
// @Tags control-plane-v3
// @Produce json
// @Param request body dto.RateLimit true "RateLimit"
// @Security ApiKeyAuth
// @Success 200
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v3/rate-limits [delete]
func (c *RateLimitController) HandleDeleteRateLimit(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	log.DebugC(ctx, "Received DELETE rate limit request body: \n\t%s", fiberCtx.Body())
	var rateLimitCreationRequest dto.RateLimit
	if err := json.Unmarshal(fiberCtx.Body(), &rateLimitCreationRequest); err != nil {
		return errorcodes.NewCpError(errorcodes.UnmarshalRequestError, fmt.Sprintf("Failed to unmarshal rate limit deletion v3 request body JSON: %v", err), err)
	}
	if err := c.rateLimitService.ValidateRequest(rateLimitCreationRequest); err != nil {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, fmt.Sprintf("Rate Limit request did not pass validation. Cause: %s", err), err)
	}

	if dr.GetMode() == dr.Standby {
		return restutils.RespondWithJson(fiberCtx, http.StatusOK, nil)
	}

	if err := c.rateLimitService.DeleteRateLimit(ctx, &rateLimitCreationRequest); err != nil {
		log.ErrorC(ctx, "Failed to delete rate limit configuration: %v", err)
		return err
	}
	return restutils.ResponseOk(fiberCtx, nil)
}
