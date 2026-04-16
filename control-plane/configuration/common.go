package config

import (
	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/events"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/tlsmode"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"os"
	"strings"
)

const (
	configServerRouteKeyPrefix     = ""
	internalGatewayListenerName    = "internal-gateway-service-listener"
	publicGatewayListenerName      = "public-gateway-service-listener"
	privateGatewayListenerName     = "private-gateway-service-listener"
	internalGatewayRouteConfigName = "internal-gateway-service-routes"
	publicGatewayRouteConfigName   = "public-gateway-service-routes"
	privateGatewayRouteConfigName  = "private-gateway-service-routes"
)

func controlPlaneClusterName() string {
	if tlsmode.GetMode() == tlsmode.Disabled {
		return "control-plane||control-plane||8080"
	} else {
		return "control-plane||control-plane||8443"
	}
}

func configServerClusterName() string {
	if tlsmode.GetMode() == tlsmode.Disabled {
		return "config-server||config-server||8080"
	} else {
		return "config-server||config-server||8443"
	}
}

var (
	logger = logging.GetLogger("common-configuration")
)

type CommonConfiguration struct {
	memStorage      dao.Dao
	entityService   *entity.Service
	changes         map[string][]memdb.Change
	envoyChangesMap map[string][]memdb.Change
	eventBus        *bus.EventBusAggregator
	services        []*defaultServiceConfiguration
	gateways        []*defaultGatewayConfiguration
	secured         bool
}

type defaultServiceConfiguration struct {
	clusterName   string
	tlsConfigName string
	httpVersion   int32
	nodeGroups    []*domain.NodeGroup
	address       string
	port          int32
}

type defaultGatewayConfiguration struct {
	gatewayName                        string
	routeConfigName                    string
	routeListenerName                  string
	bindHost                           string
	bindPort                           string
	domain                             string
	createAndSaveGatewayRoutesFunction createAndSaveGatewayRoutesFunction
	secured                            bool
}

type ServiceMeshDetector struct {
	logger logging.Logger
}

func NewServiceMeshDetector(logger logging.Logger) *ServiceMeshDetector {
	return &ServiceMeshDetector{logger: logger}
}

func (d *ServiceMeshDetector) IsIstio() bool {
	serviceMeshTypeEnv, ok := os.LookupEnv("SERVICE_MESH_TYPE")
	return ok && strings.EqualFold(serviceMeshTypeEnv, "istio")
}

func (d *ServiceMeshDetector) WrapRouteCreator(creator func(storage dao.Repository, entityService entity.ServiceInterface, virtualHostId int32, currentDeploymentVersion string,
	initialDeploymentVersion string) error, gatewayName string) func(storage dao.Repository, entityService entity.ServiceInterface, virtualHostId int32, currentDeploymentVersion string,
	initialDeploymentVersion string) error {
	if !d.IsIstio() {
		return creator
	}
	
	return func(storage dao.Repository, entityService entity.ServiceInterface, virtualHostId int32, currentDeploymentVersion string,
	initialDeploymentVersion string) error {
		d.logger.Infof("Service mesh type is set to istio, skipping %s gateway routes creation", gatewayName)
		return nil
	}
}

func NewCommonConfiguration(memStorage dao.Dao, entityService *entity.Service, secured bool) *CommonConfiguration {
	return &CommonConfiguration{
		memStorage:      memStorage,
		entityService:   entityService,
		changes:         make(map[string][]memdb.Change),
		envoyChangesMap: make(map[string][]memdb.Change),
		services:        nil,
		gateways:        nil,
		secured:         secured,
	}
}

