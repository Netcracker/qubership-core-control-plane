package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"strings"
	"testing"
	"time"

	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/events"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/lib"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	asrt "github.com/stretchr/testify/assert"
)

const TestCookieName = "sticky-cookie-v1"

func addTestService(assert *asrt.Assertions, containerName string, bgVersion string) *TestContainer {
	bluegreen := bgVersion != ""
	if !bluegreen {
		bgVersion = "v1"
	}
	srvContainer := createTraceServiceContainer(containerName, bgVersion, bluegreen)

	changes, err := lib.GenericDao.WithWTx(func(repo dao.Repository) error {
		cluster, err := repo.FindClusterByName("test-service||test-service||8080")
		if err != nil {
			log.Errorf("Test failed to load cluster: %v", err)
			return err
		}
		newEndpoint := &domain.Endpoint{
			Address:                  containerName,
			Port:                     8080,
			ClusterId:                cluster.Id,
			DeploymentVersion:        bgVersion,
			InitialDeploymentVersion: bgVersion,
			DeploymentVersionVal:     nil,
			HashPolicies:             nil,
			Hostname:                 containerName,
			OrderId:                  0,
		}
		if err := repo.SaveEndpoint(newEndpoint); err != nil {
			log.Errorf("Test failed to save endpoint: %v", err)
			return err
		}
		return repo.SaveEnvoyConfigVersion(domain.NewEnvoyConfigVersion("internal-gateway-service", domain.ClusterTable))
	})
	assert.Nil(err)
	err = lib.EventBus.Publish(bus.TopicChanges, events.NewChangeEventByNodeGroup("internal-gateway-service", changes))
	assert.Nil(err)
	time.Sleep(30 * time.Second)

	return &srvContainer
}

func run10Requests(assert *asrt.Assertions, requestsByPod map[string]int, cookieVal string, headers ...map[string]string) {
	for i := 0; i < 10; i++ {
		req := prepareRequestWithCookieAndHeaders(assert, cookieVal, headers...)
		respFromService, statusCode := SendToTraceSrvWithRetry503(assert, req)
		log.Infof("Got status %d and pod id: %v", statusCode, respFromService.PodID)
		assert.Equal(200, statusCode)
		assert.NotNil(respFromService)
		assert.NotEmpty(respFromService.PodID)
		requestsByPod[respFromService.PodID]++
		expectedVersion := ""
		if headers != nil {
			if version, ok := headers[0]["X-Version"]; ok {
				expectedVersion = version
			}
		}
		if expectedVersion == "" {
			deploymentVersions := getVersions(assert)
			for _, version := range deploymentVersions {
				if version.Stage == domain.ActiveStage {
					expectedVersion = version.Version
					break
				}
			}
		}
		assert.Equal(expectedVersion, respFromService.Version)
		log.Infof("RESPONSE: %+v", respFromService)
	}
}

func runRequestsConnectionTest(assert *asrt.Assertions, requestsByPod map[string]int, attempts int) {
	for i := 0; i < attempts; i++ {
		req, err := http.NewRequest(http.MethodGet, internalGateway.Url+"/api/v1/test-service/lb-test", nil)
		assert.Nil(err)
		respFromService, statusCode := SendToTraceSrvWithRetry503(assert, req)
		log.Infof("Got status %d and pod id: %v", statusCode, respFromService.PodID)
		assert.Equal(200, statusCode)
		assert.NotNil(respFromService)
		assert.NotEmpty(respFromService.RemoteAddr)
		requestsByPod[respFromService.RemoteAddr]++
		log.Infof("Requests by RemoteAddr: %+v", requestsByPod)
		log.Infof("RESPONSE: %+v", respFromService)
	}
}

func prepareRequestWithCookieAndHeaders(assert *asrt.Assertions, cookieVal string, headers ...map[string]string) *http.Request {
	req, err := http.NewRequest(http.MethodGet, internalGateway.Url+"/api/v1/test-service/lb-test", nil)
	assert.Nil(err)
	if cookieVal != "" {
		req.AddCookie(&http.Cookie{
			Name:     TestCookieName,
			Value:    cookieVal,
			Path:     "/",
			HttpOnly: true,
		})
	}
	if len(headers) > 0 {
		for headerName, headerVal := range headers[0] {
			req.Header.Set(headerName, headerVal)
		}
	}
	return req
}

func prepareRequestWithCookieAndHeaders2(assert *asrt.Assertions, cookieName, cookieVal string, headers ...map[string]string) *http.Request {
	req, err := http.NewRequest(http.MethodGet, internalGateway.Url+"/api/v1/test-service/lb-test", nil)
	assert.Nil(err)
	if cookieVal != "" {
		req.AddCookie(&http.Cookie{
			Name:     cookieName,
			Value:    cookieVal,
			Path:     "/",
			HttpOnly: true,
		})
	}
	if len(headers) > 0 {
		for headerName, headerVal := range headers[0] {
			req.Header.Set(headerName, headerVal)
		}
	}
	return req
}

