package lib

import (
	"context"
	"fmt"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/cert"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/clustering"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/com9n"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/composite"
	config "github.com/netcracker/qubership-core-control-plane/control-plane/v2/configuration"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/constancy"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/db"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dr"
	envoy "github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/grpc"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/errorcodes"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/listener"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/proxy"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/health"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/restutils"
	v1 "github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/v1"
	v2 "github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/v2"
	v3 "github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/v3"
	compositeV3 "github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/v3/composite"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/active"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/bluegreen"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/cleanup"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/cluster"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/configresources"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/debug"
	drSrv "github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/dr"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/gateway"
	health2 "github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/health"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/httpFilter"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/httpFilter/extAuthz"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/loadbalance"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/provider"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/ratelimit"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/factory"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/registration"
	srv1 "github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/v1"
	srv2 "github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/v2"
	srv3 "github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/v3"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/routingmode"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/statefulsession"
	tlsDef "github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/tls"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/tm"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/tlsmode"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/ui"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/websocket"
	"github.com/netcracker/qubership-core-lib-go-actuator-common/v2/tracing"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	loggerfiber "github.com/gofiber/fiber/v2/middleware/logger"
	fiberserver "github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v2"
	"github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v2/server"
	"github.com/netcracker/qubership-core-lib-go-rest-utils/v2/consul-propertysource"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"

	// swagger docs
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/docs"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const microservice_namespace = "microservice.namespace"

var (
	ctx, globalCancel = context.WithCancel(
		context.WithValue(
			context.Background(), "requestId", "",
		),
	)
	GenericDao    *dao.InMemDao
	EventBus      *bus.EventBusAggregator
	logger        logging.Logger
	shutdownHooks []func()
)

func RunServer() {
	consulPS := consul.NewLoggingPropertySource()
	propertySources := configloader.BasePropertySources()
	configloader.InitWithSourcesArray(append(propertySources, consulPS))
	consul.StartWatchingForPropertiesWithRetry(context.Background(), consulPS, func(event interface{}, err error) {
	})

	logger = logging.GetLogger("server")

	tlsmode.SetUpTlsProperties()

	defaultVersion := configloader.GetKoanf().MustString("blue-green.versions.default-version")
	if defaultVersion == "" {
		defaultVersion = "v1"
	}

	constStorageCfg, err := constancy.NewPostgresStorageConfigurator()
	if err != nil {
		panic(err)
	}
	constantStorage := constancy.NewStorage(ctx, constStorageCfg)
	entityService := entity.NewService(defaultVersion)
	inMemCfg := config.NewInMemoryStorageConfigurator(constantStorage, constantStorage)

	GenericDao = inMemCfg.GetDao()

	internalBus := bus.GetInternalBusInstance()
	grpcPublisher := bus.NewGRPCBusPublisher(GenericDao)
	grpcSubscriber := bus.NewGRPCBusSubscriber()

	EventBus = bus.NewEventBusAggregator(GenericDao, internalBus, internalBus, grpcSubscriber, grpcPublisher)

	routeComponentsFactory := factory.NewComponentsFactory(entityService)
	routingModeService := routingmode.NewService(GenericDao, defaultVersion)
	registrationService := route.NewRegistrationService(routeComponentsFactory, entityService, GenericDao, EventBus, routingModeService)
	v2RoutModeController := v2.NewRoutingModeController(routingModeService)
	v3RoutModeController := v3.NewRoutingModeController(routingModeService)
	v2RouteService := srv2.NewV2Service(routeComponentsFactory, entityService, GenericDao, EventBus, routingModeService, registrationService)
	v1RouteService := srv1.NewV1Service(entityService, GenericDao, EventBus, routingModeService, registrationService)
	v3RequestProcessor := registration.NewV3RequestProcessor(GenericDao)
	v3RouteService := srv3.NewV3Service(GenericDao, EventBus, routingModeService, registrationService, entityService, v3RequestProcessor)

	compositeService := initCompositePlatformService(GenericDao, entityService, registrationService, EventBus)
	compositeProxy := composite.CreateCompositeProxy(compositeService)
	v3CompositeController := compositeV3.NewCompositeController(compositeService)

	clusterService := cluster.NewClusterService(entityService, GenericDao, EventBus)
	certificateManager := &cert.CertificateManager{}
	tlsService := tlsDef.NewTlsService(GenericDao, EventBus, certificateManager)
	provider.Init(tlsService)
	extAuthzService := extAuthz.NewService(GenericDao, EventBus, entityService, registrationService, v3RequestProcessor)
	httpFilterSrv := httpFilter.NewWasmFilterService(GenericDao, EventBus, clusterService, entityService, extAuthzService)
	rateLimitService := ratelimit.NewService(GenericDao, EventBus, entityService)
	statefulSessionService := statefulsession.NewService(GenericDao, entityService, EventBus)
	v1controller := v1.NewController(v1RouteService, dto.RoutingV1RequestValidator{})
	v2RouteController := v2.NewRoutesController(v2RouteService, dto.RoutingV2RequestValidator{})
	v3RouteController := v3.NewRoutingConfigController(v3RouteService, dto.RoutingV3RequestValidator{})
	statefulSessionController := v3.NewStatefulSessionController(statefulSessionService, dto.RoutingV3RequestValidator{})
	v3ApplyConfigController := v3.NewApplyConfigurationController()
	v3RateLimitController := v3.NewRateLimitController(rateLimitService)
	v3ClusterTcpKeepaliveController := v3.NewClusterKeepAliveController(entityService, GenericDao, EventBus)

	loadBalanceService := loadbalance.NewLoadBalanceService(GenericDao, entityService, EventBus)
	blueGreenRegistry := bluegreen.NewVersionsRegistry(GenericDao, entityService, EventBus)
	blueGreenService := bluegreen.NewService(entityService, loadBalanceService, GenericDao, EventBus, blueGreenRegistry)

	gatewayService := gateway.NewService(GenericDao, entityService, EventBus)

	configresources.RegisterResource(v2RouteService.GetRegisterRoutesResource())
	configresources.RegisterResource(v3RouteService.GetRoutingRequestResource())
	configresources.RegisterResource(v3RouteService.GetVirtualServiceResource())
	configresources.RegisterResource(v3RouteService.GetRoutesDropResource())
	configresources.RegisterResource(httpFilterSrv.GetHttpFiltersResourceAdd())
	configresources.RegisterResource(httpFilterSrv.GetHttpFiltersResourceDrop())
	configresources.RegisterResource(clusterService.GetClusterResource())
	configresources.RegisterResource(tlsService.GetTlsDefResource())
	configresources.RegisterResource(statefulSessionService.GetStatefulSessionResource())
	configresources.RegisterResource(rateLimitService.GetRateLimitResource())
	configresources.RegisterResource(blueGreenRegistry.GetConfigRes())
	configresources.RegisterResource(gatewayService.GetConfigRes())
	v3UiService := ui.NewV3Service(GenericDao, entityService, ui.DefaultRouteSorter())
	v3UiController := ui.NewV3Controller(v3UiService)

	v2LoadBalanceController := v2.NewLoadBalanceController(loadBalanceService, dto.NewLBRequestValidator(GenericDao))
	configresources.RegisterResources(v2LoadBalanceController.GetLoadBalanceResources())
	v3LoadBalanceController := v3.NewLoadBalanceController(loadBalanceService, dto.NewLBRequestValidator(GenericDao))

	activeDCsService := createActiveDCsService(entityService, EventBus)
	v3ActiveDCsController := v3.NewActiveDCsController(activeDCsService)
	v2BlueGreenController := v2.NewBlueGreenController(blueGreenService, GenericDao)
	v3BlueGreenController := v3.NewBlueGreenController(blueGreenService, GenericDao)
	v3BGRegistryController := v3.NewBGRegistryController(blueGreenRegistry, GenericDao)
	v3HttpFilterController := v3.NewHttpFilterController(httpFilterSrv)
	v3GatewaySpecController := v3.NewGatewaySpecController(gatewayService)

	wsVersionController := websocket.NewVersionController(EventBus, GenericDao)
	wsActiveActiveController := websocket.NewActiveActiveController(EventBus, GenericDao)

	envoyConfigBuilder := envoy.DefaultEnvoyConfigurationBuilder(GenericDao, entityService, v3RouteService)

	debugService := debug.NewService(GenericDao, compositeService)
	v3DebugController := v3.NewDebugController(debugService)

	tlsController := v1.NewTlsController(tlsService)

	// TODO should be started when all services initialized and cluster node status is determined
	xdsServer := grpc.NewGRPCServer(EventBus.InternalBusSubscriber, GenericDao, envoyConfigBuilder)
	go func() {
		xdsServer.ListenAndServe()
	}()
	defer xdsServer.GracefulStop()
	updateManager := xdsServer.GetUpdateManager()

	tenantManagerWatcher := tm.NewWatcher(GenericDao, updateManager)

	nodeCommCfg := com9n.NewConfigurator(GenericDao, internalBus, grpcPublisher, grpcSubscriber, grpcSubscriber, updateManager)

	secured := configloader.GetKoanf().Bool("security.enabled")
	masterNodeInitializer := config.NewCommonMasterNodeInitializer(
		constantStorage,
		inMemCfg,
		updateManager,
		entityService,
		compositeService,
		secured)

	if dr.GetMode() == dr.Active {

		routesCleanupService := cleanup.NewRoutesCleanupService(GenericDao, EventBus, entityService)
		cleanupWorker := cleanup.NewRoutesCleanupWorker(routesCleanupService)

		lifeCycleManager := initClustering(constantStorage.DbProvider, masterNodeInitializer)
		lifeCycleManager.AddOnRoleChanged(nodeCommCfg.SetUpNodesCommunication)
		lifeCycleManager.AddOnRoleChanged(clustering.ApplyOnMasterChange(grpcPublisher.PurgeAllDeferredMessages))
		lifeCycleManager.AddOnRoleChanged(clustering.ApplyOnMasterChange(wsVersionController.ResetConnections))
		lifeCycleManager.AddOnRoleChanged(EventBus.RestartEventBus)
		lifeCycleManager.AddOnRoleChanged(tenantManagerWatcher.UpAndStartWatchSocket)
		lifeCycleManager.AddOnRoleChanged(cleanupWorker.ChangeCleanupNecessary)

		cleanupWorker.Start()
		defer cleanupWorker.Stop()
	} else {
		nodeInfo := thisNodeInfo()
		clustering.CurrentNodeState.ChangeNodeState(nodeInfo, clustering.Master)
		clustering.CurrentNodeState.SetMasterReady()

		tenantManagerWatcher.UpAndStartWatchSocket(nodeInfo, clustering.Master)

		partialReapplyListener := listener.NewPartialReloadEventListener(updateManager)
		EventBus.Subscribe(bus.TopicPartialReapply, partialReapplyListener.HandleEvent)

		drService := drSrv.Service{
			MasterInitializer: masterNodeInitializer,
			DBProvider:        constantStorage.DbProvider,
			ConstantStorage:   constantStorage,
			Dao:               GenericDao,
			Bus:               EventBus,
		}
		if err := drService.Start(); err != nil {
			logger.Panicf("Failed to start DR service: %v", err)
		}
		defer drService.Close()
	}
	healthService := health2.NewHealthService(nodeCommCfg)
	healthController := health.NewController(healthService)

	fiberConfig := fiber.Config{
		Network:      fiber.NetworkTCP,
		IdleTimeout:  30 * time.Second,
		ErrorHandler: errorcodes.DefaultErrorHandlerWrapper(errorcodes.UnknownErrorCode),
	}

	pprofPort := configloader.GetOrDefaultString("pprof.port", "6060")
	app, err := fiberserver.New(fiberConfig).
		WithPprof(pprofPort).
		WithPrometheus("/prometheus").
		WithTracer(tracing.NewZipkinTracer()).
		WithApiVersion().
		WithLogLevelsInfo().
		Process()
	if err != nil {
		logger.Error("Error while create app because: " + err.Error())
		return
	}

	if value, exists := os.LookupEnv("LOG_LEVEL"); exists && value == "debug" {
		app.Use(loggerfiber.New(loggerfiber.Config{
			Format:     "[${time}] [DEBUG] [loggerfiber] [${ip}:${port}] ${status} - ${method} ${path} ${latency}\n",
			Output:     os.Stdout, // Or a custom io.Writer for file logging
			TimeFormat: "2006-01-02T15:04:05.000",
		}))
	}

	apiV1 := app.Group("/api/v1", proxy.ProxyRequestsToMaster())
	apiV1.Post("/routes/:nodeGroup", v1controller.HandlePostRoutesWithNodeGroup)
	apiV1.Get("/routes/clusters", v1controller.HandleGetClusters)
	apiV1.Delete("/routes/clusters/:clusterId", v1controller.HandleDeleteClusterWithID)
	apiV1.Get("/routes/route-configs", v1controller.HandleGetRouteConfigs)
	apiV1.Get("/routes/node-groups", v1controller.HandleGetNodeGroups)
	apiV1.Get("/routes/listeners", v1controller.HandleGetListeners)
	apiV1.Delete("/routes/:nodeGroup", v1controller.HandleDeleteRoutesWithNodeGroup)

	apiV2 := app.Group("/api/v2", proxy.ProxyRequestsToMaster())

	apiV2.Get("/control-plane/routing/details", v2RoutModeController.HandleGetRoutingModeDetails)
	apiV2.Get("/control-plane/versions", v2BlueGreenController.HandleGetDeploymentVersions)
	apiV2.Delete("/control-plane/versions/:version", v2BlueGreenController.HandleDeleteDeploymentVersionWithID)
	apiV2.Post("/control-plane/load-balance", v2LoadBalanceController.HandlePostLoadBalance)
	apiV2.Post("/control-plane/promote/:version", v2BlueGreenController.HandlePostPromoteVersion)
	apiV2.Post("/control-plane/rollback", v2BlueGreenController.HandlePostRollbackVersion)
	apiV2.Delete("/control-plane/endpoints", v2RouteController.HandleDeleteEndpoints)
	apiV2.Delete("/control-plane/routes/uuid/:uuid", v2RouteController.HandleDeleteRouteWithUUID)
	apiV2.Get("/control-plane/versions/watch", wsVersionController.HandleVersionsWatchv2)

	apiV2RoutesReg := apiV2.Group("/control-plane/routes",
		v2RoutModeController.ValidateRoutesApplicabilityToCurrentRoutingMode(),
		v2RouteController.ValidateHeaderMatcher())
	apiV2RoutesReg.Post("/:nodeGroup", v2RouteController.HandlePostRoutesWithNodeGroup)

	apiV2DeleteRoutes := apiV2.Group("/control-plane/routes", v2.ValidateRouteDeleteRequest())
	apiV2DeleteRoutes.Delete("/:nodeGroup", v2RouteController.HandleDeleteRoutesWithNodeGroup)
	apiV2DeleteRoutes.Delete("/", v2RouteController.HandleDeleteRoutes)

	apiV3 := app.Group("/api/v3")

	// create sub-router so ProxyRequestsToMaster can be assigned only to routes assigned to this sub-router
	apiV3Proxy := apiV3.Group("", proxy.ProxyRequestsToMaster())
	apiV3Proxy.Get("/routing/details", v3RoutModeController.HandleGetRoutingModeDetails)
	apiV3Proxy.Get("/versions", v3BlueGreenController.HandleGetDeploymentVersions)
	apiV3Proxy.Delete("/versions/:version", v3BlueGreenController.HandleDeleteDeploymentVersionWithID)
	apiV3Proxy.Post("/load-balance", v3LoadBalanceController.HandlePostLoadBalance)
	apiV3Proxy.Post("/promote/:version", v3BlueGreenController.HandlePostPromoteVersion)
	apiV3Proxy.Post("/rollback", v3BlueGreenController.HandlePostRollbackVersion)
	apiV3Proxy.Delete("/endpoints", v3RouteController.HandleDeleteEndpoints)
	apiV3Proxy.Get("/versions/watch", wsVersionController.HandleVersionsWatch)
	apiV3Proxy.Get("/versions/microservices/:microservice", v3BlueGreenController.HandleGetMicroserviceVersion)
	apiV3Proxy.Get("/versions/registry", v3BGRegistryController.HandleGetMicroserviceVersions)
	apiV3Proxy.Delete("/versions/registry/services", v3BGRegistryController.HandleDeleteMicroserviceVersions)
	apiV3Proxy.Post("/versions/registry", v3BGRegistryController.HandlePostMicroserviceVersions)
	apiV3Proxy.Post("/rate-limits", v3RateLimitController.HandlePostRateLimit)
	apiV3Proxy.Get("/rate-limits", v3RateLimitController.HandleGetRateLimit)
	apiV3Proxy.Delete("/rate-limits", v3RateLimitController.HandleDeleteRateLimit)
	apiV3Proxy.Post("/clusters/tcp-keepalive", v3ClusterTcpKeepaliveController.HandlePostClusterTcpKeepAlive)

	apiV3Routes := apiV3.Group("/routes")
	apiV3Routes.Post("", v3RouteController.HandlePostRoutingConfig)
	apiV3Routes.Post("/:nodeGroup/:virtualServiceName", v3RouteController.HandleCreateVirtualService)
	apiV3Routes.Get("/:nodeGroup/:virtualServiceName", v3RouteController.HandleGetVirtualService)
	apiV3Routes.Put("/:nodeGroup/:virtualServiceName", v3RouteController.HandlePutVirtualService)
	apiV3Routes.Delete("/:nodeGroup/:virtualServiceName", v3RouteController.HandleDeleteVirtualService)
	apiV3Routes.Delete("", v3RouteController.HandleDeleteVirtualServiceRoutes)

	apiV3Proxy.Delete("/domains", v3RouteController.HandleDeleteVirtualServiceDomains)

	apiV3Proxy.Post("/config", v3ApplyConfigController.HandleConfig)
	apiV3Proxy.Post("/apply-config", v3ApplyConfigController.HandlePostConfig)
	apiV3Proxy.Get("/ui/cloud-config", v3UiController.HandleGetCloudConfig)
	apiV3Proxy.Get("/ui/:virtualHostId/:versionId/routes", v3UiController.HandleGetRoutes)
	apiV3Proxy.Get("/ui/clusters", v3UiController.HandleGetClusters)
	apiV3Proxy.Get("/ui/route/:routeUuid/details", v3UiController.HandleGetRouteDetails)

	apiV3Proxy.Post("/active-active", v3ActiveDCsController.HandleActiveActiveConfigPost)
	apiV3Proxy.Delete("/active-active", v3ActiveDCsController.HandleActiveActiveConfigDelete)

	apiV3Proxy.Post("/load-balance/stateful-session", statefulSessionController.HandlePostStatefulSession)
	apiV3Proxy.Put("/load-balance/stateful-session", statefulSessionController.HandlePutStatefulSession)
	apiV3Proxy.Delete("/load-balance/stateful-session", statefulSessionController.HandleDeleteStatefulSession)
	apiV3Proxy.Get("/load-balance/stateful-session", statefulSessionController.HandleGetStatefulSessions)

	apiV3Proxy.Get("/http-filters/:nodeGroup", v3HttpFilterController.HandleGetHttpFilters)
	apiV3Proxy.Post("/http-filters", v3HttpFilterController.HandlePostHttpFilters)
	apiV3Proxy.Delete("/http-filters", v3HttpFilterController.HandleDeleteHttpFilters)

	apiV3Proxy.Get("/gateways/specs", v3GatewaySpecController.HandleGetGatewaySpecs)
	apiV3Proxy.Post("/gateways/specs", v3GatewaySpecController.HandlePostGatewaySpecs)
	apiV3Proxy.Delete("/gateways/specs", v3GatewaySpecController.HandleDeleteGatewaySpecs)

	apiV3Proxy.Get("/tls/details", tlsController.HandleCetrificateDetails)

	apiV3ActiveWatch := apiV3.Group("/active-active/watch", proxy.ProxyRequestsToMaster())
	apiV3ActiveWatch.Get("", wsActiveActiveController.HandleActiveActiveWatch)

	apiV3CompositeApi := apiV3.Group("/composite-platform/namespaces", compositeProxy.ProxyHandler(), proxy.ProxyRequestsToMaster())
	apiV3CompositeApi.Get("", v3CompositeController.HandleGetCompositeStructure)
	apiV3CompositeApi.Post("/:namespace", v3CompositeController.HandleAddNamespaceToComposite)
	apiV3CompositeApi.Delete("/:namespace", v3CompositeController.HandleRemoveNamespaceFromComposite)

	debugApi := app.Group("/debug")
	debugApi.Get("/data-dump", v3DebugController.HandleGetDump)

	apiV3.Get("/debug/internal/dump", v3DebugController.HandleGetMeshDump)
	apiV3.Get("/debug/config-validation", v3DebugController.HandleGetConfigValidation)

	// swagger
	app.Get("/swagger-ui/swagger.json", func(ctx *fiber.Ctx) error {
		ctx.Set("Content-Type", "application/json")
		return ctx.Status(http.StatusOK).SendString(docs.SwaggerInfo.ReadDoc())
	})
	app.Get("/health", healthController.HandleLivenessProbe)
	app.Get("/ready", healthController.HandleReadinessProbe)
	app.Get("/memstats", func(c *fiber.Ctx) error {
		runtime.GC()
		memstat := runtime.MemStats{}
		runtime.ReadMemStats(&memstat)
		return restutils.RespondWithJson(c, http.StatusOK, memstat)
	})

	shutdownHooks = append(shutdownHooks, func() {
		logger.Info("Shutdown fiber server")
		if err := app.Shutdown(); err != nil {
			logger.ErrorC(ctx, "Control-plane error during server shutdown: %v", err)
		}
		logger.Info("Execute global cancel")
		globalCancel()
	})

	registerShutdownHooks()
	tlsDef.RegisterCertificateMetrics(tlsService)

	server.StartServer(app, "http.server.bind")
}

func RedirectToSwagger(ctx *fiber.Ctx) error {
	return ctx.Redirect("/swagger-ui/index.html")
}

func createActiveDCsService(entityService *entity.Service, eventBus *bus.EventBusAggregator) active.ActiveDCsService {
	localPublicGwHost, err := resolveLocalGwHost("PUBLIC_GATEWAY_ROUTE_HOST", "public")
	if err != nil {
		return active.NewDisabledActiveDCsService(err.Error())
	}
	localPrivateGwHost, err := resolveLocalGwHost("PRIVATE_GATEWAY_ROUTE_HOST", "private")
	if err != nil {
		return active.NewDisabledActiveDCsService(err.Error())
	}
	return active.NewActiveDCsService(GenericDao, entityService, eventBus, localPublicGwHost, localPrivateGwHost)
}

func resolveLocalGwHost(gwRouteHostEnvName string, prefix string) (string, error) {
	// need to find out which dc we currently in
	// by default GW route/ingress built automatically or specified via PUBLIC_GATEWAY_ROUTE_HOST/PRIVATE_GATEWAY_ROUTE_HOST env
	localGwHost := configloader.GetOrDefaultString(gwRouteHostEnvName, "")
	if localGwHost == "" {
		// '{gwRouteHostEnvName}' not specified during deploy, use host auto generated from namespace and CLOUD_PRIVATE_URL
		namespace := configloader.GetOrDefaultString(microservice_namespace, "")
		customHost := configloader.GetOrDefaultString("cloud.private.host", "")
		if namespace == "" || customHost == "" {
			return "", fmt.Errorf("cannot figure out '%sLocalGwHost'. Neither '%s' nor 'CLOUD_PRIVATE_HOST'/'MICROSERVICE_NAMESPACE' envs are provided.", prefix, gwRouteHostEnvName)
		}
		localGwHost = fmt.Sprintf("%s-gateway-%s.%s", prefix, namespace, customHost)
	}
	return localGwHost, nil
}

func thisNodeInfo() clustering.NodeInfo {
	podIP := configloader.GetKoanf().String("pod.ip")
	swimPort, _ := strconv.Atoi(configloader.GetKoanf().String("swim.port"))
	eventBusPort, _ := strconv.Atoi(configloader.GetOrDefaultString("event.bus.port", "5431"))
	serverPortString := configloader.GetOrDefaultString("http.server.bind", "8080")
	if tlsmode.GetMode() == tlsmode.Preferred {
		serverPortString = configloader.GetOrDefaultString("https.server.bind", "8443")
	}
	serverPortString = strings.Trim(serverPortString, ":")
	serverPort, err := strconv.Atoi(serverPortString)
	if err != nil {
		logger.Panicf("Could not read http server bind port: %v", err)
	}
	return clustering.NodeInfo{
		IP:       podIP,
		SWIMPort: uint16(swimPort),
		BusPort:  uint16(eventBusPort),
		HttpPort: uint16(serverPort),
	}
}

func initClustering(dbProvider db.DBProvider, initializer clustering.MasterNodeInitializer) *clustering.LifeCycleManager {
	podName := configloader.GetKoanf().String("pod.name")

	postgreSqlService := clustering.NewPostgreSqlService(dbProvider)
	electionSrv, err := clustering.CreateWithExistDb(postgreSqlService)
	if err != nil {
		panic(err)
	}

	if err := electionSrv.DeleteSeveralRecordsFromDb(); err != nil {
		panic(err)
	}

	thisNodeInfo := thisNodeInfo()
	namespace := configloader.GetOrDefaultString(microservice_namespace, "")
	lifecycleManager := clustering.NewLifeCycleManager(podName, namespace, thisNodeInfo, initializer)

	electorCfg := clustering.ElectorConfig{
		ElectionService:  electionSrv,
		LifeCycleManager: lifecycleManager,
	}
	nodeCfg := clustering.NodeConfig{
		Name: podName,
		IP:   thisNodeInfo.IP,
		Port: thisNodeInfo.SWIMPort,
	}

	elector, err := clustering.NewElector(electorCfg)
	if err != nil {
		panic(err)
	}

	serfNode, err := clustering.NewNode(nodeCfg)
	if err != nil {
		panic(err)
	}
	shutdownHooks = append(shutdownHooks, func() {
		logger.Infof("Leave from cluster")
		if err = serfNode.Leave(); err != nil {
			logger.Errorf("Error when leaving cluster: %v", err)
		}
	})

	lifecycleManager.AddOnRoleChanged(serfNode.JoinMembersNetwork)
	serfNode.AddOnMemberDropped(elector.ForceElection)
	if err = elector.Start(); err != nil {
		panic(err)
	}
	return lifecycleManager
}

func initCompositePlatformService(dao dao.Dao, entityService *entity.Service, regService *route.RegistrationService, bus bus.BusPublisher) *composite.Service {
	compositePlatformEnv := configloader.GetOrDefaultString("composite.platform", "")
	if compositePlatformEnv != "" && strings.EqualFold("true", compositePlatformEnv) {
		coreBaseNamespace := configloader.GetOrDefaultString("baseline.proj", "")
		if coreBaseNamespace != "" && !msaddr.NewNamespace(coreBaseNamespace).IsCurrentNamespace() {
			logger.Debugf("Control-plane is starting in satellite mode")
			return composite.NewService(coreBaseNamespace, composite.SatelliteMode, dao, entityService, regService, bus)
		}
	}
	logger.Debugf("Control-plane is starting in baseline mode")
	return composite.NewService(msaddr.CurrentNamespaceAsString(), composite.BaselineMode, dao, entityService, regService, bus)
}

func registerShutdownHooks() {
	go func() {
		sigint := make(chan os.Signal, 1)

		// interrupt signal sent from terminal
		signal.Notify(sigint, os.Interrupt)
		// sigterm signal sent from kubernetes
		signal.Notify(sigint, syscall.SIGTERM)

		logger.Info("OS signal '%s' received, starting shutdown", (<-sigint).String())

		for _, hook := range shutdownHooks {
			hook()
		}
	}()
}