func (config *CommonConfiguration) CreateCommonConfiguration() error {
	baseNodeGroups, findErr := config.memStorage.FindAllNodeGroups()
	if findErr != nil {
		logger.Errorf("Can't get node groups from memory database: \n %v", findErr)
		return findErr
	}
	serviceNodeGroups := config.getServiceNodeGroups(baseNodeGroups)
	extAuthHost := os.Getenv("GATEWAY_AUTH_HOST")
	if extAuthHost == "" {
		extAuthHost = "gateway-auth-extension"
	}
	ipVersion := os.Getenv("IP_STACK")
	if ipVersion == "" {
		ipVersion = "v4"
	}

	var binder string
	if ipVersion == "v4" {
		binder = "0.0.0.0"
	} else {
		binder = "::"
	}
	config.services = []*defaultServiceConfiguration{
		{
			clusterName: controlPlaneClusterName(),
			httpVersion: 1,
			nodeGroups:  baseNodeGroups,
			address:     "control-plane",
			port:        8080,
		}, // TODO OS
		//{
		//	clusterName: domain.ExtAuthClusterName,
		//	httpVersion: 2,
		//	nodeGroups:  baseNodeGroups,
		//	address:     extAuthHost,
		//	port:        10050,
		//},
		{
			clusterName: configServerClusterName(),
			httpVersion: 1,
			nodeGroups:  serviceNodeGroups,
			address:     "config-server",
			port:        8080,
		},
	}
	if config.secured {
		config.services = append(config.services, &defaultServiceConfiguration{
			clusterName: domain.ExtAuthClusterName,
			httpVersion: 2,
			nodeGroups:  baseNodeGroups,
			address:     extAuthHost,
			port:        10050,
		})
	}

    serviceMeshDetector := NewServiceMeshDetector(logger)

	config.gateways = []*defaultGatewayConfiguration{
		{
			gatewayName:                        domain.InternalGateway,
			routeConfigName:                    internalGatewayRouteConfigName,
			routeListenerName:                  internalGatewayListenerName,
			bindHost:                           binder,
			bindPort:                           "8080",
			domain:                             "*",
			createAndSaveGatewayRoutesFunction: serviceMeshDetector.WrapRouteCreator(createAndSaveInternalGatewayRoutes, domain.InternalGateway),
			secured:                            config.secured,
		},
		{
			gatewayName:                        domain.PrivateGateway,
			routeConfigName:                    privateGatewayRouteConfigName,
			routeListenerName:                  privateGatewayListenerName,
			bindHost:                           binder,
			bindPort:                           "8080",
			domain:                             "*",
			createAndSaveGatewayRoutesFunction: serviceMeshDetector.WrapRouteCreator(createAndSavePrivateGatewayRoutes, domain.PrivateGateway),
			secured:                            config.secured,
		},
		{
			gatewayName:                        domain.PublicGateway,
			routeConfigName:                    publicGatewayRouteConfigName,
			routeListenerName:                  publicGatewayListenerName,
			bindHost:                           binder,
			bindPort:                           "8080",
			domain:                             "*",
			createAndSaveGatewayRoutesFunction: serviceMeshDetector.WrapRouteCreator(createAndSavePublicGatewayRoutes, domain.PublicGateway),
			secured:                            config.secured,
		},
	}

	for _, serviceConfig := range config.services {
		for _, nodeGroup := range serviceConfig.nodeGroups {
			changes, saveErr := config.memStorage.WithWTx(func(storage dao.Repository) error {
				return config.saveDefaultServiceConfigForNodeGroup(storage, serviceConfig, nodeGroup.Name)
			})
			if saveErr != nil {
				logger.Errorf("Can't save control-plane common configuration: \n %v", saveErr)
				return saveErr
			}
			config.changes[nodeGroup.Name] = append(config.changes[nodeGroup.Name], changes...)

			generateErr := config.generateAndSaveEnvoyConfigVersionInTransaction(nodeGroup.Name, domain.ClusterTable)
			if generateErr != nil {
				logger.Errorf("Can't save envoy configuration version for node group %s and table %s: \n %v", nodeGroup.Name, domain.ClusterTable, generateErr)
				return generateErr
			}

		}
	}
	for _, gatewayConfig := range config.gateways {
		changes, saveErr := config.memStorage.WithWTx(func(storage dao.Repository) error {
			return config.saveDefaultGatewayConfig(storage, gatewayConfig)
		})
		if saveErr != nil {
			logger.Errorf("Can't save control-plane common configuration: \n %v", saveErr)
			return saveErr
		}
		config.changes[gatewayConfig.gatewayName] = append(config.changes[gatewayConfig.gatewayName], changes...)

		saveEnvoyConfigVersionsErr := config.saveEnvoyConfigVersionsForGateway(gatewayConfig.gatewayName)
		if saveEnvoyConfigVersionsErr != nil {
			logger.Errorf("Can't save envoy config versions for gateway: \n %v", saveEnvoyConfigVersionsErr)
			return saveEnvoyConfigVersionsErr
		}
	}

	return nil
}