func obtainCookie(assert *asrt.Assertions, requestsByPodId map[string]int, headers ...map[string]string) string {
	req := prepareRequestWithCookieAndHeaders(assert, "", headers...)
	resp, err := http.DefaultClient.Do(req)
	assert.Nil(err)
	assert.Equal(200, resp.StatusCode)
	log.Infof("cookie request response headers: %+v", resp.Header)
	response := ReadTraceServiceResponse(assert, resp)
	log.Infof("cookie request response body: %+v", response)
	assert.NotNil(response)
	assert.NotEmpty(response.PodID)
	requestsByPodId[response.PodID]++
	cookieString := resp.Header.Get("Set-Cookie")
	assert.NotEmpty(cookieString)
	cutCookieStr := cookieString[strings.Index(cookieString, TestCookieName):]
	cookieValInBrackets := cutCookieStr[len(TestCookieName)+1 : strings.Index(cutCookieStr, ";")]
	return strings.ReplaceAll(cookieValInBrackets, "\"", "")
}

func obtainCookie2(assert *asrt.Assertions, cookieName string, headers ...map[string]string) string {
	req := prepareRequestWithCookieAndHeaders(assert, "", headers...)
	resp, err := http.DefaultClient.Do(req)
	assert.Nil(err)
	assert.Equal(200, resp.StatusCode)
	log.Infof("cookie request response headers: %+v", resp.Header)
	response := ReadTraceServiceResponse(assert, resp)
	log.Infof("cookie request response body: %+v", response)
	assert.NotNil(response)
	cookieString := resp.Header.Get("Set-Cookie")
	assert.NotEmpty(cookieString)
	assert.Contains(cookieString, cookieName+"=")
	cutCookieStr := cookieString[strings.Index(cookieString, cookieName):]
	cookieValInBrackets := cutCookieStr[len(cookieName)+1 : strings.Index(cutCookieStr, ";")]
	return strings.ReplaceAll(cookieValInBrackets, "\"", "")
}

func someOtherRequest(assert *asrt.Assertions) {
	req, err := http.NewRequest(http.MethodGet, internalGateway.Url+"/api/v1/test-service/some-other", nil)
	assert.Nil(err)
	resp, err := http.DefaultClient.Do(req)
	assert.Nil(err)
	assert.Equal(200, resp.StatusCode)
	log.Infof("some other request response headers: %+v", resp.Header)
}

func Test_IT_LoadBalance_ClusterStatefulSessionBG(t *testing.T) {
	testStatefulSessionWithBG(t, `---
kind: StatefulSession
apiVersion: nc.core.mesh/v3
metadata:
 namespace: ''
spec:
 gateways: [ "internal-gateway-service" ]
 cluster: "test-service"
 enabled: true
 cookie:
   name: sticky-cookie
   ttl: 0
   path: /`, `---
kind: StatefulSession
apiVersion: nc.core.mesh/v3
metadata:
 namespace: ''
spec:
 gateways: [ "internal-gateway-service" ]
 cluster: "test-service"
 version: "v2"
 enabled: true
 cookie:
   name: sticky-cookie
   ttl: 0
   path: /`)
}

func Test_IT_LoadBalance_EndpointStatefulSessionBG(t *testing.T) {
	testStatefulSessionWithBG(t, `---
kind: StatefulSession
apiVersion: nc.core.mesh/v3
metadata:
 namespace: ''
spec:
 gateways: [ "internal-gateway-service" ]
 cluster: "test-service"
 version: "v1"
 port: 8080
 enabled: true
 cookie:
   name: sticky-cookie
   ttl: 0
   path: /`, `---
kind: StatefulSession
apiVersion: nc.core.mesh/v3
metadata:
 namespace: ''
spec:
 gateways: [ "internal-gateway-service" ]
 cluster: "test-service"
 version: "v2"
 port: 8080
 enabled: true
 cookie:
   name: sticky-cookie
   ttl: 0
   path: /`)
}

