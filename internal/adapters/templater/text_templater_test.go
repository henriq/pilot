package templater

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTextTemplater_Render_SuccessfulTemplate(t *testing.T) {
	sut := TextTemplater{}

	result, err := sut.Render("Hello {{.Name}}", "test", map[string]interface{}{"Name": "world"})

	require.NoError(t, err)
	assert.Equal(t, "Hello world", result)
}

func TestTextTemplater_Render_MissingKeyReturnsResultWithoutKey(t *testing.T) {
	sut := TextTemplater{}

	result, err := sut.Render("Hello {{.Name}} from {{.City}}", "test", map[string]interface{}{"Name": "world"})

	require.NoError(t, err)
	assert.Equal(t, "Hello world from <no value>", result)
}

func TestTextTemplater_Render_InvalidTemplateSyntax(t *testing.T) {
	sut := TextTemplater{}

	_, err := sut.Render("Hello {{.Name", "test", map[string]interface{}{})

	assert.Error(t, err)
}
