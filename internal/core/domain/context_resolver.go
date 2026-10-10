package domain

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
)

// ContextOverrides bundles the CLI override value objects applied during
// resolution. Its zero value (NoOverrides) applies nothing.
type ContextOverrides struct {
	ServiceGitRefs ServiceGitRefOverrides
	ImageSources   DockerImageSourceOverrides
	HelmCharts     HelmChartOverrides
}

// NoOverrides resolves a context with no CLI overrides applied.
var NoOverrides = ContextOverrides{}

// ResolveContext returns an independently-owned copy of raw with git-ref/repo
// inheritance, cache-path derivation, overrides, and profile defaults applied.
// It never mutates raw, and errors if an override targets an absent service or
// image or if any resolved ref fails ValidateGitRefShape.
func ResolveContext(raw ConfigurationContext, home string, overrides ContextOverrides) (ConfigurationContext, error) {
	resolved := copyContext(raw)

	if err := overrides.ServiceGitRefs.ValidateAgainstServices(resolved.Services); err != nil {
		return ConfigurationContext{}, err
	}
	if err := overrides.HelmCharts.ValidateAgainstServices(resolved.Services); err != nil {
		return ConfigurationContext{}, err
	}
	if err := overrides.ImageSources.ValidateAgainstImages(allDockerImages(resolved.Services)); err != nil {
		return ConfigurationContext{}, err
	}

	for serviceIndex := range resolved.Services {
		service := &resolved.Services[serviceIndex]
		overriddenRef, hasGitRefOverride := overrides.ServiceGitRefs.LookupGitRef(service.Name)
		if hasGitRefOverride {
			service.GitRef = overriddenRef
		}
		if err := validateServiceRefs(service); err != nil {
			return ConfigurationContext{}, err
		}

		for imageIndex := range service.DockerImages {
			image := &service.DockerImages[imageIndex]
			if image.GitRepoPath == "" {
				image.GitRepoPath = service.GitRepoPath
			}
			// An override replaces an image's own ref too, so the whole service
			// builds from one ref rather than a mix.
			if image.GitRef == "" || hasGitRefOverride {
				image.GitRef = service.GitRef
			}
			if err := ValidateGitRefShape(image.GitRef); err != nil {
				return ConfigurationContext{}, fmt.Errorf(
					"invalid git ref for image %q in service %q: %w", image.Name, service.Name, err,
				)
			}
			image.Path = imageCachePath(home, resolved.Name, service.Name, image.GitRepoPath, image.GitRef)
			// A local --image-source wins over the derived (or git-ref-overridden) cache path.
			if sourcePath, hasSourceOverride := overrides.ImageSources.LookupSourcePath(image.Name); hasSourceOverride {
				image.Path = sourcePath
			}
		}

		if service.IsDeployable() {
			service.HelmPath = filepath.Join(home, ".pilot", resolved.Name, "charts", ShortPathHash(service.HelmRepoPath, service.HelmBranch))
		}
		if service.GitRepoPath != "" && service.GitRef != "" {
			service.Path = filepath.Join(home, ".pilot", resolved.Name, service.Name, ShortPathHash(service.GitRepoPath, service.GitRef))
		}
		if chartDirectory, hasChartOverride := overrides.HelmCharts.LookupChartDirectory(service.Name); hasChartOverride {
			service.HelmPath = chartDirectory
			service.HelmChartRelativePath = ""
		}

		applyProfileDefaults(service)
	}

	return resolved, nil
}

func imageCachePath(home, contextName, serviceName, gitRepoPath, gitRef string) string {
	return filepath.Join(home, ".pilot", contextName, serviceName, ShortPathHash(gitRepoPath, gitRef))
}

// Both refs reach a git command line — GitRef via pilot run, HelmBranch via the
// chart clone — so they are checked wherever they came from, config or override.
func validateServiceRefs(service *Service) error {
	if service.GitRef != "" {
		if err := ValidateGitRefShape(service.GitRef); err != nil {
			return fmt.Errorf("invalid git ref for service %q: %w", service.Name, err)
		}
	}
	if service.HelmBranch != "" {
		if err := ValidateGitRefShape(service.HelmBranch); err != nil {
			return fmt.Errorf("invalid helm branch for service %q: %w", service.Name, err)
		}
	}
	return nil
}

func applyProfileDefaults(service *Service) {
	if len(service.Profiles) == 0 && service.IsDeployable() {
		service.Profiles = []string{"default"}
	}
	if !slices.Contains(service.Profiles, "all") {
		service.Profiles = append(service.Profiles, "all")
	}
}

func allDockerImages(services []Service) []DockerImage {
	var images []DockerImage
	for _, service := range services {
		images = append(images, service.DockerImages...)
	}
	return images
}

func copyContext(source ConfigurationContext) ConfigurationContext {
	copied := source
	if source.Import != nil {
		importPath := *source.Import
		copied.Import = &importPath
	}
	copied.Scripts = maps.Clone(source.Scripts)
	copied.Services = copyServices(source.Services)
	copied.LocalServices = copyLocalServices(source.LocalServices)
	return copied
}

func copyServices(source []Service) []Service {
	if source == nil {
		return nil
	}
	copied := make([]Service, len(source))
	for index, service := range source {
		service.HelmArgs = slices.Clone(service.HelmArgs)
		service.RemoteImages = slices.Clone(service.RemoteImages)
		service.Profiles = slices.Clone(service.Profiles)
		service.DockerImages = copyDockerImages(service.DockerImages)
		service.Certificates = copyCertificates(service.Certificates)
		if service.LocalPort != nil {
			localPort := *service.LocalPort
			service.LocalPort = &localPort
		}
		copied[index] = service
	}
	return copied
}

func copyDockerImages(source []DockerImage) []DockerImage {
	if source == nil {
		return nil
	}
	copied := make([]DockerImage, len(source))
	for index, image := range source {
		image.BuildArgs = slices.Clone(image.BuildArgs)
		copied[index] = image
	}
	return copied
}

func copyCertificates(source []CertificateRequest) []CertificateRequest {
	if source == nil {
		return nil
	}
	copied := make([]CertificateRequest, len(source))
	for index, certificate := range source {
		certificate.DNSNames = slices.Clone(certificate.DNSNames)
		if certificate.K8sSecret.Keys != nil {
			keys := *certificate.K8sSecret.Keys
			certificate.K8sSecret.Keys = &keys
		}
		copied[index] = certificate
	}
	return copied
}

func copyLocalServices(source []LocalService) []LocalService {
	if source == nil {
		return nil
	}
	copied := make([]LocalService, len(source))
	for index, localService := range source {
		localService.Selector = maps.Clone(localService.Selector)
		copied[index] = localService
	}
	return copied
}
