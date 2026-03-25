package v1

import (
	"encoding/json"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dr"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/restutils"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/v1"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"net/http"
	"strconv"
)

var logger logging.Logger

func init() {
	logger = logging.GetLogger("rest-controllers/v1")
}

type Controller struct {
	service   *v1.Service
	validator RequestValidator
}

type RequestValidator interface {
	Validate(request dto.RouteEntityRequest, nodeGroup string) (bool, string)
}

func NewController(service *v1.Service, validator RequestValidator) *Controller {
	this := &Controller{service: service, validator: validator}
	return this
}

// HandlePostRoutesWithNodeGroup godoc
// @Id RoutesWithNodeGroup
// @Summary Routes With Node Group
// @Description Post Routes With NodeGroup
// @Tags routes-controller-v1
// @Produce json
// @Param nodeGroup path string true "nodeGroup"
// @Param request body dto.RouteEntityRequest true "RouteEntityRequest"
// @Security ApiKeyAuth
// @Success 201
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/routes/{nodeGroup} [post]
func (c *Controller) HandlePostRoutesWithNodeGroup(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	nodeGroupParam := utils.CopyString(fiberCtx.Params("nodeGroup"))
	logger.InfoC(ctx, "Request to create routes for %v", nodeGroupParam)

	reqBody := fiberCtx.Body()
	data := dto.RouteEntityRequest{Allowed: true}
	if err := json.Unmarshal(reqBody, &data); err != nil {
		logger.ErrorC(ctx, "Failed to unmarshal routes registration v1 request body: %v", err)
		logger.DebugC(ctx, "Registration v1 request body: %v", string(reqBody))
		return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, fmt.Sprintf("Could not unmarshal request body JSON: %v", err))
	}

	if len(*data.Routes) == 0 {
		msg := "Array of routes is empty"
		logger.Error("%s", msg)
		return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, msg)
	}
	isDataValid, msg := c.validator.Validate(data, nodeGroupParam)
	if !isDataValid {
		logger.Error("%s", msg)
		return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, msg)
	}
	if dr.GetMode() == dr.Standby {
		return restutils.RespondWithJson(fiberCtx, http.StatusCreated, nil)
	}
	if err := c.service.RegisterRoutes(ctx, nodeGroupParam, data.ToRouteEntityRequestModel()); err != nil {
		logger.ErrorC(ctx, "Can't save routes %v \n %v", data, err)
		if errors.Is(err, entity.LegacyRouteDisallowed) || errors.Is(err, services.BadRouteRegistrationRequest) {
			return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, err.Error())
		}
		return restutils.RespondWithError(fiberCtx, http.StatusInternalServerError, err.Error())
	}
	return restutils.RespondWithJson(fiberCtx, http.StatusCreated, nil)
}

// HandleGetClusters godoc
// @Id GetClusters
// @Summary Get Clusters
// @Description Get Clusters
// @Tags routes-controller-v1
// @Security ApiKeyAuth
// @Success 200 {array} dto.ClusterResponse
// @Failure 500 {object} map[string]string
// @Router /api/v1/routes/clusters [get]
func (c *Controller) HandleGetClusters(fiberCtx *fiber.Ctx) error {
	clusters, err := c.service.GetClusters()
	if err != nil {
		return restutils.RespondWithError(fiberCtx, http.StatusInternalServerError, err.Error())
	} else {
		return restutils.RespondWithJson(fiberCtx, http.StatusOK, clusters)
	}
}

// HandleDeleteClusterWithID godoc
// @Id DeleteClusterWithID
// @Summary Delete Cluster With ID
// @Description Delete Cluster With ID
// @Tags routes-controller-v1
// @Produce json
// @Param clusterId path integer true "clusterId"
// @Security ApiKeyAuth
// @Success 204
// @Failure 500 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/v1/routes/clusters/{clusterId} [delete]
func (c *Controller) HandleDeleteClusterWithID(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	clusterIdVar := utils.CopyString(fiberCtx.Params("clusterId"))
	logger.InfoC(ctx, "Request to delete cluster by id %v", clusterIdVar)
	clusterId, err := strconv.Atoi(clusterIdVar)
	if err != nil {
		logger.ErrorC(ctx, "Failed to parse cluster id %v from cluster deletion request: %v", clusterIdVar, err)
		return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, err.Error())
	}
	if dr.GetMode() == dr.Standby {
		return restutils.RespondWithJson(fiberCtx, http.StatusNoContent, nil)
	}
	err = c.service.DeleteCluster(int32(clusterId))
	if err != nil {
		if err == v1.ErrNoClusterExist {
			return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, err.Error()+": "+strconv.Itoa(clusterId))
		} else {
			return restutils.RespondWithError(fiberCtx, http.StatusInternalServerError, err.Error())
		}
	} else {
		return restutils.RespondWithJson(fiberCtx, http.StatusNoContent, nil)
	}
}

