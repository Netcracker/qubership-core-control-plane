package v3

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gofiber/fiber/v3"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dr"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/errorcodes"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/restutils"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/cluster/clusterkey"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/loadbalance"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
	"net/http"
)

type LoadBalanceController struct {
	service   *loadbalance.LoadBalanceService
	validator LBRequestValidator
}

type LBRequestValidator interface {
	Validate(request dto.LoadBalanceSpec) (bool, string)
}

func NewLoadBalanceController(service *loadbalance.LoadBalanceService, validator LBRequestValidator) *LoadBalanceController {
	return &LoadBalanceController{service: service, validator: validator}
}

// HandlePostLoadBalance godoc
// @Id PostLoadBalanceV3
// @Summary Post Load Balance V3
// @Description Post Load Balance V3
// @Tags control-plane-v3
// @Produce json
// @Param request body dto.LoadBalanceSpec true "LoadBalanceSpec"
// @Security ApiKeyAuth
// @Success 200
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v3/load-balance [post]
func (c *LoadBalanceController) HandlePostLoadBalance(fiberCtx fiber.Ctx) error {
	context := fiberCtx.Context()
	data := new(dto.LoadBalanceSpec)
	if err := json.NewDecoder(bytes.NewReader(fiberCtx.Body())).Decode(data); err != nil {
		return errorcodes.NewCpError(errorcodes.UnmarshalRequestError, fmt.Sprintf("Can't parse request body: %s: ", err.Error()), err)
	}
	log.InfoC(context, "Apply load balance rules: %+v", data)

	isDataValid, msg := c.validator.Validate(*data)
	if !isDataValid {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, fmt.Sprintf("Error validation request body. Cause: %s", msg), nil)
	}

	microserviceAddress := msaddr.NewMicroserviceAddress(data.Endpoint, data.Namespace)
	clusterName := clusterkey.DefaultClusterKeyGenerator.GenerateKey(data.Cluster, microserviceAddress)

	hashPolicy, err := c.buildHashPoliciesFromDto(&data.Policies)
	if err != nil {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, fmt.Sprintf("Invalid hash policy in request: %s", err.Error()), err)
	}
	if dr.GetMode() == dr.Standby {
		return restutils.ResponseOk(fiberCtx, nil)
	}
	if err := c.service.ApplyLoadBalanceConfig(context, clusterName, data.Version, hashPolicy); err != nil {
		log.ErrorC(context, "Failed to apply load balance: %v", err)
		return err
	} else {
		return restutils.RespondWithJson(fiberCtx, http.StatusOK, nil)
	}
}

func (c *LoadBalanceController) hashPolicyIsValid(hashPolicy *domain.HashPolicy) bool {
	return (hashPolicy.HeaderName != "" && (hashPolicy.CookieName == "" && hashPolicy.QueryParamName == "" && !hashPolicy.QueryParamSourceIP.Valid)) ||
		(hashPolicy.CookieName != "" && (hashPolicy.HeaderName == "" && hashPolicy.QueryParamName == "" && !hashPolicy.QueryParamSourceIP.Valid)) ||
		(hashPolicy.QueryParamName != "" && (hashPolicy.HeaderName == "" && hashPolicy.CookieName == "" && !hashPolicy.QueryParamSourceIP.Valid)) ||
		(hashPolicy.QueryParamSourceIP.Valid && (hashPolicy.HeaderName == "" && hashPolicy.CookieName == "" && hashPolicy.QueryParamName == ""))
}

func (c *LoadBalanceController) buildHashPoliciesFromDto(policies *[]dto.HashPolicy) ([]*domain.HashPolicy, error) {
	result := make([]*domain.HashPolicy, 0, len(*policies))
	for _, policy := range *policies {
		hashPolicy := c.buildDomainHashPolicyFromDto(&policy)
		if c.hashPolicyIsValid(hashPolicy) {
			result = append(result, hashPolicy)
		} else {
			return nil, errors.New("v3: route action hash policy must have precisely one of header, cookie, queryParameter or connectionProperties settings")
		}
	}
	return result, nil
}

func (c *LoadBalanceController) buildDomainHashPolicyFromDto(policy *dto.HashPolicy) *domain.HashPolicy {
	result := new(domain.HashPolicy)
	if policy.Header != nil {
		result.HeaderName = policy.Header.HeaderName
	}
	if policy.Cookie != nil {
		result.CookieName = policy.Cookie.Name
		if policy.Cookie.Ttl != nil {
			result.CookieTTL = domain.NewNullInt(*policy.Cookie.Ttl)
		}
		result.CookiePath = policy.Cookie.Path.String
	}
	if policy.ConnectionProperties != nil {
		result.QueryParamSourceIP = policy.ConnectionProperties.SourceIp
	}
	if policy.QueryParameter != nil {
		result.QueryParamName = policy.QueryParameter.Name
	}
	if policy.Terminal.Valid {
		result.Terminal = policy.Terminal
	}
	return result
}