func testStatefulSessionWithBG(t *testing.T, statefulSessionConfigV1, statefulSessionConfigV2 string) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	srv1Container := createTraceServiceContainer("test-service", "v1", true)
	defer srv1Container.Purge()
	srv1ContainerV2 := createTraceServiceContainer("test-service", "v2", true)
	defer srv1ContainerV2.Purge()

	internalGateway.RegisterRoutesAndWait(
		assert,
		60*time.Second,
		"v1",
		dto.RouteV3{
			Destination: dto.RouteDestination{Cluster: TestCluster, Endpoint: "test-service-v1:8080"},
			Rules:       []dto.Rule{{Match: dto.RouteMatch{Prefix: "/api/v1/test-service/lb-test"}}},
		},
	)
	internalGateway.RegisterRoutesAndWait(
		assert,
		60*time.Second,
		"v2",
		dto.RouteV3{
			Destination: dto.RouteDestination{Cluster: TestCluster, Endpoint: "test-service-v2:8080"},
			Rules:       []dto.Rule{{Match: dto.RouteMatch{Prefix: "/api/v1/test-service/lb-test"}}},
		},
	)

	internalGateway.ApplyConfigAndWait(assert, 60*time.Second, statefulSessionConfigV1)

	const cookieName = "sticky-cookie-v1"

	runStickyBGRequests(assert, cookieName)

	internalGateway.ApplyConfigAndWait(assert, 60*time.Second, statefulSessionConfigV2)

	runStickyBGRequests(assert, cookieName)

	// cleanup
	_ = deleteVersion(assert, "v2")
	clusters, err := lib.GenericDao.FindAllClusters()
	assert.Nil(err)
	for _, cluster := range clusters {
		if strings.HasPrefix(cluster.Name, "test-service||test-service||") {
			internalGateway.DeleteClusterAndWait(assert, 60*time.Second, cluster.Id, cluster.Name)
		}
	}

	sessions, err := lib.GenericDao.FindAllStatefulSessionConfigs()
	assert.Nil(err)
	assert.Empty(sessions)
}

func runStickyBGRequests(assert *asrt.Assertions, cookieName string) {
	cookie := obtainCookie2(assert, cookieName) // obtains cookie from v1 pod
	log.Infof("Got cookie val: %v", cookie)

	requestToV2 := prepareRequestWithCookieAndHeaders2(assert, "", "", map[string]string{"X-Version": "v2"})
	respFromService, statusCode := SendToTraceSrvWithRetry503(assert, requestToV2)
	assert.Equal(200, statusCode)
	assert.NotNil(respFromService)
	assert.Equal("v2", respFromService.Version)

	requestToV1 := prepareRequestWithCookieAndHeaders2(assert, cookieName, cookie)
	respFromService, statusCode = SendToTraceSrvWithRetry503(assert, requestToV1)
	assert.Equal(200, statusCode)
	assert.NotNil(respFromService)
	assert.Equal("v1", respFromService.Version)

	requestToV2WithCookie := prepareRequestWithCookieAndHeaders2(assert, cookieName, cookie, map[string]string{"X-Version": "v2"})
	respFromService, statusCode = SendToTraceSrvWithRetry503(assert, requestToV2WithCookie)
	assert.Equal(200, statusCode)
	assert.NotNil(respFromService)
	assert.Equal("v2", respFromService.Version)

	requestToV2 = prepareRequestWithCookieAndHeaders2(assert, "", "", map[string]string{"X-Version": "v2"})
	respFromService, statusCode = SendToTraceSrvWithRetry503(assert, requestToV2)
	assert.Equal(200, statusCode)
	assert.NotNil(respFromService)
	assert.Equal("v2", respFromService.Version)
}

func Test_IT_LoadBalance_Cookie(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	srv1Container := createTraceServiceContainer("test-service", "v1", false)
	defer srv1Container.Purge()

	internalGateway.RegisterRoutesAndWait(
		assert,
		60*time.Second,
		"v1",
		dto.RouteV3{
			Destination: dto.RouteDestination{Cluster: TestCluster, Endpoint: "test-service:8080"},
			Rules:       []dto.Rule{{Match: dto.RouteMatch{Prefix: "/api/v1/test-service/lb-test"}}},
		},
		dto.RouteV3{
			Destination: dto.RouteDestination{Cluster: TestCluster, Endpoint: "test-service:8080"},
			Rules:       []dto.Rule{{Match: dto.RouteMatch{Prefix: "/api/v1/test-service/some-other"}}},
		},
	)
	internalGateway.ApplyConfigAndWait(assert, 60*time.Second, `---
kind: StatefulSession
apiVersion: nc.core.mesh/v3
metadata:
 namespace: ''
spec:
 gateways: ["internal-gateway-service"]
 virtualService: ""
 cluster: "test-service"
 #version: "v1"
 #port: 8080
 enabled: true
 cookie:
   name: sticky-cookie
   ttl: 0
   path: /`)

	srv2Container := addTestService(assert, "test-service2", "")
	defer srv2Container.Purge()

	requestsByPodId := make(map[string]int)

	someOtherRequest(assert)
	cookie := obtainCookie(assert, requestsByPodId)
	log.Infof("Got cookie val: %v", cookie)

	run10Requests(assert, requestsByPodId, cookie)

	srv3Container := addTestService(assert, "test-service3", "")
	defer srv3Container.Purge()

	run10Requests(assert, requestsByPodId, cookie)

	srv4Container := addTestService(assert, "test-service4", "")
	defer srv4Container.Purge()

	run10Requests(assert, requestsByPodId, cookie)

	// verify that all requests where sent to the same pod
	assert.Equal(1, len(requestsByPodId))
	for _, reqNum := range requestsByPodId {
		assert.Equal(31, reqNum)
	}

	// cleanup
	clusters, err := lib.GenericDao.FindAllClusters()
	assert.Nil(err)
	for _, cluster := range clusters {
		if strings.HasPrefix(cluster.Name, "test-service||test-service||") {
			internalGateway.DeleteClusterAndWait(assert, 60*time.Second, cluster.Id, cluster.Name)
		}
	}

	sessions, err := lib.GenericDao.FindAllStatefulSessionConfigs()
	assert.Nil(err)
	assert.Empty(sessions)
}

