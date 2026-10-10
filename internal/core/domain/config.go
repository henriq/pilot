package domain

import (
	"fmt"
	"slices"
	"strings"

	"golang.org/x/mod/semver"
)

type ConfigurationContext struct {
	Name            string            `yaml:"name"`
	MinPilotVersion string            `yaml:"minPilotVersion,omitempty"`
	Scripts         map[string]string `yaml:"scripts"`
	Import          *string           `yaml:"import,omitempty"`
	Services        []Service         `yaml:"services"`
	LocalServices   []LocalService    `yaml:"localServices,omitempty"`
}

type Service struct {
	Name                  string               `yaml:"name"`
	HelmRepoPath          string               `yaml:"helmRepoPath"`
	HelmPath              string               `yaml:"-"`
	HelmChartRelativePath string               `yaml:"helmChartRelativePath"`
	HelmBranch            string               `yaml:"helmBranch"`
	HelmArgs              []string             `yaml:"helmArgs"`
	LocalPort             *int                 `yaml:"localPort,omitempty"`
	DockerImages          []DockerImage        `yaml:"dockerImages"`
	RemoteImages          []string             `yaml:"remoteImages"`
	Profiles              []string             `yaml:"profiles,omitempty"`
	GitRepoPath           string               `yaml:"gitRepoPath"`
	GitRef                string               `yaml:"gitRef"`
	Certificates          []CertificateRequest `yaml:"certificates,omitempty"`
	Path                  string               `yaml:"-"`
	InterceptHttp         bool                 `yaml:"-"`
}

type DockerImage struct {
	Name                     string   `yaml:"name"`
	DockerfilePath           string   `yaml:"dockerfilePath,omitempty"`
	DockerfileOverride       string   `yaml:"dockerfileOverride,omitempty"`
	BuildContextRelativePath string   `yaml:"buildContextRelativePath"`
	BuildArgs                []string `yaml:"buildArgs"`
	GitRepoPath              string   `yaml:"gitRepoPath"`
	GitRef                   string   `yaml:"gitRef"`
	Path                     string   `yaml:"-"`
}

type LocalService struct {
	Name            string            `yaml:"name"`
	LocalPort       int               `yaml:"localPort"`
	KubernetesPort  int               `yaml:"kubernetesPort"`
	HealthCheckPath string            `yaml:"healthCheckPath"`
	Selector        map[string]string `yaml:"selector"`
}

// Config holds the application configuration including available services
type Config struct {
	MinPilotVersion string                 `yaml:"minPilotVersion,omitempty"`
	Contexts        []ConfigurationContext `yaml:"contexts"`
}

func CreateDefaultConfig() Config {
	return Config{
		Contexts: []ConfigurationContext{
			{
				Name: "default",
				Services: []Service{
					{
						Name: "default",
						DockerImages: []DockerImage{
							{
								Name:                     "default",
								DockerfilePath:           "Dockerfile",
								BuildContextRelativePath: ".",
								GitRepoPath:              "/tmp/bar",
								GitRef:                   "main",
							},
						},
						RemoteImages: []string{
							"postgres:latest",
						},
						HelmRepoPath:          "/tmp/foo",
						HelmChartRelativePath: "helm",
						HelmBranch:            "local",
					},
				},
				LocalServices: []LocalService{
					{
						Name:            "default",
						LocalPort:       8080,
						KubernetesPort:  80,
						HealthCheckPath: "/health",
						Selector: map[string]string{
							"app": "default",
						},
					},
				},
			},
		},
	}
}

func (s *Service) IsDeployable() bool {
	return s.HelmRepoPath != "" || s.HelmBranch != "" || s.HelmChartRelativePath != ""
}

func (c *Config) ContextExists(name string) bool {
	for _, context := range c.Contexts {
		if context.Name == name {
			return true
		}
	}
	return false
}

// FilterServices returns services matching the given names, or if no names are provided,
// services belonging to the given profile.
func (c *ConfigurationContext) FilterServices(names []string, profile string) []Service {
	var result []Service
	for _, service := range c.Services {
		if len(names) == 0 && !slices.Contains(service.Profiles, profile) {
			continue
		}
		if len(names) > 0 && !slices.Contains(names, service.Name) {
			continue
		}
		result = append(result, service)
	}
	return result
}

func (c *Config) GetContext(name string) (*ConfigurationContext, error) {
	for _, context := range c.Contexts {
		if context.Name == name {
			return &context, nil
		}
	}
	return nil, fmt.Errorf("context '%s' not found", name)
}

