// nolint
package builder

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog/log"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	contracts "github.com/ziplineeci/ziplinee-ci-contracts"
	manifest "github.com/ziplineeci/ziplinee-ci-manifest"
	foundation "github.com/ziplineeci/ziplinee-foundation"
)

// CIBuilder runs builds for different types of integrations
type CIBuilder interface {
	RunReadinessProbe(ctx context.Context, scheme, host string, port int, path, hostname string, timeoutSeconds int)
	RunEstafetteBuildJob(ctx context.Context, pipelineRunner PipelineRunner, containerRunner ContainerRunner, envvarHelper EnvvarHelper, obfuscator Obfuscator, endOfLifeHelper EndOfLifeHelper, builderConfig contracts.BuilderConfig, credentialsBytes []byte, runAsJob bool)
	RunLocalBuild(ctx context.Context, pipelineRunner PipelineRunner, containerRunner ContainerRunner, envvarHelper EnvvarHelper, builderConfig contracts.BuilderConfig, stagesToRun []string) (err error)
	RunGocdAgentBuild(ctx context.Context, pipelineRunner PipelineRunner, containerRunner ContainerRunner, envvarHelper EnvvarHelper, obfuscator Obfuscator, builderConfig contracts.BuilderConfig, credentialsBytes []byte)
	RunEstafetteCLIBuild() error
}

type ciBuilder struct {
	applicationInfo foundation.ApplicationInfo
}

// NewCIBuilder returns a new CIBuilder
func NewCIBuilder(applicationInfo foundation.ApplicationInfo) CIBuilder {
	return &ciBuilder{
		applicationInfo: applicationInfo,
	}
}

func (b *ciBuilder) RunReadinessProbe(ctx context.Context, scheme, host string, port int, path, hostname string, timeoutSeconds int) {
	err := WaitForReadinessHttpGet(ctx, scheme, host, port, path, hostname, timeoutSeconds)
	if err != nil {
		log.Fatal().Err(err).Msgf("Readiness probe failed")
	}

	// readiness probe succeeded, exiting cleanly
	os.Exit(0)
}

