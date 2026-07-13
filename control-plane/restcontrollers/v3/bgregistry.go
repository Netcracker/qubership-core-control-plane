package v3

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dr"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/errorcodes"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/restutils"
	cfgres "github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/configresources"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
)

type VersionsRegistry[R dto.ServicesVersionPayload] interface {
	cfgres.ResourceApplier[dto.ServicesVersionPayload]

	GetMicroserviceCurrentVersion(ctx context.Context, repo dao.Repository, serviceName string, namespace msaddr.Namespace, initialVersion string) ([]dto.VersionInRegistry, error)
	GetVersionsForMicroservice(ctx context.Context, repo dao.Repository, serviceName string, namespace msaddr.Namespace) ([]dto.VersionInRegistry, error)
	GetMicroservicesForVersion(ctx context.Context, repo dao.Repository, version *domain.DeploymentVersion) ([]dto.VersionInRegistry, error)
	GetAll(ctx context.Context, repo dao.Repository) ([]dto.VersionInRegistry, error)
}

type BGRegistryController struct {
	registry VersionsRegistry[dto.ServicesVersionPayload]
	dao      dao.Dao
}

func NewBGRegistryController(registry VersionsRegistry[dto.ServicesVersionPayload], dao dao.Dao) *BGRegistryController {
	return &BGRegistryController{registry: registry, dao: dao}
}

// HandlePostMicroserviceVersions godoc
// @Id PostMicroserviceVersions
// @Summary Register Microservice Versions in BG versions registry.
// @Description Register Microservice Versions in BG versions registry.
// @Tags control-plane-v3
// @Produce json
// @Param request body dto.ServicesVersionPayload true "ServicesVersionPayload"
// @Security ApiKeyAuth
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/v3/versions/registry [post]
func (v3 *BGRegistryController) HandlePostMicroserviceVersions(fiberCtx fiber.Ctx) error {
	return v3.handleRegistrationRequest(fiberCtx, http.MethodPost)
}

// HandleDeleteMicroserviceVersions godoc
// @Id DeleteMicroserviceVersions
// @Summary Delete Microservice Versions from BG versions registry.
// @Description Delete Microservice Versions from BG versions registry.
// @Tags control-plane-v3
// @Produce json
// @Param request body dto.ServicesVersionPayload true "ServicesVersionPayload"
// @Security ApiKeyAuth
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/v3/versions/registry/services [delete]
func (v3 *BGRegistryController) HandleDeleteMicroserviceVersions(fiberCtx fiber.Ctx) error {
	return v3.handleRegistrationRequest(fiberCtx, http.MethodDelete)
}

func (v3 *BGRegistryController) handleRegistrationRequest(fiberCtx fiber.Ctx, httpMethod string) error {
	payload, err := v3.getRequestPayload(fiberCtx, httpMethod)
	if err != nil || payload == nil { // both err and payload can be nil in case we already have responded on HTTP request
		return err
	}
	result, err := v3.registry.Apply(fiberCtx.Context(), *payload)
	if err != nil {
		log.ErrorC(fiberCtx.Context(), "Failed to %s microservice versions: %v", httpMethod, err)
		return err
	}
	return restutils.ResponseOk(fiberCtx, result)
}

func (v3 *BGRegistryController) getRequestPayload(fiberCtx fiber.Ctx, method string) (*dto.ServicesVersionPayload, error) {
	ctx := fiberCtx.Context()
	var payload dto.ServicesVersionPayload
	if err := json.Unmarshal(fiberCtx.Body(), &payload); err != nil {
		return nil, errorcodes.NewCpError(errorcodes.UnmarshalRequestError, fmt.Sprintf("Failed to unmarshal ServicesVersionPayload from %s request body JSON: %v", method, err), err)
	}
	log.InfoC(ctx, "v3.bg_registry Got request to %s microservice versions:\n %+v", method, payload)

	if isPayloadValid, validationErrMsg := v3.registry.Validate(ctx, payload); !isPayloadValid {
		return nil, errorcodes.NewCpError(errorcodes.ValidationRequestError, fmt.Sprintf("Invalid request payload: %s", validationErrMsg), nil)
	}

	if method == http.MethodDelete {
		if payload.Exists != nil && *payload.Exists {
			return nil, errorcodes.NewCpError(errorcodes.ValidationRequestError, "Invalid request payload: field \"exists\" cannot be \"true\" for DELETE request", nil)
		}
		payload.Exists = util.WrapValue(false)
	}

	if dr.GetMode() == dr.Standby {
		return nil, restutils.RespondWithJson(fiberCtx, http.StatusCreated, nil)
	}
	return &payload, nil
}

