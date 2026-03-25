package v1

import (
	"fmt"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/restutils"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/tls"
	"net/http"

	"github.com/gofiber/fiber/v2"
)

type TlsController struct {
	service *tls.Service
}

func NewTlsController(service *tls.Service) *TlsController {
	return &TlsController{service: service}
}

// HandleCetrificateDetails godoc
// @Id CetrificateDetails
// @Summary Capture certificates detals
// @Description Capture certificates detals
// @Tags control-plane-v3
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} ui.CertificateDetailsResponse
// @Failure 500 {object} map[string]string
// @Router /api/v3/tls/details [get]
func (c *TlsController) HandleCetrificateDetails(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	if response, err := c.service.ValidateCertificates(); err == nil {
		return restutils.ResponseOk(fiberCtx, response)
	} else {
		msg := fmt.Sprintf("Can't validate certificates: %v.", err)
		logger.ErrorC(ctx, "%s", msg)
		return restutils.RespondWithError(fiberCtx, http.StatusInternalServerError, msg)
	}
}