func (config *CommonConfiguration) PublishChanges(busInst bus.BusPublisher) error {
	for nodeGroup, changes := range config.changes {
		aggregatedChanges := changes
		if envoyChanges, found := config.envoyChangesMap[nodeGroup]; found {
			aggregatedChanges = append(aggregatedChanges, envoyChanges...)
		}
		event := events.NewChangeEventByNodeGroup(nodeGroup, aggregatedChanges)
		if err := busInst.Publish(bus.TopicChanges, event); err != nil {
			logger.Errorf("Failed to publish changes to event bus: %v", err)
			return err
		}
	}
	return nil
}

func (config *CommonConfiguration) saveDefaultServiceConfigForNodeGroup(storage dao.Repository, serviceConfig *defaultServiceConfiguration, nodeGroup string) error {
	actualVersion, err := config.entityService.GetActiveDeploymentVersion(storage)
	if err != nil {
		logger.Errorf("Common configuration failed to load active deployment version: %v", err)
		return err
	}
	//clusters
	clusterId, createClusterErr := serviceConfig.createAndSaveCluster(storage, config.entityService)
	if createClusterErr != nil {
		logger.Errorf("Saving cluster failed: %v", err)
		return createClusterErr
	}
	//connect cluster and node group
	connectErr := serviceConfig.connectClusterWithNodeGroup(storage, clusterId, nodeGroup, config.entityService)
	if connectErr != nil {
		logger.Errorf("Connecting cluster with node group failed: %v", err)
		return connectErr
	}
	//create and save endpoint
	createEndpointErr := serviceConfig.createAndSaveEndpoint(storage, config.entityService, actualVersion, clusterId)
	if createEndpointErr != nil {
		logger.Errorf("Saving endpoint failed: %v", err)
		return createEndpointErr
	}
	return nil
}

func (config *CommonConfiguration) saveDefaultGatewayConfig(storage dao.Repository, gatewayConfig *defaultGatewayConfiguration) error {
	defaultVersion := config.entityService.GetDefaultVersion()
	actualVersion, getVersionErr := config.entityService.GetActiveDeploymentVersion(storage)
	if getVersionErr != nil {
		logger.Errorf("Common configuration failed to load active deployment version: %v", getVersionErr)
		return getVersionErr
	}
	//create and save route configuration
	gatewayRouteConfigId, saveRouteConfigErr := gatewayConfig.createAndSaveRouteConfiguration(storage, config.entityService)
	if saveRouteConfigErr != nil {
		logger.Errorf("Saving route configuration failed: %v", saveRouteConfigErr)
		return saveRouteConfigErr
	}
	//create and save listeners
	saveListenergErr := gatewayConfig.createAndSaveListener(storage, config.entityService)
	if saveListenergErr != nil {
		logger.Errorf("Saving listener failed: %v", saveListenergErr)
		return saveListenergErr
	}
	//create and save virtual hosts
	virtualHostId, saveVirtualHostErr := gatewayConfig.createAndSaveVirtualHost(storage, config.entityService, gatewayRouteConfigId)
	if saveVirtualHostErr != nil {
		logger.Errorf("Saving virtual host failed: %v", saveVirtualHostErr)
		return saveVirtualHostErr
	}
	//create and save routes
	saveGatewayRoutesErr := gatewayConfig.createAndSaveGatewayRoutesFunction(storage, config.entityService, virtualHostId, actualVersion.Version, defaultVersion)
	if saveGatewayRoutesErr != nil {
		logger.Errorf("Saving gateway routes failed: %v", saveGatewayRoutesErr)
		return saveGatewayRoutesErr
	}
	return nil
}

