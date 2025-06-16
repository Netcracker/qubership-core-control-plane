package main

import (
	"database/sql"
	"errors"
	"fmt"
	container2 "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

const (
	TestCluster      = "test-service"
	TestEndpointV1   = "test-service-v1:8080"
	listenerBasePath = "configs.#[@type=\"type.googleapis.com/envoy.admin.v3.ListenersConfigDump\"]"
)

const TestTimeout = 2 * time.Minute

const Postgres = "postgres"

var (
	log = logging.GetLogger("it_control_plane")

	// Indicates whether TEST_DOCKER_URL env is set. Each test must check this flag before execution.
	skipDockerTests = false

	localHost = "localhost"
	dockerHost string
	hostIP     string

	tenantManager *tmStub

	internalGateway *GatewayContainer
	egressGateway   *GatewayContainer

	// Control-plane status
	cpIsUp = false

	tmPort int
)

type NetworkAdapter struct {
	Name string
	IP   string
}

func init() {
	// Need to set IdpServerAddr before policies updating process
	tmPort = findFreePort()
	tenantManager = StartTmStub(tmPort)
	setEnvIfNotSet("TENANT_MANAGER_API_URL", fmt.Sprintf("http://localhost:%d", tmPort))
}

func TestMain(m *testing.M) {
	log.InfoC(ctx, "Starting docker tests")

	defer func() {
		if r := recover(); r != nil {
			log.InfoC(ctx, "Docker tests panic caused by:\n %v", r)
			panic(r)
		}
	}()

	startTime := time.Now()
	deadline := startTime.Add(TestTimeout)

	// contract requires TEST_DOCKER_URL env variable to be set to run tests with docker
	// empty string is a valid value: resolveDockerHost function will resolve actual docker URL
	testDockerUrl, found := os.LookupEnv("TEST_DOCKER_URL")
	if !found {
		log.InfoC(ctx, "Env variable TEST_DOCKER_URL is not set, trying to resolve docker host")
		skipDockerTests = false
		testDockerUrl = ""
		//os.Exit(m.Run())
	}

	// resolve Docker API URL and host machine IP for future usage
	resolveDockerHost(strings.TrimSpace(testDockerUrl))

	var err error
	cli, err = client.NewClientWithOpts(client.WithHostFromEnv(), client.WithAPIVersionNegotiation())
	if err != nil {
		log.PanicC(ctx, "Could not create docker client:\n %v", err)
	}

	// create container manager
	cm = &ContainerManager{
		mutex:      &sync.Mutex{},
		containers: make(map[string]*ContainerInfo),
	}

	// create test docker network
	cm.CreateTestNetwork()

	// run actual tests and purge resources after that
	code := executeTests(m, deadline)

	cm.PurgeDockerResources()
	os.Exit(code)
}

func skipTestIfDockerDisabled(t *testing.T) {
	if skipDockerTests {
		t.Skipf("Skipping test due to docker is not configured")
	}
}

func getHostIp() string {
	exec.Command("ip", "addr", "show")
	hostIPAddr, err := net.LookupIP("host.docker.internal")
	if err != nil {
		log.WarnC(ctx, "Failed to resolve host IP addr by \"host.docker.internal\" hostname: %v", err)

		//notice! sometimes `hostname -I` can return list of IP adressess.
		// `hostname -i` can return like this '10.88.0.2 fe80::c8f1:efff:fe68:a723%eth0' or 'fe80::c8f1:efff:fe68:a723%eth0 10.88.0.2'
		
		output, err := exec.Command("hostname", "-I").Output()
		if err != nil {
			log.WarnC(ctx, "'hostname -I' returned output '%s' and error: %v", output, err)
			return getIpWithNetSh()
		}
		log.InfoC(ctx, "'hostname -I' returned: %s", output)
		return strings.SplitN(strings.TrimSpace(string(output)), " ", 2)[0]
	} else if len(hostIPAddr) == 0 {
		log.PanicC(ctx, "Resolved no host IP addresses")
	}
	return hostIPAddr[0].String()
}

func getAdaptersWithNetSh() []NetworkAdapter {
	output, err := exec.Command("netsh", "interface", "ip", "show", "address").Output()
	if err != nil {
		log.WarnC(ctx, "Failed to exec `netsh interface ip show address`: %v; output: %v", err, string(output))
		return nil
	}
	lines := strings.Split(string(output), "\r\n")
	adapters := make([]NetworkAdapter, 0)
	adapter := NetworkAdapter{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if strings.Contains(line, "Configuration for interface") {
			if adapter.IP != "" {
				adapters = append(adapters, adapter)
			}
			adapter = NetworkAdapter{}
			adapter.Name = line[strings.Index(line, "\"")+1 : strings.LastIndex(line, "\"")]
			continue
		}
		if strings.Contains(line, "IP Address:") {
			adapter.IP = strings.TrimSpace(line[strings.Index(line, "IP Address:")+12:])
			continue
		}
	}
	return adapters
}

func getIpWithNetSh() string {
	adapters := getAdaptersWithNetSh()
	log.InfoC(ctx, "IT engine discovered adapters: %+v", adapters)
	for _, adapter := range adapters { // use virtual box adapter if any
		if strings.HasPrefix(adapter.Name, "VirtualBox Host-Only Network") {
			log.InfoC(ctx, "Using adapter %+v as host IP", adapter)
			return adapter.IP
		}
	}
	for _, adapter := range adapters { // fallback to ethernet
		if strings.HasPrefix(adapter.Name, "Ethernet") {
			log.InfoC(ctx, "Using adapter %+v as host IP", adapter)
			return adapter.IP
		}
	}
	return ""
}

func resolveDockerHost(testDockerUrl string) {
	dockerAddr := testDockerUrl
	if dockerAddr == "" {
		log.InfoC(ctx, "Env TEST_DOCKER_URL is empty")

		dockerAddr = os.Getenv("DOCKER_HOST")
		if dockerAddr == "" {
			log.InfoC(ctx, "Env DOCKER_HOST is empty")
			dockerAddr = "host.docker.internal"
		}
	} else {
		if err := os.Setenv("DOCKER_HOST", dockerAddr); err != nil {
			log.PanicC(ctx, "Failed to set env DOCKER_HOST=%s\n %v", dockerAddr, err)
		}
	}

	log.InfoC(ctx, "Using docker host %s", dockerAddr)
	dockerHost = dockerAddr
	protoIdx := strings.Index(dockerHost, "://")
	if protoIdx != -1 {
		dockerHost = dockerHost[protoIdx+3:]
	}
	portIdx := strings.IndexRune(dockerHost, ':')
	if portIdx != -1 {
		dockerHost = dockerHost[:portIdx]
	}
	hostIP = getHostIp()
	log.InfoC(ctx, "Resolved host IP: %v", hostIP)
}

func setEnvIfNotSet(name, value string) {
	if actualVal, set := os.LookupEnv(name); set {
		log.InfoC(ctx, "Env %v is already set to '%v'", name, actualVal)
	} else {
		if err := os.Setenv(name, value); err != nil {
			log.PanicC(ctx, "Failed to set env %v: %v", name, err)
		}
		log.InfoC(ctx, "Env %v is set to '%v'", name, value)
	}
}

func runControlPlaneForTests(deadline time.Time) {
	setEnvIfNotSet("IDP_CLIENT_USERNAME", "control-plane")
	setEnvIfNotSet("IDP_CLIENT_PASSWORD", "control-plane")
	setEnvIfNotSet("MICROSERVICE_NAMESPACE", "test-control-plane")
	setEnvIfNotSet("POLICY_UPDATE_ENABLED", "false")
	setEnvIfNotSet("ECDH_CURVES", "P-256,P-384")

	pgContainer := cm.containers[Postgres]
	setEnvIfNotSet("PG_HOST", localHost)
	setEnvIfNotSet("PG_PORT", fmt.Sprintf("%v", pgContainer.Ports[5432]))
	setEnvIfNotSet("PG_DB", "test_control_plane")
	setEnvIfNotSet("PG_USER", "postgres")
	setEnvIfNotSet("PG_PASSWD", "12345")
	// required for it_storage_test.go
	setEnvIfNotSet("microservice.namespace", "test-control-plane")

	cpIsUp = false
	//go launchControlPlaneWithRetry(deadline)
	go launchControlPlane()

	for !cpIsUp && time.Now().Before(deadline) {
		resp, err := http.DefaultClient.Get("http://localhost:8080/ready")
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				cpIsUp = true
				break
			} else {
				log.InfoC(ctx, "Readiness check failed... Status Code: %v", resp.StatusCode)
			}
		} else {
			log.InfoC(ctx, "Failed to get control-plane readiness: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !cpIsUp {
		log.PanicC(ctx, "Failed to check control-plane readiness")
	}
}

func launchControlPlane() {
	defer func() {
		if r := recover(); r != nil {
			log.ErrorC(ctx, "Recovered from control-plane launch panic: %v", r)
			cpIsUp = false
		}
	}()
	main()
}

func executeTests(m *testing.M, deadline time.Time) int {
	createPgContainer()

	// start control-plane
	configloader.Init(configloader.BasePropertySources()...)
	runControlPlaneForTests(deadline)

	internalGateway = CreateGatewayContainer("internal-gateway-service")
	log.Infof("Gw URL: %v; Gw admin URL: %v", internalGateway.Url, internalGateway.AdminUrl)

	egressGateway = CreateGatewayContainer("egress-gateway")
	log.Infof("Gw URL: %v; Gw admin URL: %v", egressGateway.Url, egressGateway.AdminUrl)

	// run actual tests
	return m.Run()
}

func createPgContainer() {
	cm.RunContainerWithRetry(&CreateContainerOpts{
		name:  Postgres,
		image: "postgres:9.6.20",
		env: []string{
			"POSTGRES_USERNAME=postgres",
			"POSTGRES_PASSWORD=12345",
			"POSTGRES_DB=test_control_plane"},
		ports:      []nat.Port{"5432"},
		readyCheck: CheckPostgresHealth,
	})
}

func CheckPostgresHealth(pgResource *ContainerInfo) error {
	return runPgQuery(pgResource, "SELECT 1;")
}

func runPgQuery(pgResource *ContainerInfo, query string) error {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		localHost, pgResource.Ports[5432], "postgres", "12345", "test_control_plane")
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		log.ErrorC(ctx, "Postgres connection was unsuccessful during ready check:\n %v", err)
		return err
	}
	defer db.Close()

	_, err = db.Exec(query)
	if err != nil {
		log.ErrorC(ctx, "\"%s\" postgres query execution was unsuccessful:\n %v", query, err)
	}
	return err
}

