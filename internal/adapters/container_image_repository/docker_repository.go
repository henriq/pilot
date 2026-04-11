package container_image_repository

import (
	"fmt"
	"path/filepath"
	"strings"

	"pilot/internal/core"
	"pilot/internal/core/domain"
	"pilot/internal/ports"
)

var _ ports.ContainerImageRepository = (*DockerRepository)(nil)

type DockerRepository struct {
	configRepository  ports.ConfigRepository
	secretsRepository ports.SecretsRepository
	templater         ports.Templater
	commandRunner     ports.CommandRunner
}

func NewDockerRepository(
	configRepository ports.ConfigRepository,
	secretsRepository ports.SecretsRepository,
	templater ports.Templater,
	commandRunner ports.CommandRunner,
) *DockerRepository {
	return &DockerRepository{
		configRepository:  configRepository,
		secretsRepository: secretsRepository,
		templater:         templater,
		commandRunner:     commandRunner,
	}
}

func (d *DockerRepository) BuildImage(image domain.DockerImage) error {
	contextPath := filepath.Join(image.Path, image.BuildContextRelativePath)

	// Determine dockerfile path: use stdin ("-") for override, or file path
	var dockerfilePath string
	var dockerfileContent string
	if image.DockerfileOverride != "" {
		dockerfilePath = "-"
		dockerfileContent = image.DockerfileOverride
	} else {
		dockerfilePath = filepath.Join(image.Path, image.DockerfilePath)
	}

	// Execute docker build command
	args := []string{"build", "-t", image.Name, "-f", dockerfilePath}

	templateValues, err := core.CreateTemplatingValues(d.configRepository, d.secretsRepository)
	if err != nil {
		return err
	}

	for i, arg := range image.BuildArgs {
		renderedArg, err := d.templater.Render(arg, fmt.Sprintf("build-args.%d", i), templateValues)
		if err != nil {
			return err
		}
		args = append(args, renderedArg)
	}

	// Add context path as the last argument
	args = append(args, contextPath)

	var output []byte
	if dockerfileContent != "" {
		// If using dockerfile override, pipe the content via stdin
		output, err = d.commandRunner.RunWithStdin(strings.NewReader(dockerfileContent), "docker", args...)
	} else {
		output, err = d.commandRunner.Run("docker", args...)
	}

	if err != nil {
		return fmt.Errorf("failed to build image: %v\n%s", err, string(output))
	}

	return nil
}

// PullImage pulls a Docker image from a registry
func (d *DockerRepository) PullImage(imageName string) error {
	output, err := d.commandRunner.Run("docker", "pull", imageName)
	if err != nil {
		return fmt.Errorf("failed to pull image: %v\n%s", err, string(output))
	}

	return nil
}