func Test_IT_LoadBalance_Max_Request_per_connection(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	srvContainer := createTraceServiceContainer("test-service", "v1", false)
	defer srvContainer.Purge()

	internalGateway.RegisterRoutesAndWait(
		assert,
		60*time.Second,
		"v1",
		dto.RouteV3{
			Destination: dto.RouteDestination{Cluster: TestCluster, Endpoint: "test-service:8080"},
			Rules:       []dto.Rule{{Match: dto.RouteMatch{Prefix: "/api/v1/test-service/lb-test"}}},
		},
	)
	internalGateway.ApplyConfigAndWaitClusterUpdate(assert, 60*time.Second, `apiVersion: nc.core.mesh/v3
kind: Cluster
spec:
  gateways:
    - internal-gateway-service
  name: test-service||test-service||8080
  endpoints:
    - http://test-service:8080
  maxRequestsPerConnection: 2`)

	requestsByPodId := make(map[string]int)

	//the number of attempts must be a multiple of the maxRequestsPerConnection value
	runRequestsConnectionTest(assert, requestsByPodId, 10)

	assert.NotEqual(1, len(requestsByPodId))
	for _, reqNum := range requestsByPodId {
		assert.Equal(2, reqNum)
	}

	// cleanup
	clusters, err := lib.GenericDao.FindAllClusters()
	assert.Nil(err)
	for _, cluster := range clusters {
		if strings.HasPrefix(cluster.Name, "test-service||test-service||") {
			internalGateway.DeleteClusterAndWait(assert, 60*time.Second, cluster.Id, cluster.Name)
		}
	}
}


func Test_IT_LoadBalance_TCP_Connection_Idle_Timeout(t *testing.T) {
	skipTestIfDockerDisabled(t)
	assert := asrt.New(t)

	srvContainer := createTraceServiceContainer("test-service", "v1", false)
	defer srvContainer.Purge()

	internalGateway.RegisterRoutesAndWait(
		assert,
		60*time.Second,
		"v1",
		dto.RouteV3{
			Destination: dto.RouteDestination{Cluster: TestCluster, Endpoint: "test-service:8080"},
			Rules:       []dto.Rule{{Match: dto.RouteMatch{Prefix: "/api/v1/test-service/lb-test"}}},
		},
	)
	internalGateway.ApplyConfigAndWaitClusterUpdate(assert, 60*time.Second, `apiVersion: nc.core.mesh/v3
kind: Cluster
spec:
  gateways:
    - internal-gateway-service
  name: test-service||test-service||8080
  endpoints:
    - http://test-service:8080
  connectionIdleTimeout: 10`)

	runRequestsHttpConnectionTest(assert, 15 * time.Second)

	// cleanup
	clusters, err := lib.GenericDao.FindAllClusters()
	assert.Nil(err)
	for _, cluster := range clusters {
		if strings.HasPrefix(cluster.Name, "test-service||test-service||") {
			internalGateway.DeleteClusterAndWait(assert, 60*time.Second, cluster.Id, cluster.Name)
		}
	}
}

func runRequestsHttpConnectionTest(assert *asrt.Assertions, waitDuration time.Duration) {
    url := internalGateway.Url+"/api/v1/test-service/lb-test"
    doRequest := func() (connectionID string, err error) {
        var connID string
        req, _ := http.NewRequest("GET", url, nil)

        trace := &httptrace.ClientTrace{
            GotConn: func(info httptrace.GotConnInfo) {
                connID = fmt.Sprintf("%p", info.Conn)
            },
        }
        req = req.WithContext(httptrace.WithClientTrace(context.Background(), trace))
        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            return "", err
        }
        defer resp.Body.Close()

        return connID, nil
    }

    connID1, err := doRequest()
    if err != nil {
        log.Info("First request error:", err)
        return
    }

    time.Sleep(waitDuration)

    connID2, err := doRequest()
    if err != nil {
        fmt.Println("Second request error:", err)
        return
    }

	assert.NotEqual(connID1, connID2)	
}