type GatewayContainer struct {
	TestContainer
	Url         string
	HostAndPort string
	AdminUrl    string
}

func CreateGatewayContainer(serviceName string) *GatewayContainer {
	healthCheck := httpHealthCheck{PortId: 9901, Path: "/config_dump"}

	cm.RunContainerWithRetry(&CreateContainerOpts{
		name:  serviceName,
		image: "ghcr.io/netcracker/qubership-core-ingress-gateway:main-20250325151826-6",
		env: []string{"SERVICE_NAME_VARIABLE=" + serviceName,
			"ENVOY_UID=0",
			"IP_STACK=v4",
			"IP_BIND=0.0.0.0",
			"POD_HOSTNAME=localhost",
			"GW_MEMORY_LIMIT=100Mi"},
		ports:      []nat.Port{"8080", "9901"},
		extraHosts: []string{"control-plane:" + hostIP, "control-plane:" + hostIP},
		readyCheck: healthCheck.CheckHealth,
	})

	c := cm.containers[serviceName]
	return &GatewayContainer{
		TestContainer: TestContainer{Name: serviceName},
		Url:           fmt.Sprintf("http://%s:%v", localHost, c.Ports[8080]),
		HostAndPort:   fmt.Sprintf("%s:%v", localHost, c.Ports[8080]),
		AdminUrl:      fmt.Sprintf("http://%s:%v", localHost, c.Ports[9901]),
	}
}

