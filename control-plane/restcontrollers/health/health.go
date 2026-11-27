package health

import (
	"github.com/gofiber/fiber/v3"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/restutils"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/health"
	"net/http"
)

type Controller struct {
	service *health.HealthService
}

func NewController(service *health.HealthService) *Controller {
	return &Controller{
		service: service,
	}
}

func (c *Controller) HandleReadinessProbe(ctx fiber.Ctx) error {
	readiness := c.service.CheckReadiness()
	switch readiness.Status {
	case health.Ready:
		return restutils.RespondWithJson(ctx, http.StatusOK, readiness)
	case health.NotReady:
		return restutils.RespondWithJson(ctx, http.StatusServiceUnavailable, readiness)
	}
	return nil
}

func (c *Controller) HandleLivenessProbe(ctx fiber.Ctx) error {
	liveness := c.service.CheckLiveness()
	switch liveness.Status {
	case health.Up:
		return restutils.RespondWithJson(ctx, http.StatusOK, liveness)
	case health.Problem:
		return restutils.RespondWithJson(ctx, http.StatusServiceUnavailable, liveness)
	}
	return nil
}
