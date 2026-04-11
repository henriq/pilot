package kustomize

import (
	"fmt"
	"path/filepath"
	"strings"

	"pilot/internal/ports"

	"gopkg.in/yaml.v3"
)

var _ ports.KustomizeClient = (*Client)(nil)

// Kustomization represents a kustomization.yaml file.
type Kustomization struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Resources  []string `yaml:"resources"`
	Labels     []Label  `yaml:"labels,omitempty"`
	Patches    []Patch  `yaml:"patches,omitempty"`
}

// Label represents a kustomize label entry.
type Label struct {
	Pairs            map[string]string `yaml:"pairs"`
	IncludeSelectors bool              `yaml:"includeSelectors"`
}

// Patch represents a kustomize patch entry.
// Either Path or Patch should be set, not both.
type Patch struct {
	Path   string `yaml:"path,omitempty"`
	Patch  string `yaml:"patch,omitempty"`
	Target Target `yaml:"target"`
}

// PatchFile represents a patch file to be written to disk.
type PatchFile struct {
	Filename string
	Content  []byte
}

// Target identifies which resources to patch.
type Target struct {
	Kind string `yaml:"kind,omitempty"`
	Name string `yaml:"name,omitempty"`
}

// PatchOperation represents a JSON Patch operation (RFC 6902).
type PatchOperation struct {
	Op    string      `yaml:"op"`
	Path  string      `yaml:"path"`
	Value interface{} `yaml:"value,omitempty"`
}

// Client implements ports.KustomizeClient using kubectl kustomize CLI.
type Client struct {
	commandRunner ports.CommandRunner
	fileSystem    ports.FileSystem
}

// NewClient creates a new kustomize Client.
func NewClient(commandRunner ports.CommandRunner, fileSystem ports.FileSystem) *Client {
	return &Client{
		commandRunner: commandRunner,
		fileSystem:    fileSystem,
	}
}

// Apply takes raw YAML manifests and applies patches using kubectl kustomize.
// Files are written to workDir for inspection.
func (c *Client) Apply(manifests []byte, patches []ports.Patch, workDir string) ([]byte, error) {
	if len(patches) == 0 {
		return manifests, nil
	}

	// Ensure work directory exists
	if err := c.fileSystem.MkdirAll(workDir, ports.ReadWriteExecute); err != nil {
		return nil, fmt.Errorf("failed to create work directory: %w", err)
	}

	// Write manifests to resources file
	resourcesPath := filepath.Join(workDir, "resources.yaml")
	if err := c.fileSystem.WriteFile(resourcesPath, manifests, ports.ReadWrite); err != nil {
		return nil, fmt.Errorf("failed to write resources: %w", err)
	}

	// Build kustomization and patch files
	kustomization, patchFiles, err := buildKustomization(patches)
	if err != nil {
		return nil, fmt.Errorf("failed to build kustomization: %w", err)
	}

	// Write patch files
	for _, pf := range patchFiles {
		patchPath := filepath.Join(workDir, pf.Filename)
		if err := c.fileSystem.WriteFile(patchPath, pf.Content, ports.ReadWrite); err != nil {
			return nil, fmt.Errorf("failed to write patch file %s: %w", pf.Filename, err)
		}
	}

	// Write kustomization.yaml
	kustomizationYAML, err := yaml.Marshal(kustomization)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal kustomization: %w", err)
	}

	kustomizationPath := filepath.Join(workDir, "kustomization.yaml")
	if err := c.fileSystem.WriteFile(kustomizationPath, kustomizationYAML, ports.ReadWrite); err != nil {
		return nil, fmt.Errorf("failed to write kustomization.yaml: %w", err)
	}

	// Run kubectl kustomize
	output, err := c.commandRunner.Run("kubectl", "kustomize", workDir)
	if err != nil {
		return nil, fmt.Errorf("kubectl kustomize failed: %w, output: %s", err, string(output))
	}

	return output, nil
}

