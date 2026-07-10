package v3

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/gofiber/fiber/v3"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dr"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/errorcodes"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/events"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/restutils"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"net/http"
	"strings"
)

var errClusterNotFound = errors.New("no cluster found for provided clusterKey")

type ClusterKeepAliveController struct {
	service *entity.Service
	dao     dao.Dao
	bus     bus.BusPublisher
}

func NewClusterKeepAliveController(service *entity.Service, dao dao.Dao, bus bus.BusPublisher) *ClusterKeepAliveController {
	return &ClusterKeepAliveController{service: service, dao: dao, bus: bus}
}

// HandlePostClusterTcpKeepAlive godoc
// @Id PostClusterTcpKeepAlive
// @Summary Post Cluster TCP keepalive
// @Description Post Cluster TCP keepalive
// @Tags control-plane-v3
// @Produce json
// @Param request body dto.ClusterKeepAliveReq true "ClusterTcpKeepAlive"
// @Security ApiKeyAuth
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/v3/clusters/tcp-keepalive [post]
func (c *ClusterKeepAliveController) HandlePostClusterTcpKeepAlive(fiberCtx fiber.Ctx) error {
	request, err := c.readRequestBody(fiberCtx)
	if err != nil {
		return err
	}
	ctx := fiberCtx.Context()

	if dr.GetMode() == dr.Standby {
		return restutils.RespondWithJson(fiberCtx, http.StatusOK, nil)
	}

	if err := c.applyClusterKeepalive(ctx, request); err != nil {
		if err == errClusterNotFound {
			return errorcodes.NewCpError(errorcodes.ValidationRequestError, err.Error(), err)
		}
		log.ErrorC(ctx, "Failed to to apply stateful session: %v", err)
		return err
	}
	return restutils.RespondWithJson(fiberCtx, http.StatusOK, map[string]string{"message": "TCP keepalive for cluster applied successfully"})
}

func (c *ClusterKeepAliveController) readRequestBody(fiberCtx fiber.Ctx) (*dto.ClusterKeepAliveReq, error) {
	var request dto.ClusterKeepAliveReq
	if err := json.Unmarshal(fiberCtx.Body(), &request); err != nil {
		return nil, errorcodes.NewCpError(errorcodes.UnmarshalRequestError, fmt.Sprintf("Failed to unmarshal ClusterTcpKeepAlive apply request body: %v", err), err)
	}
	request.ClusterKey = strings.TrimSpace(request.ClusterKey)
	if request.ClusterKey == "" {
		errMsg := "ClusterTcpKeepAlive configuration is invalid: \"clusterKey\" field not be empty."
		return nil, errorcodes.NewCpError(errorcodes.ValidationRequestError, errMsg, errors.New(errMsg))
	}
	if request.TcpKeepalive != nil && (request.TcpKeepalive.Probes < 0 || request.TcpKeepalive.Time < 0 || request.TcpKeepalive.Interval < 0) {
		errMsg := "ClusterTcpKeepAlive configuration is invalid: \"probes\", \"time\" and \"interval\" fields must be greater than 0."
		return nil, errorcodes.NewCpError(errorcodes.ValidationRequestError, errMsg, errors.New(errMsg))
	}
	return &request, nil
}

func (c *ClusterKeepAliveController) applyClusterKeepalive(ctx context.Context, req *dto.ClusterKeepAliveReq) error {
	log.InfoC(ctx, "Applying cluster tcp keepalive: %+v", *req)
	changes, err := c.dao.WithWTx(func(repo dao.Repository) error {
		cluster, err := repo.FindClusterByName(req.ClusterKey)
		if err != nil {
			return err
		}
		if cluster == nil {
			return errClusterNotFound
		}

		if err = c.service.UpdateExistingClusterWithTcpKeepalive(repo, cluster, &domain.TcpKeepalive{
			Probes:   int32(req.TcpKeepalive.Probes),
			Time:     int32(req.TcpKeepalive.Time),
			Interval: int32(req.TcpKeepalive.Interval),
		}); err != nil {
			log.ErrorC(ctx, "Could not apply tcp keepalive for existing cluster %s:\n %v", req.ClusterKey, err)
			return err
		}
		log.InfoC(ctx, "Successfully saved tcp keepalive for cluster %s", req.ClusterKey)

		nodeGroups, err := repo.FindNodeGroupsByCluster(cluster)
		if err != nil {
			log.ErrorC(ctx, "Could not load node groups for cluster %s:\n %v", req.ClusterKey, err)
			return err
		}
		for _, nodeGroup := range nodeGroups {
			if err = repo.SaveEnvoyConfigVersion(domain.NewEnvoyConfigVersion(nodeGroup.Name, domain.ClusterTable)); err != nil {
				log.ErrorC(ctx, "Could not save EnvoyConfigVersion for cluster %s in node group %s:\n %v", req.ClusterKey, nodeGroup.Name, err)
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	if err = c.bus.Publish(bus.TopicMultipleChanges, events.NewMultipleChangeEvent(changes)); err != nil {
		log.ErrorC(ctx, "Can not publish changes with cluster tcp keepalive update:\n %v", err)
		return err
	}
	return nil
}