func (b *ciBuilder) RunEstafetteBuildJob(ctx context.Context, pipelineRunner PipelineRunner, containerRunner ContainerRunner, envvarHelper EnvvarHelper, obfuscator Obfuscator, endOfLifeHelper EndOfLifeHelper, builderConfig contracts.BuilderConfig, credentialsBytes []byte, runAsJob bool) {

	closer := b.initJaeger(b.applicationInfo.App)
	defer closer.Close()

	buildLog := contracts.BuildLog{
		RepoSource:   builderConfig.Git.RepoSource,
		RepoOwner:    builderConfig.Git.RepoOwner,
		RepoName:     builderConfig.Git.RepoName,
		RepoBranch:   builderConfig.Git.RepoBranch,
		RepoRevision: builderConfig.Git.RepoRevision,
		Steps:        make([]*contracts.BuildLogStep, 0),
	}

	rootSpanName := "RunBuildJob"
	if builderConfig.JobType == contracts.JobTypeRelease {
		rootSpanName = "RunReleaseJob"
	} else if builderConfig.JobType == contracts.JobTypeBot {
		rootSpanName = "RunBotJob"
	}

	rootSpan := opentracing.StartSpan(rootSpanName)
	defer rootSpan.Finish()

	ctx = opentracing.ContextWithSpan(ctx, rootSpan)

	// set running state, so a restarted job will show up as running once a new pod runs
	_ = endOfLifeHelper.SendBuildStartedEvent(ctx)

	go func() {
		// cancel 15 minutes before jwt expires
		expiryTime := builderConfig.CIServer.JWTExpiry
		expiryTime.Add(time.Duration(-15) * time.Minute)
		expiryDuration := expiryTime.Sub(time.Now().UTC())
		cancelTimer := time.NewTimer(expiryDuration)

		// wait for timer to fire
		<-cancelTimer.C

		log.Warn().Msgf("Canceling job at %v, before the JWT expires at %v", time.Now().UTC(), builderConfig.CIServer.JWTExpiry)

		err := endOfLifeHelper.CancelJob(ctx)
		if err != nil {
			log.Error().Err(err).Msg("Canceling job failed")
		}
	}()

	// unset all ESTAFETTE_ envvars so they don't get abused by non-estafette components
	envvarHelper.UnsetEstafetteEnvvars()

	err := envvarHelper.SetEstafetteBuilderConfigEnvvars(builderConfig)
	if err != nil {
		endOfLifeHelper.HandleFatal(ctx, buildLog, err, "Error setting estafette builder config envvars")
	}

	if os.Getenv("ESTAFETTE_LOG_FORMAT") == "v3" {
		// set some default fields added to all logs
		log.Logger = log.Logger.With().
			Str("jobName", *builderConfig.JobName).
			Interface("git", builderConfig.Git).
			Logger()
	}

	// start docker daemon
	dockerDaemonStartSpan, _ := opentracing.StartSpanFromContext(ctx, "StartDockerDaemon")
	err = containerRunner.StartDockerDaemon()
	if err != nil {
		endOfLifeHelper.HandleFatal(ctx, buildLog, err, "Error starting docker daemon")
	}

	// wait for docker daemon to be ready for usage
	containerRunner.WaitForDockerDaemon()
	dockerDaemonStartSpan.Finish()

	// listen to cancellation in order to stop any running pipeline or container
	go pipelineRunner.StopPipelineOnCancellation(ctx)

	// get current working directory
	dir := envvarHelper.GetWorkDir()
	if dir == "" {
		endOfLifeHelper.HandleFatal(ctx, buildLog, nil, "Getting working directory from environment variable ESTAFETTE_WORKDIR failed")
	}

	// set some envvars
	err = envvarHelper.SetEstafetteGlobalEnvvars()
	if err != nil {
		endOfLifeHelper.HandleFatal(ctx, buildLog, err, "Setting global environment variables failed")
	}

	// initialize obfuscator
	err = obfuscator.CollectSecrets(*builderConfig.Manifest, credentialsBytes, envvarHelper.GetPipelineName())
	if err != nil {
		endOfLifeHelper.HandleFatal(ctx, buildLog, err, "Collecting secrets to obfuscate failed")
	}

	stages := builderConfig.Stages

	// check whether this is a regular build or a release
	switch builderConfig.JobType {
	case contracts.JobTypeBuild:
		log.Info().Msgf("Starting build version %v...", builderConfig.Version.Version)

	case contracts.JobTypeRelease:
		log.Info().Msgf("Starting release %v at version %v...", builderConfig.Release.Name, builderConfig.Version.Version)

	case contracts.JobTypeBot:
		log.Info().Msgf("Starting bot %v...", builderConfig.Bot.Name)
	}

	if builderConfig.Manifest != nil || builderConfig.Manifest.Builder.BuilderType != manifest.BuilderTypeKubernetes {
		// create docker client
		err = containerRunner.CreateDockerClient()
		if err != nil {
			endOfLifeHelper.HandleFatal(ctx, buildLog, err, "Failed creating a docker client")
		}
	}

	// collect estafette envvars and run stages from manifest
	log.Info().Msgf("Running %v stages", len(stages))
	estafetteEnvvars, err := envvarHelper.CollectEstafetteEnvvarsAndLabels(*builderConfig.Manifest)
	if err != nil {
		endOfLifeHelper.HandleFatal(ctx, buildLog, err, "CollectEstafetteEnvvarsAndLabels failed")
	}

	globalEnvvars := envvarHelper.CollectGlobalEnvvars(*builderConfig.Manifest)
	envvars := envvarHelper.OverrideEnvvars(estafetteEnvvars, globalEnvvars)

	// run stages
	pipelineRunner.EnableBuilderInfoStageInjection()
	buildLog.Steps, err = pipelineRunner.RunStages(ctx, 0, stages, dir, envvars)
	if err != nil && buildLog.HasUnknownStatus() {
		endOfLifeHelper.HandleFatal(ctx, buildLog, err, "Executing stages from manifest failed")
	}

	// send result to ci-api
	buildStatus := contracts.GetAggregatedStatus(buildLog.Steps)
	_ = endOfLifeHelper.SendBuildFinishedEvent(ctx, buildStatus)
	_ = endOfLifeHelper.SendBuildJobLogEvent(ctx, buildLog)
	_ = endOfLifeHelper.SendBuildCleanEvent(ctx, buildStatus)

	// finish and flush so it gets sent to the tracing backend
	rootSpan.Finish()
	closer.Close()

	if runAsJob {
		os.Exit(0)
	} else {
		HandleExit(buildLog.Steps)
	}
}

func (b *ciBuilder) RunLocalBuild(ctx context.Context, pipelineRunner PipelineRunner, containerRunner ContainerRunner, envvarHelper EnvvarHelper, builderConfig contracts.BuilderConfig, stagesToRun []string) (err error) {

	// create docker client
	err = containerRunner.CreateDockerClient()
	if err != nil {
		return
	}

	// read yaml
	mft, err := manifest.ReadManifestFromFile(manifest.GetDefaultManifestPreferences(), ".estafette.yaml", true)
	if err != nil {
		return
	}

	// select configured stages to run
	stages := []*manifest.EstafetteStage{}
	stageNames := []string{}
	for _, s := range mft.Stages {
		stageNames = append(stageNames, s.Name)
		if foundation.StringArrayContains(stagesToRun, s.Name) {
			stages = append(stages, s)
		}
	}

	if len(stages) == 0 {
		return fmt.Errorf("Choose one of the following stages: %v", strings.Join(stageNames, ","))
	}

	// get current working directory
	dir, err := os.Getwd()
	if err != nil {
		return
	}

	// unset all ESTAFETTE_ envvars so they don't get abused by non-estafette components
	envvarHelper.UnsetEstafetteEnvvars()

	// ensure git variables are set
	err = envvarHelper.SetPipelineName(builderConfig)
	if err != nil {
		return
	}

	err = envvarHelper.SetEstafetteGlobalEnvvars()
	if err != nil {
		return
	}

	// collect estafette and 'global' envvars from manifest
	estafetteEnvvars, err := envvarHelper.CollectEstafetteEnvvarsAndLabels(mft)
	if err != nil {
		return
	}

	globalEnvvars := envvarHelper.CollectGlobalEnvvars(mft)

	// merge estafette and global envvars
	envvars := envvarHelper.OverrideEnvvars(estafetteEnvvars, globalEnvvars)

	// listen to cancellation in order to stop any running pipeline or container
	go pipelineRunner.StopPipelineOnCancellation(ctx)

	// run stages
	buildLogSteps, err := pipelineRunner.RunStages(ctx, 0, stages, dir, envvars)
	if err != nil {
		return
	}

	if !contracts.HasSucceededStatus(buildLogSteps) {
		return fmt.Errorf("Failed running stages")
	}

	return nil
}

