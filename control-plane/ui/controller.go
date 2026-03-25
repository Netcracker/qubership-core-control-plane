package ui

import (
	"context"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/errorcodes"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/restutils"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"net/http"
	"strconv"
)

const (
	defaultPageSize   = 50
	defaultPageNumber = 1
)

var (
	log = logging.GetLogger("ui")
)

type V3ApiController struct {
	service V3ApiUIService
}

type V3ApiUIService interface {
	GetAllSimplifiedRouteConfigs(ctx context.Context) ([]SimplifiedRouteConfig, error)
	GetRoutesPage(ctx context.Context, params SearchRoutesParameters) (PageRoutes, error)
	GetAllClusters(ctx context.Context) ([]Cluster, error)
	GetRouteDetails(ctx context.Context, uuid string) (RouteDetails, error)
}

func NewV3Controller(service V3ApiUIService) *V3ApiController {
	return &V3ApiController{service: service}
}

// HandleGetCloudConfig godoc
// @Id GetCloudConfig
// @Summary Get Cloud Config
// @Description Get Cloud Config
// @Tags control-plane-v3
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {array} ui.SimplifiedRouteConfig
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v3/ui/cloud-config [get]
func (c *V3ApiController) HandleGetCloudConfig(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	uiRouteConfigs, err := c.service.GetAllSimplifiedRouteConfigs(ctx)
	if err != nil {
		log.ErrorC(ctx, "Failed to get all simplified route configs caused error: %v", err)
		return err
	}
	return restutils.RespondWithJson(fiberCtx, http.StatusOK, uiRouteConfigs)
}

// HandleGetRoutes godoc
// @Id GetRoutes
// @Summary Get Routes
// @Description Get Routes
// @Tags control-plane-v3
// @Produce json
// @Param versionId path string true "versionId"
// @Param virtualHostId path string true "virtualHostId"
// @Param page query string false "page"
// @Param size query string false "size"
// @Security ApiKeyAuth
// @Success 200 {object} ui.PageRoutes
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v3/ui/{virtualHostId}/{versionId}/routes [get]
func (c *V3ApiController) HandleGetRoutes(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	params, err := ParseGetRoutesParameters(ctx, fiberCtx)
	if err != nil {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, fmt.Sprintf("Can't parse request parameters: %v", err), err)
	}

	page, err := c.service.GetRoutesPage(ctx, params)
	if err != nil {
		log.ErrorC(ctx, "Failed to make routes page with params '%+v' caused error: %v", params, err)
		return err
	}

	return restutils.RespondWithJson(fiberCtx, http.StatusOK, page)
}

// HandleGetClusters godoc
// @Id GetClustersV3
// @Summary Get Clusters V3
// @Description Get Clusters V3
// @Tags control-plane-v3
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {array} ui.Cluster
// @Failure 500 {object} map[string]string
// @Router /api/v3/ui/clusters [get]
func (c *V3ApiController) HandleGetClusters(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	uiClusters, err := c.service.GetAllClusters(ctx)
	if err != nil {
		log.ErrorC(ctx, "Failed to get all clusters caused error: %v", err)
		return err
	}
	return restutils.RespondWithJson(fiberCtx, http.StatusOK, uiClusters)
}

// HandleGetRouteDetails godoc
// @Id GetRouteDetails
// @Summary Get Route Details
// @Description Get Route Details
// @Tags control-plane-v3
// @Produce json
// @Param routeUuid path string true "routeUuid"
// @Security ApiKeyAuth
// @Success 200 {object} ui.RouteDetails
// @Failure 500 {object} map[string]string
// @Router /api/v3/ui/route/{routeUuid}/details [get]
func (c *V3ApiController) HandleGetRouteDetails(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	routeUUID := restutils.GetFiberParam(fiberCtx, "routeUuid")
	log.DebugC(ctx, "Get route details for uuid %s", routeUUID)
	routeDetails, err := c.service.GetRouteDetails(ctx, routeUUID)
	if err != nil {
		log.ErrorC(ctx, "Failed to get route details caused error %v", err)
		return err
	}
	return restutils.RespondWithJson(fiberCtx, http.StatusOK, routeDetails)
}

type SearchRoutesParameters struct {
	VirtualHostId int32
	Version       string
	Size          int
	Page          int
	Search        string
}

func (p SearchRoutesParameters) LowerBound() int {
	return (p.Page - 1) * p.Size
}

func (p SearchRoutesParameters) UpperBound() int {
	return p.Page * p.Size
}

func ParseGetRoutesParameters(ctx context.Context, fiberCtx *fiber.Ctx) (SearchRoutesParameters, error) {
	var err error
	var size, page int
	if sizeRaw := fiberCtx.Query("size"); sizeRaw != "" {
		size, err = strconv.Atoi(sizeRaw)
		if err != nil {
			return SearchRoutesParameters{}, errors.WrapPrefix(err, "parsing query parameter 'size' caused error", 1)
		}
	} else {
		log.WarnC(ctx, "Query parameter 'size' is not set in url. Using default %d", defaultPageSize)
		size = defaultPageSize
	}
	if pageRaw := fiberCtx.Query("page"); pageRaw != "" {
		page, err = strconv.Atoi(pageRaw)
		if err != nil {
			return SearchRoutesParameters{}, errors.WrapPrefix(err, "parsing query parameter 'page' caused error", 1)
		}
	} else {
		log.WarnC(ctx, "Query parameter 'page' is not set in url. Using default %d", defaultPageNumber)
		page = defaultPageNumber
	}
	var virtualHostId int32
	if vhIdRaw := utils.CopyString(fiberCtx.Params("virtualHostId", "")); vhIdRaw != "" {
		if vhId, err := strconv.ParseInt(vhIdRaw, 10, 32); err != nil {
			return SearchRoutesParameters{}, errors.WrapPrefix(err, "Parsing virtualHostId caused error: %v", 1)
		} else {
			virtualHostId = int32(vhId)
		}
	} else {
		return SearchRoutesParameters{}, errors.New("Path parameter virtualHostId is not set, but required")
	}
	var versionId string
	if versionRaw := utils.CopyString(fiberCtx.Params("versionId", "")); versionRaw != "" {
		versionId = versionRaw
	} else {
		return SearchRoutesParameters{}, errors.New("Path parameter versionId is not set, but required")
	}

	return SearchRoutesParameters{
		VirtualHostId: virtualHostId,
		Version:       versionId,
		Size:          size,
		Page:          page,
		Search:        fiberCtx.Query("search"),
	}, nil
}