func createTraceServiceContainer(serviceFamilyName, deploymentVersion string, bluegreen bool) TestContainer {
	return createTraceServiceContainerOnPort(serviceFamilyName, deploymentVersion, 8080, 8443, bluegreen)
}

func createTraceServiceContainerOnPort(serviceFamilyName, deploymentVersion string, port, httpsPort int, bluegreen bool) TestContainer {
	return createTraceServiceContainerInternal(serviceFamilyName, deploymentVersion, port, httpsPort, bluegreen, "")
}

func createTraceServiceContainerInternal(serviceFamilyName, deploymentVersion string, port, httpsPort int, bluegreen bool, ecdhCurves string) TestContainer {
	portStr := strconv.Itoa(port)
	httpsPortStr := strconv.Itoa(httpsPort)
	openshiftServiceName := serviceFamilyName
	if bluegreen {
		openshiftServiceName = fmt.Sprintf("%s-%s", serviceFamilyName, deploymentVersion)
	}
	healthCheck := httpHealthCheck{PortId: port, Path: "/health"}

	cm.RunContainerWithRetry(&CreateContainerOpts{
		name:  openshiftServiceName,
		image: "cp-test-service:1.0-SNAPSHOT",
		env: []string{
			"HTTP_SERVER_BIND=:" + portStr,
			"HTTPS_SERVER_BIND=:" + httpsPortStr,
			"MICROSERVICE_NAME=" + openshiftServiceName,
			"SERVICE_NAME=" + serviceFamilyName,
			"DEPLOYMENT_VERSION=" + deploymentVersion,
			"ECDH_CURVES=" + ecdhCurves},
		ports:      []nat.Port{nat.Port(portStr), nat.Port(httpsPortStr), "8888"},
		extraHosts: []string{"control-plane:" + hostIP},
		readyCheck: healthCheck.CheckHealth,
	})
	return TestContainer{Name: openshiftServiceName}
}

