package v3

import (
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dr"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/errorcodes"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/restutils"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/httpFilter"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/httpFilter/extAuthz"
)

type HttpFilterController struct {
	httpFilterSrv *httpFilter.Service
}

func NewHttpFilterController(httpFilterSrv *httpFilter.Service) *HttpFilterController {
	return &HttpFilterController{httpFilterSrv: httpFilterSrv}
}

// HandlePostHttpFilters godoc
// @Id PostHttpFilters
// @Summary Post HTTP Filters
// @Description Post HTTP Filters
// @Tags control-plane-v3
// @Param request body dto.HttpFiltersConfigRequestV3 true "HttpFiltersConfigRequestV3"
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/v3/http-filters [post]
func (c *HttpFilterController) HandlePostHttpFilters(fiberCtx fiber.Ctx) error {
	ctx := fiberCtx.Context()
	log.DebugC(ctx, "Received POST http filters request")

	if dr.GetMode() == dr.Standby {
		return restutils.ResponseOk(fiberCtx, map[string]string{"message": "http filters applied successfully"})
	}

	var payload dto.HttpFiltersConfigRequestV3
	if err := json.Unmarshal(fiberCtx.Body(), &payload); err != nil {
		return errorcodes.NewCpError(errorcodes.UnmarshalRequestError, fmt.Sprintf("Failed to unmarshal HttpFilter apply request body: %v", err), err)
	}

	if isValid, errMsg := c.httpFilterSrv.ValidateApply(ctx, &payload); !isValid {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, fmt.Sprintf("HttpFilter apply request validation failed: %s", errMsg), nil)
	}

	if err := c.httpFilterSrv.Apply(ctx, &payload); err != nil {
		if err == extAuthz.ErrNameTaken {
			return errorcodes.NewCpError(errorcodes.ValidationRequestError, fmt.Sprintf("HttpFilter apply request validation failed: %v", err.Error()), err)
		}
		log.ErrorC(ctx, "Failed to post http filters:\n %v", err)
		return err
	}

	return restutils.ResponseOk(fiberCtx, map[string]string{"message": "http filters applied successfully"})
}

// HandleDeleteHttpFilters godoc
// @Id DeleteHttpFilters
// @Summary Delete HTTP Filters
// @Description Delete HTTP Filters
// @Tags control-plane-v3
// @Param request body dto.HttpFiltersDropConfigRequestV3 true "HttpFiltersDropConfigRequestV3"
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/v3/http-filters [delete]
func (c *HttpFilterController) HandleDeleteHttpFilters(fiberCtx fiber.Ctx) error {
	ctx := fiberCtx.Context()
	log.DebugC(ctx, "Received DELETE http filters request")

	if dr.GetMode() == dr.Standby {
		return restutils.ResponseOk(fiberCtx, map[string]string{"message": "http filters deleted successfully"})
	}

	var payload dto.HttpFiltersDropConfigRequestV3
	if err := json.Unmarshal(fiberCtx.Body(), &payload); err != nil {
		return errorcodes.NewCpError(errorcodes.UnmarshalRequestError, fmt.Sprintf("Failed to unmarshal HttpFilter delete request body: %v", err), err)
	}

	if isValid, errMsg := c.httpFilterSrv.ValidateDelete(ctx, &payload); !isValid {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, fmt.Sprintf("HttpFilter delete request validation failed: %s", errMsg), nil)
	}

	if err := c.httpFilterSrv.Delete(ctx, &payload); err != nil {
		if err == extAuthz.ErrNameTaken {
			return errorcodes.NewCpError(errorcodes.ValidationRequestError, fmt.Sprintf("HttpFilter apply request validation failed: %s", err.Error()), err)
		}
		log.ErrorC(ctx, "Failed to delete http filters:\n %v", err)
		return err
	}

	return restutils.ResponseOk(fiberCtx, map[string]string{"message": "http filters deleted successfully"})
}

// HandleGetHttpFilters godoc
// @Id GetHttpFilters
// @Summary Get HTTP Filters
// @Description Get HTTP Filters
// @Tags control-plane-v3
// @Param nodeGroup path string true "nodeGroup"
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} dto.HttpFiltersConfigRequestV3
// @Failure 500 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/v3/http-filters/{nodeGroup} [get]
func (c *HttpFilterController) HandleGetHttpFilters(fiberCtx fiber.Ctx) error {
	ctx := fiberCtx.Context()
	log.DebugC(ctx, "Received GET http filters request")

	nodeGroup := utils.CopyString(fiberCtx.Params("nodeGroup"))
	if nodeGroup == "" {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, fmt.Sprintf("HttpFilter GET request validation failed: path variable 'nodeGroup' is mandatory"), nil)
	}

	responseData, err := c.httpFilterSrv.GetGatewayFilters(ctx, nodeGroup)
	if err != nil {
		log.ErrorC(ctx, "Failed to load %s extAuthz filter while getting http filters:\n %v", nodeGroup, err)
		return err
	}
	return restutils.ResponseOk(fiberCtx, responseData)
}