func (config *CommonConfiguration) getServiceNodeGroups(nodeGroups []*domain.NodeGroup) []*domain.NodeGroup {
	var serviceNodeGroups []*domain.NodeGroup
	for _, nodeGroup := range nodeGroups {
		if domain.IsGatewayInternal(nodeGroup.Name) || domain.IsGatewayPrivate(nodeGroup.Name) {
			serviceNodeGroups = append(serviceNodeGroups, nodeGroup)
		}
	}
	logger.Infof("Service NodeGroups are %v", serviceNodeGroups)
	return serviceNodeGroups
}

func (config *CommonConfiguration) saveEnvoyConfigVersionsForGateway(nodeGroup string) error {
	for _, tableName := range []string{domain.ListenerTable, domain.RouteConfigurationTable} {
		err := config.generateAndSaveEnvoyConfigVersionInTransaction(nodeGroup, tableName)
		if err != nil {
			logger.Errorf("Can't save envoy configuration version for node group %s and table %s: \n %v", nodeGroup, tableName, err)
			return err
		}
	}

	return nil
}

func (config *CommonConfiguration) generateAndSaveEnvoyConfigVersionInTransaction(nodeGroup, entityType string) error {
	envoyChanges, err := config.memStorage.WithWTx(func(storage dao.Repository) error {
		//generate version of these changes
		return config.generateAndSaveEnvoyConfigVersion(storage, nodeGroup, entityType)
	})
	if err != nil {
		logger.Errorf("Can't save envoy configuration version for node group %s and table %s: \n %v", nodeGroup, entityType, err)
		return err
	}
	config.envoyChangesMap[nodeGroup] = append(config.envoyChangesMap[nodeGroup], envoyChanges...)
	return nil
}

func (config *CommonConfiguration) generateAndSaveEnvoyConfigVersion(storage dao.Repository, nodeGroup, entityType string) error {
	logger.Infof("Generating new EnvoyConfigVersion for NodeGroup '%s' and entityType '%s'", nodeGroup, entityType)
	envoyConfigVersion := domain.NewEnvoyConfigVersion(nodeGroup, entityType)
	logger.Infof("Saving %v to memory database", envoyConfigVersion)
	err := storage.SaveEnvoyConfigVersion(envoyConfigVersion)
	if err != nil {
		logger.Errorf("Can't save envoy configuration version %+v: \n %v", envoyConfigVersion, err)
		return err
	}
	return nil
}

// API for services
func (service *defaultServiceConfiguration) createAndSaveCluster(storage dao.Repository, entityService *entity.Service) (int32, error) {
	cluster := domain.NewCluster2(service.clusterName, &service.httpVersion)
	if service.tlsConfigName != "" {
		tlsConfig, err := storage.FindTlsConfigByName(service.tlsConfigName)
		if err != nil {
			logger.Panicf("Can't find tls config by name using DAO: \n %v", err)
		}
		if tlsConfig == nil {
			tlsConfig = &domain.TlsConfig{
				Id:      0,
				Name:    service.tlsConfigName,
				Enabled: true,
			}
			propagateSniEnv, exists := os.LookupEnv("SNI_PROPAGATION_ENABLED")
			if exists && strings.EqualFold(strings.TrimSpace(propagateSniEnv), "true") {
				tlsConfig.SNI = service.address
				logger.Debugf("SNI with value [%s] will be propagated for cluster [%s] with default tls configuration", service.address, cluster.Name)
			}
			logger.Infof("Saving %+v to memory database", *tlsConfig)
			if err := storage.SaveTlsConfig(tlsConfig); err != nil {
				logger.Panicf("Can't save tls config using DAO: \n %v", err)
			}
		}

		cluster.TLSId = tlsConfig.Id
		cluster.TLS = tlsConfig
	}

	logger.Infof("Saving %v to memory database", cluster)
	err := entityService.PutCluster(storage, cluster)
	if err != nil {
		logger.Errorf("Can't save cluster %+v to memory database: \n %v", cluster, err)
		return -1, err
	}
	return cluster.Id, nil
}

