package container_orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dx/internal/core"
	"dx/internal/core/domain"
	"dx/internal/ports"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var _ ports.ContainerOrchestrator = (*Kubernetes)(nil)

// Kubernetes represents a client for interacting with Kubernetes
type Kubernetes struct {
	configRepository  core.ConfigRepository
	secretsRepository core.SecretsRepository
	templater         ports.Templater
	clientSet         *kubernetes.Clientset
	helmClient        ports.HelmClient
	kustomizeClient   ports.KustomizeClient
	chartWrapper      *core.ChartWrapper
	fileService       ports.FileSystem
}

func ProvideKubernetes(
	configRepository core.ConfigRepository,
	secretsRepository core.SecretsRepository,
	templater ports.Templater,
	fileService ports.FileSystem,
	helmClient ports.HelmClient,
	kustomizeClient ports.KustomizeClient,
	chartWrapper *core.ChartWrapper,
) (*Kubernetes, error) {
	// Try to load the kubeConfig from the default location
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %v", err)
	}
	kubeConfigPath := filepath.Join(home, ".kube", "config")

	// Create the config from the kubeConfig file
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes config: %v", err)
	}

	// Create the clientSet
	clientSet, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	return &Kubernetes{
		configRepository:  configRepository,
		secretsRepository: secretsRepository,
		templater:         templater,
		clientSet:         clientSet,
		helmClient:        helmClient,
		kustomizeClient:   kustomizeClient,
		chartWrapper:      chartWrapper,
		fileService:       fileService,
	}, nil
}

// CreateClusterEnvironmentKey creates a string that is used to uniquely identify the cluster and namespace
func (k *Kubernetes) CreateClusterEnvironmentKey() (string, error) {
	// Get cluster ID from kube-system namespace UID
	kubeSystemNS, err := k.clientSet.CoreV1().Namespaces().Get(context.Background(), "kube-system", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get kube-system namespace: %v", err)
	}
	clusterUID := string(kubeSystemNS.UID)

	// Get the current namespace from context
	namespace := ""
	config, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err == nil && config.CurrentContext != "" {
		if currentContext, ok := config.Contexts[config.CurrentContext]; ok && currentContext.Namespace != "" {
			namespace = currentContext.Namespace
		}
	}

	// Fail if no namespace is set
	if namespace == "" {
		return "", fmt.Errorf("no namespace set in current context")
	}

	// Create a deterministic key based only on cluster UID and namespace
	key := fmt.Sprintf("%s-%s", clusterUID, namespace)

	hash := sha256.New()
	hash.Write([]byte(key))
	return base64.URLEncoding.EncodeToString(hash.Sum(nil)), nil
}

// getCurrentNamespace returns the namespace from the current kubeconfig context.
func (k *Kubernetes) getCurrentNamespace() string {
	config, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		return "default"
	}
	if config.CurrentContext == "" {
		return "default"
	}
	if currentContext, ok := config.Contexts[config.CurrentContext]; ok && currentContext.Namespace != "" {
		return currentContext.Namespace
	}
	return "default"
}

// InstallService installs a service using helm with kustomize patches.
func (k *Kubernetes) InstallService(service *domain.Service) error {
	templateValues, err := core.CreateTemplatingValues(k.configRepository, k.secretsRepository)
	if err != nil {
		return err
	}

	var renderedArgs []string
	for i, arg := range service.HelmArgs {
		renderedArg, err := k.templater.Render(arg, fmt.Sprintf("helm-args.%d", i), templateValues)
		if err != nil {
			return err
		}
		renderedArgs = append(renderedArgs, renderedArg)
	}

	// Validate helm args don't contain dangerous flags
	if err := validateHelmArgs(renderedArgs); err != nil {
		return err
	}

	chartPath := filepath.Join(service.HelmPath, service.HelmChartRelativePath)
	namespace := k.getCurrentNamespace()

	// 1. Render helm chart to get raw manifests
	rawManifests, err := k.helmClient.Template(service.Name, chartPath, namespace, renderedArgs)
	if err != nil {
		return fmt.Errorf("failed to template helm chart: %w", err)
	}

	// 2. Build patches from LocalServices configuration
	patches, err := k.buildPatches(service.InterceptHttp)
	if err != nil {
		return fmt.Errorf("failed to build patches: %w", err)
	}

	// 3. Get context name for kustomize work dir
	contextName, err := k.configRepository.LoadCurrentContextName()
	if err != nil {
		return fmt.Errorf("failed to get context name: %w", err)
	}

	// 4. Apply kustomize patches
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	kustomizeWorkDir := filepath.Join(homeDir, ".dx", contextName, "kustomize", service.Name)
	patchedManifests, err := k.kustomizeClient.Apply(rawManifests, patches, kustomizeWorkDir)
	if err != nil {
		return fmt.Errorf("failed to apply kustomize patches: %w", err)
	}

	// 5. Generate wrapper chart
	defer k.cleanupBuildArtifacts(contextName, service.Name)
	wrapperPath, err := k.chartWrapper.Generate(core.WrapperChartConfig{
		ReleaseName:       service.Name,
		ContextName:       contextName,
		PatchedManifests:  patchedManifests,
		OriginalChartName: service.Name,
		OriginalChartPath: chartPath,
	})
	if err != nil {
		return fmt.Errorf("failed to generate wrapper chart: %w", err)
	}

	// 6. Install wrapper chart with helm
	return k.helmClient.UpgradeFromManifests(service.Name, namespace, wrapperPath)
}