func (b *ciBuilder) RunGocdAgentBuild(ctx context.Context, pipelineRunner PipelineRunner, containerRunner ContainerRunner, envvarHelper EnvvarHelper, obfuscator Obfuscator, builderConfig contracts.BuilderConfig, credentialsBytes []byte) {

	fatalHandler := NewLocalFatalHandler()

	// create docker client
	err := containerRunner.CreateDockerClient()
	if err != nil {
		fatalHandler.HandleFatal(err, "Failed creating a docker client")
	}

	// read yaml
	manifest, err := manifest.ReadManifestFromFile(builderConfig.ManifestPreferences, ".estafette.yaml", true)
	if err != nil {
		fatalHandler.HandleFatal(err, "Reading .estafette.yaml manifest failed")
	}

	// initialize obfuscator
	err = obfuscator.CollectSecrets(manifest, credentialsBytes, envvarHelper.GetPipelineName())
	if err != nil {
		fatalHandler.HandleFatal(err, "Collecting secrets to obfuscate failed")
	}

	// get current working directory
	dir, err := os.Getwd()
	if err != nil {
		fatalHandler.HandleFatal(err, "Getting current working directory failed")
	}

	// check whether this is a regular build or a release
	stages := manifest.Stages
	releaseName := os.Getenv("ESTAFETTE_RELEASE_NAME")
	buildVersion := os.Getenv("ESTAFETTE_BUILD_VERSION")
	if releaseName != "" {
		// check if the release is defined
		releaseExists := false
		for _, r := range manifest.Releases {
			if r.Name == releaseName {
				releaseExists = true
				stages = r.Stages
			}
		}
		if !releaseExists {
			fatalHandler.HandleFatal(fmt.Errorf("Release %v does not exist", releaseName), "")
		}
		log.Info().Msgf("Starting release %v at version %v...", releaseName, buildVersion)
	} else {
		log.Info().Msgf("Starting build version %v...", buildVersion)
	}

	log.Info().Msgf("Running %v stages", len(stages))

	err = envvarHelper.SetEstafetteGlobalEnvvars()
	if err != nil {
		fatalHandler.HandleFatal(err, "Setting global environment variables failed")
	}

	// collect estafette and 'global' envvars from manifest
	estafetteEnvvars, err := envvarHelper.CollectEstafetteEnvvarsAndLabels(manifest)
	if err != nil {
		fatalHandler.HandleFatal(err, "CollectEstafetteEnvvarsAndLabels failed")
	}

	globalEnvvars := envvarHelper.CollectGlobalEnvvars(manifest)

	// merge estafette and global envvars
	envvars := envvarHelper.OverrideEnvvars(estafetteEnvvars, globalEnvvars)

	// run stages
	buildLogSteps, err := pipelineRunner.RunStages(ctx, 0, stages, dir, envvars)
	if err != nil {
		fatalHandler.HandleFatal(err, "Executing stages from manifest failed")
	}

	RenderStats(buildLogSteps)

	HandleExit(buildLogSteps)
}

func (b *ciBuilder) RunEstafetteCLIBuild() error {
	return nil
}

// initJaeger returns an instance of Jaeger Tracer that can be configured with environment variables
// https://github.com/jaegertracing/jaeger-client-go#environment-variables
func (b *ciBuilder) initJaeger(service string) io.Closer {

	cfg, err := jaegercfg.FromEnv()
	if err != nil {
		log.Fatal().Err(err).Msg("Generating Jaeger config from environment variables failed")
	}

	// disable jaeger if service name is empty
	if cfg.ServiceName == "" {
		cfg.Disabled = true
	}

	closer, err := cfg.InitGlobalTracer(service, jaegercfg.Logger(jaeger.StdLogger))

	if err != nil {
		log.Fatal().Err(err).Msg("Generating Jaeger tracer failed")
	}

	return closer
}
