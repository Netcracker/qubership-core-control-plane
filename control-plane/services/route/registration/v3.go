package registration

import (
	"context"
	"fmt"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/cluster/clusterkey"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/route/creator"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/tlsmode"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
	"strconv"
	"strings"
)

type V3RequestProcessor struct {
	dao dao.Dao
}

func NewV3RequestProcessor(dao dao.Dao) V3RequestProcessor {
	return V3RequestProcessor{
		dao: dao,
	}
}

func (p V3RequestProcessor) ProcessRequestV3(ctx context.Context, request dto.RoutingConfigRequestV3, activeVersion string) ([]ProcessedRequest, error) {
	logger.InfoC(ctx, "Casting request to domain model.")
	var overallResult []ProcessedRequest

	for _, gw := range request.Gateways {
		result, err := p.processGateway(gw, request.Namespace, activeVersion, request.TlsSupported, request.ListenerPort, request.VirtualServices...)
		if err != nil {
			return nil, err
		}
		overallResult = append(overallResult, result)
	}

	return overallResult, nil
}

func (p V3RequestProcessor) ProcessVirtualServiceRequestV3(vServiceRequest dto.VirtualService, gw, activeVersion string) (ProcessedRequest, error) {
	deploymentVersionsSet := make(map[string]bool)
	result, err := p.processGateway(gw, "", activeVersion, false, util.DefaultPort, vServiceRequest)
	if err != nil {
		return ProcessedRequest{}, err
	}

	for version := range deploymentVersionsSet {
		result.DeploymentVersions = append(result.DeploymentVersions, version)
	}

	return result, nil
}

func (p V3RequestProcessor) CreateVirtualHost(virtualService *dto.VirtualService) domain.VirtualHost {
	virtualHost := domain.VirtualHost{
		Name:                   virtualService.Name,
		RequestHeadersToRemove: virtualService.RemoveHeaders,
		RateLimitId:            virtualService.RateLimit,
		Routes:                 make([]*domain.Route, 0),
		Version:                1,
	}
	virtualHost.RequestHeadersToAdd = p.ConvertRequestHeadersToDomain(virtualService.AddHeaders)
	return virtualHost
}

func (p V3RequestProcessor) ConvertRequestHeadersToDomain(addHeaders []dto.HeaderDefinition) []domain.Header {
	domainHeadersToAdd := make([]domain.Header, 0, len(addHeaders))
	for _, headerDef := range addHeaders {
		domainHeadersToAdd = append(domainHeadersToAdd, domain.Header{
			Name:  headerDef.Name,
			Value: headerDef.Value,
		})
	}
	return domainHeadersToAdd
}

type void struct{}

var member void

type routeHost struct {
	host string
	port int32
}

func (p V3RequestProcessor) CreateVirtualHostDomains(namespace string, port int, hosts []string) []*domain.VirtualHostDomain {
	if len(hosts) == 0 {
		return []*domain.VirtualHostDomain{{Domain: "*", Version: 1}}
	}
	return p.GenerateVirtualHostDomains(namespace, port, hosts)
}

func (p V3RequestProcessor) GenerateVirtualHostDomains(namespace string, port int, domains []string) []*domain.VirtualHostDomain {
	actualDomains := p.GenerateDomains(namespace, port, domains)
	vHostDomains := make([]*domain.VirtualHostDomain, len(actualDomains))
	for i, host := range actualDomains {
		vHostDomains[i] = &domain.VirtualHostDomain{
			Domain:  host,
			Version: 1,
		}
	}
	return vHostDomains
}

func (p V3RequestProcessor) GenerateVirtualHostDomainsWithVirtualHostId(domains []string, vHostId int32) []*domain.VirtualHostDomain {
	vHostDomains := make([]*domain.VirtualHostDomain, len(domains))
	for i, host := range domains {
		vHostDomains[i] = &domain.VirtualHostDomain{
			Domain:        host,
			Version:       1,
			VirtualHostId: vHostId,
		}
	}
	return vHostDomains
}

func (p V3RequestProcessor) GenerateDomains(namespace string, port int, hosts []string) []string {
	domainsSet := make(map[string]void, 5*len(hosts))
	if namespace == "" {
		namespace = msaddr.CurrentNamespaceAsString()
	}

	for _, host := range hosts {
		p.generateAndAddHostsToSet(domainsSet, namespace, host, port)
	}

	domains := make([]string, 0, len(domainsSet))
	for k := range domainsSet {
		domains = append(domains, k)
	}
	return domains
}

