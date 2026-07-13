package v3

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dr"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/errorcodes"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/restutils"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/bluegreen"
)

type BlueGreenController struct {
	blueGreenService *bluegreen.Service
	dao              dao.Dao
}

func NewBlueGreenController(blueGreenService *bluegreen.Service, dao dao.Dao) *BlueGreenController {
	return &BlueGreenController{
		blueGreenService: blueGreenService,
		dao:              dao, // TODO Get rid of dao. Controller mustn't use dao directly.
	}
}

// HandleGetMicroserviceVersion godoc
// @Id GetMicroserviceVersion
// @Summary Get Microservice Version
// @Description Get Microservice Version
// @Tags control-plane-v3
// @Produce json
// @Param microservice path string true "microservice"
// @Security ApiKeyAuth
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v3/versions/microservices/{microservice} [get]
func (v3 *BlueGreenController) HandleGetMicroserviceVersion(fiberCtx fiber.Ctx) error {
	ctx := fiberCtx.Context()
	microservice := restutils.GetFiberParam(fiberCtx, "microservice")
	log.DebugC(ctx, "Microservice %s is requesting its b/g version", microservice)
	if strings.TrimSpace(microservice) == "" {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, "Microservice name must not be empty", nil)
	}
	microservice = sanitizeMicroserviceHost(microservice)
	version, err := v3.blueGreenService.GetMicroserviceVersion(ctx, microservice)
	if err != nil {
		log.ErrorC(fiberCtx.Context(), "Failed to get microservice %s version: %v", microservice, err)
		return err
	}
	return restutils.RespondWithJson(fiberCtx, http.StatusOK, map[string]string{"version": version})
}

// HandleGetDeploymentVersions godoc
// @Id GetDeploymentVersionsV3
// @Summary Get Deployment Versions
// @Description Get Deployment Versions
// @Tags control-plane-v3
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {array} domain.DeploymentVersion
// @Failure 500 {object} map[string]string
// @Router /api/v3/versions [get]
func (v3 *BlueGreenController) HandleGetDeploymentVersions(fiberCtx fiber.Ctx) error {
	deploymentVersions, err := v3.dao.FindAllDeploymentVersions()
	if err != nil {
		log.ErrorC(fiberCtx.Context(), "Failed to get deployment versions: %v", err)
		return err
	}
	return restutils.RespondWithJson(fiberCtx, http.StatusOK, deploymentVersions)
}

// HandleDeleteDeploymentVersionWithID godoc
// @Id DeleteDeploymentVersionWithIDV3
// @Summary Delete Deployment Version With ID
// @Description Delete Deployment Version With ID
// @Tags control-plane-v3
// @Produce json
// @Param version path string true "version"
// @Security ApiKeyAuth
// @Success 200 {object} domain.DeploymentVersion
// @Failure 500 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/v3/versions/{version} [delete]
func (v3 *BlueGreenController) HandleDeleteDeploymentVersionWithID(fiberCtx fiber.Ctx) error {
	ctx := fiberCtx.Context()
	version := restutils.GetFiberParam(fiberCtx, "version")
	if version == "" {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, "Path variable 'version' must not be empty.", nil)
	}
	deploymentVersion, err := v3.dao.FindDeploymentVersion(version)
	if err != nil {
		log.ErrorC(ctx, "Failed to find deployment version %s: %v", version, err)
		return err
	}
	if deploymentVersion == nil {
		return errorcodes.NewCpError(errorcodes.NotFoundEntityError, fmt.Sprintf("Deployment version %s not found", version), nil)
	}
	// During "Switch To Rolling Mode" deployer goes through all versions and deletes them. We do not have a separate API for "Switch To Rolling Mode"
	if deploymentVersion.Stage == domain.ActiveStage {
		return errorcodes.NewCpError(errorcodes.BlueGreenConflictError, fmt.Sprintf("Deleting Active deployment version %s is forbidden", version), nil)
	}
	if dr.GetMode() == dr.Standby {
		return restutils.ResponseOk(fiberCtx, deploymentVersion)
	}
	err = v3.blueGreenService.DeleteCandidate(ctx, deploymentVersion)
	if err != nil {
		log.ErrorC(ctx, "Failed to delete candidate version %s: %v", version, err)
		return err
	}
	return restutils.RespondWithJson(fiberCtx, http.StatusOK, deploymentVersion)
}

