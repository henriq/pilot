package core

import (
	"regexp"
	"sort"
	"strings"

	"dx/internal/core/domain"
)

// templateVarRegex matches Go template variable references like {{.Secrets.KEY}} or {{- .Services.foo.path -}}
// It also handles quoted keys like {{.Services."my-service".path}} and pipelines like {{ .Secrets.KEY | quote }}
// The regex only matches up to the variable reference, not the entire {{ }} block
// Uses alternation to handle quoted strings (which may contain hyphens) and regular identifiers
var templateVarRegex = regexp.MustCompile(`\{\{-?\s*\.(\w+)\.((?:"[^"]*"|[\w.])+)`)

// indexFuncRegex matches Go template index function syntax like {{ (index .Services "my-service").path }}
// This is used when keys contain special characters like hyphens
// Supports double quotes, single quotes, and backticks
var indexFuncRegex = regexp.MustCompile(`\{\{-?\s*\(index\s+\.(\w+)\s+["'` + "`" + `]([^"'` + "`" + `]+)["'` + "`" + `]\)`)

// ExtractTemplateVariables extracts variable references (e.g., .Secrets.KEY) from template text.
// Returns map of variable type -> list of keys (e.g., "Secrets" -> ["DB_PASSWORD", "API_KEY"])
//
// For Secrets: the entire path is the key (e.g., "foo.bar" from {{.Secrets.foo.bar}})
// For Services: only the first part is the name (e.g., "api" from {{.Services.api.path}})
func ExtractTemplateVariables(template string) map[string][]string {
	result := make(map[string][]string)
	seen := make(map[string]map[string]bool)

	addKey := func(varType, key string) {
		if seen[varType] == nil {
			seen[varType] = make(map[string]bool)
		}
		if !seen[varType][key] {
			seen[varType][key] = true
			result[varType] = append(result[varType], key)
		}
	}

	// Match dot notation: {{.Services.api.path}} or {{.Secrets.KEY}}
	matches := templateVarRegex.FindAllStringSubmatch(template, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			varType := match[1] // "Secrets" or "Services"
			fullPath := match[2]

			var key string
			if varType == "Secrets" {
				// For Secrets, the entire path is the key (supports nested keys like "foo.bar")
				key = fullPath
			} else {
				// For Services (and other types), extract only the first part (the identifier)
				// e.g., "api" from "api.path" or "my-service" from "\"my-service\".path"
				keyPart := strings.Split(fullPath, ".")[0]
				key = strings.Trim(keyPart, "\"")
			}

			addKey(varType, key)
		}
	}

	// Match index function: {{ (index .Services "my-service").path }} or {{ (index .Secrets "key") }}
	indexMatches := indexFuncRegex.FindAllStringSubmatch(template, -1)
	for _, match := range indexMatches {
		if len(match) >= 3 {
			varType := match[1] // "Secrets" or "Services"
			key := match[2]     // The quoted key
			addKey(varType, key)
		}
	}

	// Sort keys for deterministic output
	for _, keys := range result {
		sort.Strings(keys)
	}

	return result
}

// ExtractSecretKeys returns all unique secret keys referenced in a ConfigurationContext.
// It scans Scripts, Service.HelmArgs, and DockerImage.BuildArgs for template references.
func ExtractSecretKeys(ctx *domain.ConfigurationContext) []string {
	if ctx == nil {
		return nil
	}
	seen := make(map[string]bool)
	var keys []string

	collect := func(templates ...string) {
		for _, tmpl := range templates {
			vars := ExtractTemplateVariables(tmpl)
			for _, key := range vars["Secrets"] {
				if !seen[key] {
					seen[key] = true
					keys = append(keys, key)
				}
			}
		}
	}

	// Scripts
	for _, script := range ctx.Scripts {
		collect(script)
	}

	// Services
	for _, svc := range ctx.Services {
		collect(svc.HelmArgs...)
		for _, img := range svc.DockerImages {
			collect(img.BuildArgs...)
		}
	}

	sort.Strings(keys)
	return keys
}

// ExtractServiceReferences returns all unique service names referenced in a template string.
// This is used to find service dependencies in scripts.
// Results are sorted alphabetically for deterministic output.
func ExtractServiceReferences(template string) []string {
	vars := ExtractTemplateVariables(template)
	services := vars["Services"]
	sort.Strings(services)
	return services
}
