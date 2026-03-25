package composite

import (
	"context"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/gofiber/fiber/v2"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/clustering"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/errorcodes"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/proxy"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/restutils"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/tlsmode"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/rest"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/valyala/fasthttp"
	"net/http"
	"strings"
	"time"
)

var log = logging.GetLogger("composite")

type ServiceMode int

const (
	// BaselineMode means that this control-plane is placed in the baseline namespace of the composite platform
	BaselineMode ServiceMode = iota
	// SatelliteMode means that this control-plane is placed in the satellite namespace of the composite platform
	SatelliteMode
)

func (mode ServiceMode) String() string {
	switch mode {
	case BaselineMode:
		return "BaselineMode"
	case SatelliteMode:
		return "SatelliteMode"
	default:
		return fmt.Sprintf("<Unknown composite.ServiceMode: %v>", int(mode))
	}
}

type Service struct {
	mode              ServiceMode
	coreBaseNamespace string
	dao               dao.Dao
	entityService     *entity.Service
	regService        *route.RegistrationService
	bus               bus.BusPublisher
	tenantManagerUrl  string
}

type Structure struct {
	Baseline   string   `json:"baseline"`
	Satellites []string `json:"satellites"`
}

var ErrSatelliteMode = errors.New("composite: service initialized in Satellite mode")

func NewService(coreBaseNamespace string, mode ServiceMode, dao dao.Dao, entityService *entity.Service, regService *route.RegistrationService, bus bus.BusPublisher) *Service {
	if mode == BaselineMode || mode == SatelliteMode {

		return &Service{
			mode:              mode,
			coreBaseNamespace: coreBaseNamespace,
			dao:               dao,
			entityService:     entityService,
			regService:        regService,
			bus:               bus,
			tenantManagerUrl:  tlsmode.UrlFromProperty(tlsmode.Http, "tenant.manager.api.url", domain.InternalGateway),
		}
	}
	log.Panicf("Trying to initialize composite Service in unsupported mode: %v", mode)
	return nil
}

func CreateCompositeProxy(srv *Service) *proxy.Service {
	serverPortString := configloader.GetOrDefaultString("http.server.bind", "8080")
	if tlsmode.GetMode() == tlsmode.Preferred {
		serverPortString = configloader.GetOrDefaultString("https.server.bind", "8443")
	}
	return proxy.NewService(fmt.Sprintf("control-plane.%s%s", srv.Baseline(), serverPortString),
		func(ctx *fiber.Ctx) bool { // condition to serve
			return srv.mode == BaselineMode
		},
		func(ctx *fiber.Ctx) bool { // condition to proxy
			return srv.mode == SatelliteMode
		},
		func(ctx *fiber.Ctx) error { // fallback function
			log.Warnf("Got request to the composite API, but this control-plane is not a part of the composite platform")
			return restutils.RespondWithError(ctx, http.StatusBadRequest, "This control-plane is not a part of the composite platform")
		})
}

func (srv *Service) Baseline() string {
	return srv.coreBaseNamespace
}

func (srv *Service) Mode() ServiceMode {
	return srv.mode
}

func (srv *Service) InitSatellite(timeout time.Duration) error {
	log.Debugf("Init satellite started: timeout=%v", timeout)
	if err := srv.registerInBaselineWithRetry(timeout); err != nil {
		log.Errorf("Failed to register in baseline: %v", err)
		return err
	}
	return srv.createFallbackRoutesToBaseline()
}

func (srv *Service) registerInBaselineWithRetry(timeout time.Duration) error {
	reqUrl := fmt.Sprintf("http://control-plane.%s:8080/api/v3/composite-platform/namespaces/%s", srv.coreBaseNamespace, msaddr.CurrentNamespaceAsString())
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		log.Infof("Attempt to register this namespace in composite with baseline %s", srv.coreBaseNamespace)
		response, err := rest.Client.DoRetryRequest(context.Background(), http.MethodPost, reqUrl, nil, log)
		if err != nil {
			err = errors.New(fmt.Sprintf("failed to register this namespace in composite with baseline %s: %v", srv.coreBaseNamespace, err))
			log.Errorf("%s", err.Error())
			clustering.AppendFatal(err)
		} else if response.StatusCode() == fasthttp.StatusOK || response.StatusCode() == fasthttp.StatusCreated {
			log.Infof("Successfully registered this namespace in composite with baseline %s", srv.coreBaseNamespace)
			fasthttp.ReleaseResponse(response)
			return nil
		} else {
			err := errors.New(fmt.Sprintf("unexpected response code when registering this namespace in composite with baseline %s: %v", srv.coreBaseNamespace, response.StatusCode()))
			log.Errorf("%s", err.Error())
			fasthttp.ReleaseResponse(response)
			clustering.AppendFatal(err)
		}

		time.Sleep(500 * time.Millisecond)
	}
	return errors.New(fmt.Sprintf("failed to register this namespace in composite with baseline %s: timeout exceeded", srv.coreBaseNamespace))
}