// HandleGetRouteConfigs godoc
// @Id GetRouteConfigs
// @Summary Get route configs
// @Description Get route configs
// @Tags routes-controller-v1
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {array} dto.RouteConfigurationResponse
// @Failure 500 {object} map[string]string
// @Router /api/v1/routes/route-configs [get]
func (c *Controller) HandleGetRouteConfigs(fiberCtx *fiber.Ctx) error {
	routeConfigs, err := c.service.GetRouteConfigurations()
	if err != nil {
		return restutils.RespondWithError(fiberCtx, 500, err.Error())
	} else {
		return restutils.RespondWithJson(fiberCtx, 200, routeConfigs)
	}
}

// HandleGetNodeGroups godoc
// @Id GetNodeGroups
// @Summary Get route node-groups
// @Description Get route node-groups
// @Tags routes-controller-v1
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {array} domain.NodeGroup
// @Failure 500 {object} map[string]string
// @Router /api/v1/routes/node-groups [get]
func (c *Controller) HandleGetNodeGroups(fiberCtx *fiber.Ctx) error {
	nodeGroups, err := c.service.GetNodeGroups()
	if err != nil {
		return restutils.RespondWithError(fiberCtx, 500, err.Error())
	} else {
		return restutils.RespondWithJson(fiberCtx, 200, nodeGroups)
	}
}

// HandleGetListeners godoc
// @Id GetListeners
// @Summary Get route listeners
// @Description Get route listeners
// @Tags routes-controller-v1
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {array} domain.Listener
// @Failure 500 {object} map[string]string
// @Router /api/v1/routes/listeners [get]
func (c *Controller) HandleGetListeners(fiberCtx *fiber.Ctx) error {
	listeners, err := c.service.GetListeners()
	if err != nil {
		return restutils.RespondWithError(fiberCtx, 500, err.Error())
	} else {
		return restutils.RespondWithJson(fiberCtx, 200, listeners)
	}
}

// HandleDeleteRoutesWithNodeGroup godoc
// @Id DeleteRoutesWithNodeGroupV1
// @Summary Delete Routes With Node Group V1
// @Description Delete Routes With Node Group V1
// @Tags routes-controller-v1
// @Produce json
// @Param nodeGroup path string true "nodeGroup"
// @Param from query string false "from"
// @Param namespace query string false "namespace"
// @Security ApiKeyAuth
// @Success 204 {array} domain.Route
// @Failure 500 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/v1/routes/{nodeGroup} [delete]
func (c *Controller) HandleDeleteRoutesWithNodeGroup(fiberCtx *fiber.Ctx) error {
	nodeGroup := utils.CopyString(fiberCtx.Params("nodeGroup"))
	ctx := fiberCtx.UserContext()
	if nodeGroup == "" {
		return restutils.RespondWithError(fiberCtx, http.StatusBadRequest, "Empty nodeGroup path param")
	}

	if dr.GetMode() == dr.Standby {
		return restutils.ResponseNoContent(fiberCtx, []*domain.Route{})
	}
	from := string(fiberCtx.Context().FormValue("from"))
	namespace := string(fiberCtx.Context().FormValue("namespace"))
	logger.Debugf("Deleting routes, from param: `%s`, namespace param: `%s`", from, namespace)

	deletedRoutes, err := c.service.DeleteRoutes(ctx, nodeGroup, from, namespace)
	if err != nil {
		logger.DebugC(ctx, "Deleting routes caused error: %+v", err)
		return restutils.RespondWithError(fiberCtx, 500, err.Error())
	}
	return restutils.ResponseNoContent(fiberCtx, deletedRoutes)
}
