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
	"pilot/internal/core"
	"pilot/internal/core/handler"
	"pilot/internal/ports"
)

// Adapter providers

func ProvideCommandRunner() *command_runner.OsCommandRunner {
	return command_runner.NewOsCommandRunner()
}

func ProvideGitClient(commandRunner ports.CommandRunner, fileSystem ports.FileSystem) *scm.GitClient {
	return scm.NewGitClient(commandRunner, fileSystem)
}

func ProvideGit(gitClient *scm.GitClient, fileSystem ports.FileSystem) *scm.Git {
	return scm.NewGit(gitClient, fileSystem)
}

func ProvideDockerRepository(
	configRepository ports.ConfigRepository,
	secretsRepository ports.SecretsRepository,
	templater ports.Templater,
	commandRunner ports.CommandRunner,
) *container_image_repository.DockerRepository {
	return container_image_repository.NewDockerRepository(configRepository, secretsRepository, templater, commandRunner)
}

func ProvideHelmClient(commandRunner ports.CommandRunner) *container_orchestrator.HelmClient {
	return container_orchestrator.NewHelmClient(commandRunner)
}

func ProvideKustomizeClient(commandRunner ports.CommandRunner, fileSystem ports.FileSystem) *kustomize.Client {
	return kustomize.NewClient(commandRunner, fileSystem)
}

func ProvideKubernetes(
	configRepository ports.ConfigRepository,
	secretsRepository ports.SecretsRepository,
	templater ports.Templater,
	fileSystem ports.FileSystem,
	helmClient ports.HelmClient,
	kustomizeClient ports.KustomizeClient,
	chartWrapper *core.ChartWrapper,
) (*container_orchestrator.Kubernetes, error) {
	return container_orchestrator.NewKubernetes(configRepository, secretsRepository, templater, fileSystem, helmClient, kustomizeClient, chartWrapper)
}

func ProvideFileSystem() *filesystem.OsFileSystem {
	return filesystem.NewOsFileSystem()
}

func ProvideKeyring() *keyring.ZalandoKeyring {
	return keyring.NewZalandoKeyring("se.henriq.pilot")
}

// LegacyKeyring wraps a ports.Keyring configured for the old "dx" keyring service.
// Used to distinguish from the current keyring in Wire dependency injection.
type LegacyKeyring struct {
	ports.Keyring
}

func ProvideLegacyKeyring() *LegacyKeyring {
	return &LegacyKeyring{Keyring: keyring.NewZalandoKeyring("se.henriq.dx")}
}

func ProvideEncryptor() *symmetric_encryptor.AesGcmEncryptor {
	return symmetric_encryptor.NewAesGcmEncryptor()
}

func ProvideTemplater() *templater.TextTemplater {
	return templater.NewTextTemplater()
}

func ProvideTerminalInput() *terminal.TerminalInput {
	return terminal.NewTerminalInput()
}

func ProvideCertificateAuthority(fileSystem ports.FileSystem, encryptor ports.SymmetricEncryptor) *certificate_authority.X509CertificateAuthority {
	return certificate_authority.NewX509CertificateAuthority(fileSystem, encryptor)
}

func ProvideConfigRepository(
	fileSystem ports.FileSystem,
	secretsRepository ports.SecretsRepository,
	templater ports.Templater,
) *config_repository.FileSystemConfigRepository {
	return config_repository.NewFileSystemConfigRepository(fileSystem, secretsRepository, templater)
}

func ProvideSecretRepository(
	fileSystem ports.FileSystem,
	keyring ports.Keyring,
	encryptor ports.SymmetricEncryptor,
) *secret_repository.EncryptedFileSecretRepository {
	return secret_repository.NewEncryptedFileSecretRepository(fileSystem, keyring, encryptor)
}

// Core providers

func ProvideDevProxyConfigGenerator() *core.DevProxyConfigGenerator {
	return core.NewDevProxyConfigGenerator()
}

func ProvideDevProxyManager(
	configRepository ports.ConfigRepository,
	fileSystem ports.FileSystem,
	containerImageRepository ports.ContainerImageRepository,
	containerOrchestrator ports.ContainerOrchestrator,
	configGenerator *core.DevProxyConfigGenerator,
) *core.DevProxyManager {
	return core.NewDevProxyManager(configRepository, fileSystem, containerImageRepository, containerOrchestrator, configGenerator)
}

func ProvideEnvironmentEnsurer(configRepository ports.ConfigRepository, containerOrchestrator ports.ContainerOrchestrator) core.EnvironmentEnsurer {
	return core.NewEnvironmentEnsurer(configRepository, containerOrchestrator)
}

func ProvideChartWrapper(fileSystem ports.FileSystem) *core.ChartWrapper {
	return core.NewChartWrapper(fileSystem)
}

func ProvideCertificateProvisioner(
	certificateAuthority ports.CertificateAuthority,
	secretStore ports.SecretStore,
	keyring ports.Keyring,
	encryptor ports.SymmetricEncryptor,
) *core.CertificateProvisioner {
	return core.NewCertificateProvisioner(certificateAuthority, secretStore, keyring, encryptor)
}

// Handler providers

func ProvideMigrateCommandHandler(
	legacyKeyring *LegacyKeyring,
	newKeyring ports.Keyring,
	terminalInput ports.TerminalInput,
	fileSystem ports.FileSystem,
) handler.MigrateCommandHandler {
	return handler.NewMigrateCommandHandler(legacyKeyring.Keyring, newKeyring, terminalInput, fileSystem)
}
