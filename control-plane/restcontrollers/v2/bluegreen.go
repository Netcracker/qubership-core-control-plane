package v2

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dr"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/restutils"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/bluegreen"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"net/http"
	"strconv"
)

var logger logging.Logger

func init() {
	logger = logging.GetLogger("rest-controllers/v2")
}

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

// HandleGetDeploymentVersions godoc
// @Id GetDeploymentVersionsV2
// @Summary Get Deployment Versions
// @Description Get Deployment Versions
// @Tags control-plane-v2
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {array} domain.DeploymentVersion
// @Failure 500 {object} map[string]string
// @Router /api/v2/control-plane/versions [get]
func (v2 *BlueGreenController) HandleGetDeploymentVersions(fiberCtx *fiber.Ctx) error {
	deploymentVersions, err := v2.dao.FindAllDeploymentVersions()
	if err != nil {
		return restutils.RespondWithError(fiberCtx, http.StatusInternalServerError, err.Error())
	}
	return restutils.RespondWithJson(fiberCtx, http.StatusOK, deploymentVersions)
}

// HandleDeleteDeploymentVersionWithID godoc
// @Id DeleteDeploymentVersionWithIDV2
// @Summary Delete Deployment Version With ID V2
// @Description Delete Deployment Version With ID V2
// @Tags control-plane-v2
// @Produce json
// @Param version path string true "version"
// @Security ApiKeyAuth
// @Success 200 {object} domain.DeploymentVersion
// @Failure 500 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v2/control-plane/versions/{version} [delete]
func (v2 *BlueGreenController) HandleDeleteDeploymentVersionWithID(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	version := utils.CopyString(fiberCtx.Params("version"))
	if version == "" {
		logger.ErrorC(ctx, "Empty version path param")
		return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, "Empty version path param")
	}
	deploymentVersion, err := v2.dao.FindDeploymentVersion(version)
	if err != nil {
		logger.ErrorC(ctx, "Error while trying to find deployment version %s \n %v", version, err)
		return restutils.RespondWithError(fiberCtx, http.StatusInternalServerError, err.Error())
	}
	if deploymentVersion == nil {
		logger.ErrorC(ctx, "deployment version %s not found", version)
		fiberCtx.Context().NotFound()
		return nil
	}
	if deploymentVersion.Stage == domain.ActiveStage {
		errorMsg := fmt.Sprintf("Deleting active deployment version %s is forbidden", version)
		logger.ErrorC(ctx, "%s", errorMsg)
		return restutils.RespondWithError(fiberCtx, http.StatusForbidden, errorMsg)
	}
	if dr.GetMode() == dr.Standby {
		return restutils.RespondWithJson(fiberCtx, http.StatusOK, deploymentVersion)
	}
	err = v2.blueGreenService.DeleteCandidate(ctx, deploymentVersion)
	if err != nil {
		logger.ErrorC(ctx, "Can't delete candidate version %s \n %v", version, err)
		return restutils.RespondWithError(fiberCtx, http.StatusInternalServerError, err.Error())
	}
	return restutils.RespondWithJson(fiberCtx, http.StatusOK, deploymentVersion)
}

// HandlePostPromoteVersion godoc
// @Id PostPromoteVersionV2
// @Summary Post Promote Version
// @Description Post Promote Version
// @Tags control-plane-v2
// @Produce json
// @Param version path string true "version"
// @Security ApiKeyAuth
// @Success 202 {array} domain.DeploymentVersion
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v2/control-plane/promote/{version} [post]
func (v2 *BlueGreenController) HandlePostPromoteVersion(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	version := utils.CopyString(fiberCtx.Params("version"))
	if version == "" {
		logger.ErrorC(ctx, "Empty version path param")
		return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, "Empty version path param")
	}
	logger.InfoC(ctx, "Request to Promote version '%s'", version)
	var archiveSize int
	var err error
	archiveSizeString := string(fiberCtx.Context().FormValue("archiveSize"))
	if archiveSizeString == "" {
		archiveSize = 1
	} else {
		archiveSize, err = strconv.Atoi(archiveSizeString)
		if err != nil {
			logger.ErrorC(ctx, "Can't convert archiveSize to integer %v", err)
			return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, err.Error())
		}
	}
	all, _ := v2.dao.FindAllDeploymentVersions()
	logger.InfoC(ctx, "Current versions state: %v", all)
	deploymentVersion, err := v2.dao.FindDeploymentVersion(version)
	if err != nil {
		logger.ErrorC(ctx, "Can't get deployment version by version %s \n %v", version, err)
		return restutils.RespondWithError(fiberCtx, http.StatusInternalServerError, err.Error())
	}
	if deploymentVersion == nil {
		logger.ErrorC(ctx, "Deployment is version %s not found", version)
		fiberCtx.Context().NotFound()
		return nil
	}
	if deploymentVersion.Stage != domain.CandidateStage {
		logger.ErrorC(ctx, "Promote non candidate version (%s) is prohibited", version)
		return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, fmt.Sprintf("Promote non candidate version (%s) is prohibited", version))
	}
	if dr.GetMode() == dr.Standby {
		return restutils.RespondWithJson(fiberCtx, http.StatusAccepted, []*domain.DeploymentVersion{})
	}
	var dVersions []*domain.DeploymentVersion
	dVersions, err = v2.blueGreenService.Promote(ctx, deploymentVersion, archiveSize)
	if err != nil {
		logger.ErrorC(ctx, "Can't promote version %v with archive size %d \n %v", deploymentVersion, archiveSize, err)
		return restutils.RespondWithError(fiberCtx, http.StatusInternalServerError, err.Error())
	}
	return restutils.RespondWithJson(fiberCtx, http.StatusAccepted, dVersions)
}

// HandlePostRollbackVersion godoc
// @Id PostRollbackVersionV2
// @Summary Post Rollback VersionV2
// @Description Post Rollback VersionV2
// @Tags control-plane-v2
// @Produce json
// @Security ApiKeyAuth
// @Success 202 {array} domain.DeploymentVersion
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v2/control-plane/rollback [post]
func (v2 *BlueGreenController) HandlePostRollbackVersion(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	logger.InfoC(ctx, "Request to Rollback versions state")
	if dr.GetMode() == dr.Standby {
		return restutils.RespondWithJson(fiberCtx, http.StatusAccepted, []*domain.DeploymentVersion{})
	}
	all, _ := v2.dao.FindAllDeploymentVersions()
	logger.InfoC(ctx, "Current versions state: %v", all)
	dVersions, err := v2.blueGreenService.Rollback(ctx)
	if err == nil {
		return restutils.RespondWithJson(fiberCtx, http.StatusAccepted, dVersions)
	}
	logger.ErrorC(ctx, "Can't execute rollback %v", err)
	return err
}
