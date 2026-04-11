//go:build wireinject
// +build wireinject

package app

import (
	"pilot/internal/adapters/certificate_authority"
	"pilot/internal/adapters/command_runner"
	"pilot/internal/adapters/config_repository"
	"pilot/internal/adapters/container_image_repository"
	"pilot/internal/adapters/container_orchestrator"
	"pilot/internal/adapters/filesystem"
	"pilot/internal/adapters/keyring"
	"pilot/internal/adapters/kustomize"
	"pilot/internal/adapters/scm"
	"pilot/internal/adapters/secret_repository"
	"pilot/internal/adapters/symmetric_encryptor"
	"pilot/internal/adapters/templater"
	"pilot/internal/adapters/terminal"
	"pilot/internal/core/handler"
	"pilot/internal/ports"

	"github.com/google/wire"
)

// AdapterSet wires adapter providers (from providers.go) to port interfaces.
var AdapterSet = wire.NewSet(
	ProvideCommandRunner,
	wire.Bind(new(ports.CommandRunner), new(*command_runner.OsCommandRunner)),
	ProvideGitClient,
	ProvideGit,
	wire.Bind(new(ports.Scm), new(*scm.Git)),
	ProvideDockerRepository,
	wire.Bind(new(ports.ContainerImageRepository), new(*container_image_repository.DockerRepository)),
	ProvideHelmClient,
	wire.Bind(new(ports.HelmClient), new(*container_orchestrator.HelmClient)),
	ProvideKustomizeClient,
	wire.Bind(new(ports.KustomizeClient), new(*kustomize.Client)),
	ProvideKubernetes,
	wire.Bind(new(ports.ContainerOrchestrator), new(*container_orchestrator.Kubernetes)),
	wire.Bind(new(ports.SecretStore), new(*container_orchestrator.Kubernetes)),
	ProvideFileSystem,
	wire.Bind(new(ports.FileSystem), new(*filesystem.OsFileSystem)),
	ProvideKeyring,
	wire.Bind(new(ports.Keyring), new(*keyring.ZalandoKeyring)),
	ProvideEncryptor,
	wire.Bind(new(ports.SymmetricEncryptor), new(*symmetric_encryptor.AesGcmEncryptor)),
	ProvideTemplater,
	wire.Bind(new(ports.Templater), new(*templater.TextTemplater)),
	ProvideTerminalInput,
	wire.Bind(new(ports.TerminalInput), new(*terminal.TerminalInput)),
	ProvideCertificateAuthority,
	wire.Bind(new(ports.CertificateAuthority), new(*certificate_authority.X509CertificateAuthority)),
	ProvideConfigRepository,
	wire.Bind(new(ports.ConfigRepository), new(*config_repository.FileSystemConfigRepository)),
	ProvideSecretRepository,
	wire.Bind(new(ports.SecretsRepository), new(*secret_repository.EncryptedFileSecretRepository)),
)

// CoreSet wires core domain providers (from providers.go).
var CoreSet = wire.NewSet(
	ProvideDevProxyConfigGenerator,
	ProvideDevProxyManager,
	ProvideEnvironmentEnsurer,
	ProvideChartWrapper,
	ProvideCertificateProvisioner,
)

// CommandHandlerSet combines all sets needed for command handlers.
var CommandHandlerSet = wire.NewSet(
	AdapterSet,
	CoreSet,
)

func InjectConfigRepo() (ports.ConfigRepository, error) {
	wire.Build(AdapterSet)
	return &config_repository.FileSystemConfigRepository{}, nil
}

func InjectSecretRepository() (ports.SecretsRepository, error) {
	wire.Build(AdapterSet)
	return &secret_repository.EncryptedFileSecretRepository{}, nil
}

func InjectBuildCommandHandler() (handler.BuildCommandHandler, error) {
	wire.Build(CommandHandlerSet, handler.NewBuildCommandHandler)
	return handler.BuildCommandHandler{}, nil
}

func InjectInstallCommandHandler() (handler.InstallCommandHandler, error) {
	wire.Build(CommandHandlerSet, handler.NewInstallCommandHandler)
	return handler.InstallCommandHandler{}, nil
}

func InjectUninstallCommandHandler() (handler.UninstallCommandHandler, error) {
	wire.Build(CommandHandlerSet, handler.NewUninstallCommandHandler)
	return handler.UninstallCommandHandler{}, nil
}

func InjectGenEnvKeyCommandHandler() (handler.GenEnvKeyCommandHandler, error) {
	wire.Build(CommandHandlerSet, handler.NewGenEnvKeyCommandHandler)
	return handler.GenEnvKeyCommandHandler{}, nil
}

func InjectContextCommandHandler() (handler.ContextCommandHandler, error) {
	wire.Build(CommandHandlerSet, handler.NewContextCommandHandler)
	return handler.ContextCommandHandler{}, nil
}

func InjectInitializeCommandHandler() (handler.InitializeCommandHandler, error) {
	wire.Build(CommandHandlerSet, handler.NewInitializeCommandHandler)
	return handler.InitializeCommandHandler{}, nil
}

func InjectSecretCommandHandler() (handler.SecretCommandHandler, error) {
	wire.Build(CommandHandlerSet, handler.NewSecretCommandHandler)
	return handler.SecretCommandHandler{}, nil
}

func InjectRunCommandHandler() (handler.RunCommandHandler, error) {
	wire.Build(CommandHandlerSet, handler.NewRunCommandHandler)
	return handler.RunCommandHandler{}, nil
}

func InjectShowVarsCommandHandler() (handler.ShowVarsCommandHandler, error) {
	wire.Build(CommandHandlerSet, handler.NewShowVarsCommandHandler)
	return handler.ShowVarsCommandHandler{}, nil
}

func InjectGenerateCommandHandler() (handler.GenerateCommandHandler, error) {
	wire.Build(CommandHandlerSet, handler.NewGenerateCommandHandler)
	return handler.GenerateCommandHandler{}, nil
}

func InjectPullCommandHandler() (handler.PullCommandHandler, error) {
	wire.Build(CommandHandlerSet, handler.NewPullCommandHandler)
	return handler.PullCommandHandler{}, nil
}

func InjectMigrateCommandHandler() (handler.MigrateCommandHandler, error) {
	wire.Build(CommandHandlerSet, ProvideLegacyKeyring, ProvideMigrateCommandHandler)
	return handler.MigrateCommandHandler{}, nil
}

func InjectCACommandHandler() (handler.CACommandHandler, error) {
	wire.Build(CommandHandlerSet, handler.NewCACommandHandler)
	return handler.CACommandHandler{}, nil
}
