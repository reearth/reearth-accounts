package e2e

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type cerbosContainer struct {
	container testcontainers.Container
	host      string
	port      string
}

func newCerbosContainer() (*cerbosContainer, error) {
	ctx := context.Background()

	policiesDir, err := filepath.Abs("testdata/policies")
	if err != nil {
		return nil, err
	}

	req := testcontainers.ContainerRequest{
		Image:        "cerbos/cerbos:0.40.0",
		ExposedPorts: []string{"3593/tcp"},
		WaitingFor:   wait.ForListeningPort("3593/tcp"),
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      policiesDir,
				ContainerFilePath: "/policies",
				FileMode:          0444,
			},
		},
		Cmd: []string{"server", "--config", "/policies/.cerbos.yaml"},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get container host: %v", err)
	}

	mappedPort, err := container.MappedPort(ctx, "3593")
	if err != nil {
		return nil, fmt.Errorf("failed to get container port: %v", err)
	}

	return &cerbosContainer{
		container: container,
		host:      host,
		port:      mappedPort.Port(),
	}, nil
}

func (c *cerbosContainer) getAddress() string {
	return fmt.Sprintf("%s:%s", c.host, c.port)
}

func (c *cerbosContainer) terminate() error {
	return c.container.Terminate(context.Background())
}
