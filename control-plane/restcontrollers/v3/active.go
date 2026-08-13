package v3

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dr"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/errorcodes"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/restutils"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/active"
)

type ActiveDCsController struct {
	service active.ActiveDCsService
}

func NewActiveDCsController(service active.ActiveDCsService) *ActiveDCsController {
	return &ActiveDCsController{service: service}
}

// HandleActiveActiveConfigPost godoc
// @Id ActiveActiveConfigPost
// @Summary Active Config Post
// @Description Active Config Post
// @Tags control-plane-v3
// @Produce json
// @Param request body dto.ActiveDCsV3 true "ActiveDCsV3"
// @Security ApiKeyAuth
// @Success 200
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v3/active-active [post]
func (c *ActiveDCsController) HandleActiveActiveConfigPost(fiberCtx fiber.Ctx) error {
	context := fiberCtx.Context()
	activeDCsConfig := &dto.ActiveDCsV3{}
	if err := json.NewDecoder(bytes.NewReader(fiberCtx.Body())).Decode(activeDCsConfig); err != nil {
		return errorcodes.NewCpError(errorcodes.UnmarshalRequestError, fmt.Sprintf("Failed to unmarshal ActiveActive config creation request body: %v", err), err)
	}
	if err := c.validateRequest(activeDCsConfig); err != nil {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, fmt.Sprintf("Request for ActiveActive config creation is invalid. Cause: %v", err), err)
	}
	log.InfoC(context, "Applying config: %s", activeDCsConfig.String())
	if dr.GetMode() == dr.Standby {
		return restutils.RespondWithJson(fiberCtx, http.StatusOK, nil)
	}
	err := c.service.ApplyActiveDCsConfig(context, activeDCsConfig)
	if err != nil {
		log.ErrorC(context, "Failed to apply active-active configuration: %v", err)
		return err
	} else {
		return restutils.RespondWithJson(fiberCtx, http.StatusOK, nil)
	}
}

// HandleActiveActiveConfigDelete godoc
// @Id ActiveActiveConfigDelete
// @Summary Active Config Delete
// @Description Active Config Delete
// @Tags control-plane-v3
// @Produce json
// @Security ApiKeyAuth
// @Success 200
// @Failure 500 {object} map[string]string
// @Router /api/v3/active-active [delete]
func (c *ActiveDCsController) HandleActiveActiveConfigDelete(fiberCtx fiber.Ctx) error {
	context := fiberCtx.Context()
	if dr.GetMode() == dr.Standby {
		return restutils.RespondWithJson(fiberCtx, http.StatusOK, nil)
	}
	err := c.service.DeleteActiveDCsConfig(context)
	if err != nil {
		log.ErrorC(context, "Failed to delete active-active configuration: %v", err)
		return err
	} else {
		return restutils.RespondWithJson(fiberCtx, http.StatusOK, nil)
	}
}

func (c *ActiveDCsController) validateRequest(activeDCs *dto.ActiveDCsV3) error {
	if strings.TrimSpace(activeDCs.Protocol) == "" {
		return fmt.Errorf("'protocol' cannot be empty")
	} else if activeDCs.Protocol != active.ProtocolHttp &&
		activeDCs.Protocol != active.ProtocolHttps {
		return fmt.Errorf("'protocol' can be either %s or %s", active.ProtocolHttp, active.ProtocolHttps)
	}
	if len(activeDCs.PublicGwHosts) == 0 || len(activeDCs.PrivateGwHosts) == 0 {
		return fmt.Errorf("'publicGwHosts' or 'privateGwHosts' cannot be empty")
	}
	if len(activeDCs.PublicGwHosts) != len(activeDCs.PrivateGwHosts) {
		return fmt.Errorf("'publicGwHosts' and 'privateGwHosts' must contain the same amount of elements")
	}
	for _, gw := range [][]string{activeDCs.PublicGwHosts, activeDCs.PrivateGwHosts} {
		for _, host := range gw {
			host := strings.TrimSpace(host)
			if host == "" {
				return fmt.Errorf("'publicGwHosts' and 'privateGwHosts' cannot contain empty elements")
			}
			if strings.Contains(host, ":") {
				return fmt.Errorf("host elements from 'publicGwHosts' and 'privateGwHosts' cannot contain ':'")
			}
		}
	}
	return nil
}