// HandleGetMicroserviceVersions godoc
// @Id GetMicroserviceVersions
// @Summary Get Microservice Versions from BG versions registry
// @Description Get Microservice Versions from BG versions registry. Not every combination of the request params allowed,
// here are the allowed request params combinations:
// 1) none 2) version 3) serviceName + namespace 4) serviceName + namespace + initialVersion
// @Tags control-plane-v3
// @Produce json
// @Param version query string false "version"
// @Param initialVersion query string false "initialVersion"
// @Param serviceName query string false "serviceName"
// @Param namespace query string false "namespace"
// @Security ApiKeyAuth
// @Success 200 {array} dto.VersionInRegistry
// @Failure 500 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/v3/versions/registry [get]
func (v3 *BGRegistryController) HandleGetMicroserviceVersions(fiberCtx fiber.Ctx) error {
	ctx := fiberCtx.Context()
	version := string(fiberCtx.RequestCtx().FormValue("version"))
	initialVersion := string(fiberCtx.RequestCtx().FormValue("initialVersion"))
	serviceName := string(fiberCtx.RequestCtx().FormValue("serviceName"))
	namespace := string(fiberCtx.RequestCtx().FormValue("namespace"))
	log.DebugC(ctx, "HandleGetMicroserviceVersion from registry with params version=%s, initialVersion=%s, serviceName=%s, namespace=%s", version, initialVersion, serviceName, namespace)

	if len(serviceName) == 0 {
		if len(initialVersion) != 0 {
			return errorcodes.NewCpError(errorcodes.ValidationRequestError, "initialVersion param cannot be specified without serviceName", nil)
		}

		if len(version) == 0 { // no request params - getting all
			return v3.getAllMicroserviceVersions(fiberCtx)
		}
		// request to search microservices by deployment version
		return v3.getMicroservicesByVersion(fiberCtx, version)
	}

	// filtering microservice version by service name (and namespace)

	if len(version) != 0 {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, "Version param cannot be specified with serviceName (use initialVersion if you need to get actual BG version for concrete microservice)", nil)
	}

	// request to get all versions of the microservice
	if len(initialVersion) == 0 {
		return v3.getVersionsForMicroservice(fiberCtx, serviceName, namespace)
	}

	// getting current microservice version based on its initial version
	return v3.getCurrentMicroserviceVersion(fiberCtx, serviceName, namespace, initialVersion)
}

func (v3 *BGRegistryController) getAllMicroserviceVersions(fiberCtx fiber.Ctx) error {
	if result, err := v3.registry.GetAll(fiberCtx.Context(), v3.dao); err != nil {
		log.ErrorC(fiberCtx.Context(), "Failed to get all microservice versions: %v", err)
		return err
	} else {
		return restutils.ResponseOk(fiberCtx, result)
	}
}

func (v3 *BGRegistryController) getMicroservicesByVersion(fiberCtx fiber.Ctx, version string) error {
	if result, err := v3.registry.GetMicroservicesForVersion(fiberCtx.Context(), v3.dao, &domain.DeploymentVersion{Version: version}); err != nil {
		log.ErrorC(fiberCtx.Context(), "Failed to get microservice versions %s : %v", version, err)
		return err
	} else {
		return restutils.ResponseOk(fiberCtx, result)
	}
}

func (v3 *BGRegistryController) getVersionsForMicroservice(fiberCtx fiber.Ctx, serviceName, namespace string) error {
	if result, err := v3.registry.GetVersionsForMicroservice(fiberCtx.Context(), v3.dao, serviceName, msaddr.Namespace{Namespace: namespace}); err != nil {
		log.ErrorC(fiberCtx.Context(), "Failed to get microservice %s version in namespace %s: %v", serviceName, namespace, err)
		return err
	} else {
		return restutils.ResponseOk(fiberCtx, result)
	}
}

func (v3 *BGRegistryController) getCurrentMicroserviceVersion(fiberCtx fiber.Ctx, serviceName, namespace, initialVersion string) error {
	if result, err := v3.registry.GetMicroserviceCurrentVersion(fiberCtx.Context(), v3.dao, serviceName, msaddr.Namespace{Namespace: namespace}, initialVersion); err != nil {
		log.ErrorC(fiberCtx.Context(), "Failed to get microservice %s version in namespace %s: %v", serviceName, namespace, err)
		return err
	} else {
		return restutils.ResponseOk(fiberCtx, result)
	}
}
