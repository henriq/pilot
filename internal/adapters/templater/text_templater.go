package templater

import (
	"dx/internal/ports"
	"fmt"
	"os"
	"strings"
	"text/template"
)

var _ ports.Templater = (*TextTemplater)(nil)

type TextTemplater struct{}

func ProvideTextTemplater() *TextTemplater {
	return &TextTemplater{}
}

func (t TextTemplater) Render(templateText string, templateName string, values map[string]interface{}) (string, error) {
	tmpl, err := template.New(templateName).Option("missingkey=error").Parse(templateText)
	if err != nil {
		return "", err
	}
	var result strings.Builder
	err = tmpl.Execute(&result, values)
	if err != nil {
		originalErr := err
		// Retry with missingkey=zero
		tmpl, err = template.New(templateName).Parse(templateText)
		if err != nil {
			return "", err
		}
		var resultWithMissingKeys strings.Builder
		err = tmpl.Execute(&resultWithMissingKeys, values)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(os.Stderr, "WARN: %v\n", originalErr)
		return resultWithMissingKeys.String(), nil
	}

	return result.String(), nil
}
