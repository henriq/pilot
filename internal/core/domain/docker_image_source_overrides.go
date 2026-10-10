package domain

// DockerImageSourceOverrides maps a Docker image name to a local directory that
// should be used as the build source in place of cloning the image's configured
// git repository.
type DockerImageSourceOverrides struct {
	overrides nameKeyedOverrides
}

func NewDockerImageSourceOverrides(sourcePathByImage map[string]string) DockerImageSourceOverrides {
	return DockerImageSourceOverrides{overrides: newNameKeyedOverrides(sourcePathByImage, "image")}
}

func (o DockerImageSourceOverrides) IsEmpty() bool {
	return o.overrides.isEmpty()
}

func (o DockerImageSourceOverrides) LookupSourcePath(imageName string) (string, bool) {
	return o.overrides.lookup(imageName)
}

// FindUnusedOverrides returns the image names with overrides that are not
// present in the given list, sorted lexically.
func (o DockerImageSourceOverrides) FindUnusedOverrides(imagesInScope []DockerImage) []string {
	return o.overrides.findUnused(dockerImageNames(imagesInScope))
}

// ValidateAgainstImages returns an error if any override targets an image name
// not present in the given list.
func (o DockerImageSourceOverrides) ValidateAgainstImages(images []DockerImage) error {
	return o.overrides.validateAgainst(dockerImageNames(images))
}
