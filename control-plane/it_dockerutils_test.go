package main

import (
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	image2 "github.com/docker/docker/api/types/image"
	network2 "github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util"
	"io"
	"net"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

const (
	NetworkName = "testnet"
)

var (
	cm                *ContainerManager
	cli               *client.Client
	pulledImagesCache = map[string]bool{
		"cp-test-service:1.0-SNAPSHOT": true,
	}
)

type ContainerManager struct {
	mutex      *sync.Mutex
	networkID  string
	containers map[string]*ContainerInfo
}

type ContainerInfo struct {
	ID    string
	Name  string
	Ports map[int]int
}

type CreateContainerOpts struct {
	name       string
	image      string
	env        []string
	ports      []nat.Port
	extraHosts []string
	entryPoint []string
	readyCheck func(c *ContainerInfo) error
}

func (cm *ContainerManager) RunContainerWithRetry(opts *CreateContainerOpts) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if _, alreadyStarted := cm.containers[opts.name]; alreadyStarted {
		return
	}

	pullImages(opts.image)
	err := runWithRetry(func() error {
		c, err := runDockerContainer(opts)
		if err == nil {
			if cm.containers == nil {
				cm.containers = map[string]*ContainerInfo{opts.name: c}
			} else {
				cm.containers[opts.name] = c
			}
		}
		return err
	}, func() {
		stopAndRemoveContainer(opts.name)
	}, 5, 500*time.Millisecond)
	if err != nil {
		log.PanicC(ctx, "Error in run container %s after all retries:\n %v", opts.name, err)
	}

	log.InfoC(ctx, "Checking %s readiness", opts.name)
	deadline := time.Now().Add(1 * time.Minute)
	for time.Now().Before(deadline) {
		if err = opts.readyCheck(cm.containers[opts.name]); err == nil {
			break
		}
		log.ErrorC(ctx, "Ready check for %s failed:\n %v", opts.name, err)

		logsStream, errStr := cli.ContainerLogs(ctx, cm.containers[opts.name].ID, container.LogsOptions{ShowStdout: true, ShowStderr: true })
		if errStr != nil {
			log.InfoC(ctx, "Failed to get cont logs:\n %v", errStr)
		
		}
		logs, _ := io.ReadAll(logsStream)
		log.InfoC(ctx, "Container logs:\n %v", string(logs))
		
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		log.PanicC(ctx, "Error in check container %s readiness after all retries:\n %v", opts.name, err)
	}
	log.InfoC(ctx, "Container %s is ready", opts.name)
}

func (cm *ContainerManager) CreateTestNetwork() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	err := runWithRetry(func() error {
		var err error
		cm.networkID, err = createNetwork(NetworkName)
		return err
	}, func() {
		deleteNetwork(NetworkName)
	}, 5, 200*time.Millisecond)
	if err != nil {
		log.PanicC(ctx, "Failed to create test network")
	}
}

func pullImages(images ...string) {
	for _, image := range images {
		pullImage(image)
	}
}

func pullImage(image string) {
	if alreadyPulled := pulledImagesCache[image]; alreadyPulled {
		return
	}
	pull, err := cli.ImagePull(ctx, image, image2.PullOptions{})

	if err != nil {
		log.PanicC(ctx, "Could not pull %s:\n %v", image, err)
	}

	defer pull.Close()

	_, _ = io.ReadAll(pull)
	pulledImagesCache[image] = true
}

func stopAndRemoveContainer(containerName string) {
	log.Info("Stop and remove container %s", containerName)
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		log.ErrorC(ctx, "Error during list container %s:\n %v", containerName, err)
	}

	for _, c := range containers {
		if !util.SliceContains(c.Names, "/"+containerName) {
			continue
		}
		log.Info("Found container %+v", c)
		if err = cli.ContainerStop(ctx, c.ID, container.StopOptions{}); err != nil {
			log.ErrorC(ctx, "Error during stop container %s:\n %v", containerName, err)
		}
		log.Info("Container %s has stopped", containerName)

		if err = cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true}); err != nil {
			log.ErrorC(ctx, "Error during remove container %s:\n %v", containerName, err)
		}
		log.Info("Container %s removed", containerName)
	}
}

func runDockerContainer(opts *CreateContainerOpts) (*ContainerInfo, error) {
	log.InfoC(ctx, "Creating docker container %s", opts.name)

	r := ContainerInfo{Name: opts.name, Ports: map[int]int{}}
	exposedPorts := nat.PortSet{}
	for _, port := range opts.ports {
		exposedPorts[port] = struct{}{}
	}
	portBindings := nat.PortMap{}
	for exposedPort := range exposedPorts {
		hostPort := findFreePort()
		r.Ports[exposedPort.Int()] = hostPort
		portBindings[exposedPort] = []nat.PortBinding{{HostPort: fmt.Sprintf("%v", hostPort)}}
	}
	log.InfoC(ctx, "Container %s, port bindings: %+v", opts.name, portBindings)
	createResponse, err := cli.ContainerCreate(ctx, &container.Config{
		Hostname:     opts.name,
		Env:          opts.env,
		Image:        opts.image,
		ExposedPorts: exposedPorts,
		Entrypoint:   opts.entryPoint,
	},
		&container.HostConfig{
			PortBindings: portBindings,
			ExtraHosts:   opts.extraHosts,
		}, &network2.NetworkingConfig{EndpointsConfig: map[string]*network2.EndpointSettings{NetworkName: {
			Aliases: []string{opts.name},
			//NetworkID:           cm.networkID,
		}}}, nil, opts.name)
	if err != nil {
		log.ErrorC(ctx, "client.CreateContainer %s err:\n %v", opts.name, err)
		return nil, err
	}

	if err = cli.ContainerStart(ctx, createResponse.ID, container.StartOptions{}); err != nil {
		log.ErrorC(ctx, "Starting container %s error:\n %v", opts.name, err)
		return nil, err
	}

	log.InfoC(ctx, "Container %s (%s) has started successfully", createResponse.ID, opts.name)

	r.ID = createResponse.ID
	return &r, nil
}

func createNetwork(networkName string) (string, error) {
	nt, err := cli.NetworkCreate(ctx, networkName, network2.CreateOptions{})
	if err != nil {
		log.ErrorC(ctx, "client.NetworkCreate err:\n %v", err)
		return "", err
	}
	return nt.ID, nil
}

func deleteNetwork(networkName string) {
	networks, err := cli.NetworkList(ctx, types.NetworkListOptions{
		Filters: filters.NewArgs(filters.Arg("name", networkName)),
	})
	if err != nil {
		log.ErrorC(ctx, "client.NetworkList err:\n %v", err)
		return
	}

	for _, nt := range networks {
		if err = cli.NetworkRemove(ctx, nt.ID); err != nil {
			log.ErrorC(ctx, "client.NetworkRemove err:\n %v", err)
		}
	}
}

func runWithRetry(runOperation func() error, recoverAttempt func(), attempts int, delay time.Duration) error {
	err := runOperation()
	if err != nil {
		log.ErrorC(ctx, "Operation attempt err:\n %v", err)

		for i := 1; i < attempts; i++ {
			recoverAttempt()

			time.Sleep(delay)

			if err = runOperation(); err != nil {
				log.ErrorC(ctx, "Operation attempt err:\n %v", err)
			}
		}
	}
	return err
}

func findFreePort() int {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return port
}
