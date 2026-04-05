//go:build wireinject
// +build wireinject

package app

import (
	"dx/internal/adapters/certificate_authority"
	"dx/internal/adapters/command_runner"
	"dx/internal/adapters/container_image_repository"
	"dx/internal/adapters/container_orchestrator"
	"dx/internal/adapters/filesystem"
	"dx/internal/adapters/keyring"
	"dx/internal/adapters/kustomize"
	"dx/internal/adapters/scm"
	"dx/internal/adapters/symmetric_encryptor"
	"dx/internal/adapters/templater"
	"dx/internal/adapters/terminal"
	"dx/internal/core"
	"dx/internal/core/handler"
	"dx/internal/ports"

	"github.com/google/wire"
)

var Adapter = wire.NewSet(
	command_runner.ProvideOsCommandRunner,
	wire.Bind(new(ports.CommandRunner), new(*command_runner.OsCommandRunner)),
	scm.ProvideGitClient,
	scm.ProvideGit,
	wire.Bind(new(ports.Scm), new(*scm.Git)),
	container_image_repository.ProvideDockerRepository,
	wire.Bind(new(ports.ContainerImageRepository), new(*container_image_repository.DockerRepository)),
	container_orchestrator.ProvideHelmClient,
	wire.Bind(new(ports.HelmClient), new(*container_orchestrator.HelmClient)),
	kustomize.ProvideKustomizeClient,
	wire.Bind(new(ports.KustomizeClient), new(*kustomize.Client)),
	container_orchestrator.ProvideKubernetes,
	wire.Bind(new(ports.ContainerOrchestrator), new(*container_orchestrator.Kubernetes)),
	wire.Bind(new(ports.SecretStore), new(*container_orchestrator.Kubernetes)),
	filesystem.ProvideOsFileSystem,
	wire.Bind(new(ports.FileSystem), new(*filesystem.OsFileSystem)),
	keyring.ProvideZalandoKeyring,
	symmetric_encryptor.ProvideAesGcmEncryptor,
	wire.Bind(new(ports.SymmetricEncryptor), new(*symmetric_encryptor.AesGcmEncryptor)),
	templater.ProvideTextTemplater,
	terminal.ProvideTerminalInput,
	wire.Bind(new(ports.TerminalInput), new(*terminal.TerminalInput)),
	certificate_authority.ProvideX509CertificateAuthority,
	wire.Bind(new(ports.CertificateAuthority), new(*certificate_authority.X509CertificateAuthority)),
)

// CoreSet provides domain/core dependencies
var CoreSet = wire.NewSet(
	core.ProvideFileSystemConfigRepository,
	wire.Bind(new(core.ConfigRepository), new(*core.FileSystemConfigRepository)),
	core.ProvideDevProxyConfigGenerator,
	core.ProvideDevProxyManager,
	core.ProvideEncryptedFileSecretRepository,
	core.ProvideEnvironmentEnsurer,
	core.ProvideChartWrapper,
	core.ProvideCertificateProvisioner,
)

// CommandHandlerSet combines all sets needed for command handlers
var CommandHandlerSet = wire.NewSet(
	Adapter,
	CoreSet,
)

func InjectConfigRepo() (core.ConfigRepository, error) {
	wire.Build(
		Adapter,
		core.ProvideFileSystemConfigRepository,
		wire.Bind(new(core.ConfigRepository), new(*core.FileSystemConfigRepository)),
		core.ProvideEncryptedFileSecretRepository,
	)
	return &core.FileSystemConfigRepository{}, nil
}

func InjectSecretRepository() (core.SecretsRepository, error) {
	wire.Build(
		Adapter,
		CoreSet,
	)
	return &core.EncryptedFileSecretRepository{}, nil
}

func InjectBuildCommandHandler() (handler.BuildCommandHandler, error) {
	wire.Build(
		CommandHandlerSet,
		handler.ProvideBuildCommandHandler,
	)
	return handler.BuildCommandHandler{}, nil
}

func InjectInstallCommandHandler() (handler.InstallCommandHandler, error) {
	wire.Build(
		CommandHandlerSet,
		handler.ProvideInstallCommandHandler,
	)
	return handler.InstallCommandHandler{}, nil
}

func InjectUninstallCommandHandler() (handler.UninstallCommandHandler, error) {
	wire.Build(
		CommandHandlerSet,
		handler.ProvideUninstallCommandHandler,
	)
	return handler.UninstallCommandHandler{}, nil
}

func InjectGenEnvKeyCommandHandler() (handler.GenEnvKeyCommandHandler, error) {
	wire.Build(
		CommandHandlerSet,
		handler.ProvideGenEnvKeyCommandHandler,
	)
	return handler.GenEnvKeyCommandHandler{}, nil
}

func InjectContextCommandHandler() (handler.ContextCommandHandler, error) {
	wire.Build(
		CommandHandlerSet,
		handler.ProvideContextCommandHandler,
	)
	return handler.ContextCommandHandler{}, nil
}

func InjectInitializeCommandHandler() (handler.InitializeCommandHandler, error) {
	wire.Build(
		CommandHandlerSet,
		handler.ProvideInitializeCommandHandler,
	)
	return handler.InitializeCommandHandler{}, nil
}

func InjectSecretCommandHandler() (handler.SecretCommandHandler, error) {
	wire.Build(
		CommandHandlerSet,
		handler.ProvideSecretCommandHandler,
	)
	return handler.SecretCommandHandler{}, nil
}

func InjectRunCommandHandler() (handler.RunCommandHandler, error) {
	wire.Build(
		CommandHandlerSet,
		handler.ProvideRunCommandHandler,
	)
	return handler.RunCommandHandler{}, nil
}

func InjectShowVarsCommandHandler() (handler.ShowVarsCommandHandler, error) {
	wire.Build(
		CommandHandlerSet,
		handler.ProvideShowVarsCommandHandler,
	)
	return handler.ShowVarsCommandHandler{}, nil
}

func InjectGenerateCommandHandler() (handler.GenerateCommandHandler, error) {
	wire.Build(
		CommandHandlerSet,
		handler.ProvideGenerateCommandHandler,
	)
	return handler.GenerateCommandHandler{}, nil
}

func InjectPullCommandHandler() (handler.PullCommandHandler, error) {
	wire.Build(
		CommandHandlerSet,
		handler.ProvidePullCommandHandler,
	)
	return handler.PullCommandHandler{}, nil
}

func InjectCACommandHandler() (handler.CACommandHandler, error) {
	wire.Build(
		CommandHandlerSet,
		handler.ProvideCACommandHandler,
	)
	return handler.CACommandHandler{}, nil
}