func (service *defaultServiceConfiguration) connectClusterWithNodeGroup(storage dao.Repository, clusterId int32, nodeGroup string, entityService *entity.Service) error {
	entity := &domain.ClustersNodeGroup{ClustersId: clusterId, NodegroupsName: nodeGroup}
	err := entityService.PutClustersNodeGroupIfAbsent(storage, entity)
	if err != nil {
		logger.Errorf("Can't save cluster with node group %+v to memory database: \n %v", entity, err)
		return err
	}
	return nil
}

func (service *defaultServiceConfiguration) createAndSaveEndpoint(storage dao.Repository, entityService *entity.Service, deploymentVersion *domain.DeploymentVersion, clusterId int32) error {
	endpoint := domain.NewEndpoint(service.address, service.port, deploymentVersion.Version, deploymentVersion.Version, clusterId)
	logger.Infof("Saving %v to memory database", endpoint)
	err := entityService.PutEndpoint(storage, endpoint)
	if err != nil {
		logger.Errorf("Can't save endpoint %+v to memory database: \n %v", endpoint, err)
		return nil
	}

	return nil
}

// API for gateways
func (gateway *defaultGatewayConfiguration) createAndSaveRouteConfiguration(storage dao.Repository, entityService *entity.Service) (int32, error) {
	routeConfiguration := domain.NewRouteConfiguration(gateway.routeConfigName, gateway.gatewayName)
	logger.Infof("Saving %v to memory database", routeConfiguration)
	err := entityService.PutRouteConfig(storage, routeConfiguration)
	if err != nil {
		logger.Errorf("Can't save route configuration %+v to memory database: \n %v", routeConfiguration, err)
		return -1, err
	}
	return routeConfiguration.Id, nil
}

func (gateway *defaultGatewayConfiguration) createAndSaveVirtualHost(storage dao.Repository, entityService *entity.Service, routeConfigId int32) (int32, error) {
	virtualHost := domain.NewVirtualHost(gateway.gatewayName, routeConfigId)
	virtualHostDomain := &domain.VirtualHostDomain{
		Domain:  gateway.domain,
		Version: 1,
	}
	virtualHost.Domains = []*domain.VirtualHostDomain{virtualHostDomain}
	logger.Infof("Saving %v to memory database", virtualHost)
	err := entityService.PutVirtualHost(storage, virtualHost)
	if err != nil {
		logger.Errorf("Can't save %v to memory database: %v", virtualHost, err)
		return -1, err
	}
	return virtualHost.Id, nil
}

func (gateway *defaultGatewayConfiguration) createAndSaveListener(storage dao.Repository, entityService *entity.Service) error {
	listener := domain.NewListener(gateway.routeListenerName, gateway.bindHost,
		gateway.bindPort, gateway.gatewayName, gateway.routeConfigName)
	logger.Infof("Saving %v to memory database", listener)
	err := entityService.PutListener(storage, listener)
	if err != nil {
		logger.Errorf("Can't save listener %+v to memory database: \n %v", listener, err)
		return err
	}

	if gateway.secured {
		err = gateway.createExtAuthzFilterForListener(storage)
	}
	return err
}

func (gateway *defaultGatewayConfiguration) createExtAuthzFilterForListener(storage dao.Repository) error {
	filter := domain.ExtAuthzFilter{
		Name:        gateway.gatewayName + "-" + domain.ExtAuthClusterName,
		ClusterName: "local-cluster",
		Timeout:     int64(15000),
		NodeGroup:   gateway.gatewayName,
	}
	err := storage.SaveExtAuthzFilter(&filter)
	if err != nil {
		logger.Errorf("Can't save extAuthz filter spec %+v to memory database: \n %v", filter, err)
	}
	return err
}
