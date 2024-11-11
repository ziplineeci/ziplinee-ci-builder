package builder

import (
	"context"

	contracts "github.com/ziplineeci/ziplinee-ci-contracts"
	manifest "github.com/ziplineeci/ziplinee-ci-manifest"
)

// ContainerRunner allows containers to be started
//
//go:generate mockgen -package=builder -destination ./container_runner_mock.go -source=container_runner.go
type ContainerRunner interface {
	IsImagePulled(ctx context.Context, stageName string, containerImage string) bool
	IsTrustedImage(stageName string, containerImage string) bool
	HasInjectedCredentials(stageName string, containerImage string) bool
	PullImage(ctx context.Context, stageName, parentStageName string, containerImage string) error
	GetImageSize(ctx context.Context, containerImage string) (int64, error)
	StartStageContainer(ctx context.Context, depth int, dir string, envvars map[string]string, stage manifest.ZiplineeStage, stageIndex int) (containerID string, err error)
	StartServiceContainer(ctx context.Context, envvars map[string]string, service manifest.ZiplineeService) (containerID string, err error)
	RunReadinessProbeContainer(ctx context.Context, parentStage manifest.ZiplineeStage, service manifest.ZiplineeService, readiness manifest.ReadinessProbe) (err error)
	TailContainerLogs(ctx context.Context, containerID, parentStageName, stageName string, stageType contracts.LogType, depth int, multiStage *bool) (err error)
	StopSingleStageServiceContainers(ctx context.Context, parentStage manifest.ZiplineeStage)
	StopMultiStageServiceContainers(ctx context.Context)
	StartDockerDaemon() error
	WaitForDockerDaemon()
	CreateDockerClient() error
	CreateNetworks(ctx context.Context) error
	DeleteNetworks(ctx context.Context) error
	StopAllContainers(ctx context.Context)
	Info(ctx context.Context) string
}
