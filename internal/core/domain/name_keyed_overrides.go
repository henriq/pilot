package domain

import (
	"fmt"
	"maps"
	"sort"
	"strings"
)

type nameKeyedOverrides struct {
	valueByName map[string]string
	entityNoun  string
}

func newNameKeyedOverrides(valueByName map[string]string, entityNoun string) nameKeyedOverrides {
	return nameKeyedOverrides{valueByName: maps.Clone(valueByName), entityNoun: entityNoun}
}

func (o nameKeyedOverrides) isEmpty() bool {
	return len(o.valueByName) == 0
}

func (o nameKeyedOverrides) lookup(name string) (string, bool) {
	value, ok := o.valueByName[name]
	return value, ok
}

func (o nameKeyedOverrides) findUnused(namesInScope []string) []string {
	if o.isEmpty() {
		return nil
	}
	return sortedNamesMissingFrom(o.valueByName, namesInScope)
}

func (o nameKeyedOverrides) validateAgainst(knownNames []string) error {
	if o.isEmpty() {
		return nil
	}
	unknown := sortedNamesMissingFrom(o.valueByName, knownNames)
	if len(unknown) == 0 {
		return nil
	}
	return fmt.Errorf(
		"%s(s) not found: %s; available %ss:\n%s",
		o.entityNoun,
		strings.Join(quoteAll(unknown), ", "),
		o.entityNoun,
		bulletedNames(knownNames),
	)
}

func sortedNamesMissingFrom(valueByName map[string]string, names []string) []string {
	present := make(map[string]struct{}, len(names))
	for _, name := range names {
		present[name] = struct{}{}
	}
	var missing []string
	for name := range valueByName {
		if _, ok := present[name]; !ok {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

func quoteAll(values []string) []string {
	quoted := make([]string, len(values))
	for index, value := range values {
		quoted[index] = fmt.Sprintf("%q", value)
	}
	return quoted
}

func bulletedNames(names []string) string {
	sorted := make([]string, len(names))
	copy(sorted, names)
	sort.Strings(sorted)
	for index, name := range sorted {
		sorted[index] = "  - " + name
	}
	return strings.Join(sorted, "\n")
}

func serviceNames(services []Service) []string {
	names := make([]string, 0, len(services))
	for _, service := range services {
		names = append(names, service.Name)
	}
	return names
}

func dockerImageNames(images []DockerImage) []string {
	names := make([]string, 0, len(images))
	for _, image := range images {
		names = append(names, image.Name)
	}
	return names
}