func (p V3RequestProcessor) generateAndAddHostsToSet(hostsSet map[string]void, namespace string, host string, port int) {
	host = strings.TrimSpace(host)
	if host == "" {
		return
	}

	if idx := strings.Index(host, "://"); idx != -1 {
		host = host[idx+3:]
	}

	if strings.Contains(host, "*") {
		hostsSet[host] = member
	} else {
		hostAndPort := strings.Split(host, ":")
		if len(hostAndPort) > 2 { // this must be IPv6
			hostsSet[host] = member // copy as-is
		} else if len(hostAndPort) == 2 {
			if strings.Contains(host, ".") {
				hostsSet[host] = member
				p.lookupAndAddIpAddressesToSet(hostsSet, hostAndPort[0], ":"+hostAndPort[1])
			} else {
				p.addDomainsToSet(hostsSet, hostAndPort[0], namespace, ":"+hostAndPort[1])
			}
		} else { // no port specified
			if strings.Contains(host, ".") {
				hostsSet[host] = member
				p.lookupAndAddIpAddressesToSet(hostsSet, host, "")
			} else {
				if port != 0 {
					p.addDomainsToSet(hostsSet, hostAndPort[0], namespace, ":"+strconv.Itoa(port))
				} else {
					p.addDomainsToSet(hostsSet, hostAndPort[0], namespace, ":8080")
					if tlsmode.GetMode() == tlsmode.Preferred {
						p.addDomainsToSet(hostsSet, hostAndPort[0], namespace, ":8443")
					}
				}
			}
		}
	}
}

func (p V3RequestProcessor) lookupAndAddIpAddressesToSet(hostsSet map[string]void, host string, portSuffix string) {
	logger.Debug("IP addresses lookup for virtual hosts is stubbed in this release")
}

func (p V3RequestProcessor) addDomainsToSet(domainsSet map[string]void, host, namespace, portSuffix string) {
	domainsSet[fmt.Sprintf("%s%s", host, portSuffix)] = member
	domainsSet[fmt.Sprintf("%s.%s%s", host, namespace, portSuffix)] = member
	domainsSet[fmt.Sprintf("%s.%s.svc%s", host, namespace, portSuffix)] = member
	domainsSet[fmt.Sprintf("%s.%s.svc.cluster.local%s", host, namespace, portSuffix)] = member
	p.lookupAndAddIpAddressesToSet(domainsSet, fmt.Sprintf("%s.%s.svc.cluster.local", host, namespace), portSuffix)
}

func (p V3RequestProcessor) ProcessDestination(destination dto.RouteDestination, namespace, deploymentVersion string, tlsSupported bool) *domain.Cluster {
	clusterName, msAddress := p.resolveClusterNameAndMsAddress(destination, namespace, tlsSupported)
	return p.processClusterInternal(destination, msAddress, clusterName, deploymentVersion)
}

func (p V3RequestProcessor) resolveClusterNameAndMsAddress(destination dto.RouteDestination, namespace string, tlsSupported bool) (string, *msaddr.MicroserviceAddress) {
	endpointTlsSupported := p.isEndpointSupportedTls(destination) || tlsSupported
	routeEndpoint := p.getEndpoint(destination, endpointTlsSupported)
	msAddress := msaddr.NewMicroserviceAddress(routeEndpoint, namespace)
	clusterName := clusterkey.DefaultClusterKeyGenerator.GenerateKey(destination.Cluster, msAddress)

	return clusterName, msAddress
}

func (p V3RequestProcessor) processClusterInternal(destination dto.RouteDestination, msAddress *msaddr.MicroserviceAddress, clusterName, deploymentVersion string) *domain.Cluster {
	// build cluster
	cluster := domain.NewCluster2(clusterName, destination.HttpVersion)

	// build endpoint
	endpoint := resolveEndpoint(cluster, msAddress, deploymentVersion)

	if destination.TlsConfigName != "" {
		cluster.TLS = &domain.TlsConfig{
			Name: destination.TlsConfigName,
		}
	} else if endpoint.Protocol == "https" || endpoint.Port == 443 {
		cluster.TLS = &domain.TlsConfig{
			Name: destination.Cluster + "-tls",
		}
	}

	//Change if add new CircuitBreaker or thresholds
	if destination.CircuitBreaker.Threshold.MaxConnections != 0 {
		cluster.CircuitBreaker = &domain.CircuitBreaker{
			Threshold: &domain.Threshold{
				MaxConnections: int32(destination.CircuitBreaker.Threshold.MaxConnections),
			},
		}
	}

	if destination.TcpKeepalive != nil {
		cluster.TcpKeepalive = &domain.TcpKeepalive{
			Probes:   int32(destination.TcpKeepalive.Probes),
			Time:     int32(destination.TcpKeepalive.Time),
			Interval: int32(destination.TcpKeepalive.Interval),
		}
	}

	return cluster
}

