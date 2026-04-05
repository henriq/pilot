package domain

import (
	"fmt"
	"strings"
)

type ConfigurationContext struct {
	Name          string            `yaml:"name"`
	Scripts       map[string]string `yaml:"scripts"`
	Import        *string           `yaml:"import,omitempty"`
	Services      []Service         `yaml:"services"`
	LocalServices []LocalService    `yaml:"localServices,omitempty"`
}

// Service represents a deployable service with its Docker configuration
type Service struct {
	Name                  string               `yaml:"name"`
	HelmRepoPath          string               `yaml:"helmRepoPath"`
	HelmPath              string               `yaml:"-"` // Will be ignored during YAML serialization
	HelmChartRelativePath string               `yaml:"helmChartRelativePath"`
	HelmBranch            string               `yaml:"helmBranch"`
	HelmArgs              []string             `yaml:"helmArgs"`
	LocalPort             *int                 `yaml:"localPort,omitempty"` // Using pointer to make nullable
	DockerImages          []DockerImage        `yaml:"dockerImages"`
	RemoteImages          []string             `yaml:"remoteImages"`
	Profiles              []string             `yaml:"profiles,omitempty"`
	GitRepoPath           string               `yaml:"gitRepoPath"`
	GitRef                string               `yaml:"gitRef"`
	Certificates          []CertificateRequest `yaml:"certificates,omitempty"`
	Path                  string               `yaml:"-"` // Will be ignored during YAML serialization
	InterceptHttp         bool                 `yaml:"-"` // Runtime flag, not persisted
}

type DockerImage struct {
	Name                     string   `yaml:"name"`
	DockerfilePath           string   `yaml:"dockerfilePath,omitempty"`
	DockerfileOverride       string   `yaml:"dockerfileOverride,omitempty"`
	BuildContextRelativePath string   `yaml:"buildContextRelativePath"`
	BuildArgs                []string `yaml:"buildArgs"`
	GitRepoPath              string   `yaml:"gitRepoPath"`
	GitRef                   string   `yaml:"gitRef"`
	Path                     string   `yaml:"-"` // Will be ignored during YAML serialization
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
	Contexts []ConfigurationContext `yaml:"contexts"`
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

func (c *Config) ContextExists(name string) bool {
	for _, context := range c.Contexts {
		if context.Name == name {
			return true
		}
	}
	return false
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

func (c *Config) Validate() error {
	for i, ctx := range c.Contexts {
		if ctx.Name == "" {
			return fmt.Errorf("context at index %d has empty name", i)
		}
		// Validate context name doesn't contain path traversal characters
		if strings.Contains(ctx.Name, "..") ||
			strings.Contains(ctx.Name, "/") ||
			strings.Contains(ctx.Name, "\\") ||
			strings.Contains(ctx.Name, "\x00") {
			return fmt.Errorf("context '%s' contains invalid characters (path traversal not allowed)", ctx.Name)
		}

		for j, svc := range ctx.Services {
			if svc.Name == "" {
				return fmt.Errorf("service at index %d in context '%s' has empty name", j, ctx.Name)
			}
			if svc.HelmRepoPath == "" {
				return fmt.Errorf("service '%s' in context '%s' has empty helmPath", svc.Name, ctx.Name)
			}
			if svc.HelmBranch == "" {
				return fmt.Errorf("service '%s' in context '%s' has empty helmBranch", svc.Name, ctx.Name)
			}
			if svc.HelmChartRelativePath == "" {
				return fmt.Errorf("service '%s' in context '%s' has empty helmChartRelativePath", svc.Name, ctx.Name)
			}

			for k, img := range svc.DockerImages {
				if img.Name == "" {
					return fmt.Errorf(
						"docker image at index %d for service '%s' in context '%s' has empty name",
						k,
						svc.Name,
						ctx.Name,
					)
				}
				if img.DockerfilePath == "" && strings.TrimSpace(img.DockerfileOverride) == "" {
					return fmt.Errorf(
						"docker image '%s' for service '%s' in context '%s' must have either dockerfilePath or dockerfileOverride",
						img.Name,
						svc.Name,
						ctx.Name,
					)
				}
				if img.BuildContextRelativePath == "" {
					return fmt.Errorf(
						"docker image '%s' for service '%s' in context '%s' has empty buildContextRelativePath",
						img.Name,
						svc.Name,
						ctx.Name,
					)
				}
				if img.GitRepoPath == "" {
					return fmt.Errorf(
						"docker image '%s' for service '%s' in context '%s' has empty gitRepoPath",
						img.Name,
						svc.Name,
						ctx.Name,
					)
				}
				if img.GitRef == "" {
					return fmt.Errorf(
						"docker image '%s' for service '%s' in context '%s' has empty gitRef",
						img.Name,
						svc.Name,
						ctx.Name,
					)
				}
			}

			// Validate Certificates
			for _, cert := range svc.Certificates {
				if err := cert.Validate(svc.Name, ctx.Name); err != nil {
					return err
				}
			}

			// Check if RemoteImages contains empty strings
			for k, remoteImg := range svc.RemoteImages {
				if remoteImg == "" {
					return fmt.Errorf(
						"remote image at index %d for service '%s' in context '%s' is empty",
						k,
						svc.Name,
						ctx.Name,
					)
				}
			}
		}

		// Validate LocalServices
		for j, localSvc := range ctx.LocalServices {
			if localSvc.Name == "" {
				return fmt.Errorf("local service at index %d in context '%s' has empty name", j, ctx.Name)
			}
			if localSvc.KubernetesPort <= 0 {
				return fmt.Errorf(
					"local service '%s' in context '%s' has invalid kubernetesPort",
					localSvc.Name,
					ctx.Name,
				)
			}
			if localSvc.Selector == nil {
				return fmt.Errorf(
					"local service '%s' in context '%s' has empty selector",
					localSvc.Name,
					ctx.Name,
				)
			}
		}
	}

	if len(c.Contexts) == 0 {
		return fmt.Errorf("no contexts defined in configuration")
	}

	return nil
}
