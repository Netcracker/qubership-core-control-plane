package v3

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dr"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/errorcodes"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/restutils"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/configresources"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util"
	"net/http"

	"github.com/gofiber/fiber/v3"
)

var (
	firstOrderConfigs = []string{"TlsDef"}
	thirdOrderConfigs = []string{"StatefulSession", "LoadBalance"}
)

type ApplyConfigController struct {
}

func NewApplyConfigurationController() *ApplyConfigController {
	return &ApplyConfigController{}
}

// HandlePostConfig godoc
// @Id HandlePostConfig
// @Summary Post Apply Config
// @Description Post Apply Config
// @Tags control-plane-v3
// @Produce json
// @Param request body []configresources.ConfigResource true "ConfigResource"
// @Security ApiKeyAuth
// @Success 200 {array} v3.ApplyResult
// @Failure 400 {object} errorcodes.CpErrCodeError
// @Failure 500 {object} errorcodes.CpErrCodeError
// @Router /api/v3/apply-config [post]
func (c *ApplyConfigController) HandlePostConfig(fiberCtx fiber.Ctx) error {
	ctx := fiberCtx.Context()
	log.DebugC(ctx, "Received request body: \n\t%s", string(fiberCtx.Body()))
	normalizedBody, err := util.NormalizeJsonOrYamlInput(string(fiberCtx.Body()))
	if err != nil {
		return errorcodes.NewCpError(errorcodes.UnmarshalRequestError, fmt.Sprintf("Converting request body to json caused error: %v", err), err)
	}
	var configResources []configresources.ConfigResource
	err = json.Unmarshal([]byte(normalizedBody), &configResources)
	if err != nil {
		return errorcodes.NewCpError(errorcodes.UnmarshalRequestError, fmt.Sprintf("Unmarshalling json string to array of ConfigResource caused error: %v", err), err)
	}
	results := make([]ApplyResult, len(configResources))
	if dr.GetMode() == dr.Standby {
		return restutils.RespondWithJson(fiberCtx, http.StatusOK, results)
	}
	orderedConfigs := orderConfigs(configResources)
	idx := 0
	isAllResourcesAppliedSuccessfully := applyConfigs(ctx, orderedConfigs[0], &idx, results)
	isAllResourcesAppliedSuccessfully = applyConfigs(ctx, orderedConfigs[1], &idx, results) && isAllResourcesAppliedSuccessfully
	isAllResourcesAppliedSuccessfully = applyConfigs(ctx, orderedConfigs[2], &idx, results) && isAllResourcesAppliedSuccessfully
	if isAllResourcesAppliedSuccessfully {
		return restutils.RespondWithJson(fiberCtx, http.StatusOK, results)
	} else {
		var causes []errorcodes.CpErrCodeError
		for _, result := range results {
			if result.Response.Error != nil {
				var errorCode *errorcodes.CpErrCodeError
				errCodeError := errorcodes.GetCpErrCodeErrorOrNil(result.Response.Error)
				if errCodeError != nil {
					errorCode = errorcodes.NewRestErrorWithMeta(errCodeError.ErrorCode, errCodeError.Error(), errCodeError.GetHttpCode(), result.Request)
				} else {
					errorCode = errorcodes.NewRestErrorWithMeta(errorcodes.ApplyConfigError, result.Response.Error.Error(), result.Response.Code, result.Request)
				}
				causes = append(causes, *errorCode)
			} else {
				errorCode := errorcodes.NewRestErrorWithMeta(errorcodes.OkErrorCode, "", result.Response.Code, result.Request)
				causes = append(causes, *errorCode)
			}
		}
		return errorcodes.NewMultiCauseError(errorcodes.MultiCauseApplyConfigError, causes)
	}
}

// HandleConfig godoc
// @Id HandleConfig
// @Summary  Apply Config
// @Description  Apply Config
// @Tags control-plane-v3
// @Produce json
// @Param request body []configresources.ConfigResource true "ConfigResource"
// @Security ApiKeyAuth
// @Success 200 {array} v3.ApplyResult
// @Failure 400 {object} errorcodes.CpErrCodeError
// @Failure 500 {object} errorcodes.CpErrCodeError
// @Router /api/v3/config [post]
func (c *ApplyConfigController) HandleConfig(fiberCtx fiber.Ctx) error {
	return c.HandlePostConfig(fiberCtx)
}

func applyConfigs(ctx context.Context, configs []configresources.ConfigResource, idx *int, results []ApplyResult) bool {
	isAllResourcesAppliedSuccessfully := true
	for _, config := range configs {
		handlingResult, err := configresources.HandleConfigResource(ctx, config)
		if err != nil {
			if err.ErrCodeError.GetDetail() == configresources.ErrIsOverridden.Error() {
				results[*idx] = NewApplyResult(config, http.StatusOK, nil, "Configuration wasn't applied because flag \"overridden\" is set to true")
				*idx++
				continue
			}
			isAllResourcesAppliedSuccessfully = false
			results[*idx] = NewApplyResult(config, err.GetHttpCode(), err, handlingResult)
		} else {
			results[*idx] = NewApplyResult(config, http.StatusOK, nil, handlingResult)
		}
		*idx++
	}
	return isAllResourcesAppliedSuccessfully
}

func orderConfigs(configResources []configresources.ConfigResource) map[int][]configresources.ConfigResource {
	orderedConfigs := make(map[int][]configresources.ConfigResource)
	orderedConfigs[0] = []configresources.ConfigResource{}
	orderedConfigs[1] = []configresources.ConfigResource{}
	orderedConfigs[2] = []configresources.ConfigResource{}
	for _, config := range configResources {
		key := config.Key()
		if key.Kind == "" {
			orderedConfigs[1] = append(orderedConfigs[1], config)
			continue
		}
		if util.SliceContains(firstOrderConfigs, key.Kind) {
			orderedConfigs[0] = append(orderedConfigs[0], config)
		} else if util.SliceContains(thirdOrderConfigs, key.Kind) {
			orderedConfigs[2] = append(orderedConfigs[2], config)
		} else {
			orderedConfigs[1] = append(orderedConfigs[1], config)
		}
	}
	return orderedConfigs
}

type ApplyResult struct {
	Request  configresources.ConfigResource `json:"request"`
	Response HandlingResponse               `json:"response"`
}

type HandlingResponse struct {
	Code  int         `json:"code"`
	Error error       `json:"error"`
	Data  interface{} `json:"data"`
}

func NewApplyResult(request configresources.ConfigResource, code int, error error, data interface{}) ApplyResult {
	return ApplyResult{
		Request: request,
		Response: HandlingResponse{
			Code:  code,
			Error: error,
			Data:  data,
		},
	}
}