// buildPatches creates kustomize patches based on LocalServices configuration.
// When interceptHttp is true, service targetPorts are redirected to mitmproxy ports.
// When false, they are redirected directly to HAProxy frontend ports.
func (k *Kubernetes) buildPatches(interceptHttp bool) ([]ports.Patch, error) {
	configContext, err := k.configRepository.LoadCurrentConfigurationContext()
	if err != nil {
		return nil, err
	}

	var patches []ports.Patch

	// Add recreatedAt annotation to all Deployments to force pod recreation
	patches = append(patches, ports.Patch{
		Target: ports.PatchTarget{Kind: "Deployment"},
		Operations: []ports.PatchOperation{
			{
				Op:    "add",
				Path:  "/spec/template/metadata/annotations/kubectl.kubernetes.io~1recreatedAt",
				Value: time.Now().Format(time.RFC3339),
			},
		},
	})

	// When intercepting, redirect to mitmproxy ports; otherwise redirect to HAProxy frontends
	startPort := core.DevProxyHAProxyStartPort
	if interceptHttp {
		startPort = core.DevProxyMitmproxyStartPort
	}

	// Add service selector patches for each LocalService
	targetPort := startPort
	for _, localService := range configContext.LocalServices {
		patches = append(patches, ports.Patch{
			Target: ports.PatchTarget{Kind: "Service", Name: localService.Name},
			Operations: []ports.PatchOperation{
				{Op: "replace", Path: "/spec/selector/app", Value: "dev-proxy"},
				{Op: "replace", Path: "/spec/ports/0/targetPort", Value: targetPort},
			},
		})
		targetPort++
	}

	return patches, nil
}

// InstallDevProxy installs the dev-proxy (no patches needed).
func (k *Kubernetes) InstallDevProxy(service *domain.Service) error {
	templateValues, err := core.CreateTemplatingValues(k.configRepository, k.secretsRepository)
	if err != nil {
		return err
	}

	var renderedArgs []string
	for i, arg := range service.HelmArgs {
		renderedArg, err := k.templater.Render(arg, fmt.Sprintf("helm-args.%d", i), templateValues)
		if err != nil {
			return err
		}
		renderedArgs = append(renderedArgs, renderedArg)
	}

	// Validate helm args don't contain dangerous flags
	if err := validateHelmArgs(renderedArgs); err != nil {
		return err
	}

	chartPath := filepath.Join(service.HelmPath, service.HelmChartRelativePath)
	namespace := k.getCurrentNamespace()

	// For dev-proxy, no patches needed - just template and install
	rawManifests, err := k.helmClient.Template(service.Name, chartPath, namespace, renderedArgs)
	if err != nil {
		return fmt.Errorf("failed to template helm chart: %w", err)
	}

	contextName, err := k.configRepository.LoadCurrentContextName()
	if err != nil {
		return fmt.Errorf("failed to get context name: %w", err)
	}

	// Generate wrapper chart without patches
	defer k.cleanupBuildArtifacts(contextName, service.Name)
	wrapperPath, err := k.chartWrapper.Generate(core.WrapperChartConfig{
		ReleaseName:       service.Name,
		ContextName:       contextName,
		PatchedManifests:  rawManifests, // No patches applied
		OriginalChartName: service.Name,
		OriginalChartPath: chartPath,
	})
	if err != nil {
		return fmt.Errorf("failed to generate wrapper chart: %w", err)
	}

	return k.helmClient.UpgradeFromManifests(service.Name, namespace, wrapperPath)
}