func (srv *Service) AddCompositeNamespace(context context.Context, namespace string) error {
	if srv.mode != BaselineMode {
		return ErrSatelliteMode
	}
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return errorcodes.NewCpError(errorcodes.ValidationRequestError, "composite: attempt to add empty composite satellite namespace to the composite structure", nil)
	}
	if _, err := srv.dao.WithWTx(func(dao dao.Repository) error {
		if existingSatellite, err := dao.FindCompositeSatelliteByNamespace(namespace); err != nil {
			log.ErrorC(context, "Failed to check composite satellite %s existence using DAO: %v", namespace, err)
			return err
		} else if existingSatellite != nil {
			log.InfoC(context, "Namespace %s is already satellite in this composite platform", namespace)
			return nil
		}
		if err := dao.SaveCompositeSatellite(&domain.CompositeSatellite{Namespace: namespace}); err != nil {
			log.ErrorC(context, "Failed to save composite satellite namespace %s using DAO: %v", namespace, err)
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	if err := srv.syncDefaultTenant(context); err != nil {
		return err
	}
	log.InfoC(context, "Namespace %s has been added as a satellite to this composite platform", namespace)
	return nil
}

func (srv *Service) syncDefaultTenant(context context.Context) error {
	log.InfoC(context, "Attempt to synchronize default tenant")
	reqUrl := srv.tenantManagerUrl + "/api/v4/tenant-manager/tenants/default/sync"
	// we do request with no retry here,
	// because this is a middle ground we are passing through
	// during registration satellite namespace in baseline in composite mode
	// and source of request will do retry if it's required (see registerInBaselineWithRetry).
	response, err := rest.Client.DoRequest(context, http.MethodPost, reqUrl, nil, log)
	if err != nil {
		err = errors.New(fmt.Sprintf("failed to synchronize default tenant: %v", err))
		log.ErrorC(context, "%s", err.Error())
		return err
	} else if response.StatusCode() == fasthttp.StatusOK {
		log.InfoC(context, "Default tenant synchronized successfully.")
		fasthttp.ReleaseResponse(response)
		return nil
	} else {
		if errorCodeError := errorcodes.NewRemoteRestErrorOrNil(response); errorCodeError != nil {
			fasthttp.ReleaseResponse(response)
			return errorCodeError
		}
		err = errors.New(fmt.Sprintf("unexpected response on synchronize default tenant request: %v", response.StatusCode()))
		fasthttp.ReleaseResponse(response)
		return err
	}
}

func (srv *Service) RemoveCompositeNamespace(namespace string) error {
	if srv.mode != BaselineMode {
		return ErrSatelliteMode
	}
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return errors.New("composite: attempt to delete empty composite satellite namespace from the composite structure")
	}
	if _, err := srv.dao.WithWTx(func(dao dao.Repository) error {
		if existingSatellite, err := dao.FindCompositeSatelliteByNamespace(namespace); err != nil {
			log.Errorf("Failed to check composite satellite %s existence using DAO: %v", namespace, err)
			return err
		} else if existingSatellite == nil {
			log.Infof("There is no satellite in this composite platform with namespace %s", namespace)
			return nil
		}
		if err := dao.DeleteCompositeSatellite(namespace); err != nil {
			log.Errorf("Failed to delete composite satellite namespace %s using DAO: %v", namespace, err)
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	log.Infof("Namespace %s has been deleted from this composite platform", namespace)
	return nil
}

func (srv *Service) GetCompositeStructure() (Structure, error) {
	if srv.mode != BaselineMode {
		return Structure{}, ErrSatelliteMode
	}
	loadedSatellites, err := srv.dao.WithRTxVal(func(dao dao.Repository) (interface{}, error) {
		satellites, err := dao.FindAllCompositeSatellites()
		if err != nil {
			log.Errorf("Failed to load composite platform structure using DAO: %v", err)
			return nil, err
		}
		return satellites, nil
	})
	if err != nil {
		return Structure{}, err
	}
	var structure Structure
	satellites := loadedSatellites.([]*domain.CompositeSatellite)
	structure.Baseline = msaddr.CurrentNamespaceAsString()
	structure.Satellites = make([]string, len(satellites))
	for idx, satellite := range satellites {
		structure.Satellites[idx] = satellite.Namespace
	}
	return structure, nil
}