func (c *ConfigurationContext) GetService(name string) *Service {
	for _, service := range c.Services {
		if service.Name == name {
			return &service
		}
	}
	return nil
}

// NormalizePilotVersion returns the version with a leading "v" so it parses as semver.
func NormalizePilotVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	if !strings.HasPrefix(version, "v") {
		return "v" + version
	}
	return version
}

// IsValidPilotVersion reports whether the given string is valid semver, with or without a leading "v".
func IsValidPilotVersion(version string) bool {
	return semver.IsValid(NormalizePilotVersion(version))
}

// HigherPilotVersion returns whichever of a and b sorts higher in semver order; an empty value loses to any non-empty value.
func HigherPilotVersion(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	if semver.Compare(NormalizePilotVersion(a), NormalizePilotVersion(b)) >= 0 {
		return a
	}
	return b
}

// PilotVersionMeetsMinimum strips prerelease and build metadata before comparing so a prerelease build of vX.Y.Z is accepted as meeting a vX.Y.Z minimum.
func PilotVersionMeetsMinimum(current, minimum string) bool {
	return semver.Compare(releasePilotVersion(current), releasePilotVersion(minimum)) >= 0
}

func releasePilotVersion(version string) string {
	canonical := semver.Canonical(NormalizePilotVersion(version))
	return strings.TrimSuffix(canonical, semver.Prerelease(canonical))
}

// EffectiveMinPilotVersion returns the highest minimum-version requirement declared across the root config and every context, or "" when none is set.
func (c *Config) EffectiveMinPilotVersion() string {
	highest := c.MinPilotVersion
	for _, ctx := range c.Contexts {
		highest = HigherPilotVersion(highest, ctx.MinPilotVersion)
	}
	return highest
}

// ValidateContextName checks that a context name doesn't contain path traversal characters.
func ValidateContextName(name string) error {
	if name == "" {
		return fmt.Errorf("context name cannot be empty")
	}
	if strings.Contains(name, "..") ||
		strings.Contains(name, "/") ||
		strings.Contains(name, "\\") ||
		strings.Contains(name, "\x00") {
		return fmt.Errorf("contains invalid characters (path traversal not allowed)")
	}
	return nil
}

func (c *Config) Validate() error {
	if c.MinPilotVersion != "" && !IsValidPilotVersion(c.MinPilotVersion) {
		return fmt.Errorf("minPilotVersion %q is not valid semver", c.MinPilotVersion)
	}
	if len(c.Contexts) == 0 {
		return fmt.Errorf("no contexts defined in configuration")
	}
	for i, ctx := range c.Contexts {
		if err := ctx.validate(i); err != nil {
			return err
		}
	}
	return nil
}