// UninstallService deletes a service using helm uninstall and cleans up wrapper chart.
func (k *Kubernetes) UninstallService(service *domain.Service) error {
	namespace := k.getCurrentNamespace()
	err := k.helmClient.Uninstall(service.Name, namespace)
	if err != nil {
		return err
	}

	// Clean up wrapper chart directory
	contextName, err := k.configRepository.LoadCurrentContextName()
	if err != nil {
		// Log warning but don't fail - the service was uninstalled
		return nil
	}

	// Ignore cleanup errors - the service was already uninstalled
	k.cleanupBuildArtifacts(contextName, service.Name)
	return nil
}

// cleanupBuildArtifacts removes the wrapper chart and kustomize work directory for a service.
func (k *Kubernetes) cleanupBuildArtifacts(contextName, serviceName string) {
	_ = k.chartWrapper.Cleanup(contextName, serviceName)
	_ = k.fileService.RemoveAll(filepath.Join("~", ".dx", contextName, "kustomize", serviceName))
}

func (k *Kubernetes) HasDeployedServices() (bool, error) {
	namespace := k.getCurrentNamespace()
	releases, err := k.helmClient.List("managed-by=dx", namespace)
	if err != nil {
		return false, err
	}
	return len(releases) > 1, nil
}

// devProxyChecksumAnnotation is the annotation key used to store the dev-proxy configuration checksum.
// This must match the annotation key used in the dev-proxy Helm template.
const devProxyChecksumAnnotation = "checksum"

// GetDevProxyChecksum returns the checksum annotation from the existing dev-proxy deployment.
// Returns an empty string if the deployment doesn't exist.
func (k *Kubernetes) GetDevProxyChecksum() (string, error) {
	namespace := k.getCurrentNamespace()

	deployment, err := k.clientSet.AppsV1().Deployments(namespace).Get(
		context.Background(),
		"dev-proxy",
		metav1.GetOptions{},
	)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to get dev-proxy deployment: %w", err)
	}

	annotations := deployment.Spec.Template.Annotations
	if annotations == nil {
		return "", nil
	}

	return annotations[devProxyChecksumAnnotation], nil
}

// blockedHelmFlags contains flags that should not be allowed in user-provided helm args.
// These flags could be used to bypass security controls or execute arbitrary code.
var blockedHelmFlags = []string{
	// Code execution risks
	"--post-renderer", // Could execute arbitrary code

	// Cluster/context manipulation
	"--kubeconfig",   // Could redirect to different cluster
	"--kube-context", // Could switch context unexpectedly

	// Configuration file manipulation
	"--repository-config", // Could point to malicious repo config
	"--registry-config",   // Could point to malicious registry config

	// TLS/certificate manipulation
	"--ca-file",                  // Could point to attacker-controlled CA
	"--cert-file",                // Could leak or use unauthorized certificates
	"--key-file",                 // Could leak or use unauthorized private keys
	"--insecure-skip-tls-verify", // Enables MITM attacks

	// Credential exposure
	"--password",       // Could expose passwords in process list
	"--username",       // Could be used with password for auth bypass
	"--kube-token",     // Could use unauthorized tokens
	"--kube-as",        // Could impersonate other users
	"--kube-as-group",  // Could impersonate groups
	"--kube-as-uid",    // Could impersonate by UID
	"--kube-ca-file",   // Could use unauthorized CA
	"--kube-apiserver", // Could redirect to malicious API server
}

// validateHelmArgs checks that user-provided helm args don't contain dangerous flags.
func validateHelmArgs(args []string) error {
	for _, arg := range args {
		argLower := strings.ToLower(arg)
		for _, blocked := range blockedHelmFlags {
			// Check for exact match or prefix match (e.g., "--post-renderer=" or "--post-renderer ")
			if argLower == blocked || strings.HasPrefix(argLower, blocked+"=") {
				return fmt.Errorf("helm argument %q is not allowed for security reasons", blocked)
			}
		}
	}
	return nil
}