func (p V3RequestProcessor) processGateway(gw, namespace, activeVersion string, tlsSupported bool, listenerPort int, virtualServices ...dto.VirtualService) (ProcessedRequest, error) {
	result := ProcessedRequest{
		NodeGroups:          make([]domain.NodeGroup, 0),
		Listeners:           make([]domain.Listener, 0),
		RouteConfigurations: make([]domain.RouteConfiguration, 0),
		ClusterNodeGroups:   make(map[string][]string),
		GroupedRoutes:       NewGroupedRoutesMap(),
		Clusters:            make([]domain.Cluster, 0),
		DeploymentVersions:  make([]string, 0),
		ClusterTlsConfig:    make(map[string]string),
		RouteRateLimit:      make(map[string]string),
	}

	commonEntityBuilder := NewCommonEntityBuilder(gw)

	// build node group
	result.NodeGroups = append(result.NodeGroups, commonEntityBuilder.CreateNodeGroup())

	// build listener
	result.Listeners = append(result.Listeners, commonEntityBuilder.CreateListenerWithCustomPort(listenerPort, tlsSupported))

	// build route config
	routeConfig := commonEntityBuilder.CreateRouteConfiguration()
	deploymentVersionsSet := make(map[string]bool)

	for _, virtualService := range virtualServices {
		// build virtual host
		virtualHost := p.CreateVirtualHost(&virtualService)

		for _, routeV3 := range virtualService.RouteConfiguration.Routes {
			// build cluster
			clusterName, msAddress := p.resolveClusterNameAndMsAddress(routeV3.Destination, namespace, tlsSupported)

			// resolve version
			initialDeploymentVersion, deploymentVersion, err := ResolveVersions(p.dao, clusterName, virtualService.RouteConfiguration.Version, activeVersion)
			if err != nil {
				return ProcessedRequest{}, err
			}
			logger.Infof("Resolved initialDeploymentVersion as '%s' and DeploymentVersion as '%s'", initialDeploymentVersion, deploymentVersion)
			if _, found := deploymentVersionsSet[deploymentVersion]; !found {
				deploymentVersionsSet[deploymentVersion] = true
			}

			// build endpoint
			cluster := p.processClusterInternal(routeV3.Destination, msAddress, clusterName, deploymentVersion)
			if cluster.TLS != nil {
				result.ClusterTlsConfig[cluster.Name] = cluster.TLS.Name
				cluster.TLS = nil
			}
			result.ClusterNodeGroups[cluster.Name] = []string{gw}
			result.Clusters = append(result.Clusters, *cluster)

			// build routes
			endpoint := cluster.Endpoints[0]
			endpointAddr := fmt.Sprintf("%s:%v", endpoint.Address, endpoint.Port)
			for _, rule := range routeV3.Rules {
				domainHeaderMatchers := dto.HeaderMatchersToDomain(rule.Match.HeaderMatchers)
				// fixup doublecheck key of this map for unique values.
				//   If route prefix+path is not unique, need to use other unique key!
				result.RouteRateLimit[rule.Match.Prefix+"||"+rule.Match.Path] = rule.RateLimit

				routeEntry := creator.NewRouteEntry(
					rule.Match.Prefix,
					rule.PrefixRewrite,
					namespace,
					creator.GetInt64Timeout(rule.Timeout),
					creator.GetInt64Timeout(rule.IdleTimeout),
					domainHeaderMatchers)
				isValidRoute := routeEntry.IsValidRoute()
				if !isValidRoute {
					logger.Warnf("The route hasn't been registered. Detected a bad route in the request. microserviceUrl = %s, Route entry = %v", endpointAddr, routeEntry)
					continue
				}
				route := routeEntry.CreateRoute(0, routeEntry.GetFrom(), endpointAddr, cluster.Name, routeEntry.GetTimeout(), routeEntry.GetIdleTimeout(),
					deploymentVersion, initialDeploymentVersion, domainHeaderMatchers, p.ConvertRequestHeadersToDomain(rule.AddHeaders), rule.RemoveHeaders)

				isAllowed := true
				if rule.Allowed != nil {
					isAllowed = *rule.Allowed
				}
				isDenied := false
				if rule.Deny != nil {
					isDenied = *rule.Deny
				}
				if isAllowed && !isDenied{
					routeEntry.ConfigureAllowedRoute(route)
					if rule.StatefulSession != nil {
						route.StatefulSession = rule.StatefulSession.ToRouteStatefulSession(gw)
					}
					route.RateLimitId = rule.RateLimit
					route.LuaFilterId = rule.LuaFilter
				} else {
					routeEntry.ConfigureProhibitedRoute(route, isDenied)
				}

				route.HostRewriteLiteral = strings.TrimSpace(rule.HostRewrite)

				virtualHost.Routes = append(virtualHost.Routes, route)
				// save route to grouped map for usage by RoutesAutoGenerator
				result.GroupedRoutes.PutRoute(namespace, cluster.Name, deploymentVersion, route)
			}
		}

		// build virtual host domains
		virtualHost.Domains = p.CreateVirtualHostDomains(namespace, listenerPort, virtualService.Hosts)

		routeConfig.VirtualHosts = append(routeConfig.VirtualHosts, &virtualHost)
	}
	result.RouteConfigurations = append(result.RouteConfigurations, routeConfig)
	for version := range deploymentVersionsSet {
		result.DeploymentVersions = append(result.DeploymentVersions, version)
	}
	return result, nil
}

func (p V3RequestProcessor) getEndpoint(destination dto.RouteDestination, tlsSupported bool) string {
	if tlsSupported && tlsmode.GetMode() == tlsmode.Preferred && destination.TlsEndpoint != "" {
		return destination.TlsEndpoint
	}

	return destination.Endpoint
}

func (p V3RequestProcessor) isEndpointSupportedTls(destination dto.RouteDestination) bool {
	return strings.Contains(destination.Endpoint, util.TlsProtocol) || destination.TlsSupported
}
