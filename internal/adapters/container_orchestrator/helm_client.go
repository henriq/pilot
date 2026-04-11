package container_orchestrator

import (
	"fmt"
	"strings"

	"pilot/internal/ports"
)

var _ ports.HelmClient = (*HelmClient)(nil)

// HelmClient implements ports.HelmClient using the helm CLI.
type HelmClient struct {
	commandRunner ports.CommandRunner
}

// NewHelmClient creates a new HelmClient.
func NewHelmClient(runner ports.CommandRunner) *HelmClient {
	return &HelmClient{
		commandRunner: runner,
	}
}

// Template renders a helm chart and returns the manifests as YAML.
func (h *HelmClient) Template(name, chartPath, namespace string, args []string) ([]byte, error) {
	cmdArgs := []string{"template", name, chartPath}

	if namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", namespace)
	}

	cmdArgs = append(cmdArgs, args...)

	output, err := h.commandRunner.Run("helm", cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("helm template failed: %w, output: %s", err, string(output))
	}

	return output, nil
}

// UpgradeFromManifests installs/upgrades using pre-rendered manifests in a wrapper chart.
func (h *HelmClient) UpgradeFromManifests(name, namespace, wrapperChartPath string) error {
	cmdArgs := []string{
		"upgrade",
		"--install",
		"--labels", "managed-by=pilot",
		name,
		wrapperChartPath,
	}

	if namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", namespace)
	}

	output, err := h.commandRunner.Run("helm", cmdArgs...)
	if err != nil {
		return fmt.Errorf("helm upgrade failed: %w, output: %s", err, string(output))
	}

	return nil
}

// Uninstall removes a helm release.
func (h *HelmClient) Uninstall(name, namespace string) error {
	cmdArgs := []string{"uninstall", name}
	if namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", namespace)
	}

	output, err := h.commandRunner.Run("helm", cmdArgs...)
	if err != nil {
		return fmt.Errorf("failed to uninstall helm chart: %w, output: %s", err, string(output))
	}
	return nil
}

// List returns release names matching the label selector.
func (h *HelmClient) List(labelSelector, namespace string) ([]string, error) {
	cmdArgs := []string{"list", "-l", labelSelector, "--short"}
	if namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", namespace)
	}

	output, err := h.commandRunner.Run("helm", cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to list helm charts: %w, output: %s", err, string(output))
	}

	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return []string{}, nil
	}
	return strings.Split(trimmed, "\n"), nil
}