// HandlePostPromoteVersion godoc
// @Id PostPromoteVersionV3
// @Summary Post Promote Version
// @Description Post Promote Version
// @Tags control-plane-v3
// @Produce json
// @Param version path string true "version"
// @Param archiveSize query string false "archiveSize"
// @Security ApiKeyAuth
// @Success 202 {array} domain.DeploymentVersion
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v3/promote/{version} [post]
func (v3 *BlueGreenController) HandlePostPromoteVersion(fiberCtx fiber.Ctx) error {
	ctx := fiberCtx.Context()
	version := restutils.GetFiberParam(fiberCtx, "version")
	if version == "" {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, "Path variable 'version' must not be empty.", nil)
	}
	log.InfoC(ctx, "Request to Promote version '%s'", version)
	var archiveSize int
	var err error
	archiveSizeString := string(fiberCtx.RequestCtx().FormValue("archiveSize"))
	if archiveSizeString == "" {
		archiveSize = 1
	} else {
		archiveSize, err = strconv.Atoi(archiveSizeString)
		if err != nil {
			return errorcodes.NewCpError(errorcodes.ValidationRequestError, fmt.Sprintf("Can't convert archiveSize to integer %v", err), err)
		}
	}
	all, _ := v3.dao.FindAllDeploymentVersions()
	log.InfoC(ctx, "Current versions state: %v", all)
	deploymentVersion, err := v3.dao.FindDeploymentVersion(version)
	if err != nil {
		log.ErrorC(ctx, "Failed to get deployment version by version %s: %v", version, err)
		return err
	}
	if deploymentVersion == nil {
		return errorcodes.NewCpError(errorcodes.NotFoundEntityError, fmt.Sprintf("Deployment version %s not found", version), nil)
	}
	if deploymentVersion.Stage != domain.CandidateStage {
		return errorcodes.NewCpError(errorcodes.BlueGreenConflictError, fmt.Sprintf("Promote non candidate version %s deployment version %s is prohibited", deploymentVersion.Stage, version), nil)
	}
	var dVersions []*domain.DeploymentVersion

	if dr.GetMode() == dr.Standby {
		return restutils.RespondWithJson(fiberCtx, http.StatusAccepted, dVersions)
	}

	dVersions, err = v3.blueGreenService.Promote(ctx, deploymentVersion, archiveSize)
	if err != nil {
		log.ErrorC(ctx, "Failed to promote version %v with archive size %d: %v", deploymentVersion, archiveSize, err)
		return err
	}
	return restutils.RespondWithJson(fiberCtx, http.StatusAccepted, dVersions)
}

// HandlePostRollbackVersion godoc
// @Id PostRollbackVersion
// @Summary Post Rollback Version
// @Description Post Rollback Version
// @Tags control-plane-v3
// @Produce json
// @Security ApiKeyAuth
// @Success 202 {array} domain.DeploymentVersion
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v3/rollback [post]
func (v3 *BlueGreenController) HandlePostRollbackVersion(fiberCtx fiber.Ctx) error {
	ctx := fiberCtx.Context()
	log.InfoC(ctx, "Request to Rollback versions state")
	if dr.GetMode() == dr.Standby {
		return restutils.RespondWithJson(fiberCtx, http.StatusAccepted, []*domain.DeploymentVersion{})
	}
	all, _ := v3.dao.FindAllDeploymentVersions()
	log.InfoC(ctx, "Current versions state: %v", all)
	dVersions, err := v3.blueGreenService.Rollback(ctx)
	if err == nil {
		return restutils.RespondWithJson(fiberCtx, http.StatusAccepted, dVersions)
	}

	log.ErrorC(ctx, "Failed to execute rollback: %v", err)
	return err
}

func sanitizeMicroserviceHost(microservice string) string {
	if idx := strings.Index(microservice, "://"); idx != -1 {
		microservice = microservice[idx+3:]
	}
	if idx := strings.Index(microservice, ":"); idx != -1 {
		microservice = microservice[0:idx]
	}
	return microservice
}
