package domain

import (
	"fmt"
	"strings"
)

// ServiceGitRefOverrides maps a service name to a git ref that should replace
// the service's configured gitRef when cloning its sources.
type ServiceGitRefOverrides struct {
	overrides nameKeyedOverrides
}

// NewServiceGitRefOverrides rejects any ref failing ValidateGitRefShape, so an
// unvalidated ref cannot reach the git command line whichever caller builds it.
func NewServiceGitRefOverrides(gitRefByService map[string]string) (ServiceGitRefOverrides, error) {
	for serviceName, gitRef := range gitRefByService {
		if err := ValidateGitRefShape(gitRef); err != nil {
			return ServiceGitRefOverrides{}, fmt.Errorf("invalid git ref for service %q: %w", serviceName, err)
		}
	}
	return ServiceGitRefOverrides{overrides: newNameKeyedOverrides(gitRefByService, "service")}, nil
}

// ValidateGitRefShape rejects the security-relevant parts of git's refname
// grammar: a leading "-", "..", "@{", ".lock", and whitespace, control, glob,
// and refspec metacharacters. It accepts a superset of what git accepts, so a
// pass is not proof that git will take the ref.
func ValidateGitRefShape(gitRef string) error {
	if gitRef == "" {
		return fmt.Errorf("git ref cannot be empty")
	}
	if strings.HasPrefix(gitRef, "-") {
		return fmt.Errorf("git ref cannot start with '-'")
	}
	if strings.HasPrefix(gitRef, "/") || strings.HasSuffix(gitRef, "/") {
		return fmt.Errorf("git ref cannot start or end with '/'")
	}
	if strings.HasSuffix(gitRef, ".lock") {
		return fmt.Errorf("git ref cannot end with '.lock'")
	}
	if strings.Contains(gitRef, "..") {
		return fmt.Errorf("git ref cannot contain '..'")
	}
	if strings.Contains(gitRef, "@{") {
		return fmt.Errorf("git ref cannot contain '@{'")
	}
	if index := strings.IndexAny(gitRef, " \t~^:?*[\\"); index >= 0 {
		return fmt.Errorf("git ref contains a disallowed character %q at byte %d", gitRef[index], index)
	}
	if index := strings.IndexFunc(gitRef, func(r rune) bool {
		return r < 0x20 || r == 0x7f
	}); index >= 0 {
		return fmt.Errorf("git ref contains a control character at byte %d", index)
	}
	return nil
}

func (o ServiceGitRefOverrides) IsEmpty() bool {
	return o.overrides.isEmpty()
}

func (o ServiceGitRefOverrides) LookupGitRef(serviceName string) (string, bool) {
	return o.overrides.lookup(serviceName)
}

// FindUnusedOverrides returns the service names with overrides that are not
// present in the given list, sorted lexically.
func (o ServiceGitRefOverrides) FindUnusedOverrides(servicesInScope []Service) []string {
	return o.overrides.findUnused(serviceNames(servicesInScope))
}

// ValidateAgainstServices returns an error if any override targets a service
// name not present in the given list.
func (o ServiceGitRefOverrides) ValidateAgainstServices(services []Service) error {
	return o.overrides.validateAgainst(serviceNames(services))
}
