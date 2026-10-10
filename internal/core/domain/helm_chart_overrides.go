package domain

// HelmChartOverrides maps a service name to a local chart directory that
// should be rendered in place of the service's configured Helm chart.
type HelmChartOverrides struct {
	overrides nameKeyedOverrides
}

func NewHelmChartOverrides(chartDirectoryByService map[string]string) HelmChartOverrides {
	return HelmChartOverrides{overrides: newNameKeyedOverrides(chartDirectoryByService, "service")}
}

func (o HelmChartOverrides) IsEmpty() bool {
	return o.overrides.isEmpty()
}

func (o HelmChartOverrides) LookupChartDirectory(serviceName string) (string, bool) {
	return o.overrides.lookup(serviceName)
}

// FindUnusedOverrides returns the service names with overrides that are not
// present in the given list, sorted lexically.
func (o HelmChartOverrides) FindUnusedOverrides(servicesInScope []Service) []string {
	return o.overrides.findUnused(serviceNames(servicesInScope))
}

// ValidateAgainstServices returns an error if any override targets a service
// name not present in the given list.
func (o HelmChartOverrides) ValidateAgainstServices(services []Service) error {
	return o.overrides.validateAgainst(serviceNames(services))
}