// buildKustomization creates a Kustomization from patches and returns patch files to write.
func buildKustomization(patches []ports.Patch) (Kustomization, []PatchFile, error) {
	kustomization := Kustomization{
		APIVersion: "kustomize.config.k8s.io/v1beta1",
		Kind:       "Kustomization",
		Resources:  []string{"resources.yaml"},
		Labels: []Label{
			{
				Pairs:            map[string]string{"managed-by": "pilot"},
				IncludeSelectors: false,
			},
		},
	}

	var patchFiles []PatchFile

	for _, p := range patches {
		// Separate add operations (strategic merge) from replace/remove (JSON patch)
		var addOps []ports.PatchOperation
		var jsonPatchOps []ports.PatchOperation

		for _, op := range p.Operations {
			if op.Op == "add" {
				addOps = append(addOps, op)
			} else {
				jsonPatchOps = append(jsonPatchOps, op)
			}
		}

		// Create file-based strategic merge patches for add operations
		for _, op := range addOps {
			patchContent, err := buildStrategicMergePatch(p.Target, op)
			if err != nil {
				return Kustomization{}, nil, fmt.Errorf("failed to build strategic merge patch for %s: %w", p.Target.Kind, err)
			}
			if len(patchContent) > 0 {
				filename := fmt.Sprintf("patch-%s.yaml", patchNameFromPath(op.Path))

				patchFiles = append(patchFiles, PatchFile{
					Filename: filename,
					Content:  patchContent,
				})

				kustomization.Patches = append(kustomization.Patches, Patch{
					Path: filename,
					Target: Target{
						Kind: p.Target.Kind,
						Name: p.Target.Name,
					},
				})
			}
		}

		// Create inline JSON patch for replace/remove operations
		if len(jsonPatchOps) > 0 {
			var ops []PatchOperation
			for _, op := range jsonPatchOps {
				patchOp := PatchOperation{
					Op:   op.Op,
					Path: op.Path,
				}
				if op.Op != "remove" {
					patchOp.Value = op.Value
				}
				ops = append(ops, patchOp)
			}

			opsYAML, err := yaml.Marshal(ops)
			if err != nil {
				return Kustomization{}, nil, fmt.Errorf("failed to marshal JSON patch operations: %w", err)
			}
			kustomization.Patches = append(kustomization.Patches, Patch{
				Patch: string(opsYAML),
				Target: Target{
					Kind: p.Target.Kind,
					Name: p.Target.Name,
				},
			})
		}
	}

	return kustomization, patchFiles, nil
}

// buildStrategicMergePatch creates a YAML strategic merge patch from a JSON pointer path.
// The patch includes apiVersion, kind, and a placeholder name for kustomize compatibility.
func buildStrategicMergePatch(target ports.PatchTarget, op ports.PatchOperation) ([]byte, error) {
	path := op.Path
	path = strings.TrimPrefix(path, "/")

	parts := strings.Split(path, "/")
	for i, part := range parts {
		parts[i] = unescapeJSONPointer(part)
	}

	// Build the nested structure for the patch content
	content := make(map[string]interface{})
	current := content

	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = op.Value
		} else {
			next := make(map[string]interface{})
			current[part] = next
			current = next
		}
	}

	// Strategic merge patch needs apiVersion, kind, and metadata.name
	// The name is a placeholder since we use target selector to match resources
	result := map[string]interface{}{
		"apiVersion": apiVersionForKind(target.Kind),
		"kind":       target.Kind,
		"metadata": map[string]interface{}{
			"name": "placeholder",
		},
	}

	// Merge content into result
	for k, v := range content {
		result[k] = v
	}

	patchYAML, err := yaml.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal strategic merge patch: %w", err)
	}

	return patchYAML, nil
}

// apiVersionForKind returns the appropriate apiVersion for common Kubernetes kinds.
func apiVersionForKind(kind string) string {
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet":
		return "apps/v1"
	case "Service", "ConfigMap", "Secret", "Namespace", "ServiceAccount", "PersistentVolumeClaim":
		return "v1"
	case "Ingress":
		return "networking.k8s.io/v1"
	case "Job", "CronJob":
		return "batch/v1"
	case "Role", "RoleBinding", "ClusterRole", "ClusterRoleBinding":
		return "rbac.authorization.k8s.io/v1"
	default:
		return "v1"
	}
}

// unescapeJSONPointer decodes JSON pointer escape sequences per RFC 6901:
// ~1 decodes to "/" (must be done first), ~0 decodes to "~"
func unescapeJSONPointer(s string) string {
	s = strings.ReplaceAll(s, "~1", "/")
	s = strings.ReplaceAll(s, "~0", "~")
	return s
}

// patchNameFromPath extracts a descriptive name from a JSON pointer path.
// E.g., "/spec/template/metadata/annotations/kubectl.kubernetes.io~1recreatedAt" -> "recreated-at"
func patchNameFromPath(path string) string {
	// Get the last segment of the path
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return "patch"
	}

	lastPart := parts[len(parts)-1]
	if lastPart == "" {
		return "patch"
	}

	lastPart = unescapeJSONPointer(lastPart)

	// Extract the part after the last "/" (for paths like "kubectl.kubernetes.io/recreatedAt")
	if idx := strings.LastIndex(lastPart, "/"); idx != -1 {
		lastPart = lastPart[idx+1:]
	}

	// Convert camelCase to kebab-case
	var result strings.Builder
	for i, r := range lastPart {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('-')
		}
		result.WriteRune(r)
	}

	return strings.ToLower(result.String())
}
