package v2

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dr"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/restutils"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/cluster/clusterkey"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/configresources"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/loadbalance"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
	"net/http"
	"reflect"
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
// @Id PostLoadBalanceV2
// @Summary Post Load Balance V2
// @Description Post Load Balance V2
// @Tags control-plane-v2
// @Produce json
// @Param request body dto.LoadBalanceSpec true "loadBalanceSpec"
// @Security ApiKeyAuth
// @Success 200
// @Failure 400 {object} map[string]string
// @Router /api/v2/control-plane/load-balance [post]
func (c *LoadBalanceController) HandlePostLoadBalance(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	data := new(dto.LoadBalanceSpec)
	if err := json.NewDecoder(bytes.NewReader(fiberCtx.Body())).Decode(data); err != nil {
		logger.ErrorC(ctx, "Error during parsing data : %+v", err.Error())
		return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, fmt.Sprintf("Invalid request payload: %s", err.Error()))
	}
	logger.InfoC(ctx, "Apply load balance rules: %+v", data)

	isDataValid, msg := c.validator.Validate(*data)
	if !isDataValid {
		logger.Error("%s", msg)
		return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, fmt.Sprintf("Error processing request body: %s", msg))
	}

	microserviceAddress := msaddr.NewMicroserviceAddress(data.Endpoint, data.Namespace)
	clusterName := clusterkey.DefaultClusterKeyGenerator.GenerateKey(data.Cluster, microserviceAddress)

	hashPolicy, err := c.buildHashPoliciesFromDto(&data.Policies)
	if err != nil {
		return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, fmt.Sprintf("Invalid hash policy in request: %s", err.Error()))
	}
	if dr.GetMode() == dr.Standby {
		return restutils.RespondWithJson(fiberCtx, http.StatusOK, nil)
	}
	if err := c.service.ApplyLoadBalanceConfig(ctx, clusterName, data.Version, hashPolicy); err != nil {
		return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, fmt.Sprintf("Error processing request: %s", err.Error()))
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
			return nil, errors.New("v2: route action hash policy must have precisely one of header, cookie, queryParameter or connectionProperties settings")
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

func (c *LoadBalanceController) GetLoadBalanceResources() []configresources.Resource {
	resources := []configresources.Resource{
		configresources.ResourceProto{
			GetKeyFunc: func() configresources.ResourceKey {
				return configresources.ResourceKey{
					APIVersion: "",
					Kind:       "LoadBalance",
				}
			},
			GetDefFunc: c.getDefFunc,
		},
		configresources.ResourceProto{
			GetKeyFunc: func() configresources.ResourceKey {
				return configresources.ResourceKey{
					APIVersion: "nc.core.mesh/v3",
					Kind:       "LoadBalance",
				}
			},
			GetDefFunc: c.getDefFunc,
		},
	}

	return resources
}

func (c *LoadBalanceController) getDefFunc() configresources.ResourceDef {
	return configresources.ResourceDef{
		Type: reflect.TypeOf(dto.LoadBalanceSpec{}),
		Validate: func(ctx context.Context, md configresources.Metadata, entity interface{}) (bool, string) {
			lbSpec := entity.(*dto.LoadBalanceSpec)
			if lbSpec.Policies == nil {
				lbSpec.Policies = []dto.HashPolicy{}
			}
			isDataValid, msg := c.validator.Validate(*lbSpec)
			if !isDataValid {
				return false, msg
			}
			return true, ""
		},
		Handler: func(ctx context.Context, md configresources.Metadata, entity interface{}) (interface{}, error) {
			lbSpec := entity.(*dto.LoadBalanceSpec)
			logger.InfoC(ctx, "Apply load balance rules: %+v", lbSpec)
			microserviceAddress := msaddr.NewMicroserviceAddress(lbSpec.Endpoint, lbSpec.Namespace)
			clusterName := clusterkey.DefaultClusterKeyGenerator.GenerateKey(lbSpec.Cluster, microserviceAddress)

			hashPolicy, err := c.buildHashPoliciesFromDto(&lbSpec.Policies)
			if err != nil {
				return nil, err
			}
			if err := c.service.ApplyLoadBalanceConfig(ctx, clusterName, lbSpec.Version, hashPolicy); err != nil {
				return nil, err
			}
			return "LoadBalance configuration has applied", nil
		},
		IsOverriddenByCR: func(ctx context.Context, metadata configresources.Metadata, entity interface{}) bool {
			return entity.(*dto.LoadBalanceSpec).Overridden
		},
	}
}
