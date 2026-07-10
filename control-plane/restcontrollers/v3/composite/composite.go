package composite

import (
	"errors"
	"fmt"
	"github.com/gofiber/fiber/v3"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/composite"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/errorcodes"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/restutils"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"net/http"
	"strings"
)

var logger = logging.GetLogger("rest-controllers/v3/composite")

type Controller struct {
	srv *composite.Service
}

func NewCompositeController(srv *composite.Service) *Controller {
	return &Controller{srv: srv}
}

// HandleGetCompositeStructure godoc
// @Id HandleGetCompositeStructure
// @Summary Get Composite Structure
// @Description Get Composite Structure
// @Tags control-plane-v3
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} composite.Structure
// @Failure 500 {object} map[string]string
// @Router /api/v3/composite-platform/namespaces [get]
func (c *Controller) HandleGetCompositeStructure(fiberCtx fiber.Ctx) error {
	context := fiberCtx.Context()
	logger.InfoC(context, "Request to get composite platform structure")
	structure, err := c.srv.GetCompositeStructure()
	if err != nil {
		logger.ErrorC(context, "Failed to get composite platform structure: %v", err)
		return err
	}
	return restutils.ResponseOk(fiberCtx, &structure)
}

// HandleAddNamespaceToComposite godoc
// @Id AddNamespaceToComposite
// @Summary Add Namespace To Composite
// @Description Add Namespace To Composite
// @Tags control-plane-v3
// @Produce json
// @Param namespace path string true "namespace"
// @Security ApiKeyAuth
// @Success 200
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v3/composite-platform/namespaces/{namespace} [post]
func (c *Controller) HandleAddNamespaceToComposite(fiberCtx fiber.Ctx) error {
	context := fiberCtx.Context()
	namespace := restutils.GetFiberParam(fiberCtx, "namespace")
	logger.InfoC(context, "Request to add namespace %s to the composite platform as a satellite", namespace)
	if err := c.validateNamespace(namespace); err != nil {
		if rootCauseErr := errorcodes.GetCpErrCodeErrorOrNil(err); rootCauseErr != nil {
			return rootCauseErr
		}
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, fmt.Sprintf("Invalid namespace '%s': %v", namespace, err), err)
	}

	err := c.srv.AddCompositeNamespace(context, namespace)
	if err != nil {
		logger.ErrorC(context, "Failed to add namespace to the composite: %v", err)
		return err
	}
	logger.InfoC(context, "Successfully added namespace %s to the composite platform as a satellite", namespace)
	fiberCtx.Status(http.StatusOK)
	return nil
}

// HandleRemoveNamespaceFromComposite godoc
// @Id RemoveNamespaceFromComposite
// @Summary Remove Namespace From Composite
// @Description Remove Namespace From Composite
// @Tags control-plane-v3
// @Produce json
// @Param namespace path string true "namespace"
// @Security ApiKeyAuth
// @Success 200
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v3/composite-platform/namespaces/{namespace} [delete]
func (c *Controller) HandleRemoveNamespaceFromComposite(fiberCtx fiber.Ctx) error {
	context := fiberCtx.Context()
	namespace := restutils.GetFiberParam(fiberCtx, "namespace")
	logger.InfoC(context, "Request to remove namespace %s from the composite platform as a satellite", namespace)
	if err := c.validateNamespace(namespace); err != nil {
		if rootCauseErr := errorcodes.GetCpErrCodeErrorOrNil(err); rootCauseErr != nil {
			return rootCauseErr
		}
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, fmt.Sprintf("Invalid namespace '%s': %v", namespace, err), err)
	}

	err := c.srv.RemoveCompositeNamespace(namespace)
	if err != nil {
		logger.ErrorC(context, "Failed to delete namespace from the composite: %v", err)
		return err
	}
	logger.InfoC(context, "Successfully removed namespace %s from the composite platform", namespace)
	fiberCtx.Status(http.StatusOK)
	return nil
}

// validateNamespace validates namespace name the same way as kubernetes does: using the definition of a label in
// DNS (RFC 1123)
func (c *Controller) validateNamespace(namespace string) error {
	if strings.TrimSpace(namespace) == "" {
		return errors.New("namespace path variable must no be empty")
	}
	if msaddr.NewNamespace(namespace).IsCurrentNamespace() && c.srv.Mode() == composite.BaselineMode {
		return errorcodes.NewCpError(errorcodes.CompositeConflictError, "This operation cannot be performed on the baseline namespace", nil)
	}
	return util.IsDNS1123Label(namespace)
}
