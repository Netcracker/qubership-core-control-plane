package v3

import (
	"fmt"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/data"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/restutils"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/debug"
	"net/http"

	"github.com/gofiber/fiber/v2"
)

type DebugService interface {
	DumpDataSnapshot() (*data.Snapshot, error)
	ValidateConfig() (*debug.StatusConfig, error)
}

type DebugController struct {
	service DebugService
}

func NewDebugController(service DebugService) *DebugController {
	return &DebugController{
		service: service,
	}
}

// HandleGetDump godoc
// @Id GetDump
// @Summary Get dump registry
// @Description Get dump registry
// @Tags control-plane-v3
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /debug/data-dump [get]
func (c *DebugController) HandleGetDump(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	if snapshot, err := c.service.DumpDataSnapshot(); err == nil {
		return restutils.RespondWithJson(fiberCtx, 200, snapshot)
	} else {
		msg := fmt.Sprintf("Can't take dump of in-memory data: %v.", err)
		log.ErrorC(ctx, "%s", msg)
		return restutils.RespondWithError(fiberCtx, http.StatusInternalServerError, msg)
	}
}

// HandleGetMeshDump godoc
// @Id GetMeshDump
// @Summary Get mesh dump registry
// @Description Get mesh dump registry
// @Tags control-plane-v3
// @Produce json
// @Produce application/zip
// @Security ApiKeyAuth
// @Success 200 {object} map[string]string
// @Success 200 {file} file "ZIP file"
// @Failure 500 {object} map[string]string
// @Router /api/v3/debug/internal/dump [get]
func (c *DebugController) HandleGetMeshDump(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	snapshot, err := c.service.DumpDataSnapshot()

	if err != nil {
		msg := fmt.Sprintf("Can't take dump of in-memory data: %v.", err)
		log.ErrorC(ctx, "%s", msg)
		return restutils.RespondWithError(fiberCtx, http.StatusInternalServerError, msg)
	}

	contentType := fiberCtx.Get("Accept")
	if contentType == "application/json" {
		return restutils.RespondWithJson(fiberCtx, 200, snapshot)
	} else {
		return restutils.RespondWithZip(fiberCtx, 200, snapshot, "mesh_dump.json", "mesh_dump.zip")
	}
}

// HandleGetConfigValidation godoc
// @Id GetConfigValidation
// @Summary Get config validation report
// @Description Get config validation report
// @Tags control-plane-v3
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v3/debug/config-validation [get]
func (c *DebugController) HandleGetConfigValidation(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	problem, err := c.service.ValidateConfig()
	if err != nil {
		msg := fmt.Sprintf("Can't validate config: %v.", err)
		log.ErrorC(ctx, "%s", msg)
		return restutils.RespondWithError(fiberCtx, http.StatusInternalServerError, msg)
	}
	return restutils.RespondWithJson(fiberCtx, 200, problem)
}