func (c *ConfigurationContext) validate(index int) error {
	if c.Name == "" {
		return fmt.Errorf("context at index %d has empty name", index)
	}
	if err := ValidateContextName(c.Name); err != nil {
		return fmt.Errorf("context '%s': %w", c.Name, err)
	}
	if c.MinPilotVersion != "" && !IsValidPilotVersion(c.MinPilotVersion) {
		return fmt.Errorf("context %q: minPilotVersion %q is not valid semver", c.Name, c.MinPilotVersion)
	}
	for i, svc := range c.Services {
		if err := svc.validate(c.Name, i); err != nil {
			return err
		}
	}
	for i, localSvc := range c.LocalServices {
		if err := localSvc.validate(c.Name, i); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) validate(contextName string, index int) error {
	if s.Name == "" {
		return fmt.Errorf("service at index %d in context '%s' has empty name", index, contextName)
	}
	if strings.Contains(s.Name, ":") {
		return fmt.Errorf(
			"service '%s' in context '%s' has invalid name; ':' is not permitted",
			s.Name,
			contextName,
		)
	}
	if controlIndex := strings.IndexFunc(s.Name, func(r rune) bool {
		return r < 0x20 || r == 0x7f
	}); controlIndex >= 0 {
		return fmt.Errorf(
			"service '%s' in context '%s' has invalid name; contains a control character at byte %d",
			s.Name,
			contextName,
			controlIndex,
		)
	}
	// The name is the one unhashed part of every derived cache path. A "/" is
	// allowed and simply nests; ".." would escape ~/.pilot entirely.
	if strings.Contains(s.Name, "..") || strings.Contains(s.Name, "\x00") {
		return fmt.Errorf(
			"service '%s' in context '%s' has invalid name; '..' is not permitted",
			s.Name,
			contextName,
		)
	}
	if err := s.validateHelmConfiguration(contextName); err != nil {
		return err
	}
	for i, img := range s.DockerImages {
		if err := img.validate(contextName, s.Name, s.GitRepoPath, s.GitRef, i); err != nil {
			return err
		}
	}
	for _, cert := range s.Certificates {
		if err := cert.Validate(s.Name, contextName); err != nil {
			return err
		}
	}
	for i, remoteImg := range s.RemoteImages {
		if remoteImg == "" {
			return fmt.Errorf(
				"remote image at index %d for service '%s' in context '%s' is empty",
				i,
				s.Name,
				contextName,
			)
		}
	}
	return nil
}

func (s *Service) validateHelmConfiguration(contextName string) error {
	helmFieldsSet := 0
	if s.HelmRepoPath != "" {
		helmFieldsSet++
	}
	if s.HelmBranch != "" {
		helmFieldsSet++
	}
	if s.HelmChartRelativePath != "" {
		helmFieldsSet++
	}
	if helmFieldsSet != 0 && helmFieldsSet != 3 {
		return fmt.Errorf(
			"service '%s' in context '%s' has partial helm configuration: helmRepoPath, helmBranch, and helmChartRelativePath must all be set or all be empty",
			s.Name,
			contextName,
		)
	}
	if s.IsDeployable() {
		return nil
	}
	if len(s.Certificates) > 0 {
		return fmt.Errorf(
			"non-deployable service '%s' in context '%s' must not declare certificates",
			s.Name,
			contextName,
		)
	}
	if s.LocalPort != nil {
		return fmt.Errorf(
			"non-deployable service '%s' in context '%s' must not declare localPort",
			s.Name,
			contextName,
		)
	}
	return nil
}

func (i *DockerImage) validate(contextName, serviceName, serviceGitRepoPath, serviceGitRef string, index int) error {
	if i.Name == "" {
		return fmt.Errorf(
			"docker image at index %d for service '%s' in context '%s' has empty name",
			index,
			serviceName,
			contextName,
		)
	}
	if strings.Contains(i.Name, ":") {
		return fmt.Errorf(
			"docker image '%s' for service '%s' in context '%s' has invalid name; ':' is not permitted",
			i.Name,
			serviceName,
			contextName,
		)
	}
	if index := strings.IndexFunc(i.Name, func(r rune) bool {
		return r < 0x20 || r == 0x7f
	}); index >= 0 {
		return fmt.Errorf(
			"docker image '%s' for service '%s' in context '%s' has invalid name; contains a control character at byte %d",
			i.Name,
			serviceName,
			contextName,
			index,
		)
	}
	if i.DockerfilePath == "" && strings.TrimSpace(i.DockerfileOverride) == "" {
		return fmt.Errorf(
			"docker image '%s' for service '%s' in context '%s' must have either dockerfilePath or dockerfileOverride",
			i.Name,
			serviceName,
			contextName,
		)
	}
	if i.BuildContextRelativePath == "" {
		return fmt.Errorf(
			"docker image '%s' for service '%s' in context '%s' has empty buildContextRelativePath",
			i.Name,
			serviceName,
			contextName,
		)
	}
	if i.GitRepoPath == "" && serviceGitRepoPath == "" {
		return fmt.Errorf(
			"docker image '%s' for service '%s' in context '%s' has empty gitRepoPath",
			i.Name,
			serviceName,
			contextName,
		)
	}
	if i.GitRef == "" && serviceGitRef == "" {
		return fmt.Errorf(
			"docker image '%s' for service '%s' in context '%s' has empty gitRef",
			i.Name,
			serviceName,
			contextName,
		)
	}
	return nil
}

func (l *LocalService) validate(contextName string, index int) error {
	if l.Name == "" {
		return fmt.Errorf("local service at index %d in context '%s' has empty name", index, contextName)
	}
	if l.KubernetesPort <= 0 {
		return fmt.Errorf(
			"local service '%s' in context '%s' has invalid kubernetesPort",
			l.Name,
			contextName,
		)
	}
	if l.Selector == nil {
		return fmt.Errorf(
			"local service '%s' in context '%s' has empty selector",
			l.Name,
			contextName,
		)
	}
	return nil
}