type httpHealthCheck struct {
	PortId int
	Path   string
}

func (healthChecker httpHealthCheck) CheckHealth(c *ContainerInfo) error {
	url := fmt.Sprintf("http://%s:%v%s", localHost, c.Ports[healthChecker.PortId], healthChecker.Path)
	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		log.ErrorC(ctx, "Error in request to %v: %v", healthChecker, err)
		return err
	}
	if 200 != resp.StatusCode {
		log.ErrorC(ctx, "Got invalid status code from %v: %d", healthChecker, resp.StatusCode)
		return errors.New(fmt.Sprintf("Got invalid status code from %v: %d", healthChecker, resp.StatusCode))
	}
	defer resp.Body.Close()
	return nil
}

type TestContainer struct {
	Name string
}

func (container *TestContainer) GetPort(port int) int {
	return cm.containers[container.Name].Ports[port]
}

func (container *TestContainer) Purge() {
	// all the conainers are cached for the entire test bundle run - this function purge is no-op
	//log.InfoC(ctx, "Purging docker resource %v (ID: %v; Name: %v; Image: %v)", container.Name, container.Container.ID, container.Container.Name, container.Container.Image)
	//if err := pool.Purge(container.Resource); err != nil {
	//	log.PanicC(ctx, "Could not purge resource %s: %s", container.Name, err)
	//}
	//delete(containerManager.Containers, container.Name)
	//log.InfoC(ctx, "Test container %s purged successfully", container.Name)
}

func (cm *ContainerManager) PurgeDockerResources() {
	defer func() {
		if r := recover(); r != nil {
			log.WarnC(ctx, "Recovered from panic in docker resources purge:\n %v", r)
		}
	}()

	for name, container := range cm.containers {
		log.InfoC(ctx, "Purging docker resource %v (ID: %v; Name: %v)", name, container.ID, container.Name)
		if err := cli.ContainerRemove(ctx, container.ID, container2.RemoveOptions{Force: true}); err != nil {
			log.ErrorC(ctx, "Could not purge resource %s: %s", name, err)
		}
		log.InfoC(ctx, "Test container %s purged successfully", name)
	}

	if err := cli.NetworkRemove(ctx, cm.networkID); err != nil {
		log.ErrorC(ctx, "Failed to remove test docker network: %v", err)
	}
}
